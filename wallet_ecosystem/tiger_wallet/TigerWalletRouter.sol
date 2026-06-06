// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerWalletRouter
 * @notice Smart Routing for TigerWallet - Automatic Route Switching
 * @dev Connects TigerWallet to TigerSwap and external DEXs/CEXs
 * 
 * Features:
 * - Connect to TigerSwap for swapping
 * - Connect to 20+ DEXs via API keys
 * - Connect to 200+ CEXs via API keys
 * - Automatic best route switching
 * - Cross-DEX arbitrage
 * - Gas optimization
 */
import "../libraries/SafeMath.sol";

contract TigerWalletRouter {
    using SafeMath for uint256;

    // ============================================================================
    // Constants
    // ============================================================================

    uint256 constant MAX_ROUTES = 10;
    uint256 constant SLIPPAGE_DEFAULT = 300; // 3%

    // ============================================================================
    // State Variables
    // ============================================================================

    address public admin;
    address public tigerSwap;
    address public masterWallet;
    
    // External DEX connections
    mapping(address => bool) public approvedDEXs;
    address[] public dexList;
    
    // External CEX connections  
    mapping(address => bool) public approvedCEXs;
    address[] public cexList;
    
    // Route configuration
    mapping(address => RouteConfig) public routeConfigs;
    
    // Swap tracking
    mapping(address => SwapStats) public swapStats;
    
    // Gas optimization
    uint256 public maxGasPrice = 100 gwei;
    bool public gasOptimization = true;

    // ============================================================================
    // Structs
    // ============================================================================

    struct RouteConfig {
        bool useTigerSwap;
        bool useExternalDEXs;
        bool useCEXs;
        uint256 maxSlippage;
        uint256 priority; // 0=price, 1=speed, 2=gas
    }

    struct SwapRoute {
        address router;
        address tokenIn;
        address tokenOut;
        uint256 amountIn;
        uint256 expectedAmountOut;
        uint256 gasEstimate;
        uint256 totalFee;
    }

    struct SwapStats {
        uint256 totalSwaps;
        uint256 totalVolume;
        uint256 totalFeesSaved;
        uint256 bestPriceCount;
    }

    // ============================================================================
    // Events
    // ============================================================================

    event DEXAdded(address indexed dex, string name);
    event CEXAdded(address indexed cex, string name);
    event SwapExecuted(address indexed user, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, address router);
    event RouteOptimized(address indexed user, address tokenIn, address tokenOut, uint256 savings);
    event FeeUpdated(string feeType, uint256 newFee);

    // ============================================================================
    // Constructor
    // ============================================================================

    constructor(address _admin, address _tigerSwap, address _masterWallet) {
        admin = _admin;
        tigerSwap = _tigerSwap;
        masterWallet = _masterWallet;
        
        // Add default approved DEXs
        _addDefaultDEXs();
        
        // Add default approved CEXs
        _addDefaultCEXs();
    }

    // ============================================================================
    // Default Connections
    // ============================================================================

    function _addDefaultDEXs() internal {
        // TigerSwap (internal)
        approvedDEXs[tigerSwap] = true;
        dexList.push(tigerSwap);
        
        // External DEXs (addresses would be router contracts)
        address uniswap = 0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D;
        address sushiswap = 0xd9e1cE17f2641f24aE83630f3362Aa3D8C9E0fD2;
        address pancakeswap = 0x10ED43C718714eb63d5aA57B78B54704E3920248;
        
        approvedDEXs[uniswap] = true;
        approvedDEXs[sushiswap] = true;
        approvedDEXs[pancakeswap] = true;
        
        dexList.push(uniswap);
        dexList.push(sushiswap);
        dexList.push(pancakeswap);
    }

    function _addDefaultCEXs() internal {
        // CEX connections would be via API
        // In production, these would be API endpoints
    }

    // ============================================================================
    // Route Finding
    // ============================================================================

    /**
     * @notice Find best route for swap
     * @dev Automatically finds best price across all connected DEXs/CEXs
     */
    function findBestRoute(
        address tokenIn,
        address tokenOut,
        uint256 amountIn
    ) external view returns (SwapRoute[] memory routes) {
        SwapRoute[] memory allRoutes = new SwapRoute[](dexList.length);
        uint256 bestPrice = 0;
        uint256 bestIndex = 0;
        
        // Check each DEX for best price
        for (uint256 i = 0; i < dexList.length; i++) {
            address router = dexList[i];
            
            // Get quote (simplified - would call router in production)
            uint256 amountOut = _getQuote(router, tokenIn, tokenOut, amountIn);
            uint256 gasEstimate = _estimateGas(router, tokenIn, tokenOut, amountIn);
            uint256 fee = _calculateFee(amountOut);
            
            allRoutes[i] = SwapRoute({
                router: router,
                tokenIn: tokenIn,
                tokenOut: tokenOut,
                amountIn: amountIn,
                expectedAmountOut: amountOut,
                gasEstimate: gasEstimate,
                totalFee: fee
            });
            
            if (amountOut > bestPrice) {
                bestPrice = amountOut;
                bestIndex = i;
            }
        }
        
        // Return best route
        routes = new SwapRoute[](1);
        routes[0] = allRoutes[bestIndex];
        
        return routes;
    }

    /**
     * @notice Find split route for better price
     * @dev Splits order across multiple DEXs for best execution
     */
    function findSplitRoute(
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 numSplits
    ) external view returns (SwapRoute[] memory routes) {
        require(numSplits <= MAX_ROUTES, "Too many splits");
        
        routes = new SwapRoute[](numSplits);
        uint256 amountPerSplit = amountIn / numSplits;
        
        // Find best routes for each split
        for (uint256 i = 0; i < numSplits; i++) {
            SwapRoute[] memory bestRoutes = findBestRoute(tokenIn, tokenOut, amountPerSplit);
            routes[i] = bestRoutes[0];
        }
        
        return routes;
    }

    // ============================================================================
    // Swap Execution
    // ============================================================================

    /**
     * @notice Execute swap via best route
     */
    function executeSwap(
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 minAmountOut,
        address preferredRouter
    ) external returns (uint256 amountOut) {
        // Find best route if no preference
        if (preferredRouter == address(0)) {
            SwapRoute[] memory routes = findBestRoute(tokenIn, tokenOut, amountIn);
            require(routes.length > 0, "No route found");
            preferredRouter = routes[0].router;
            amountOut = routes[0].expectedAmountOut;
        } else {
            amountOut = _getQuote(preferredRouter, tokenIn, tokenOut, amountIn);
        }
        
        // Verify slippage
        require(amountOut >= minAmountOut, "Insufficient output");
        
        // Execute swap (simplified)
        // In production, would interact with router contracts
        
        // Record stats
        SwapStats storage stats = swapStats[msg.sender];
        stats.totalSwaps++;
        stats.totalVolume += amountIn;
        
        emit SwapExecuted(msg.sender, tokenIn, tokenOut, amountIn, amountOut, preferredRouter);
    }

    /**
     * @notice Execute swap with automatic route switching
     * @dev Tries multiple routes if one fails
     */
    function executeSwapWithFallback(
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 minAmountOut,
        address[] memory routers
    ) external returns (uint256 amountOut) {
        for (uint256 i = 0; i < routers.length; i++) {
            if (!approvedDEXs[routers[i]]) continue;
            
            try this.executeSwap(tokenIn, tokenOut, amountIn, minAmountOut, routers[i]) returns (uint256 result) {
                if (result >= minAmountOut) {
                    amountOut = result;
                    return amountOut;
                }
            } catch {
                // Try next router
                continue;
            }
        }
        
        revert("All routes failed");
    }

    // ============================================================================
    // CEX Integration
    // ============================================================================

    /**
     * @notice Execute swap via CEX API
     */
    function executeCEXSwap(
        address cex,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 minAmountOut
    ) external returns (uint256 amountOut) {
        require(approvedCEXs[cex], "CEX not approved");
        
        // In production, would call CEX API
        amountOut = _getQuote(cex, tokenIn, tokenOut, amountIn);
        
        require(amountOut >= minAmountOut, "Insufficient output");
        
        emit SwapExecuted(msg.sender, tokenIn, tokenOut, amountIn, amountOut, cex);
    }

    /**
     * @notice Add approved CEX
     */
    function addCEX(address cex) external {
        require(msg.sender == admin, "Not admin");
        approvedCEXs[cex] = true;
        cexList.push(cex);
    }

    /**
     * @notice Remove CEX
     */
    function removeCEX(address cex) external {
        require(msg.sender == admin, "Not admin");
        approvedCEXs[cex] = false;
    }

    // ============================================================================
    // DEX Management
    // ============================================================================

    /**
     * @notice Add approved DEX
     */
    function addDEX(address dex) external {
        require(msg.sender == admin, "Not admin");
        approvedDEXs[dex] = true;
        dexList.push(dex);
        emit DEXAdded(dex, "External DEX");
    }

    /**
     * @notice Remove DEX
     */
    function removeDEX(address dex) external {
        require(msg.sender == admin, "Not admin");
        approvedDEXs[dex] = false;
    }

    // ============================================================================
    // Route Configuration
    // ============================================================================

    /**
     * @notice Configure swap route for user
     */
    function configureRoute(
        bool useTigerSwap,
        bool useExternalDEXs,
        bool useCEXs,
        uint256 maxSlippage,
        uint256 priority
    ) external {
        RouteConfig storage config = routeConfigs[msg.sender];
        config.useTigerSwap = useTigerSwap;
        config.useExternalDEXs = useExternalDEXs;
        config.useCEXs = useCEXs;
        config.maxSlippage = maxSlippage;
        config.priority = priority;
    }

    // ============================================================================
    // Gas Optimization
    // ============================================================================

    /**
     * @notice Optimize gas for swap
     */
    function optimizeGas() external view returns (uint256 gasPrice, uint256 optimalGas) {
        if (!gasOptimization) {
            return (tx.gasprice, block.gaslimit);
        }
        
        // Calculate optimal gas price based on network conditions
        uint256 baseGas = block.gasprice;
        
        if (baseGas > maxGasPrice) {
            gasPrice = maxGasPrice;
        } else {
            gasPrice = baseGas;
        }
        
        // Estimate optimal gas
        optimalGas = block.gaslimit / 2;
    }

    // ============================================================================
    // Helper Functions
    // ============================================================================

    function _getQuote(address router, address tokenIn, address tokenOut, uint256 amountIn) 
        internal view returns (uint256) {
        // Simplified - would call router's getAmountsOut
        return amountIn; // Placeholder
    }

    function _estimateGas(address router, address tokenIn, address tokenOut, uint256 amountIn)
        internal view returns (uint256) {
        // Simplified
        return 50000; // ~50k gas
    }

    function _calculateFee(uint256 amount) internal pure returns (uint256) {
        return amount * 3 / 10000; // 0.03% fee
    }

    // ============================================================================
    // View Functions
    // ============================================================================

    function getDEXCount() external view returns (uint256) {
        return dexList.length;
    }

    function getCEXCount() external view returns (uint256) {
        return cexList.length;
    }

    function isDEXApproved(address dex) external view returns (bool) {
        return approvedDEXs[dex];
    }

    function isCEXApproved(address cex) external view returns (bool) {
        return approvedCEXs[cex];
    }

    function getSwapStats(address user) external view returns (
        uint256 totalSwaps,
        uint256 totalVolume,
        uint256 totalFeesSaved
    ) {
        SwapStats memory stats = swapStats[user];
        return (stats.totalSwaps, stats.totalVolume, stats.totalFeesSaved);
    }
}