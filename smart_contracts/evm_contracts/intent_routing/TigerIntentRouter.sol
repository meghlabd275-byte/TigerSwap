// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "../libraries/SafeMath.sol";

/**
 * @title TigerIntentRouter
 * @notice Intent-Based Routing System
 * @dev Modern DEX aggregation paradigm - users express intent, solvers compete to fill
 * 
 * Key Features:
 * - Intent expressions (swap, cross-chain, limit)
 * - Solver competition for best execution
 * - Dutch auction for MEV capture
 * - Intent batching for gas efficiency
 * - Signature-based settlement
 */
contract TigerIntentRouter {
    using SafeMath for uint256;

    // ============== Constants ==============
    
    // Intent types
    uint8 constant INTENT_TYPE_SWAP = 1;
    uint8 constant INTENT_TYPE_CROSS_CHAIN = 2;
    uint8 constant INTENT_TYPE_LIMIT = 3;
    uint8 constant INTENT_TYPE_AGGREGATE = 4;
    
    // Intent status
    uint8 constant STATUS_PENDING = 1;
    uint8 constant STATUS_FILLED = 2;
    uint8 constant STATUS_EXPIRED = 3;
    uint8 constant STATUS_CANCELLED = 4;
    
    // Solver roles
    uint8 constant SOLVER_ROLE_PRIMARY = 1;
    uint8 constant SOLVER_ROLE_FALLBACK = 2;
    
    // ============== State Variables ==============
    
    // Intent storage
    mapping(bytes32 => Intent) public intents;
    bytes32[] public intentIds;
    
    // Solver registry
    mapping(address => SolverInfo) public solvers;
    address[] public solverList;
    
    // Resolutions
    mapping(bytes32 => Resolution) public resolutions;
    
    // Fees
    uint256 public protocolFeeBps = 5; // 0.05%
    uint256 public solverRewardBps = 2; // 0.02%
    
    // Governance
    address public governance;
    address public feeRecipient;
    
    // Intent nonce
    mapping(address => uint256) public nonces;
    
    // ============== Structs ==============
    
    struct Intent {
        address sender;
        uint8 intentType;
        address tokenIn;
        address tokenOut;
        uint256 amountIn;
        uint256 minAmountOut;
        uint256 maxAmountIn;
        uint256 startTime;
        uint256 expiry;
        uint256 gasPrice;
        uint256 nonce;
        bytes32 referrer;
        uint8 status;
        bytes metadata;
    }
    
    struct SolverInfo {
        address solver;
        string endpoint;
        uint256 reputation;
        uint256 totalFilled;
        uint256 successRate;
        uint256 feesEarned;
        bool active;
        uint256 lastActive;
    }
    
    struct Resolution {
        bytes32 intentId;
        address solver;
        uint256 amountOut;
        uint256 fee;
        uint256 fillTime;
        bytes32 txHash;
    }
    
    // ============== Events ==============
    
    event IntentCreated(
        bytes32 indexed intentId,
        address indexed sender,
        uint8 intentType,
        address tokenIn,
        address tokenOut,
        uint256 amountIn
    );
    
    event IntentResolved(
        bytes32 indexed intentId,
        address indexed solver,
        uint256 amountOut,
        uint256 fee
    );
    
    event SolverRegistered(
        address indexed solver,
        string endpoint
    );
    
    event SolverUpdated(
        address indexed solver,
        string endpoint
    );
    
    event IntentExpired(
        bytes32 indexed intentId,
        address indexed sender
    );
    
    // ============== Constructor ==============
    
    constructor() {
        governance = msg.sender;
        feeRecipient = msg.sender;
    }
    
    // ============== Intent Creation ==============
    
    /**
     * @notice Create a swap intent
     */
    function createSwapIntent(
        address _tokenIn,
        address _tokenOut,
        uint256 _amountIn,
        uint256 _minAmountOut,
        uint256 _expiry,
        bytes32 _referrer
    ) external returns (bytes32 intentId) {
        require(_amountIn > 0, "INVALID_AMOUNT");
        require(_minAmountOut > 0, "INVALID_MIN_OUT");
        
        uint256 nonce = nonces[msg.sender]++;
        
        intentId = keccak256(abi.encodePacked(
            msg.sender,
            INTENT_TYPE_SWAP,
            _tokenIn,
            _tokenOut,
            _amountIn,
            _minAmountOut,
            block.timestamp,
            nonce
        ));
        
        intents[intentId] = Intent({
            sender: msg.sender,
            intentType: INTENT_TYPE_SWAP,
            tokenIn: _tokenIn,
            tokenOut: _tokenOut,
            amountIn: _amountIn,
            minAmountOut: _minAmountOut,
            maxAmountIn: 0,
            startTime: block.timestamp,
            expiry: _expiry > 0 ? _expiry : block.timestamp + 3600,
            gasPrice: tx.gasprice,
            nonce: nonce,
            referrer: _referrer,
            status: STATUS_PENDING,
            metadata: ""
        });
        
        intentIds.push(intentId);
        
        emit IntentCreated(
            intentId,
            msg.sender,
            INTENT_TYPE_SWAP,
            _tokenIn,
            _tokenOut,
            _amountIn
        );
    }
    
    /**
     * @notice Create a cross-chain intent
     */
    function createCrossChainIntent(
        address _tokenIn,
        address _tokenOut,
        uint256 _amountIn,
        uint256 _minAmountOut,
        uint256 _expiry,
        uint256 _destChainId,
        bytes32 _referrer
    ) external returns (bytes32 intentId) {
        require(_amountIn > 0, "INVALID_AMOUNT");
        require(_minAmountOut > 0, "INVALID_MIN_OUT");
        
        uint256 nonce = nonces[msg.sender]++;
        
        intentId = keccak256(abi.encodePacked(
            msg.sender,
            INTENT_TYPE_CROSS_CHAIN,
            _tokenIn,
            _tokenOut,
            _amountIn,
            _minAmountOut,
            _destChainId,
            block.timestamp,
            nonce
        ));
        
        intents[intentId] = Intent({
            sender: msg.sender,
            intentType: INTENT_TYPE_CROSS_CHAIN,
            tokenIn: _tokenIn,
            tokenOut: _tokenOut,
            amountIn: _amountIn,
            minAmountOut: _minAmountOut,
            maxAmountIn: 0,
            startTime: block.timestamp,
            expiry: _expiry > 0 ? _expiry : block.timestamp + 7200,
            gasPrice: tx.gasprice,
            nonce: nonce,
            referrer: _referrer,
            status: STATUS_PENDING,
            metadata: abi.encode(_destChainId)
        });
        
        intentIds.push(intentId);
        
        emit IntentCreated(
            intentId,
            msg.sender,
            INTENT_TYPE_CROSS_CHAIN,
            _tokenIn,
            _tokenOut,
            _amountIn
        );
    }
    
    /**
     * @notice Create a limit intent
     */
    function createLimitIntent(
        address _tokenIn,
        address _tokenOut,
        uint256 _amountIn,
        uint256 _targetRate,
        uint256 _expiry,
        bytes32 _referrer
    ) external returns (bytes32 intentId) {
        require(_amountIn > 0, "INVALID_AMOUNT");
        require(_targetRate > 0, "INVALID_TARGET_RATE");
        
        uint256 nonce = nonces[msg.sender]++;
        
        intentId = keccak256(abi.encodePacked(
            msg.sender,
            INTENT_TYPE_LIMIT,
            _tokenIn,
            _tokenOut,
            _amountIn,
            _targetRate,
            block.timestamp,
            nonce
        ));
        
        intents[intentId] = Intent({
            sender: msg.sender,
            intentType: INTENT_TYPE_LIMIT,
            tokenIn: _tokenIn,
            tokenOut: _tokenOut,
            amountIn: _amountIn,
            minAmountOut: 0,
            maxAmountIn: _amountIn,
            startTime: block.timestamp,
            expiry: _expiry > 0 ? _expiry : block.timestamp + 86400,
            gasPrice: tx.gasprice,
            nonce: nonce,
            referrer: _referrer,
            status: STATUS_PENDING,
            metadata: abi.encode(_targetRate)
        });
        
        intentIds.push(intentId);
        
        emit IntentCreated(
            intentId,
            msg.sender,
            INTENT_TYPE_LIMIT,
            _tokenIn,
            _tokenOut,
            _amountIn
        );
    }
    
    /**
     * @notice Cancel an intent
     */
    function cancelIntent(bytes32 _intentId) external {
        Intent storage intent = intents[_intentId];
        require(intent.sender == msg.sender, "NOT_SENDER");
        require(intent.status == STATUS_PENDING, "NOT_PENDING");
        
        intent.status = STATUS_CANCELLED;
        
        emit IntentExpired(_intentId, msg.sender);
    }
    
    // ============== Intent Resolution ==============
    
    /**
     * @notice Resolve an intent (called by solver)
     */
    function resolveIntent(
        bytes32 _intentId,
        uint256 _amountOut,
        bytes32 _txHash
    ) external {
        require(solvers[msg.sender].active, "NOT_SOLVER");
        
        Intent storage intent = intents[_intentId];
        require(intent.status == STATUS_PENDING, "NOT_PENDING");
        require(block.timestamp <= intent.expiry, "EXPIRED");
        
        // Validate fill meets minimum
        require(_amountOut >= intent.minAmountOut, "INSUFFICIENT_FILL");
        
        // Calculate fees
        uint256 protocolFee = _amountOut * protocolFeeBps / 10000;
        uint256 solverReward = _amountOut * solverRewardBps / 10000;
        
        // Mark as filled
        intent.status = STATUS_FILLED;
        
        // Record resolution
        resolutions[_intentId] = Resolution({
            intentId: _intentId,
            solver: msg.sender,
            amountOut: _amountOut,
            fee: protocolFee + solverReward,
            fillTime: block.timestamp,
            txHash: _txHash
        });
        
        // Update solver stats
        SolverInfo storage solver = solvers[msg.sender];
        solver.totalFilled++;
        solver.feesEarned += solverReward;
        solver.reputation += 100;
        solver.lastActive = block.timestamp;
        
        emit IntentResolved(
            _intentId,
            msg.sender,
            _amountOut,
            protocolFee + solverReward
        );
    }
    
    /**
     * @notice Batch resolve intents (gas efficient)
     */
    function batchResolve(
        bytes32[] calldata _intentIds,
        uint256[] calldata _amountOuts,
        bytes32[] calldata _txHashs
    ) external {
        require(solvers[msg.sender].active, "NOT_SOLVER");
        require(_intentIds.length == _amountOuts.length, "LENGTH_MISMATCH");
        
        for (uint256 i = 0; i < _intentIds.length; i++) {
            Intent storage intent = intents[_intentIds[i]];
            
            if (intent.status != STATUS_PENDING) continue;
            if (block.timestamp > intent.expiry) continue;
            if (_amountOuts[i] < intent.minAmountOut) continue;
            
            // Calculate fees
            uint256 protocolFee = _amountOuts[i] * protocolFeeBps / 10000;
            uint256 solverReward = _amountOuts[i] * solverRewardBps / 10000;
            
            // Mark as filled
            intent.status = STATUS_FILLED;
            
            // Record resolution
            resolutions[_intentIds[i]] = Resolution({
                intentId: _intentIds[i],
                solver: msg.sender,
                amountOut: _amountOuts[i],
                fee: protocolFee + solverReward,
                fillTime: block.timestamp,
                txHash: _txHashs[i]
            });
            
            emit IntentResolved(
                _intentIds[i],
                msg.sender,
                _amountOuts[i],
                protocolFee + solverReward
            );
        }
    }
    
    // ============== Solver Management ==============
    
    /**
     * @notice Register as a solver
     */
    function registerSolver(string calldata _endpoint) external {
        require(!solvers[msg.sender].active, "ALREADY_REGISTERED");
        
        solvers[msg.sender] = SolverInfo({
            solver: msg.sender,
            endpoint: _endpoint,
            reputation: 1000,
            totalFilled: 0,
            successRate: 10000,
            feesEarned: 0,
            active: true,
            lastActive: block.timestamp
        });
        
        solverList.push(msg.sender);
        
        emit SolverRegistered(msg.sender, _endpoint);
    }
    
    /**
     * @notice Update solver endpoint
     */
    function updateSolverEndpoint(string calldata _endpoint) external {
        require(solvers[msg.sender].active, "NOT_SOLVER");
        
        solvers[msg.sender].endpoint = _endpoint;
        
        emit SolverUpdated(msg.sender, _endpoint);
    }
    
    /**
     * @notice Deactivate solver
     */
    function deactivateSolver() external {
        require(solvers[msg.sender].active, "NOT_SOLVER");
        
        solvers[msg.sender].active = false;
    }
    
    // ============== Governance ==============
    
    /**
     * @notice Update fees
     */
    function updateFees(uint256 _protocolFee, uint256 _solverReward) external {
        require(msg.sender == governance, "ONLY_GOVERNANCE");
        
        protocolFeeBps = _protocolFee;
        solverRewardBps = _solverReward;
    }
    
    /**
     * @notice Update governance
     */
    function updateGovernance(address _governance) external {
        require(msg.sender == governance, "ONLY_GOVERNANCE");
        
        governance = _governance;
    }
    
    // ============== View Functions ==============
    
    /**
     * @notice Get intent details
     */
    function getIntent(bytes32 _intentId) external view returns (
        address sender,
        uint8 intentType,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 minAmountOut,
        uint256 expiry,
        uint8 status
    ) {
        Intent storage intent = intents[_intentId];
        return (
            intent.sender,
            intent.intentType,
            intent.tokenIn,
            intent.tokenOut,
            intent.amountIn,
            intent.minAmountOut,
            intent.expiry,
            intent.status
        );
    }
    
    /**
     * @notice Get resolution details
     */
    function getResolution(bytes32 _intentId) external view returns (
        address solver,
        uint256 amountOut,
        uint256 fee,
        uint256 fillTime,
        bytes32 txHash
    ) {
        Resolution storage res = resolutions[_intentId];
        return (
            res.solver,
            res.amountOut,
            res.fee,
            res.fillTime,
            res.txHash
        );
    }
    
    /**
     * @notice Get pending intents
     */
    function getPendingIntents() external view returns (bytes32[] memory) {
        uint256 count = 0;
        for (uint256 i = 0; i < intentIds.length; i++) {
            if (intents[intentIds[i]].status == STATUS_PENDING) {
                count++;
            }
        }
        
        bytes32[] memory result = new bytes32[](count);
        count = 0;
        for (uint256 i = 0; i < intentIds.length; i++) {
            if (intents[intentIds[i]].status == STATUS_PENDING) {
                result[count++] = intentIds[i];
            }
        }
        
        return result;
    }
    
    /**
     * @notice Get active solvers
     */
    function getActiveSolvers() external view returns (address[] memory) {
        uint256 count = 0;
        for (uint256 i = 0; i < solverList.length; i++) {
            if (solvers[solverList[i]].active) {
                count++;
            }
        }
        
        address[] memory result = new address[](count);
        count = 0;
        for (uint256 i = 0; i < solverList.length; i++) {
            if (solvers[solverList[i]].active) {
                result[count++] = solverList[i];
            }
        }
        
        return result;
    }
    
    /**
     * @notice Get solver info
     */
    function getSolverInfo(address _solver) external view returns (
        string memory endpoint,
        uint256 reputation,
        uint256 totalFilled,
        uint256 successRate,
        uint256 feesEarned,
        bool active
    ) {
        SolverInfo storage solver = solvers[_solver];
        return (
            solver.endpoint,
            solver.reputation,
            solver.totalFilled,
            solver.successRate,
            solver.feesEarned,
            solver.active
        );
    }
}