// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerHooks
 * @notice Extensible Hook System for TigerSwap V4
 * @dev Enables custom pool behavior through hook callbacks
 * 
 * Hook Points (8 total):
 * 1. beforeInitialize / afterInitialize
 * 2. beforeModifyPosition / afterModifyPosition
 * 3. beforeSwap / afterSwap
 * 4. beforeDonate / afterDonate
 * 
 * Permission Bits:
 * 0x01: beforeInitialize
 * 0x02: afterInitialize  
 * 0x04: beforeModifyPosition
 * 0x08: afterModifyPosition
 * 0x10: beforeSwap
 * 0x20: afterSwap
 * 0x40: beforeDonate
 * 0x80: afterDonate
 */
contract TigerHooks {
    // ============== Constants ==============
    
    // Hook permission bits
    uint8 constant BEFORE_INITIALIZE = 0x01;
    uint8 constant AFTER_INITIALIZE = 0x02;
    uint8 constant BEFORE_MODIFY_POSITION = 0x04;
    uint8 constant AFTER_MODIFY_POSITION = 0x08;
    uint8 constant BEFORE_SWAP = 0x10;
    uint8 constant AFTER_SWAP = 0x20;
    uint8 constant BEFORE_DONATE = 0x40;
    uint8 constant AFTER_DONATE = 0x80;
    
    // ============== State ==============
    
    // Hook address => permissions
    mapping(address => uint8) public hookPermissions;
    
    // Hook address => hook implementation
    mapping(address => ITigerHook) public hookImplementations;
    
    // Registered hooks
    mapping(address => bool) public registeredHooks;
    
    // Pool manager reference
    address public poolManager;
    
    // ============== Events ==============
    
    event HookRegistered(address indexed hook, uint8 permissions);
    event HookUnregistered(address indexed hook);
    event HookCall(address indexed hook, bytes32 hookHash, bool success);
    
    // ============== Constructor ==============
    
    constructor(address _poolManager) {
        poolManager = _poolManager;
    }
    
    // ============== Registration ==============
    
    /**
     * @notice Register a hook with permissions
     */
    function registerHook(address _hook, uint8 _permissions) external {
        require(msg.sender == poolManager, "ONLY_POOL_MANAGER");
        require(_hook != address(0), "INVALID_HOOK");
        require(_permissions > 0, "NO_PERMISSIONS");
        
        hookPermissions[_hook] = _permissions;
        registeredHooks[_hook] = true;
        
        emit HookRegistered(_hook, _permissions);
    }
    
    /**
     * @notice Unregister a hook
     */
    function unregisterHook(address _hook) external {
        require(msg.sender == poolManager, "ONLY_POOL_MANAGER");
        
        delete hookPermissions[_hook];
        delete registeredHooks[_hook];
        
        emit HookUnregistered(_hook);
    }
    
    // ============== Hook Callbacks ==============
    
    /**
     * @notice Before initialize hook
     */
    function beforeInitialize(
        address _hook,
        address _sender,
        ITigerPoolManager.PoolKey calldata _poolKey,
        uint160 _sqrtPriceX96
    ) internal returns (bytes memory) {
        if (!registeredHooks[_hook]) return "";
        if ((hookPermissions[_hook] & BEFORE_INITIALIZE) == 0) return "";
        
        try ITigerHook(_hook).beforeInitialize(_sender, _poolKey, _sqrtPriceX96) 
            returns (bytes memory data) {
            emit HookCall(_hook, keccak256("beforeInitialize"), true);
            return data;
        } catch {
            emit HookCall(_hook, keccak256("beforeInitialize"), false);
            return "";
        }
    }
    
    /**
     * @notice After initialize hook
     */
    function afterInitialize(
        address _hook,
        address _sender,
        ITigerPoolManager.PoolKey calldata _poolKey,
        uint160 _sqrtPriceX96,
        int24 _tick
    ) internal {
        if (!registeredHooks[_hook]) return;
        if ((hookPermissions[_hook] & AFTER_INITIALIZE) == 0) return;
        
        try ITigerHook(_hook).afterInitialize(
            _sender, 
            _poolKey, 
            _sqrtPriceX96, 
            _tick
        ) {
            emit HookCall(_hook, keccak256("afterInitialize"), true);
        } catch {
            emit HookCall(_hook, keccak256("afterInitialize"), false);
        }
    }
    
    /**
     * @notice Before modify position hook
     */
    function beforeModifyPosition(
        address _hook,
        address _sender,
        ITigerPoolManager.PoolKey calldata _poolKey,
        ITigerPoolManager.ModifyPositionParams calldata _params
    ) internal returns (bytes memory) {
        if (!registeredHooks[_hook]) return "";
        if ((hookPermissions[_hook] & BEFORE_MODIFY_POSITION) == 0) return "";
        
        try ITigerHook(_hook).beforeModifyPosition(_sender, _poolKey, _params)
            returns (bytes memory data) {
            emit HookCall(_hook, keccak256("beforeModifyPosition"), true);
            return data;
        } catch {
            emit HookCall(_hook, keccak256("beforeModifyPosition"), false);
            return "";
        }
    }
    
    /**
     * @notice After modify position hook
     */
    function afterModifyPosition(
        address _hook,
        address _sender,
        ITigerPoolManager.PoolKey calldata _poolKey,
        ITigerPoolManager.ModifyPositionParams calldata _params,
        ITigerPoolManager.PositionDelta calldata _delta
    ) internal {
        if (!registeredHooks[_hook]) return;
        if ((hookPermissions[_hook] & AFTER_MODIFY_POSITION) == 0) return;
        
        try ITigerHook(_hook).afterModifyPosition(
            _sender, 
            _poolKey, 
            _params, 
            _delta
        ) {
            emit HookCall(_hook, keccak256("afterModifyPosition"), true);
        } catch {
            emit HookCall(_hook, keccak256("afterModifyPosition"), false);
        }
    }
    
    /**
     * @notice Before swap hook
     */
    function beforeSwap(
        address _hook,
        address _sender,
        ITigerPoolManager.PoolKey calldata _poolKey,
        ITigerPoolManager.SwapParams calldata _params
    ) internal returns (bytes memory) {
        if (!registeredHooks[_hook]) return "";
        if ((hookPermissions[_hook] & BEFORE_SWAP) == 0) return "";
        
        try ITigerHook(_hook).beforeSwap(_sender, _poolKey, _params)
            returns (bytes memory data) {
            emit HookCall(_hook, keccak256("beforeSwap"), true);
            return data;
        } catch {
            emit HookCall(_hook, keccak256("beforeSwap"), false);
            return "";
        }
    }
    
    /**
     * @notice After swap hook
     */
    function afterSwap(
        address _hook,
        address _sender,
        ITigerPoolManager.PoolKey calldata _poolKey,
        ITigerPoolManager.SwapParams calldata _params,
        ITigerPoolManager.SwapDelta calldata _delta
    ) internal returns (int256) {
        if (!registeredHooks[_hook]) return 0;
        if ((hookPermissions[_hook] & AFTER_SWAP) == 0) return 0;
        
        try ITigerHook(_hook).afterSwap(_sender, _poolKey, _params, _delta)
            returns (int256 hookReturn) {
            emit HookCall(_hook, keccak256("afterSwap"), true);
            return hookReturn;
        } catch {
            emit HookCall(_hook, keccak256("afterSwap"), false);
            return 0;
        }
    }
    
    /**
     * @notice Before donate hook
     */
    function beforeDonate(
        address _hook,
        address _sender,
        ITigerPoolManager.PoolKey calldata _poolKey,
        uint256 _amount0,
        uint256 _amount1
    ) internal returns (bytes memory) {
        if (!registeredHooks[_hook]) return "";
        if ((hookPermissions[_hook] & BEFORE_DONATE) == 0) return "";
        
        try ITigerHook(_hook).beforeDonate(_sender, _poolKey, _amount0, _amount1)
            returns (bytes memory data) {
            emit HookCall(_hook, keccak256("beforeDonate"), true);
            return data;
        } catch {
            emit HookCall(_hook, keccak256("beforeDonate"), false);
            return "";
        }
    }
    
    /**
     * @notice After donate hook
     */
    function afterDonate(
        address _hook,
        address _sender,
        ITigerPoolManager.PoolKey calldata _poolKey,
        uint256 _amount0,
        uint256 _amount1
    ) internal {
        if (!registeredHooks[_hook]) return;
        if ((hookPermissions[_hook] & AFTER_DONATE) == 0) return;
        
        try ITigerHook(_hook).afterDonate(_sender, _poolKey, _amount0, _amount1) {
            emit HookCall(_hook, keccak256("afterDonate"), true);
        } catch {
            emit HookCall(_hook, keccak256("afterDonate"), false);
        }
    }
    
    // ============== View Functions ==============
    
    /**
     * @notice Check if hook has specific permission
     */
    function hasPermission(address _hook, uint8 _permission) external view returns (bool) {
        return (hookPermissions[_hook] & _permission) == _permission;
    }
    
    /**
     * @notice Get hook permissions
     */
    function getHookPermissions(address _hook) external view returns (uint8) {
        return hookPermissions[_hook];
    }
}

/**
 * @title ITigerHook
 * @notice Interface for hook implementations
 */
interface ITigerHook {
    // Called before pool initialization
    function beforeInitialize(
        address sender,
        ITigerPoolManager.PoolKey calldata poolKey,
        uint160 sqrtPriceX96
    ) external returns (bytes memory);
    
    // Called after pool initialization
    function afterInitialize(
        address sender,
        ITigerPoolManager.PoolKey calldata poolKey,
        uint160 sqrtPriceX96,
        int24 tick
    ) external;
    
    // Called before position modification
    function beforeModifyPosition(
        address sender,
        ITigerPoolManager.PoolKey calldata poolKey,
        ITigerPoolManager.ModifyPositionParams calldata params
    ) external returns (bytes memory);
    
    // Called after position modification
    function afterModifyPosition(
        address sender,
        ITigerPoolManager.PoolKey calldata poolKey,
        ITigerPoolManager.ModifyPositionParams calldata params,
        ITigerPoolManager.PositionDelta calldata delta
    ) external;
    
    // Called before swap
    function beforeSwap(
        address sender,
        ITigerPoolManager.PoolKey calldata poolKey,
        ITigerPoolManager.SwapParams calldata params
    ) external returns (bytes memory);
    
    // Called after swap
    function afterSwap(
        address sender,
        ITigerPoolManager.PoolKey calldata poolKey,
        ITigerPoolManager.SwapParams calldata params,
        ITigerPoolManager.SwapDelta calldata delta
    ) external returns (int256);
    
    // Called before donation
    function beforeDonate(
        address sender,
        ITigerPoolManager.PoolKey calldata poolKey,
        uint256 amount0,
        uint256 amount1
    ) external returns (bytes memory);
    
    // Called after donation
    function afterDonate(
        address sender,
        ITigerPoolManager.PoolKey calldata poolKey,
        uint256 amount0,
        uint256 amount1
    ) external;
}

/**
 * @title ITigerPoolManager
 * @notice Pool manager interface (placeholder for integration)
 */
interface ITigerPoolManager {
    struct PoolKey {
        address currency0;
        address currency1;
        address hooks;
        uint24 fee;
        int24 tickSpacing;
        bytes32 parameters;
    }
    
    struct ModifyPositionParams {
        int24 tickLower;
        int24 tickUpper;
        int128 liquidityDelta;
    }
    
    struct PositionDelta {
        int256 amount0;
        int256 amount1;
    }
    
    struct SwapParams {
        bool zeroForOne;
        int256 amountSpecified;
        uint160 sqrtPriceLimitX96;
    }
    
    struct SwapDelta {
        int256 amount0;
        int256 amount1;
    }
}