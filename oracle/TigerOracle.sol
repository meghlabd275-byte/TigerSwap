// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";

/**
 * @title TigerOracle
 * @notice Multi-Source Price Oracle System
 * @dev Aggregates prices from multiple sources for robust price feeds
 */

contract TigerOracle is AccessControl, ReentrancyGuard {
    bytes32 public constant WARDEN_ROLE = keccak256("WARDEN_ROLE");
    bytes32 public constant PRICE_FEEDER_ROLE = keccak256("PRICE_FEEDER_ROLE");
    
    // Precision
    uint256 public constant PRECISION = 1e18;
    uint256 public constant TWAP_INTERVAL = 30 minutes;
    
    // Price data
    struct PriceData {
        uint256 price;
        uint256 timestamp;
        uint256 volatility;
    }
    
    // Pair price feed
    mapping(bytes32 => PriceData) public prices;
    
    // Historical prices for TWAP
    mapping(bytes32 => uint256[]) public priceHistory;
    mapping(bytes32 => uint256[]) public timestampHistory;
    
    // Aggregator config
    mapping(bytes32 => AggregatorConfig) public aggregatorConfigs;
    
    // Chainlink price feeds
    mapping(address => address) public chainlinkFeeds;
    
    // Events
    event PriceUpdated(bytes32 indexed pair, uint256 price, uint256 timestamp);
    event PriceFeederAdded(address indexed feeder);
    event PriceFeederRemoved(address indexed feeder);
    event AggregatorConfigured(bytes32 indexed pair, uint256 minSources, uint256 maxDeviationBps);
    event ChainlinkFeedSet(address indexed token, address indexed feed);
    
    struct AggregatorConfig {
        uint256 minSources;
        uint256 maxDeviationBps;
        bool enabled;
        address[] sources;
        uint256 lastUpdateTime;
    }
    
    modifier onlyPriceFeeders() {
        require(hasRole(PRICE_FEEDER_ROLE, msg.sender), "Not price feeder";
        _;
    }
    
    constructor(address _admin) {
        _grantRole(DEFAULT_ADMIN_ROLE, _admin);
        _grantRole(WARDEN_ROLE, _admin);
        _grantRole(PRICE_FEEDER_ROLE, _admin);
    }
    
    /**
     * @notice Get current price for a pair
     * @param pair Token pair (e.g., ETH-USDC)
     * @return Current price
     */
    function getPrice(bytes32 pair) external view returns (uint256) {
        return prices[pair].price;
    }
    
    /**
     * @notice Get price data
     * @param pair Token pair
     * @return price, timestamp, volatility
     */
    function getPriceData(bytes32 pair) external view returns (uint256, uint256, uint256) {
        PriceData memory data = prices[pair];
        return (data.price, data.timestamp, data.volatility);
    }
    
    /**
     * @notice Get TWAP price
     * @param pair Token pair
     * @param interval TWAP interval in seconds
     * @return TWAP price
     */
    function getTWAP(bytes32 pair, uint256 interval) external view returns (uint256) {
        uint256[] memory history = priceHistory[pair];
        uint256[] memory times = timestampHistory[pair];
        
        if (history.length == 0) {
            return 0;
        }
        
        uint256 startTime = block.timestamp - interval;
        uint256 sum;
        uint256 count;
        
        for (uint256 i = history.length; i > 0; i--) {
            if (times[i-1] >= startTime) {
                sum += history[i-1];
                count++;
            }
        }
        
        return count > 0 ? sum / count : 0;
    }
    
    /**
     * @notice Update price (feeder function)
     * @param pair Token pair
     * @param price New price
     * @param volatility Volatility metric
     */
    function updatePrice(bytes32 pair, uint256 price, uint256 volatility) 
        external 
        onlyPriceFeeders 
    {
        require(price > 0, "Invalid price");
        
        prices[pair] = PriceData({
            price: price,
            timestamp: block.timestamp,
            volatility: volatility
        });
        
        // Add to history
        priceHistory[pair].push(price);
        timestampHistory[pair].push(block.timestamp);
        
        // Keep only last 1000 prices
        if (priceHistory[pair].length > 1000) {
            priceHistory[pair][0] = priceHistory[pair][priceHistory[pair].length - 1000];
        }
        
        emit PriceUpdated(pair, price, block.timestamp);
    }
    
    /**
     * @notice Batch update prices
     * @param pairs Array of pairs
     * @param newPrices Array of prices
     */
    function batchUpdatePrice(bytes32[] calldata pairs, uint256[] calldata newPrices) 
        external 
        onlyPriceFeeders 
    {
        require(pairs.length == newPrices.length, "Length mismatch");
        
        for (uint256 i = 0; i < pairs.length; i++) {
            if (newPrices[i] > 0) {
                prices[pairs[i]] = PriceData({
                    price: newPrices[i],
                    timestamp: block.timestamp,
                    volatility: 0
                });
            }
        }
    }
    
    /**
     * @notice Configure price aggregator
     * @param pair Token pair
     * @param minSources Minimum number of sources
     * @param maxDeviationBps Maximum deviation in basis points
     */
    function configureAggregator(
        bytes32 pair, 
        uint256 minSources, 
        uint256 maxDeviationBps
    ) external onlyRole(DEFAULT_ADMIN_ROLE) {
        aggregatorConfigs[pair] = AggregatorConfig({
            minSources: minSources,
            maxDeviationBps: maxDeviationBps,
            enabled: true,
            sources: new address[](0),
            lastUpdateTime: block.timestamp
        });
        
        emit AggregatorConfigured(pair, minSources, maxDeviationBps);
    }
    
    /**
     * @notice Set Chainlink price feed
     * @param token Token address
     * @param feed Chainlink aggregator address
     */
    function setChainlinkFeed(address token, address feed) 
        external 
        onlyRole(DEFAULT_ADMIN_ROLE) 
    {
        require(token != address(0), "Invalid token");
        chainlinkFeeds[token] = feed;
        
        emit ChainlinkFeedSet(token, feed);
    }
    
    /**
     * @notice Get Chainlink price
     * @param token Token address
     * @return Chainlink price
     */
    function getChainlinkPrice(address token) external view returns (uint256) {
        address feed = chainlinkFeeds[token];
        if (feed == address(0)) {
            return 0;
        }
        
        // Call Chainlink feed
        (int256 answer, , , uint256 updatedAt, ) = IChainlinkAggregator(feed).latestRoundData();
        
        require(updatedAt >= block.timestamp - 1 hours, "Stale price");
        
        return uint256(answer);
    }
    
    /**
     * @notice Add price feeder
     * @param feeder Address to add
     */
    function addPriceFeeder(address feeder) external onlyRole(DEFAULT_ADMIN_ROLE) {
        _grantRole(PRICE_FEEDER_ROLE, feeder);
        emit PriceFeederAdded(feeder);
    }
    
    /**
     * @notice Remove price feeder
     * @param feeder Address to remove
     */
    function removePriceFeeder(address feeder) external onlyRole(DEFAULT_ADMIN_ROLE) {
        _revokeRole(PRICE_FEEDER_ROLE, feeder);
        emit PriceFeederRemoved(feeder);
    }
}

/**
 * @dev Chainlink Aggregator Interface (minimal)
 */
interface IChainlinkAggregator {
    function latestRoundData() external view returns (
        int256 answer,
        uint256 updatedAt,
        uint256 answeredInRound,
        uint256 updatedAt,
        uint256 answeredInRound
    );
}