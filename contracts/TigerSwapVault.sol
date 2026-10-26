// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";

/**
 * @title TigerSwapVault
 * @notice Single-asset auto-compounding vault.
 *
 * Depositors receive shares priced against the vault's total assets. Rewards
 * are harvested by the keeper, swapped into the underlying through an
 * operator-supplied path, and then folded back into total assets without
 * minting new shares. Because the share price is totalAssets / totalSupply,
 * that raises the value of every existing share, which is the compounding.
 *
 * Shares are always rounded down on deposit and up on withdrawal, so a user can
 * never extract more than they contributed plus their share of the gains.
 */
contract TigerSwapVault {
    IERC20 public immutable asset;
    address public immutable router;
    address public immutable factory;

    string public name;
    string public symbol;
    uint8 public constant decimals = 18;

    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    /// @notice Callers allowed to call harvest().
    mapping(address => bool) public isKeeper;
    address public owner;
    address public pendingOwner;

    /// @notice Minimum shares ever minted on the first deposit. They are minted
    /// to address(0) so the vault can never be reset to a zero share price.
    uint256 public constant MINIMUM_SHARES = 1e3;

    event Deposit(address indexed caller, address indexed owner, uint256 assets, uint256 shares);
    event Withdraw(address indexed caller, address indexed owner, uint256 assets, uint256 shares);
    event Harvested(address indexed keeper, address rewardToken, uint256 rewardAmount, uint256 assetsAdded);
    event KeeperUpdated(address indexed keeper, bool allowed);
    event Approval(address indexed owner, address indexed spender, uint256 value);
    event Transfer(address indexed from, address indexed to, uint256 value);
    event OwnershipTransferStarted(address indexed previousOwner, address indexed newOwner);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    modifier onlyKeeper() {
        require(isKeeper[msg.sender] || msg.sender == owner, "not keeper");
        _;
    }

    uint256 private _status = 1;

    modifier nonReentrant() {
        require(_status == 1, "reentrant call");
        _status = 2;
        _;
        _status = 1;
    }

    constructor(address _asset, address _router, address _factory, string memory _name, string memory _symbol) {
        require(_asset != address(0) && _router != address(0) && _factory != address(0), "zero address");
        asset = IERC20(_asset);
        router = _router;
        factory = _factory;
        name = _name;
        symbol = _symbol;
        owner = msg.sender;
        emit OwnershipTransferred(address(0), msg.sender);
    }

    // ---- Accounting --------------------------------------------------------

    /// @notice Total underlying assets the vault controls.
    function totalAssets() public view returns (uint256) {
        return asset.balanceOf(address(this));
    }

    /// @notice Shares minted for a given deposit.
    function previewDeposit(uint256 _assets) public view returns (uint256) {
        uint256 supply = totalSupply;
        if (supply == 0) {
            return _assets > MINIMUM_SHARES ? _assets - MINIMUM_SHARES : 0;
        }
        return (_assets * supply) / totalAssets();
    }

    /// @notice Shares burned to withdraw a given amount.
    function previewWithdraw(uint256 _assets) public view returns (uint256) {
        uint256 supply = totalSupply;
        if (supply == 0) {
            return 0;
        }
        uint256 assetsAvailable = totalAssets();
        require(_assets <= assetsAvailable, "insufficient assets");
        // Round up so the vault never gives out assets for too few shares.
        return (_assets * supply + assetsAvailable - 1) / assetsAvailable;
    }

    /// @notice Assets returned for a given number of shares.
    function previewRedeem(uint256 _shares) public view returns (uint256) {
        uint256 supply = totalSupply;
        if (supply == 0) {
            return 0;
        }
        return (_shares * totalAssets()) / supply;
    }

    // ---- Deposits ----------------------------------------------------------

    function deposit(uint256 _assets, address _receiver) external nonReentrant returns (uint256 shares) {
        require(_assets > 0, "amount required");
        require(_receiver != address(0), "zero receiver");

        shares = previewDeposit(_assets);
        require(shares > 0, "zero shares");

        if (totalSupply == 0) {
            totalSupply = MINIMUM_SHARES;
            emit Transfer(address(0), address(0), MINIMUM_SHARES);
        }

        require(asset.transferFrom(msg.sender, address(this), _assets), "transfer in failed");

        totalSupply += shares;
        balanceOf[_receiver] += shares;

        emit Transfer(address(0), _receiver, shares);
        emit Deposit(msg.sender, _receiver, _assets, shares);
    }

    // ---- Withdrawals -------------------------------------------------------

    function withdraw(uint256 _assets, address _receiver, address _owner_) external nonReentrant returns (uint256 shares) {
        require(_receiver != address(0), "zero receiver");
        shares = previewWithdraw(_assets);
        require(shares > 0, "zero shares");

        if (msg.sender != _owner_) {
            uint256 allowed = allowance[_owner_][msg.sender];
            require(allowed >= shares, "insufficient allowance");
            if (allowed != type(uint256).max) {
                allowance[_owner_][msg.sender] = allowed - shares;
            }
        }

        balanceOf[_owner_] -= shares;
        totalSupply -= shares;

        require(asset.transfer(_receiver, _assets), "transfer out failed");

        emit Transfer(_owner_, address(0), shares);
        emit Withdraw(msg.sender, _owner_, _assets, shares);
    }

    function redeem(uint256 _shares, address _receiver, address _owner_) external nonReentrant returns (uint256 assets) {
        require(_receiver != address(0), "zero receiver");
        require(_shares > 0, "amount required");

        if (msg.sender != _owner_) {
            uint256 allowed = allowance[_owner_][msg.sender];
            require(allowed >= _shares, "insufficient allowance");
            if (allowed != type(uint256).max) {
                allowance[_owner_][msg.sender] = allowed - _shares;
            }
        }

        assets = previewRedeem(_shares);
        require(assets > 0, "zero assets");

        balanceOf[_owner_] -= _shares;
        totalSupply -= _shares;

        require(asset.transfer(_receiver, assets), "transfer out failed");

        emit Transfer(_owner_, address(0), _shares);
        emit Withdraw(msg.sender, _owner_, assets, _shares);
    }

    // ---- Harvesting --------------------------------------------------------

    /// @notice Swap a reward token held by the vault into the underlying and
    /// compound it into total assets, raising the share price.
    /// @dev `_path` must start at `_rewardToken` and end at the vault asset.
    function harvest(address _rewardToken, uint256 _amount, address[] calldata _path, uint256 _minAssetsOut)
        external
        onlyKeeper
        nonReentrant
        returns (uint256 assetsAdded)
    {
        require(_rewardToken != address(asset), "reward is the asset");
        require(_amount > 0, "amount required");
        require(_path.length >= 2, "bad path");
        require(_path[0] == _rewardToken, "path start mismatch");
        require(_path[_path.length - 1] == address(asset), "path end mismatch");

        uint256 balanceBefore = totalAssets();

        IERC20(_rewardToken).approve(router, _amount);
        uint256[] memory amounts =
            ITigerSwapRouterHarvest(router).swapExactTokensForTokens(_amount, _minAssetsOut, _path, address(this), block.timestamp);

        assetsAdded = amounts[amounts.length - 1];
        require(assetsAdded > 0, "nothing harvested");
        // totalAssets() rose by exactly assetsAdded; no shares were minted, so
        // every existing share is now worth more.
        require(totalAssets() == balanceBefore + assetsAdded, "accounting mismatch");

        emit Harvested(msg.sender, _rewardToken, _amount, assetsAdded);
    }

    // ---- ERC20 shares ------------------------------------------------------

    function approve(address _spender, uint256 _amount) external returns (bool) {
        allowance[msg.sender][_spender] = _amount;
        emit Approval(msg.sender, _spender, _amount);
        return true;
    }

    function transfer(address _to, uint256 _amount) external returns (bool) {
        require(balanceOf[msg.sender] >= _amount, "insufficient balance");
        balanceOf[msg.sender] -= _amount;
        balanceOf[_to] += _amount;
        emit Transfer(msg.sender, _to, _amount);
        return true;
    }

    function transferFrom(address _from, address _to, uint256 _amount) external returns (bool) {
        require(balanceOf[_from] >= _amount, "insufficient balance");
        uint256 allowed = allowance[_from][msg.sender];
        require(allowed >= _amount, "insufficient allowance");
        if (allowed != type(uint256).max) {
            allowance[_from][msg.sender] = allowed - _amount;
        }
        balanceOf[_from] -= _amount;
        balanceOf[_to] += _amount;
        emit Transfer(_from, _to, _amount);
        return true;
    }

    // ---- Admin -------------------------------------------------------------

    function setKeeper(address _keeper, bool _allowed) external onlyOwner {
        require(_keeper != address(0), "zero address");
        isKeeper[_keeper] = _allowed;
        emit KeeperUpdated(_keeper, _allowed);
    }

    /// @notice Rescue tokens other than the vault asset, which is user money.
    function sweep(address _token, address _to, uint256 _amount) external onlyOwner {
        require(_token != address(asset), "cannot sweep the asset");
        require(_to != address(0), "zero address");
        require(IERC20(_token).transfer(_to, _amount), "sweep failed");
    }

    function transferOwnership(address _newOwner) external onlyOwner {
        require(_newOwner != address(0), "zero address");
        pendingOwner = _newOwner;
        emit OwnershipTransferStarted(owner, _newOwner);
    }

    function acceptOwnership() external {
        require(msg.sender == pendingOwner, "not pending owner");
        address previous = owner;
        owner = pendingOwner;
        pendingOwner = address(0);
        emit OwnershipTransferred(previous, owner);
    }
}

/// @notice Router surface the vault needs for harvesting.
interface ITigerSwapRouterHarvest {
    function swapExactTokensForTokens(
        uint256 amountIn,
        uint256 amountOutMin,
        address[] calldata path,
        address to,
        uint256 deadline
    ) external returns (uint256[] memory amounts);
}
