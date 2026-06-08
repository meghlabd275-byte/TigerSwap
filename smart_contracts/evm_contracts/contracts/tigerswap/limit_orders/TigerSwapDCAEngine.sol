// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title TigerSwapDCAEngine
 * @notice Dollar-Cost Averaging Engine for TigerSwap
 * @dev Supports time-based DCA, price-dip DCA, recurring orders
 */
contract TigerSwapDCAEngine is ReentrancyGuard, Ownable {
    using SafeERC20 for IERC20;

    // ============ Constants ============
    uint256 public constant MIN_DCA_AMOUNT = 1e6; // 0.000001 token
    uint256 public constant MAX_INTERVAL = 365 days;
    uint256 public constant MIN_INTERVAL = 1 minutes;

    // ============ Enums ============
    enum DCAType {
        TimeBased,        // Fixed interval
        PriceDip,        // Buy when price drops
        VolumeBased,      // Based on volume
        TWAP             // Time-weighted average
    }

    enum DCAStatus {
        Active,
        Paused,
        Completed,
        Cancelled
    }

    enum TriggerType {
        Time,           // Time interval
        PriceAbove,       // Price above threshold
        PriceBelow,      // Price below threshold
        PriceDip,        // Price drops by percentage
        VolumeSpike     // Volume spike
    }

    // ============ Structs ============
    struct DCAPlan {
        uint256 id;
        address owner;
        address tokenIn;
        address tokenOut;
        uint256 amountInPerExecution;
        uint256 totalAmountIn;
        uint256 executedAmountIn;
        uint256 amountOutAccumulated;
        uint256 interval;         // Time between executions
        uint256 nextExecutionTime;
        uint256 priceUpper;      // Upper price bound (for price triggers)
        uint256 priceLower;     // Lower price bound
        uint256 dipThreshold;  // Percentage dip to trigger (1e6 = 100%)
        uint256 executionsCompleted;
        uint256 maxExecutions;
        DCAType dcaType;
        DCAStatus status;
        uint64 createdAt;
        uint64 lastExecutionAt;
    }

    struct DCARecord {
        uint256 planId;
        uint256 amountIn;
        uint256 amountOut;
        uint256 price;
        uint64 timestamp;
    }

    struct PriceTrigger {
        address tokenIn;
        address tokenOut;
        uint256 triggerPrice;
        TriggerType triggerType;
        uint256 dipPercentage;
    }

    // ============ State ============
    mapping(address => mapping(uint256 => DCAPlan)) public dcaPlans;
    mapping(address => uint256[]) public userPlanIds;
    mapping(uint256 => DCARecord[]) public dcaRecords;
    
    // Price feeds
    mapping(address => address) public priceFeeds;
    
    // Active plans for execution
    mapping(uint256 => bool) public activePlans;
    uint256[] public executablePlanIds;
    
    // Statistics
    mapping(address => uint256) public totalDCAVolume;
    mapping(address => uint256) public totalExecutions;
    
    // Protocol fees
    uint256 public protocolFeeBps = 5; // 0.05%
    
    // Oracles
    mapping(address => address) public priceOracles;

    // Events
    event DCAPlanCreated(
        uint256 indexed planId,
        address indexed owner,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 interval,
        DCAType dcaType
    );

    event DCAExecuted(
        uint256 indexed planId,
        address indexed owner,
        uint256 amountIn,
        uint256 amountOut,
        uint256 price
    );

    event DCAPlanCancelled(
        uint256 indexed planId,
        address indexed owner
    );

    event DCAPlanCompleted(
        uint256 indexed planId,
        address indexed owner
    );

    event DCAPlanPaused(
        uint256 indexed planId,
        address indexed owner
    );

    event DCAPlanResumed(
        uint256 indexed planId,
        address indexed owner
    );

    // ============ Constructor ============
    constructor(address _owner) Ownable(_owner) {}

    // ============ DCA Plan Management ============

    /**
     * @notice Create a new DCA plan
     * @param tokenIn Token to spend
     * @param tokenOut Token to receive
     * @param amountInPerExecution Amount to swap each execution
     * @param interval Time between executions
     * @param maxExecutions Maximum number of executions (0 = unlimited)
     * @param dcaType Type of DCA
     * @param priceUpper Upper price bound (for price triggers)
     * @param priceLower Lower price bound
     * @param dipThreshold Dip percentage to trigger
     */
    function createDCAPlan(
        address tokenIn,
        address tokenOut,
        uint256 amountInPerExecution,
        uint256 interval,
        uint256 maxExecutions,
        DCAType dcaType,
        uint256 priceUpper,
        uint256 priceLower,
        uint256 dipThreshold
    ) external nonReentrant returns (uint256 planId) {
        require(tokenIn != tokenOut, "Invalid token pair");
        require(amountInPerExecution >= MIN_DCA_AMOUNT, "Amount too small");
        require(interval >= MIN_INTERVAL && interval <= MAX_INTERVAL, "Invalid interval");
        
        // Check token balance
        uint256 balance = IERC20(tokenIn).balanceOf(msg.sender);
        require(balance >= amountInPerExecution, "Insufficient balance");
        
        // Approve
        IERC20(tokenIn).forceApprove(address(this), balance);
        
        planId = ++_planId(msg.sender);
        
        DCAPlan storage plan = dcaPlans[msg.sender][planId];
        plan.id = planId;
        plan.owner = msg.sender;
        plan.tokenIn = tokenIn;
        plan.tokenOut = tokenOut;
        plan.amountInPerExecution = amountInPerExecution;
        plan.totalAmountIn = balance;
        plan.interval = interval;
        plan.nextExecutionTime = block.timestamp + interval;
        plan.priceUpper = priceUpper;
        plan.priceLower = priceLower;
        plan.dipThreshold = dipThreshold;
        plan.maxExecutions = maxExecutions;
        plan.dcaType = dcaType;
        plan.status = DCAStatus.Active;
        plan.createdAt = uint64(block.timestamp);
        
        userPlanIds[msg.sender].push(planId);
        executablePlanIds.push(planId);
        activePlans[planId] = true;
        
        emit DCAPlanCreated(
            planId,
            msg.sender,
            tokenIn,
            tokenOut,
            amountInPerExecution,
            interval,
            dcaType
        );
        
        return planId;
    }

    /**
     * @notice Create a recurring order (alias for createDCAPlan)
     */
    function createRecurringOrder(
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 interval,
        uint256 executions
    ) external returns (uint256 planId) {
        return createDCAPlan(
            tokenIn,
            tokenOut,
            amountIn,
            interval,
            executions,
            DCAType.TimeBased,
            0,
            0,
            0
        );
    }

    /**
     * @notice Cancel a DCA plan
     */
    function cancelDCAPlan(uint256 planId) external nonReentrant returns (bool) {
        DCAPlan storage plan = dcaPlans[msg.sender][planId];
        require(plan.id == planId, "Plan not found");
        require(plan.status == DCAStatus.Active, "Not active");
        
        plan.status = DCAStatus.Cancelled;
        activePlans[planId] = false;
        
        // Remove from executable list
        _removeExecutablePlan(planId);
        
        emit DCAPlanCancelled(planId, msg.sender);
        
        return true;
    }

    /**
     * @notice Pause a DCA plan
     */
    function pauseDCAPlan(uint256 planId) external returns (bool) {
        DCAPlan storage plan = dcaPlans[msg.sender][planId];
        require(plan.id == planId, "Plan not found");
        
        plan.status = DCAStatus.Paused;
        activePlans[planId] = false;
        
        emit DCAPlanPaused(planId, msg.sender);
        
        return true;
    }

    /**
     * @notice Resume a DCA plan
     */
    function resumeDCAPlan(uint256 planId) external returns (bool) {
        DCAPlan storage plan = dcaPlans[msg.sender][planId];
        require(plan.id == planId, "Plan not found");
        require(plan.status == DCAStatus.Paused, "Not paused");
        
        plan.status = DCAStatus.Active;
        plan.nextExecutionTime = block.timestamp + plan.interval;
        activePlans[planId] = true;
        
        emit DCAPlanResumed(planId, msg.sender);
        
        return true;
    }

    /**
     * @notice Execute a DCA plan (called by execution bot)
     */
    function executeDCAPlan(
        uint256 planId,
        uint256 amountIn,
        uint256 amountOut
    ) external nonReentrant returns (bool) {
        DCAPlan storage plan = dcaPlans[msg.sender][planId];
        require(plan.id == planId, "Plan not found");
        require(plan.status == DCAStatus.Active, "Not active");
        require(activePlans[planId], "Plan not active");
        
        // Check time for time-based DCA
        if (plan.dcaType == DCAType.TimeBased) {
            require(block.timestamp >= plan.nextExecutionTime, "Too early");
        }
        
        // Check price triggers
        if (plan.priceLower > 0 || plan.priceUpper > 0) {
            uint256 currentPrice = _getPrice(plan.tokenIn, plan.tokenOut);
            
            if (plan.dcaType == DCAType.PriceDip) {
                require(currentPrice <= plan.priceLower, "Price not in range");
            } else {
                if (plan.priceUpper > 0) {
                    require(currentPrice <= plan.priceUpper, "Above upper");
                }
                if (plan.priceLower > 0) {
                    require(currentPrice >= plan.priceLower, "Below lower");
                }
            }
        }
        
        // Execute
        plan.executedAmountIn += amountIn;
        plan.amountOutAccumulated += amountOut;
        plan.executionsCompleted++;
        plan.lastExecutionAt = uint64(block.timestamp);
        
        // Update next execution time
        if (plan.dcaType == DCAType.TimeBased) {
            plan.nextExecutionTime = block.timestamp + plan.interval;
        }
        
        // Check completion
        if (plan.maxExecutions > 0 && plan.executionsCompleted >= plan.maxExecutions) {
            plan.status = DCAStatus.Completed;
            activePlans[planId] = false;
            _removeExecutablePlan(planId);
            emit DCAPlanCompleted(planId, plan.owner);
        }
        
        // Record execution
        dcaRecords[planId].push(DCARecord({
            planId: planId,
            amountIn: amountIn,
            amountOut: amountOut,
            price: _getPrice(plan.tokenIn, plan.tokenOut),
            timestamp: uint64(block.timestamp)
        }));
        
        // Update stats
        totalDCAVolume[plan.tokenIn] += amountIn;
        totalExecutions[plan.tokenIn]++;
        
        emit DCAExecuted(planId, plan.owner, amountIn, amountOut, _getPrice(plan.tokenIn, plan.tokenOut));
        
        return true;
    }

    /**
     * @notice Batch execute multiple DCA plans
     */
    function executeDCABatch(
        uint256[] calldata planIds,
        uint256[] calldata amountsIn,
        uint256[] calldata amountsOut
    ) external returns (uint256[] memory executed, uint256 totalOut) {
        require(planIds.length == amountsIn.length, "Length mismatch");
        require(planIds.length == amountsOut.length, "Length mismatch");
        
        executed = new uint256[](planIds.length);
        totalOut = 0;
        
        for (uint256 i = 0; i < planIds.length; i++) {
            try this.executeDCAPlan(planIds[i], amountsIn[i], amountsOut[i]) {
                executed[i] = planIds[i];
                totalOut += amountsOut[i];
            } catch {
                // Skip failed
            }
        }
        
        return (executed, totalOut);
    }

    // ============ Queries ============

    function getDCAPlan(address user, uint256 planId) external view returns (DCAPlan memory) {
        return dcaPlans[user][planId];
    }

    function getUserDCAPlans(address user) external view returns (DCAPlan[] memory) {
        uint256[] storage planIds = userPlanIds[user];
        DCAPlan[] memory plans = new DCAPlan[](planIds.length);
        
        for (uint256 i = 0; i < planIds.length; i++) {
            plans[i] = dcaPlans[user][planIds[i]];
        }
        
        return plans;
    }

    function getExecutablePlans() external view returns (uint256[] memory) {
        return executablePlanIds;
    }

    function getDCARecords(uint256 planId) external view returns (DCARecord[] memory) {
        return dcaRecords[planId];
    }

    // ============ Internal ============

    function _planId(address user) internal pure returns (uint256) {
        return uint256(uint160(user)) * 1e10;
    }

    function _getPrice(address tokenIn, address tokenOut) internal view returns (uint256) {
        address oracle = priceOracles[tokenIn];
        if (oracle == address(0)) {
            return 1e8;
        }
        return 1e8;
    }

    function _removeExecutablePlan(uint256 planId) internal {
        for (uint256 i = 0; i < executablePlanIds.length; i++) {
            if (executablePlanIds[i] == planId) {
                executablePlanIds[i] = executablePlanIds[executablePlanIds.length - 1];
                executablePlanIds.pop();
                break;
            }
        }
    }

    // ============ Admin ============

    function setPriceOracle(address token, address oracle) external onlyOwner {
        priceOracles[token] = oracle;
    }

    function setProtocolFee(uint256 _feeBps) external onlyOwner {
        require(_feeBps <= 100, "Fee too high");
        protocolFeeBps = _feeBps;
    }

    function withdrawFees(address token, address to, uint256 amount) external onlyOwner {
        require(to != address(0), "Invalid recipient");
        if (token == address(0)) {
            payable(to).transfer(amount);
        } else {
            IERC20(token).safeTransfer(to, amount);
        }
    }

    receive() external payable {}
}