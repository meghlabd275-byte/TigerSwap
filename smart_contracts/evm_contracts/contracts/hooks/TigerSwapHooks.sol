// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * @title TigerSwapHooks
 * @notice Uniswap V4/Balancer V3-style hooks framework
 * @dev External contracts modify pool behavior at lifecycle events
 */
contract TigerSwapHooks {
    /// @notice Hook types
    enum HookType {
        BeforeInitialize,
        AfterInitialize,
        BeforeModifyPosition,
        AfterModifyPosition,
        BeforeSwap,
        AfterSwap,
        BeforeDonate,
        AfterDonate,
        BeforeFlash,
        AfterFlash
    }

    /// @notice Pool hook configuration
    struct HookConfig {
        address hookAddress;
        HookType hookType;
        bool enabled;
        uint256 gasLimit;
    }

    /// @notice Hook context for callbacks
    struct HookContext {
        address pool;
        address token0;
        address token1;
        address sender;
        int256 amount0Delta;
        int256 amount1Delta;
        bytes data;
    }

    /// @notice Hook results
    struct HookResult {
        bool success;
        int256 amount0Delta;
        int256 amount1Delta;
        uint256 gasUsed;
        bytes errorData;
    }

    /// @notice Pool hook configurations
    mapping(address => HookConfig[]) public poolHooks;

    /// @notice Flash accounting
    mapping(address => FlashAccount) public flashAccounts;

    /// @notice Transient storage
    mapping(address => mapping(bytes32 => uint256)) public transientStorage;

    /// @notice Flash account
    struct FlashAccount {
        int256 token0Balance;
        int256 token1Balance;
        bool locked;
        uint256 lockDepth;
    }

    /// @notice Events
    event HookRegistered(address indexed pool, address indexed hook, HookType hookType);
    event HookExecuted(address indexed pool, HookType hookType, bool success);
    event FlashLoanExecuted(address indexed pool, address borrower, uint256 amount0, uint256 amount1);
    event FlashLoanRepaid(address indexed pool, uint256 amount0, uint256 amount1);
    event TransientStorageSet(address indexed pool, bytes32 key, uint256 value);

    /// @notice Errors
    error PoolNotFound();
    error HookNotRegistered();
    error HookExecutionFailed();
    error FlashLoanNotRepaid();
    error FlashLocked();
    error InvalidAmount();

    /// @notice Fee for flash loans (in basis points)
    uint256 public constant FLASH_FEE = 100; // 0.01%

    /// @notice Admin
    address public admin;

    modifier onlyAdmin() {
        require(msg.sender == admin, "Not admin");
        _;
    }

    constructor() {
        admin = msg.sender;
    }

    /**
     * @notice Register hooks for a pool
     * @param _pool Pool address
     * @param _hookAddress Hook contract address
     * @param _hookType Hook type
     */
    function registerHook(
        address _pool,
        address _hookAddress,
        HookType _hookType
    ) external onlyAdmin {
        poolHooks[_pool].push(HookConfig({
            hookAddress: _hookAddress,
            hookType: _hookType,
            enabled: true,
            gasLimit: 500000
        }));

        emit HookRegistered(_pool, _hookAddress, _hookType);
    }

    /**
     * @notice Execute before initialize hook
     * @param _pool Pool address
     * @param _token0 Token 0
     * @param _token1 Token 1
     * @param _data Hook data
     * @return Hook result
     */
    function beforeInitialize(
        address _pool,
        address _token0,
        address _token1,
        bytes calldata _data
    ) external returns (HookResult memory) {
        HookConfig[] storage hooks = poolHooks[_pool];
        
        for (uint256 i = 0; i < hooks.length; i++) {
            HookConfig storage config = hooks[i];
            if (!config.enabled || config.hookType != HookType.BeforeInitialize) continue;

            try IHook(config.hookAddress).beforeInitialize(
                HookContext({
                    pool: _pool,
                    token0: _token0,
                    token1: _token1,
                    sender: msg.sender,
                    amount0Delta: 0,
                    amount1Delta: 0,
                    data: _data
                })
            ) returns (HookResult memory result) {
                emit HookExecuted(_pool, HookType.BeforeInitialize, result.success);
                if (!result.success) return result;
            } catch {
                return HookResult({
                    success: false,
                    amount0Delta: 0,
                    amount1Delta: 0,
                    gasUsed: 0,
                    errorData: "Hook failed"
                });
            }
        }

        return HookResult({
            success: true,
            amount0Delta: 0,
            amount1Delta: 0,
            gasUsed: 0,
            errorData: ""
        });
    }

    /**
     * @notice Execute after initialize hook
     * @param _pool Pool address
     * @param _token0 Token 0
     * @param _token1 Token 1
     * @param _data Hook data
     * @return Hook result
     */
    function afterInitialize(
        address _pool,
        address _token0,
        address _token1,
        bytes calldata _data
    ) external returns (HookResult memory) {
        HookConfig[] storage hooks = poolHooks[_pool];
        
        for (uint256 i = 0; i < hooks.length; i++) {
            HookConfig storage config = hooks[i];
            if (!config.enabled || config.hookType != HookType.AfterInitialize) continue;

            try IHook(config.hookAddress).afterInitialize(
                HookContext({
                    pool: _pool,
                    token0: _token0,
                    token1: _token1,
                    sender: msg.sender,
                    amount0Delta: 0,
                    amount1Delta: 0,
                    data: _data
                })
            ) returns (HookResult memory result) {
                emit HookExecuted(_pool, HookType.AfterInitialize, result.success);
                if (!result.success) return result;
            } catch {
                return HookResult({
                    success: false,
                    amount0Delta: 0,
                    amount1Delta: 0,
                    gasUsed: 0,
                    errorData: "Hook failed"
                });
            }
        }

        return HookResult({
            success: true,
            amount0Delta: 0,
            amount1Delta: 0,
            gasUsed: 0,
            errorData: ""
        });
    }

    /**
     * @notice Execute before swap hook
     * @param _pool Pool address
     * @param _sender Sender
     * @param _amount0Delta Amount 0 delta
     * @param _amount1Delta Amount 1 delta
     * @param _data Hook data
     * @return Hook result
     */
    function beforeSwap(
        address _pool,
        address _sender,
        int256 _amount0Delta,
        int256 _amount1Delta,
        bytes calldata _data
    ) external returns (HookResult memory) {
        HookConfig[] storage hooks = poolHooks[_pool];
        
        for (uint256 i = 0; i < hooks.length; i++) {
            HookConfig storage config = hooks[i];
            if (!config.enabled || config.hookType != HookType.BeforeSwap) continue;

            try IHook(config.hookAddress).beforeSwap(
                HookContext({
                    pool: _pool,
                    token0: address(0),
                    token1: address(0),
                    sender: _sender,
                    amount0Delta: _amount0Delta,
                    amount1Delta: _amount1Delta,
                    data: _data
                })
            ) returns (HookResult memory result) {
                emit HookExecuted(_pool, HookType.BeforeSwap, result.success);
                if (!result.success) return result;
            } catch {
                return HookResult({
                    success: false,
                    amount0Delta: 0,
                    amount1Delta: 0,
                    gasUsed: 0,
                    errorData: "Hook failed"
                });
            }
        }

        return HookResult({
            success: true,
            amount0Delta: 0,
            amount1Delta: 0,
            gasUsed: 0,
            errorData: ""
        });
    }

    /**
     * @notice Execute after swap hook
     * @param _pool Pool address
     * @param _sender Sender
     * @param _amount0Delta Amount 0 delta
     * @param _amount1Delta Amount 1 delta
     * @param _data Hook data
     * @return Hook result
     */
    function afterSwap(
        address _pool,
        address _sender,
        int256 _amount0Delta,
        int256 _amount1Delta,
        bytes calldata _data
    ) external returns (HookResult memory) {
        HookConfig[] storage hooks = poolHooks[_pool];
        
        for (uint256 i = 0; i < hooks.length; i++) {
            HookConfig storage config = hooks[i];
            if (!config.enabled || config.hookType != HookType.AfterSwap) continue;

            try IHook(config.hookAddress).afterSwap(
                HookContext({
                    pool: _pool,
                    token0: address(0),
                    token1: address(0),
                    sender: _sender,
                    amount0Delta: _amount0Delta,
                    amount1Delta: _amount1Delta,
                    data: _data
                })
            ) returns (HookResult memory result) {
                emit HookExecuted(_pool, HookType.AfterSwap, result.success);
                if (!result.success) return result;
            } catch {
                return HookResult({
                    success: false,
                    amount0Delta: 0,
                    amount1Delta: 0,
                    gasUsed: 0,
                    errorData: "Hook failed"
                });
            }
        }

        return HookResult({
            success: true,
            amount0Delta: 0,
            amount1Delta: 0,
            gasUsed: 0,
            errorData: ""
        });
    }

    /**
     * @notice Execute flash loan
     * @param _pool Pool address
     * @param _borrower Borrower address
     * @param _amount0 Amount 0 to borrow
     * @param _amount1 Amount 1 to borrow
     * @param _data Callback data
     */
    function flash(
        address _pool,
        address _borrower,
        uint256 _amount0,
        uint256 _amount1,
        bytes calldata _data
    ) external {
        if (_amount0 == 0 && _amount1 == 0) revert InvalidAmount();

        FlashAccount storage account = flashAccounts[_pool];
        
        if (account.locked) revert FlashLocked();

        // Lock flash accounting
        account.locked = true;
        account.lockDepth++;
        account.token0Balance += int256(_amount0);
        account.token1Balance += int256(_amount1);

        emit FlashLoanExecuted(_pool, _borrower, _amount0, _amount1);

        // Call callback
        IFlashBorrower(_borrower).onFlashLoan(
            msg.sender,
            _amount0,
            _amount1,
            FLASH_FEE,
            _data
        );

        // Check if balance is sufficient (repayment check)
        if (account.token0Balance < 0 || account.token1Balance < 0) {
            revert FlashLoanNotRepaid();
        }

        // Unlock
        account.locked = false;
        account.lockDepth--;

        emit FlashLoanRepaid(_pool, _amount0, _amount1);
    }

    /**
     * @notice Set transient storage value
     * @param _pool Pool address
     * @param _key Storage key
     * @param _value Storage value
     */
    function setTransientStorage(
        address _pool,
        bytes32 _key,
        uint256 _value
    ) external {
        require(!account.locked || account.lockDepth == 0, "Locked");
        
        transientStorage[_pool][_key] = _value;
        
        emit TransientStorageSet(_pool, _key, _value);
    }

    /**
     * @notice Get transient storage value
     * @param _pool Pool address
     * @param _key Storage key
     * @return Stored value
     */
    function getTransientStorage(
        address _pool,
        bytes32 _key
    ) external view returns (uint256) {
        return transientStorage[_pool][_key];
    }

    /**
     * @notice Get pool hook count
     * @param _pool Pool address
     * @return Hook count
     */
    function getHookCount(address _pool) external view returns (uint256) {
        return poolHooks[_pool].length;
    }

    /**
     * @notice Get pool hooks
     * @param _pool Pool address
     * @param _index Hook index
     * @return Hook configuration
     */
    function getHook(address _pool, uint256 _index) external view returns (
        address hookAddress,
        HookType hookType,
        bool enabled,
        uint256 gasLimit
    ) {
        HookConfig storage config = poolHooks[_pool][_index];
        return (
            config.hookAddress,
            config.hookType,
            config.enabled,
            config.gasLimit
        );
    }
}

/**
 * @title IHook
 * @notice Hook interface
 */
interface IHook {
    function beforeInitialize(HookContext calldata) external returns (HookResult memory);
    function afterInitialize(HookContext calldata) external returns (HookResult memory);
    function beforeSwap(HookContext calldata) external returns (HookResult memory);
    function afterSwap(HookContext calldata) external returns (HookResult memory);
}

/**
 * @title IFlashBorrower
 * @notice Flash loan borrower interface
 */
interface IFlashBorrower {
    function onFlashLoan(
        address sender,
        uint256 amount0,
        uint256 amount1,
        uint256 fee,
        bytes calldata data
    ) external;
}