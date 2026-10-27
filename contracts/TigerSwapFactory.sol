// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";

/**
 * @title TigerSwapFactory
 * @notice Factory contract for creating and managing liquidity pools
 * @dev Implements Uniswap V2 style factory with additional features
 */
contract TigerSwapFactory {
    /// @notice Denominator for the protocol fee, expressed in basis points.
    uint256 public constant PROTOCOL_FEE_DENOMINATOR = 10000;
    /// @notice Protocol's cut of swap fees, in basis points. It is minted as
    ///         new LP tokens to `feeTo` when a pool is first touched, so it
    ///         comes out of LP growth rather than the reserves.
    uint256 public protocolFee = 2; // 0.02%
    /// @notice Recipient of the protocol's LP share. While unset the protocol
    ///         fee is not charged and LPs keep the whole swap fee.
    address public feeTo;

    /// @notice Fee for swapping
    uint256 public swapFee = 30; // 0.3%

    /// @notice Mapping from token pair to pool address
    mapping(address => mapping(address => address)) public getPool;
    
    /// @notice List of all pools
    address[] public allPools;
    
    /// @notice Mapping of pool to its existence
    mapping(address => bool) public isPool;

    /// @notice Registry of deployed pools per family (0 xyK, 1 concentrated,
    ///         2 stable, 3 weighted). Family pools (concentrated, stable,
    ///         weighted) are deployed by the operator and registered here so a
    ///         single factory can resolve every TigerSwap pool family.
    mapping(uint8 => mapping(address => mapping(address => address))) public getFamilyPool;

    /// @notice Owner of the factory
    address public owner;
    
    /// @notice Paused state
    bool public paused;

    /// @notice Token pair information
    struct PoolInfo {
        address token0;
        address token1;
        address pool;
        uint256 reserve0;
        uint256 reserve1;
        uint256 totalSupply;
    }

    /// @notice Events
    event PoolCreated(address indexed token0, address indexed token1, address pool, uint256);
    event FeeUpdated(uint256 swapFee, uint256 protocolFee);
    event FeeToUpdated(address indexed feeTo);
    event OwnershipTransferred(address indexed oldOwner, address indexed newOwner);
    event Paused(address account);
    event Unpaused(address account);

    /// @notice Modifier for only owner
    modifier onlyOwner() {
        require(msg.sender == owner, "TigerSwap: FORBIDDEN");
        _;
    }

    /// @notice Modifier for when not paused
    modifier whenNotPaused() {
        require(!paused, "TigerSwap: PAUSED");
        _;
    }

    constructor() {
        owner = msg.sender;
        emit OwnershipTransferred(address(0), msg.sender);
    }

    /**
     * @notice Create a new pool for token pair
     * @param tokenA Address of first token
     * @param tokenB Address of second token
     * @return pool Address of created pool
     */
    function createPool(address tokenA, address tokenB) external whenNotPaused returns (address pool) {
        require(tokenA != tokenB, "TigerSwap: IDENTICAL_ADDRESSES");
        require(tokenA != address(0), "TigerSwap: ZERO_ADDRESS");
        require(tokenB != address(0), "TigerSwap: ZERO_ADDRESS");

        (address token0, address token1) = tokenA < tokenB ? (tokenA, tokenB) : (tokenB, tokenA);
        require(getPool[token0][token1] == address(0), "TigerSwap: POOL_EXISTS");

        // Create pool using CREATE2
        bytes32 salt = keccak256(abi.encodePacked(token0, token1));
        bytes memory bytecode = type(TigerSwapPool).creationCode;
        bytes memory initCode = bytes.concat(bytecode, abi.encode(token0, token1));
        
        assembly {
            pool := create2(0, add(initCode, 32), mload(initCode), salt)
        }

        require(pool != address(0), "TigerSwap: DEPLOY_FAILED");

        getPool[token0][token1] = pool;
        getPool[token1][token0] = pool;
        allPools.push(pool);
        isPool[pool] = true;

        emit PoolCreated(token0, token1, pool, allPools.length);
    }

    /**
     * @notice Get pool address for token pair
     * @param tokenA First token address
     * @param tokenB Second token address
     * @return Pool address
     */
    function getPoolAddress(address tokenA, address tokenB) external view returns (address) {
        (address token0, address token1) = tokenA < tokenB ? (tokenA, tokenB) : (tokenB, tokenA);
        return getPool[token0][token1];
    }

    /**
     * @notice Register a family pool (concentrated, stable or weighted) that the
     *         operator has deployed, so it can be resolved by family.
     * @dev The xyK family is created through createPool and is already indexed.
     * @param tokenA First token address
     * @param tokenB Second token address
     * @param family Family id: 0 xyK, 1 concentrated, 2 stable, 3 weighted
     * @param pool Address of the deployed pool contract
     */
    function registerFamilyPool(address tokenA, address tokenB, uint8 family, address pool) external onlyOwner {
        require(tokenA != tokenB, "TigerSwap: IDENTICAL_ADDRESSES");
        require(tokenA != address(0) && tokenB != address(0), "TigerSwap: ZERO_ADDRESS");
        require(pool != address(0), "TigerSwap: ZERO_ADDRESS");

        (address token0, address token1) = tokenA < tokenB ? (tokenA, tokenB) : (tokenB, tokenA);

        if (family == 0) {
            getPool[token0][token1] = pool;
            getPool[token1][token0] = pool;
        } else {
            getFamilyPool[family][token0][token1] = pool;
            getFamilyPool[family][token1][token0] = pool;
        }
        if (!isPool[pool]) {
            allPools.push(pool);
            isPool[pool] = true;
        }

        emit PoolCreated(token0, token1, pool, allPools.length);
    }

    /**
     * @notice Resolve a pool by token pair and family.
     * @param tokenA First token address
     * @param tokenB Second token address
     * @param family Family id: 0 xyK, 1 concentrated, 2 stable, 3 weighted
     * @return pool Address of the pool, or the zero address if none is registered
     */
    function getPoolForFamily(address tokenA, address tokenB, uint8 family) external view returns (address pool) {
        (address token0, address token1) = tokenA < tokenB ? (tokenA, tokenB) : (tokenB, tokenA);
        if (family == 0) {
            return getPool[token0][token1];
        }
        return getFamilyPool[family][token0][token1];
    }

    /**
     * @notice Get number of pools
     * @return Number of pools
     */
    function allPoolsLength() external view returns (uint256) {
        return allPools.length;
    }

    /**
     * @notice Set swap fee
     * @param _swapFee New swap fee
     */
    function setSwapFee(uint256 _swapFee) external onlyOwner {
        require(_swapFee <= 1000, "TigerSwap: FEE_TOO_HIGH");
        swapFee = _swapFee;
        emit FeeUpdated(swapFee, protocolFee);
    }

    /**
     * @notice Set protocol fee
     * @param _protocolFee New protocol fee
     */
    function setProtocolFee(uint256 _protocolFee) external onlyOwner {
        require(_protocolFee <= 100, "TigerSwap: FEE_TOO_HIGH");
        protocolFee = _protocolFee;
        emit FeeUpdated(swapFee, protocolFee);
    }

    /**
     * @notice Set the recipient of the protocol's LP share. Setting it to the
     *         zero address stops the protocol fee from being charged.
     * @param _feeTo New recipient
     */
    function setFeeTo(address _feeTo) external onlyOwner {
        feeTo = _feeTo;
        emit FeeToUpdated(_feeTo);
    }

    /**
     * @notice Pause the factory
     */
    function pause() external onlyOwner {
        paused = true;
        emit Paused(msg.sender);
    }

    /**
     * @notice Unpause the factory
     */
    function unpause() external onlyOwner {
        paused = false;
        emit Unpaused(msg.sender);
    }

    /**
     * @notice Transfer ownership
     * @param newOwner New owner address
     */
    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "TigerSwap: ZERO_ADDRESS");
        emit OwnershipTransferred(owner, newOwner);
        owner = newOwner;
    }
}

/**
 * @title TigerSwapPool
 * @notice Liquidity pool contract
 */
contract TigerSwapPool {
    string public name = "TigerSwap Liquidity Pool";
    string public symbol = "TIG-LP";
    uint8 public constant decimals = 18;
    uint256 public totalSupply;

    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    address public token0;
    address public token1;
    uint256 public reserve0;
    uint256 public reserve1;
    uint32 public blockTimestampLast;

    uint256 public price0CumulativeLast;
    uint256 public price1CumulativeLast;

    /// @dev Reserves product at the last fee accrual; zero means none yet.
    uint256 public kLastStored;

    bytes32 public DOMAIN_SEPARATOR;
    bytes32 public constant PERMIT_TYPEHASH = keccak256("Permit(address owner,address spender,uint256 value,uint256 nonce,uint256 deadline)");

    mapping(address => uint256) public nonces;

    uint256 private unlocked = 1;

    /// @notice Permanently locked initial liquidity, as in Uniswap V2.
    uint256 public constant MINIMUM_LIQUIDITY = 1000;

    event Mint(address indexed sender, uint256 amount0, uint256 amount1);
    event Burn(address indexed sender, uint256 amount0, uint256 amount1, address indexed to);
    event Swap(address indexed sender, uint256 amount0In, uint256 amount1In, uint256 amount0Out, uint256 amount1Out, address indexed to);
    event Sync(uint256 reserve0, uint256 reserve1);
    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);

    modifier lock() {
        require(unlocked == 1, "TigerSwap: LOCKED");
        unlocked = 0;
        _;
        unlocked = 1;
    }

    /// @notice The factory that deployed this pool, used for the fee settings.
    address public factory;

    constructor(address _token0, address _token1) {
        token0 = _token0;
        token1 = _token1;
        factory = msg.sender;

        uint256 chainId;
        assembly {
            chainId := chainid()
        }
        DOMAIN_SEPARATOR = keccak256(
            abi.encode(
                keccak256("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"),
                keccak256(bytes(name)),
                keccak256(bytes("1")),
                chainId,
                address(this)
            )
        );
    }

    /**
     * @notice Update reserves
     */
    /**
     * @notice Current reserves and the timestamp they were last synced.
     */
    function getReserves() external view returns (uint112, uint112, uint32) {
        return (uint112(reserve0), uint112(reserve1), blockTimestampLast);
    }

    /**
     * @notice Update reserves
     */
    function _update(uint256 balance0, uint256 balance1) internal {
        require(balance0 <= type(uint112).max && balance1 <= type(uint112).max, "TigerSwap: OVERFLOW");
        reserve0 = uint112(balance0);
        reserve1 = uint112(balance1);

        uint32 blockTimestamp = uint32(block.timestamp % 2**32);
        uint32 timeElapsed = blockTimestamp - blockTimestampLast;
        if (timeElapsed > 0 && reserve0 > 0 && reserve1 > 0) {
            price0CumulativeLast += uint256(reserve1) * timeElapsed / reserve0;
            price1CumulativeLast += uint256(reserve0) * timeElapsed / reserve1;
        }
        blockTimestampLast = blockTimestamp;
        emit Sync(reserve0, reserve1);
    }

    /**
     * @notice Mint LP tokens
     * @dev Called by factory when liquidity is added
     */
    function mint(address to) external lock returns (uint256 liquidity) {
        _mintFee();

        uint256 balance0 = IERC20(token0).balanceOf(address(this));
        uint256 balance1 = IERC20(token1).balanceOf(address(this));
        uint256 amount0 = balance0 - reserve0;
        uint256 amount1 = balance1 - reserve1;

        uint256 _totalSupply = totalSupply;
        if (_totalSupply == 0) {
            liquidity = sqrt(amount0 * amount1);
            require(liquidity > MINIMUM_LIQUIDITY, "TigerSwap: INSUFFICIENT_LIQUIDITY");
            // Lock the smallest unit of LP forever so the pool can never be
            // emptied to a zero supply.
            _mint(address(0), MINIMUM_LIQUIDITY);
            liquidity -= MINIMUM_LIQUIDITY;
        } else {
            liquidity = min(amount0 * _totalSupply / reserve0, amount1 * _totalSupply / reserve1);
        }

        require(liquidity > 0, "TigerSwap: INSUFFICIENT_LIQUIDITY");
        _mint(to, liquidity);
        _update(balance0, balance1);

        // Liquidity changes are not swap fees, so the fee baseline restarts.
        kLastStored = reserve0 * reserve1;
    }

    /**
     * @notice Burn LP tokens and withdraw tokens
     * @param to Address to receive tokens
     * @return amount0 Amount of token0 received
     * @return amount1 Amount of token1 received
     */
    function burn(address to) external lock returns (uint256 amount0, uint256 amount1) {
        _mintFee();

        uint256 balance0 = IERC20(token0).balanceOf(address(this));
        uint256 balance1 = IERC20(token1).balanceOf(address(this));
        uint256 liquidity = balanceOf[address(this)];

        uint256 _totalSupply = totalSupply;
        amount0 = liquidity * balance0 / _totalSupply;
        amount1 = liquidity * balance1 / _totalSupply;

        require(amount0 > 0 && amount1 > 0, "TigerSwap: INSUFFICIENT_LIQUIDITY_BURNED");
        
        _burn(address(this), liquidity);
        _safeTransfer(token0, to, amount0);
        _safeTransfer(token1, to, amount1);

        balance0 = IERC20(token0).balanceOf(address(this));
        balance1 = IERC20(token1).balanceOf(address(this));
        _update(balance0, balance1);

        kLastStored = reserve0 * reserve1;

        emit Burn(msg.sender, amount0, amount1, to);
    }

    /**
     * @notice Swap tokens
     * @param amount0Out Amount of token0 out
     * @param amount1Out Amount of token1 out
     * @param to Address to receive output
     * @param data Additional data
     */
    function swap(uint256 amount0Out, uint256 amount1Out, address to, bytes calldata data) external lock {
        require(amount0Out > 0 || amount1Out > 0, "TigerSwap: INSUFFICIENT_OUTPUT_AMOUNT");
        require(amount0Out < reserve0 && amount1Out < reserve1, "TigerSwap: INSUFFICIENT_LIQUIDITY");

        uint256 balance0 = IERC20(token0).balanceOf(address(this)) - amount0Out;
        uint256 balance1 = IERC20(token1).balanceOf(address(this)) - amount1Out;

        require(balance0 * balance1 >= reserve0 * reserve1, "TigerSwap: K");

        uint256 amount0In = balance0 > reserve0 - amount0Out ? balance0 - (reserve0 - amount0Out) : 0;
        uint256 amount1In = balance1 > reserve1 - amount1Out ? balance1 - (reserve1 - amount1Out) : 0;

        require(amount0In > 0 || amount1In > 0, "TigerSwap: INSUFFICIENT_INPUT_AMOUNT");

        _update(balance0, balance1);

        if (amount0Out > 0) _safeTransfer(token0, to, amount0Out);
        if (amount1Out > 0) _safeTransfer(token1, to, amount1Out);

        emit Swap(msg.sender, amount0In, amount1In, amount0Out, amount1Out, to);
    }

    /**
     * @notice Force balances to match reserves
     */
    function skim(address to) external lock {
        _safeTransfer(token0, to, IERC20(token0).balanceOf(address(this)) - reserve0);
        _safeTransfer(token1, to, IERC20(token1).balanceOf(address(this)) - reserve1);
    }

    /**
     * @notice Force reserves to match balances
     */
    function sync() external lock {
        _update(IERC20(token0).balanceOf(address(this)), IERC20(token1).balanceOf(address(this)));
    }

    // --- ERC20 Functions ---

    function _mint(address to, uint256 value) internal {
        totalSupply += value;
        balanceOf[to] += value;
        emit Transfer(address(0), to, value);
    }

    function _burn(address from, uint256 value) internal {
        balanceOf[from] -= value;
        totalSupply -= value;
        emit Transfer(from, address(0), value);
    }

    function _transfer(address from, address to, uint256 value) internal {
        balanceOf[from] -= value;
        balanceOf[to] += value;
        emit Transfer(from, to, value);
    }

    function _approve(address owner, address spender, uint256 value) internal {
        allowance[owner][spender] = value;
        emit Approval(owner, spender, value);
    }

    function transfer(address to, uint256 value) external returns (bool) {
        _transfer(msg.sender, to, value);
        return true;
    }

    function transferFrom(address from, address to, uint256 value) external returns (bool) {
        if (allowance[from][msg.sender] != type(uint256).max) {
            allowance[from][msg.sender] -= value;
        }
        _transfer(from, to, value);
        return true;
    }

    function approve(address spender, uint256 value) external returns (bool) {
        _approve(msg.sender, spender, value);
        return true;
    }

    function permit(address owner, address spender, uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s) external {
        require(deadline >= block.timestamp, "TigerSwap: EXPIRED");
        bytes32 digest = keccak256(
            abi.encodePacked(
                "\x19\x01",
                DOMAIN_SEPARATOR,
                keccak256(abi.encode(PERMIT_TYPEHASH, owner, spender, value, nonces[owner]++, deadline))
            )
        );
        address recovered = ecrecover(digest, v, r, s);
        require(recovered != address(0) && recovered == owner, "TigerSwap: INVALID_SIGNATURE");
        _approve(owner, spender, value);
    }

    // --- Utility Functions ---

    function _safeTransfer(address token, address to, uint256 value) internal {
        (bool success, bytes memory data) = token.call(abi.encodeWithSelector(IERC20.transfer.selector, to, value));
        require(success && (data.length == 0 || abi.decode(data, (bool))), "TigerSwap: TRANSFER_FAILED");
    }

    function min(uint256 a, uint256 b) internal pure returns (uint256) {
        return a < b ? a : b;
    }

    /**
     * @notice Mint the protocol's share of the fees accrued since the last call.
     * @dev The share is proportional to the growth in sqrt(k). It is minted as
     *      new LP tokens to the factory's `feeTo`, so it dilutes LPs rather
     *      than draining the reserves.
     * @return liquidity LP tokens minted to the fee recipient
     */
    function mintFee() external lock returns (uint256 liquidity) {
        return _mintFee();
    }

    /// @dev Accrues the protocol's share and restarts the fee baseline. Called
    ///      at the start of mint and burn so fees are settled before the
    ///      reserves move for a reason other than swapping.
    function _mintFee() internal returns (uint256 liquidity) {
        address feeToRecipient = ITigerSwapFactory(factory).feeTo();
        uint256 kLast = kLastStored;

        if (feeToRecipient != address(0) && kLast != 0) {
            uint256 rootK = sqrt(reserve0 * reserve1);
            uint256 rootKLast = sqrt(kLast);

            if (rootK > rootKLast) {
                uint256 protocolFeeBps = ITigerSwapFactory(factory).protocolFee();
                if (protocolFeeBps > 0) {
                    uint256 numerator = totalSupply * (rootK - rootKLast);
                    uint256 denominator = rootK * (PROTOCOL_FEE_DENOMINATOR / protocolFeeBps) + rootK;
                    liquidity = numerator / denominator;
                    if (liquidity > 0) {
                        _mint(feeToRecipient, liquidity);
                    }
                }
            }
        }

        kLastStored = reserve0 * reserve1;
    }

    /// @dev Denominator used to split the protocol's share; must match the
    ///      factory constant so the two cannot drift apart.
    uint256 private constant PROTOCOL_FEE_DENOMINATOR = 10_000;

    function sqrt(uint256 x) internal pure returns (uint256 z) {
        if (x == 0) return 0;
        z = x;
        uint256 y = x / 2 + 1;
        while (y < z) {
            z = y;
            y = (x / y + y) / 2;
        }
    }
}
