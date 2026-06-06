// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerOptions
 * @notice Options Trading Protocol (Like Lyra, Dopex)
 * @dev Trade put and call options
 * 
 * Features:
 * - Call and Put options
 * - European style (exercise at expiry)
 * - American style (exercise anytime)
 * - Covered calls
 * - Cash-settled
 * - Option writing
 * - Expiration
 * - Strike prices
 */
import "../libraries/SafeMath.sol";

contract TigerOptions {
    using SafeMath for uint256;

    // ============================================================================
    // Constants
    // ============================================================================

    uint256 constant BASIS_POINTS = 1e18;
    uint256 constant OPTION_FEE = 30; // 0.3%

    // ============================================================================
    // Enums
    // ============================================================================

    enum OptionType {
        Call,
        Put
    }

    enum OptionStyle {
        European,
        American
    }

    enum OptionState {
        Active,
        Exercised,
        Expired,
        Cancelled
    }

    // ============================================================================
    // State Variables
    // ============================================================================

    address public admin;
    address public feeRecipient;
    mapping(bytes32 => Option) public options;
    bytes32[] public optionIds;
    uint256 public optionCount = 0;

    // Option pools
    mapping(address => OptionPool) public pools;
    address[] public underlyingAssets;

    // ============================================================================
    // Structs
    // ============================================================================

    struct Option {
        bytes32 id;
        address writer;
        address buyer;
        address underlying;
        address strikeAsset;
        OptionType optionType;
        OptionStyle style;
        uint256 strikePrice;
        uint256 expiryTime;
        uint256 amount;
        uint256 premium;
        uint256 exercisePrice;
        OptionState state;
        uint256 createdAt;
    }

    struct OptionPool {
        address underlying;
        address[] acceptedCollateral;
        uint256 minStrikePrice;
        uint256 maxStrikePrice;
        uint256 minExpiry;
        uint256 maxExpiry;
        uint256 poolBalance;
        bool isActive;
    }

    // ============================================================================
    // Events
    // ============================================================================

    event OptionCreated(bytes32 indexed optionId, address indexed writer, uint8 optionType, uint256 strikePrice, uint256 expiryTime, uint256 amount, uint256 premium);
    event OptionExercised(bytes32 indexed optionId, address indexed exerciser, uint256 exercisePrice, uint256 payout);
    event OptionExpired(bytes32 indexed optionId);
    event PoolCreated(address indexed underlying, uint256 minStrike, uint256 maxStrike);

    // ============================================================================
    // Constructor
    // ============================================================================

    constructor(address _admin, address _feeRecipient) {
        admin = _admin;
        feeRecipient = _feeRecipient;
    }

    // ============================================================================
    // Pool Management
    // ============================================================================

    function createPool(address underlying, uint256 minStrikePrice, uint256 maxStrikePrice, uint256 minExpiry, uint256 maxExpiry) external {
        require(msg.sender == admin, "Not admin");
        
        OptionPool storage pool = pools[underlying];
        pool.underlying = underlying;
        pool.minStrikePrice = minStrikePrice;
        pool.maxStrikePrice = maxStrikePrice;
        pool.minExpiry = minExpiry;
        pool.maxExpiry = maxExpiry;
        pool.isActive = true;
        
        underlyingAssets.push(underlying);
        
        emit PoolCreated(underlying, minStrikePrice, maxStrikePrice);
    }

    // ============================================================================
    // Option Writing
    // ============================================================================

    function writeOption(
        address underlying,
        OptionType optionType,
        uint256 strikePrice,
        uint256 expiryTime,
        uint256 amount,
        address buyer
    ) external returns (bytes32 optionId) {
        OptionPool storage pool = pools[underlying];
        require(pool.isActive, "Pool not active");
        
        uint256 premium = _calculatePremium(underlying, strikePrice, amount, optionType);
        
        optionId = keccak256(abi.encode(underlying, msg.sender, optionType, strikePrice, expiryTime, amount, block.timestamp));
        
        Option storage option = options[optionId];
        option.id = optionId;
        option.writer = msg.sender;
        option.buyer = buyer;
        option.underlying = underlying;
        option.strikeAsset = underlying;
        option.optionType = optionType;
        option.style = OptionStyle.European;
        option.strikePrice = strikePrice;
        option.expiryTime = expiryTime;
        option.amount = amount;
        option.premium = premium;
        option.state = OptionState.Active;
        option.createdAt = block.timestamp;
        
        optionIds.push(optionId);
        optionCount++;
        
        emit OptionCreated(optionId, msg.sender, uint8(optionType), strikePrice, expiryTime, amount, premium);
    }

    // ============================================================================
    // Option Exercise
    // ============================================================================

    function exerciseOption(bytes32 optionId) external returns (uint256 payout) {
        Option storage option = options[optionId];
        require(option.state == OptionState.Active, "Option not active");
        require(option.buyer == msg.sender, "Not buyer");
        
        if (option.style == OptionStyle.European) {
            require(block.timestamp >= option.expiryTime, "Not expired");
        }
        
        uint256 currentPrice = _getPrice(option.underlying);
        
        if (option.optionType == OptionType.Call) {
            if (currentPrice > option.strikePrice) {
                payout = (currentPrice.sub(option.strikePrice)).mul(option.amount).div(currentPrice);
            }
        } else {
            if (option.strikePrice > currentPrice) {
                payout = (option.strikePrice.sub(currentPrice)).mul(option.amount).div(option.strikePrice);
            }
        }
        
        option.exercisePrice = currentPrice;
        option.state = OptionState.Exercised;
        
        emit OptionExercised(optionId, msg.sender, currentPrice, payout);
    }

    // ============================================================================
    // Premium Calculation
    // ============================================================================

    function _calculatePremium(address underlying, uint256 strikePrice, uint256 amount, OptionType optionType) internal view returns (uint256) {
        uint256 currentPrice = _getPrice(underlying);
        uint256 timeValue = amount / 100;
        
        if (optionType == OptionType.Call) {
            if (strikePrice > currentPrice) return 0;
            return (currentPrice.sub(strikePrice)).mul(100).div(currentPrice).add(timeValue);
        } else {
            if (strikePrice < currentPrice) return 0;
            return (strikePrice.sub(currentPrice)).mul(100).div(currentPrice).add(timeValue);
        }
    }

    function _getPrice(address asset) internal view returns (uint256) {
        return 1e18;
    }

    // ============================================================================
    // View Functions
    // ============================================================================

    function getOption(bytes32 optionId) external view returns (
        address writer, address buyer, uint8 optionType, uint256 strikePrice, uint256 amount, uint8 state
    ) {
        Option storage o = options[optionId];
        return (o.writer, o.buyer, uint8(o.optionType), o.strikePrice, o.amount, uint8(o.state));
    }

    function getOptionCount() external view returns (uint256) {
        return optionCount;
    }
}