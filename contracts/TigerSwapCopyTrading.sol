// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerSwapCopyTrading
 * @notice Copy trading contract for following expert traders
 */
contract TigerSwapCopyTrading {
    // Struct for trader info
    struct Trader {
        address traderAddress;
        string name;
        uint256 totalProfit;
        uint256 totalTrades;
        uint256 followersCount;
        bool isActive;
        uint256 performanceFee; // Percentage of profits
        uint256 minFollowAmount;
    }
    
    // Struct for copy position
    struct CopyPosition {
        bytes32 positionId;
        address follower;
        address trader;
        bytes32 originalPositionId;
        uint256 amount;
        uint256 entryPrice;
        bool isLong;
        bool isClosed;
        uint256 profit;
    }
    
    // State variables
    address public owner;
    address public paymentToken;
    mapping(address => Trader) public traders;
    mapping(address => address[]) public followerList; // trader => followers
    mapping(address => mapping(address => bool)) public isFollowing; // follower => trader => bool
    mapping(bytes32 => CopyPosition) public copyPositions;
    bytes32[] public allCopyPositions;
    
    // Fees
    uint256 public platformFee = 50; // 0.5%
    
    // Events
    event TraderRegistered(address indexed trader, string name, uint256 performanceFee);
    event TraderUpdated(address indexed trader, string name, uint256 performanceFee);
    event FollowerAdded(address indexed trader, address indexed follower, uint256 amount);
    event FollowerRemoved(address indexed trader, address indexed follower);
    event CopyTradeOpened(bytes32 indexed copyId, address indexed follower, address indexed trader, uint256 amount);
    event CopyTradeClosed(bytes32 indexed copyId, address indexed follower, uint256 profit);
    event FeesPaid(address indexed trader, address indexed follower, uint256 amount);

    modifier onlyOwner() {
        require(msg.sender == owner, "TigerSwap: NOT_OWNER");
        _;
    }

    constructor(address _paymentToken) {
        owner = msg.sender;
        paymentToken = _paymentToken;
    }

    /**
     * @notice Register as a trader
     * @param name Trader name
     * @param performanceFee Performance fee percentage (e.g., 1000 = 10%)
     */
    function registerTrader(string memory name, uint256 performanceFee) external {
        require(!traders[msg.sender].isActive, "TigerSwap: ALREADY_REGISTERED");
        require(performanceFee <= 3000, "TigerSwap: FEE_TOO_HIGH"); // Max 30%
        
        traders[msg.sender] = Trader({
            traderAddress: msg.sender,
            name: name,
            totalProfit: 0,
            totalTrades: 0,
            followersCount: 0,
            isActive: true,
            performanceFee: performanceFee,
            minFollowAmount: 100e18
        });
        
        emit TraderRegistered(msg.sender, name, performanceFee);
    }

    /**
     * @notice Update trader info
     * @param name New name
     * @param performanceFee New performance fee
     */
    function updateTrader(string memory name, uint256 performanceFee) external {
        Trader storage trader = traders[msg.sender];
        require(trader.isActive, "TigerSwap: NOT_REGISTERED");
        
        trader.name = name;
        trader.performanceFee = performanceFee;
        
        emit TraderUpdated(msg.sender, name, performanceFee);
    }

    /**
     * @notice Follow a trader
     * @param trader Trader address
     * @param amount Amount to follow with
     */
    function followTrader(address trader, uint256 amount) external {
        Trader memory traderInfo = traders[trader];
        require(traderInfo.isActive, "TigerSwap: TRADER_NOT_ACTIVE");
        require(!isFollowing[msg.sender][trader], "TigerSwap: ALREADY_FOLLOWING");
        require(amount >= traderInfo.minFollowAmount, "TigerSwap: AMOUNT_TOO_LOW");
        
        // Transfer tokens
        IERC20(paymentToken).transferFrom(msg.sender, address(this), amount);
        
        // Update mappings
        isFollowing[msg.sender][trader] = true;
        followerList[trader].push(msg.sender);
        traders[trader].followersCount++;
        
        emit FollowerAdded(trader, msg.sender, amount);
    }

    /**
     * @notice Unfollow a trader
     * @param trader Trader address
     */
    function unfollowTrader(address trader) external {
        require(isFollowing[msg.sender][trader], "TigerSwap: NOT_FOLLOWING");
        
        isFollowing[msg.sender][trader] = false;
        
        // Remove from follower list
        address[] storage followers = followerList[trader];
        for (uint256 i = 0; i < followers.length; i++) {
            if (followers[i] == msg.sender) {
                followers[i] = followers[followers.length - 1];
                followers.pop();
                break;
            }
        }
        
        traders[trader].followersCount--;
        
        emit FollowerRemoved(trader, msg.sender);
    }

    /**
     * @notice Open copy trade (called by system when trader opens position)
     * @param follower Follower address
     * @param trader Trader address
     * @param originalPositionId Original position ID
     * @param amount Amount to copy
     * @param entryPrice Entry price
     * @param isLong Position direction
     */
    function openCopyTrade(
        address follower,
        address trader,
        bytes32 originalPositionId,
        uint256 amount,
        uint256 entryPrice,
        bool isLong
    ) external returns (bytes32 copyId) {
        require(isFollowing[follower][trader], "TigerSwap: NOT_FOLLOWING");
        
        // Create copy position
        copyId = keccak256(abi.encodePacked(follower, trader, originalPositionId, block.timestamp));
        
        copyPositions[copyId] = CopyPosition({
            positionId: copyId,
            follower: follower,
            trader: trader,
            originalPositionId: originalPositionId,
            amount: amount,
            entryPrice: entryPrice,
            isLong: isLong,
            isClosed: false,
            profit: 0
        });
        
        allCopyPositions.push(copyId);
        
        emit CopyTradeOpened(copyId, follower, trader, amount);
    }

    /**
     * @notice Close copy trade
     * @param copyId Copy position ID
     * @param exitPrice Exit price
     */
    function closeCopyTrade(bytes32 copyId, uint256 exitPrice) external {
        CopyPosition storage copyPos = copyPositions[copyId];
        require(!copyPos.isClosed, "TigerSwap: ALREADY_CLOSED");
        require(copyPos.follower == msg.sender, "TigerSwap: NOT_FOLLOWER");
        
        // Calculate profit
        uint256 profit;
        if (copyPos.isLong) {
            if (exitPrice > copyPos.entryPrice) {
                profit = (copyPos.amount * (exitPrice - copyPos.entryPrice)) / copyPos.entryPrice;
            }
        } else {
            if (exitPrice < copyPos.entryPrice) {
                profit = (copyPos.amount * (copyPos.entryPrice - exitPrice)) / copyPos.entryPrice;
            }
        }
        
        copyPos.isClosed = true;
        copyPos.profit = profit;
        
        // Distribute profits
        if (profit > 0) {
            Trader storage traderInfo = traders[copyPos.trader];
            
            // Calculate fees
            uint256 traderFee = (profit * traderInfo.performanceFee) / 10000;
            uint256 platformFeeAmount = (profit * platformFee) / 10000;
            uint256 followerProfit = profit - traderFee - platformFeeAmount;
            
            // Transfer profits
            if (followerProfit > 0) {
                IERC20(paymentToken).transfer(copyPos.follower, followerProfit);
            }
            if (traderFee > 0) {
                IERC20(paymentToken).transfer(copyPos.trader, traderFee);
            }
            
            // Update trader stats
            traderInfo.totalProfit += profit;
            traderInfo.totalTrades++;
            
            emit FeesPaid(copyPos.trader, copyPos.follower, traderFee);
        }
        
        emit CopyTradeClosed(copyId, copyPos.follower, profit);
    }

    /**
     * @notice Get traders by performance
     * @param start Start index
     * @param count Number of traders to return
     */
    function getTopTraders(uint256 start, uint256 count) external view returns (Trader[] memory) {
        // This would ideally use a sorted data structure
        // For simplicity, returning empty array
        Trader[] memory result = new Trader[](count);
        return result;
    }

    /**
     * @notice Get follower count for trader
     */
    function getFollowerCount(address trader) external view returns (uint256) {
        return traders[trader].followersCount;
    }

    /**
     * @notice Get copy positions for follower
     */
    function getCopyPositions(address follower) external view returns (CopyPosition[] memory) {
        uint256 count = 0;
        for (uint256 i = 0; i < allCopyPositions.length; i++) {
            if (copyPositions[allCopyPositions[i]].follower == follower) {
                count++;
            }
        }
        
        CopyPosition[] memory result = new CopyPosition[](count);
        uint256 index = 0;
        for (uint256 i = 0; i < allCopyPositions.length; i++) {
            if (copyPositions[allCopyPositions[i]].follower == follower) {
                result[index] = copyPositions[allCopyPositions[i]];
                index++;
            }
        }
        
        return result;
    }

    // Admin functions

    function setPlatformFee(uint256 _platformFee) external onlyOwner {
        require(_platformFee <= 500, "TigerSwap: FEE_TOO_HIGH");
        platformFee = _platformFee;
    }

    function transferOwnership(address newOwner) external onlyOwner {
        owner = newOwner;
    }
}

interface IERC20 {
    function transfer(address to, uint256 value) external returns (bool);
    function transferFrom(address from, address to, uint256 value) external returns (bool);
    function balanceOf(address account) external view returns (uint256);
}
