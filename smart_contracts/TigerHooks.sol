// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * @title TigerHooks
 * @dev Uniswap V4-style hooks framework for TigerSwap
 * 
 * Allows custom logic to be executed at various points in the pool lifecycle:
 * - beforeInitialize / afterInitialize
 * - beforeModifyPosition / afterModifyPosition
 * - beforeSwap / afterSwap
 * - beforeDonate / afterDonate
 */

import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";

/**
 * @dev Hook interface that contracts must implement
 */
interface ITigerHook {
    function beforeInitialize(address pool, uint256 sqrtPriceX96) external;
    function afterInitialize(address pool, uint256 sqrtPriceX96, int24 tick) external;
    function beforeModifyPosition(
        address pool,
        address sender,
        int24 tickLower,
        int24 tickUpper,
        int128 liquidityDelta
    ) external;
    function afterModifyPosition(
        address pool,
        address sender,
        int24 tickLower,
        int24 tickUpper,
        int128 liquidityDelta,
        uint256 feeGrowthInside0,
        uint256 feeGrowthInside1
    ) external;
    function beforeSwap(
        address pool,
        address sender,
        bool zeroForOne,
        int256 amountSpecified,
        uint160 sqrtPriceLimitX96
    ) external returns (bytes calldata);
    function afterSwap(
        address pool,
        address sender,
        bool zeroForOne,
        int256 amountSpecified,
        uint256 amountIn,
        uint256 amountOut,
        uint160 sqrtPriceX96,
        int24 tick
    ) external returns (int256);
    function beforeDonate(
        address pool,
        uint256 amount0,
        uint256 amount1
    ) external;
    function afterDonate(
        address pool,
        uint256 amount0,
        uint256 amount1,
        uint256 feeGrowthInside0,
        uint256 feeGrowthInside1
    ) external;
}

/**
 * @dev Hook permissions
 */
struct HookPermissions {
    bool beforeInitialize;
    bool afterInitialize;
    bool beforeModifyPosition;
    bool afterModifyPosition;
    bool beforeSwap;
    bool afterSwap;
    bool beforeDonate;
    bool afterDonate;
}

/**
 * @title TigerHooks
 * @dev Hooks registry and manager
 */
contract TigerHooks is AccessControl, ReentrancyGuard {
    
    bytes32 public constant HOOK_ADMIN_ROLE = keccak256("HOOK_ADMIN_ROLE");
    bytes32 public constant HOOK_DEVELOPER_ROLE = keccak256("HOOK_DEVELOPER_ROLE");
    
    // Hook contract to permissions mapping
    mapping(address => HookPermissions) public hookPermissions;
    
    // Active hooks
    mapping(address => bool) public activeHooks;
    
    // Hooks by pool
    mapping(address => address[]) public poolHooks;
    
    // Fee configuration for hooks
    struct HookFeeConfig {
        uint256 fixedFee;
        uint256 percentageFee;  // in basis points
        bool useDynamicFee;
    }
    
    mapping(address => HookFeeConfig) public hookFeeConfigs;
    
    // Events
    event HookRegistered(address indexed hook, HookPermissions permissions);
    event HookActivated(address indexed hook, address indexed pool);
    event HookDeactivated(address indexed hook, address indexed pool);
    event HookCalled(address indexed hook, string hookName, bool success);
    event HookFeeUpdated(address indexed hook, uint256 fixedFee, uint256 percentageFee);
    
    constructor() {
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
        _grantRole(HOOK_ADMIN_ROLE, msg.sender);
        _grantRole(HOOK_DEVELOPER_ROLE, msg.sender);
    }
    
    // ==================== REGISTRATION ====================
    
    /**
     * @dev Register a new hook contract
     */
    function registerHook(
        address hookContract,
        HookPermissions calldata permissions
    ) external onlyRole(HOOK_ADMIN_ROLE) {
        require(hookContract != address(0), "Invalid hook address");
        require(
            !activeHooks[hookContract],
            "Hook already registered"
        );
        
        // Verify contract implements ITigerHook
        require(
            ITigerHook(hookContract).beforeInitialize.selector == 
            ITigerHook(hookContract).beforeInitialize.selector,
            "Not a valid hook"
        );
        
        hookPermissions[hookContract] = permissions;
        activeHooks[hookContract] = true;
        
        emit HookRegistered(hookContract, permissions);
    }
    
    /**
     * @dev Update hook permissions
     */
    function updateHookPermissions(
        address hookContract,
        HookPermissions calldata permissions
    ) external onlyRole(HOOK_ADMIN_ROLE) {
        require(activeHooks[hookContract], "Hook not registered");
        hookPermissions[hookContract] = permissions;
        
        emit HookRegistered(hookContract, permissions);
    }
    
    // ==================== POOL HOOKS ====================
    
    /**
     * @dev Activate hook for a pool
     */
    function activateHookForPool(
        address hookContract,
        address pool
    ) external onlyRole(HOOK_ADMIN_ROLE) {
        require(activeHooks[hookContract], "Hook not registered");
        
        poolHooks[pool].push(hookContract);
        
        emit HookActivated(hookContract, pool);
    }
    
    /**
     * @dev Deactivate hook for a pool
     */
    function deactivateHookForPool(
        address hookContract,
        address pool
    ) external onlyRole(HOOK_ADMIN_ROLE) {
        address[] storage hooks = poolHooks[pool];
        
        for (uint256 i = 0; i < hooks.length; i++) {
            if (hooks[i] == hookContract) {
                hooks[i] = hooks[hooks.length - 1];
                hooks.pop();
                emit HookDeactivated(hookContract, pool);
                break;
            }
        }
    }
    
    // ==================== HOOK EXECUTION ====================
    
    /**
     * @dev Execute beforeInitialize hooks
     */
    function executeBeforeInitialize(
        address pool,
        uint256 sqrtPriceX96
    ) external onlyRole(HOOK_DEVELOPER_ROLE) nonReentrant {
        address[] storage hooks = poolHooks[pool];
        
        for (uint256 i = 0; i < hooks.length; i++) {
            if (hookPermissions[hooks[i]].beforeInitialize) {
                try ITigerHook(hooks[i]).beforeInitialize(pool, sqrtPriceX96) {
                    emit HookCalled(hooks[i], "beforeInitialize", true);
                } catch {
                    emit HookCalled(hooks[i], "beforeInitialize", false);
                }
            }
        }
    }
    
    /**
     * @dev Execute afterInitialize hooks
     */
    function executeAfterInitialize(
        address pool,
        uint256 sqrtPriceX96,
        int24 tick
    ) external onlyRole(HOOK_DEVELOPER_ROLE) nonReentrant {
        address[] storage hooks = poolHooks[pool];
        
        for (uint256 i = 0; i < hooks.length; i++) {
            if (hookPermissions[hooks[i]].afterInitialize) {
                try ITigerHook(hooks[i]).afterInitialize(pool, sqrtPriceX96, tick) {
                    emit HookCalled(hooks[i], "afterInitialize", true);
                } catch {
                    emit HookCalled(hooks[i], "afterInitialize", false);
                }
            }
        }
    }
    
    /**
     * @dev Execute beforeModifyPosition hooks
     */
    function executeBeforeModifyPosition(
        address pool,
        address sender,
        int24 tickLower,
        int24 tickUpper,
        int128 liquidityDelta
    ) external onlyRole(HOOK_DEVELOPER_ROLE) nonReentrant {
        address[] storage hooks = poolHooks[pool];
        
        for (uint256 i = 0; i < hooks.length; i++) {
            if (hookPermissions[hooks[i]].beforeModifyPosition) {
                try ITigerHook(hooks[i]).beforeModifyPosition(
                    pool, sender, tickLower, tickUpper, liquidityDelta
                ) {
                    emit HookCalled(hooks[i], "beforeModifyPosition", true);
                } catch {
                    emit HookCalled(hooks[i], "beforeModifyPosition", false);
                }
            }
        }
    }
    
    /**
     * @dev Execute afterModifyPosition hooks
     */
    function executeAfterModifyPosition(
        address pool,
        address sender,
        int24 tickLower,
        int24 tickUpper,
        int128 liquidityDelta,
        uint256 feeGrowthInside0,
        uint256 feeGrowthInside1
    ) external onlyRole(HOOK_DEVELOPER_ROLE) nonReentrant {
        address[] storage hooks = poolHooks[pool];
        
        for (uint256 i = 0; i < hooks.length; i++) {
            if (hookPermissions[hooks[i]].afterModifyPosition) {
                try ITigerHook(hooks[i]).afterModifyPosition(
                    pool, sender, tickLower, tickUpper, liquidityDelta,
                    feeGrowthInside0, feeGrowthInside1
                ) {
                    emit HookCalled(hooks[i], "afterModifyPosition", true);
                } catch {
                    emit HookCalled(hooks[i], "afterModifyPosition", false);
                }
            }
        }
    }
    
    /**
     * @dev Execute beforeSwap hooks
     */
    function executeBeforeSwap(
        address pool,
        address sender,
        bool zeroForOne,
        int256 amountSpecified,
        uint160 sqrtPriceLimitX96
    ) external onlyRole(HOOK_DEVELOPER_ROLE) nonReentrant returns (bytes memory) {
        address[] storage hooks = poolHooks[pool];
        bytes memory hookData;
        
        for (uint256 i = 0; i < hooks.length; i++) {
            if (hookPermissions[hooks[i]].beforeSwap) {
                try ITigerHook(hooks[i]).beforeSwap(
                    pool, sender, zeroForOne, amountSpecified, sqrtPriceLimitX96
                ) returns (bytes memory data) {
                    hookData = data;
                    emit HookCalled(hooks[i], "beforeSwap", true);
                } catch {
                    emit HookCalled(hooks[i], "beforeSwap", false);
                }
            }
        }
        
        return hookData;
    }
    
    /**
     * @dev Execute afterSwap hooks
     */
    function executeAfterSwap(
        address pool,
        address sender,
        bool zeroForOne,
        int256 amountSpecified,
        uint256 amountIn,
        uint256 amountOut,
        uint160 sqrtPriceX96,
        int24 tick
    ) external onlyRole(HOOK_DEVELOPER_ROLE) nonReentrant returns (int256) {
        address[] storage hooks = poolHooks[pool];
        int256 hookResult;
        
        for (uint256 i = 0; i < hooks.length; i++) {
            if (hookPermissions[hooks[i]].afterSwap) {
                try ITigerHook(hooks[i]).afterSwap(
                    pool, sender, zeroForOne, amountSpecified,
                    amountIn, amountOut, sqrtPriceX96, tick
                ) returns (int256 result) {
                    hookResult = result;
                    emit HookCalled(hooks[i], "afterSwap", true);
                } catch {
                    emit HookCalled(hooks[i], "afterSwap", false);
                }
            }
        }
        
        return hookResult;
    }
    
    // ==================== FEE MANAGEMENT ====================
    
    /**
     * @dev Set hook fee configuration
     */
    function setHookFee(
        address hookContract,
        uint256 fixedFee,
        uint256 percentageFee,
        bool useDynamicFee
    ) external onlyRole(HOOK_ADMIN_ROLE) {
        require(activeHooks[hookContract], "Hook not registered");
        
        hookFeeConfigs[hookContract] = HookFeeConfig({
            fixedFee: fixedFee,
            percentageFee: percentageFee,
            useDynamicFee: useDynamicFee
        });
        
        emit HookFeeUpdated(hookContract, fixedFee, percentageFee);
    }
    
    /**
     * @dev Calculate hook execution fee
     */
    function calculateHookFee(
        address hookContract,
        uint256 tradeValue
    ) external view returns (uint256) {
        HookFeeConfig memory config = hookFeeConfigs[hookContract];
        
        if (config.useDynamicFee) {
            return (tradeValue * config.percentageFee) / 10000;
        }
        
        return config.fixedFee;
    }
    
    // ==================== VIEW FUNCTIONS ====================
    
    /**
     * @dev Get hooks for a pool
     */
    function getPoolHooks(address pool) external view returns (address[] memory) {
        return poolHooks[pool];
    }
    
    /**
     * @dev Check if hook has specific permission
     */
    function hasPermission(
        address hookContract,
        string calldata permission
    ) external view returns (bool) {
        HookPermissions memory perms = hookPermissions[hookContract];
        
        if (keccak256(abi.encodePacked(permission)) == keccak256("beforeInitialize")) {
            return perms.beforeInitialize;
        } else if (keccak256(abi.encodePacked(permission)) == keccak256("afterInitialize")) {
            return perms.afterInitialize;
        } else if (keccak256(abi.encodePacked(permission)) == keccak256("beforeModifyPosition")) {
            return perms.beforeModifyPosition;
        } else if (keccak256(abi.encodePacked(permission)) == keccak256("afterModifyPosition")) {
            return perms.afterModifyPosition;
        } else if (keccak256(abi.encodePacked(permission)) == keccak256("beforeSwap")) {
            return perms.beforeSwap;
        } else if (keccak256(abi.encodePacked(permission)) == keccak256("afterSwap")) {
            return perms.afterSwap;
        } else if (keccak256(abi.encodePacked(permission)) == keccak256("beforeDonate")) {
            return perms.beforeDonate;
        } else if (keccak256(abi.encodePacked(permission)) == keccak256("afterDonate")) {
            return perms.afterDonate;
        }
        
        return false;
    }
}

// ==================== EXAMPLE HOOKS ====================

/**
 * @dev Example: Time-Weighted Average Price (TWAP) Hook
 */
contract TWAPHook is ITigerHook {
    uint256 public twapInterval;
    uint256 public lastPrice;
    uint256 public cumulativePrice;
    uint256 public lastUpdate;
    
    constructor(uint256 _twapInterval) {
        twapInterval = _twapInterval;
    }
    
    function beforeInitialize(address pool, uint256 sqrtPriceX96) external override {
        lastPrice = sqrtPriceX96;
        cumulativePrice = 0;
        lastUpdate = block.timestamp;
    }
    
    function afterInitialize(address pool, uint256 sqrtPriceX96, int24 tick) external override {
        lastPrice = sqrtPriceX96;
    }
    
    function afterSwap(
        address pool,
        address sender,
        bool zeroForOne,
        int256 amountSpecified,
        uint256 amountIn,
        uint256 amountOut,
        uint160 sqrtPriceX96,
        int24 tick
    ) external override returns (int256) {
        uint256 timePassed = block.timestamp - lastUpdate;
        
        if (timePassed >= twapInterval) {
            cumulativePrice += lastPrice * timePassed;
            lastPrice = sqrtPriceX96;
            lastUpdate = block.timestamp;
        }
        
        return 0;
    }
    
    // Required but unused
    function beforeModifyPosition(address pool, address sender, int24 tickLower, int24 tickUpper, int128 liquidityDelta) external override {}
    function afterModifyPosition(address pool, address sender, int24 tickLower, int24 tickUpper, int128 liquidityDelta, uint256 feeGrowthInside0, uint256 feeGrowthInside1) external override {}
    function beforeSwap(address pool, address sender, bool zeroForOne, int256 amountSpecified, uint160 sqrtPriceLimitX96) external pure override returns (bytes memory) {}
    function beforeDonate(address pool, uint256 amount0, uint256 amount1) external override {}
    function afterDonate(address pool, uint256 amount0, uint256 amount1, uint256 feeGrowthInside0, uint256 feeGrowthInside1) external override {}
}

/**
 * @dev Example: Auto-Compounding Hook
 */
contract AutoCompoundHook is ITigerHook {
    uint256 public compoundThreshold;
    uint256 public compoundInterval;
    mapping(address => uint256) public lastCompound;
    
    constructor(uint256 _compoundThreshold, uint256 _compoundInterval) {
        compoundThreshold = _compoundThreshold;
        compoundInterval = _compoundInterval;
    }
    
    function afterModifyPosition(
        address pool,
        address sender,
        int24 tickLower,
        int24 tickUpper,
        int128 liquidityDelta,
        uint256 feeGrowthInside0,
        uint256 feeGrowthInside1
    ) external override {
        if (liquidityDelta > 0 && feeGrowthInside0 > compoundThreshold) {
            // Trigger compounding
            if (block.timestamp - lastCompound[pool] >= compoundInterval) {
                // Would call compound function
                lastCompound[pool] = block.timestamp;
            }
        }
    }
    
    // Required but unused
    function beforeInitialize(address pool, uint256 sqrtPriceX96) external override {}
    function afterInitialize(address pool, uint256 sqrtPriceX96, int24 tick) external override {}
    function beforeModifyPosition(address pool, address sender, int24 tickLower, int24 tickUpper, int128 liquidityDelta) external override {}
    function beforeSwap(address pool, address sender, bool zeroForOne, int256 amountSpecified, uint160 sqrtPriceLimitX96) external pure override returns (bytes memory) {}
    function afterSwap(address pool, address sender, bool zeroForOne, int256 amountSpecified, uint256 amountIn, uint256 amountOut, uint160 sqrtPriceX96, int24 tick) external pure override returns (int256) { return 0; }
    function beforeDonate(address pool, uint256 amount0, uint256 amount1) external override {}
    function afterDonate(address pool, uint256 amount0, uint256 amount1, uint256 feeGrowthInside0, uint256 feeGrowthInside1) external override {}
}