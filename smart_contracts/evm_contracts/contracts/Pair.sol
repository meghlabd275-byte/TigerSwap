// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "../libraries/SafeMath.sol";
import "../interfaces/ITigerSwapPair.sol";
import "../interfaces/ITigerSwapFactory.sol";

/**
 * @title TigerSwapPair
 * @notice AMM Pair contract - the core of TigerSwap DEX
 * @dev Implements Uniswap V2 style AMM with constant product formula
 */
contract TigerSwapPair {
    using SafeMath for uint256;

    // Unique identifier for this pair
    bytes32 public constant IDENTIFIER = keccak256("TigerSwapPair");

    // Store reserves in slot (packed for gas optimization)
    uint112 private reserve0;           // uses single storage slot
    uint112 private reserve1;          // uses single storage slot
    uint32 private blockTimestampLast; // uses single storage slot

    // Cumulative prices for TWAP oracle
    uint256 public price0CumulativeLast;
    uint256 public price1CumulativeLast;

    // Lock for reentrancy protection
    bool public isLocked;

    // Factory reference
    address public factory;

    // Token addresses (ordered)
    address public token0;
    address public token1;

    // Liquidity token tracking
    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;

    // Constant product k
    uint256 private kLast;

    // Events
    event Mint(address indexed sender, uint256 amount0, uint256 amount1);
    event Burn(address indexed sender, uint256 amount0, uint256 amount1, address indexed to);
    event Swap(
        address indexed sender,
        uint256 amount0Out,
        uint256 amount1Out,
        uint256 amount1In,
        address indexed to
    );
    event Sync(uint112 reserve0, uint112 reserve1);
    event Initialized(address indexed token0, address indexed token1);

    modifier onlyWhileUnlocked() {
        require(!isLocked, "TigerSwap: LOCKED");
        _;
    }

    constructor() {
        factory = msg.sender;
    }

    /**
     * @notice Initialize the pair - called once by factory
     */
    function initialize(address _token0, address _token1) external {
        require(msg.sender == factory, "TigerSwap: FORBIDDEN");
        require(token0 == address(0), "TigerSwap: ALREADY_INITIALIZED");
        
        token0 = _token0;
        token1 = _token1;
        
        emit Initialized(_token0, _token1);
    }

    /**
     * @notice Get current reserves
     */
    function getReserves() public view returns (uint112 _reserve0, uint112 _reserve1, uint32 _blockTimestampLast) {
        _reserve0 = reserve0;
        _reserve1 = reserve1;
        _blockTimestampLast = blockTimestampLast;
    }

    /**
     * @notice Update reserves and cumulative prices
     */
    function _update(uint256 balance0, uint256 balance1, uint112 _reserve0, uint112 _reserve1) private {
        require(balance0 <= type(uint112).max && balance1 <= type(uint112).max, "TigerSwap: OVERFLOW");

        uint32 blockTimestamp = uint32(block.timestamp % 2**32);
        uint32 timeElapsed = blockTimestamp - blockTimestampLast;

        if (timeElapsed > 0 && _reserve0 != 0 && _reserve1 != 0) {
            // Prevent overflow in cumulative price calculation
            price0CumulativeLast += uint256(_reserve1) * timeElapsed / _reserve0;
            price1CumulativeLast += uint256(_reserve0) * timeElapsed / _reserve1;
        }

        reserve0 = uint112(balance0);
        reserve1 = uint112(balance1);
        blockTimestampLast = blockTimestamp;

        emit Sync(reserve0, reserve1);
    }

    /**
     * @notice Mint new liquidity tokens
     */
    function _mint(address to, uint256 amount) internal {
        totalSupply += amount;
        balanceOf[to] += amount;
    }

    /**
     * @notice Burn liquidity tokens
     */
    function _burn(address from, uint256 amount) internal {
        totalSupply -= amount;
        balanceOf[from] -= amount;
    }

    /**
     * @notice Safe transfer (reentrancy guard)
     */
    function _safeTransfer(address token, address to, uint256 value) private {
        (bool success, bytes memory data) = token.call(abi.encodeWithSignature("transfer(address,uint256)", to, value));
        require(success && (data.length == 0 || abi.decode(data, (bool))), "TigerSwap: TRANSFER_FAILED");
    }

    /**
     * @notice Update k last value for fee calculation
     */
    function _updateK() private {
        kLast = reserve0 * reserve1;
    }

    // ==================== PUBLIC FUNCTIONS ====================

    /**
     * @notice Mint liquidity - add liquidity to pool
     * @dev Called by Router after calculating amounts
     */
    function mint(address to) external onlyWhileUnlocked returns (uint256 liquidity) {
        (uint112 _reserve0, uint112 _reserve1,) = getReserves();
        uint256 balance0 = IERC20(token0).balanceOf(address(this));
        uint256 balance1 = IERC20(token1).balanceOf(address(this));
        
        uint256 amount0 = balance0 - _reserve0;
        uint256 amount1 = balance1 - _reserve1;

        bool feeOn = _mintFee(_reserve0, _reserve1);
        
        uint256 _totalSupply = totalSupply;
        if (_totalSupply == 0) {
            // First liquidity provision - use geometric mean
            liquidity = Math.sqrt(amount0 * amount1) - 1000; // 1000 for initial LP tokens
            _mint(address(0), 1000); // Pre-mint for protocol
        } else {
            // Proportional to existing liquidity
            liquidity = Math.min(
                amount0 * _totalSupply / _reserve0,
                amount1 * _totalSupply / _reserve1
            );
        }

        require(liquidity > 0, "TigerSwap: INSUFFICIENT_LIQUIDITY_MINTED");
        _mint(to, liquidity);

        _update(balance0, balance1, _reserve0, _reserve1);
        if (feeOn) _updateK();
    }

    /**
     * @notice Burn liquidity - remove liquidity from pool
     */
    function burn(address to) external onlyWhileUnlocked returns (uint256 amount0, uint256 amount1) {
        (uint112 _reserve0, uint112 _reserve1,) = getReserves();
        address _token0 = token0;
        address _token1 = token1;
        
        uint256 liquidity = balanceOf[address(this)];
        bool feeOn = _mintFee(_reserve0, _reserve1);

        uint256 _totalSupply = totalSupply;
        amount0 = liquidity * _reserve0 / _totalSupply;
        amount1 = liquidity * _reserve1 / _totalSupply;
        
        require(amount0 > 0 && amount1 > 0, "TigerSwap: INSUFFICIENT_LIQUIDITY_BURNED");
        
        _burn(address(this), liquidity);
        _safeTransfer(_token0, to, amount0);
        _safeTransfer(_token1, to, amount1);

        uint256 balance0 = IERC20(_token0).balanceOf(address(this));
        uint256 balance1 = IERC20(_token1).balanceOf(address(this));

        _update(balance0, balance1, _reserve0, _reserve1);
        if (feeOn) _updateK();

        emit Burn(msg.sender, amount0, amount1, to);
    }

    /**
     * @notice Swap tokens - execute a trade
     */
    function swap(uint256 amount0Out, uint256 amount1Out, address to) external onlyWhileUnlocked {
        require(amount0Out > 0 || amount1Out > 0, "TigerSwap: INSUFFICIENT_OUTPUT_AMOUNT");
        require(amount0Out < reserve0 && amount1Out < reserve1, "TigerSwap: INSUFFICIENT_LIQUIDITY");

        // Reentrancy protection
        isLocked = true;

        // Transfer tokens out
        if (amount0Out > 0) _safeTransfer(token0, to, amount0Out);
        if (amount1Out > 0) _safeTransfer(token1, to, amount1Out);

        uint256 balance0 = IERC20(token0).balanceOf(address(this));
        uint256 balance1 = IERC20(token1).balanceOf(address(this));

        // Ensure we received enough tokens to cover the swap
        uint256 amount0In = balance0 > reserve0 - amount0Out ? balance0 - (reserve0 - amount0Out) : 0;
        uint256 amount1In = balance1 > reserve1 - amount1Out ? balance1 - (reserve1 - amount1Out) : 0;

        require(amount0In > 0 || amount1In > 0, "TigerSwap: INSUFFICIENT_INPUT_AMOUNT");
        
        // Update reserves and k
        _update(balance0, balance1, reserve0, reserve1);
        _updateK();

        isLocked = false;

        emit Swap(msg.sender, amount0Out, amount1Out, amount1In, to);
    }

    /**
     * @notice Force balance updates (for emergency清算)
     */
    function skim(address to) external onlyWhileUnlocked {
        uint256 balance0 = IERC20(token0).balanceOf(address(this));
        uint256 balance1 = IERC20(token1).balanceOf(address(this));
        
        _safeTransfer(token0, to, balance0 - reserve0);
        _safeTransfer(token1, to, balance1 - reserve1);
    }

    /**
     * @notice Sync reserves to actual balances
     */
    function sync() external onlyWhileUnlocked {
        _update(
            IERC20(token0).balanceOf(address(this)),
            IERC20(token1).balanceOf(address(this)),
            reserve0,
            reserve1
        );
    }

    /**
     * @notice Calculate protocol fee
     */
    function _mintFee(uint112 _reserve0, uint112 _reserve1) private returns (bool feeOn) {
        feeOn = ITigerSwapFactory(factory).feeTo() != address(0);
        
        if (feeOn) {
            if (kLast != 0) {
                uint256 rootK = Math.sqrt(uint256(_reserve0) * _reserve1);
                uint256 rootKLast = Math.sqrt(kLast);
                
                if (rootK > rootKLast) {
                    uint256 numerator = totalSupply * (rootK - rootKLast);
                    uint256 denominator = rootK * 50 + rootKLast; // 50 = protocol fee BPS
                    uint256 liquidity = numerator / denominator;
                    
                    if (liquidity > 0) {
                        _mint(ITigerSwapFactory(factory).feeTo(), liquidity);
                    }
                }
            }
        } else {
            if (kLast != 0) kLast = 0;
        }
    }
}

// Minimal ERC20 interface
interface IERC20 {
    function balanceOf(address) external view returns (uint256);
    function transfer(address, uint256) external returns (bool);
}

// Math library
library Math {
    function min(uint256 a, uint256 b) internal pure returns (uint256) {
        return a < b ? a : b;
    }

    function sqrt(uint256 y) internal pure returns (uint256 z) {
        if (y > 3) {
            z = y;
            uint256 x = y / 2 + 1;
            while (x < z) {
                z = x;
                x = (y / x + x) / 2;
            }
        } else if (y != 0) {
            z = 1;
        }
    }
}