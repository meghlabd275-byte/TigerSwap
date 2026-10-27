// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";

/// @dev Median price oracle used to price copy positions.
interface ITigerSwapPriceOracle {
    function getPrice(address asset) external view returns (uint256);
}

/**
 * @title TigerSwapCopyTrading
 * @notice Lets a follower mirror a registered trader's positions.
 *
 * The follower's funds stay theirs. Each follower has a balance of the payment
 * token held by this contract, and only the follower can spend it. Copying a
 * trade moves part of that balance into a per-position escrow; closing the
 * position returns the escrow plus any profit, or the reduced escrow after a
 * loss. Nothing a follower deposits can ever be paid to another follower.
 *
 * Two details make the accounting safe:
 *   - Escrow is capped by the follower's free balance, so a copy position can
 *     never be opened against money that belongs to someone else.
 *   - A position's entry price comes from the price oracle, not from the
 *     caller, so a copy position cannot be opened at a fabricated price and
 *     then closed for an unearned profit.
 */
contract TigerSwapCopyTrading {
    struct Trader {
        address traderAddress;
        string name;
        uint256 totalProfit;
        uint256 totalTrades;
        uint256 followersCount;
        bool isActive;
        uint256 performanceFee; // basis points of profit, max 3000
        uint256 minFollowAmount;
    }

    struct CopyPosition {
        bytes32 positionId;
        address follower;
        address trader;
        bytes32 originalPositionId;
        uint256 amount; // escrowed principal for this position
        uint256 entryPrice;
        bool isLong;
        bool isClosed;
        uint256 profit;
    }

    uint256 public constant BPS_DENOMINATOR = 10_000;
    uint256 public constant MAX_PERFORMANCE_FEE = 3000;
    uint256 public constant MAX_PLATFORM_FEE = 500;

    address public owner;
    address public futureOwner;
    address public paymentToken;
    /// @notice Median oracle used to price copy positions.
    address public priceOracle;
    /// @notice Asset the oracle is queried for.
    address public priceAsset;
    /// @notice Executor that may open copy positions on a follower's behalf.
    address public executor;

    mapping(address => Trader) public traders;
    mapping(address => address[]) public followerList;
    mapping(address => mapping(address => bool)) public isFollowing;
    mapping(bytes32 => CopyPosition) public copyPositions;
    bytes32[] public allCopyPositions;

    /// @notice Follower => token held by this contract and owned by them.
    mapping(address => mapping(address => uint256)) public balances;
    /// @notice Follower => total escrowed across their open copy positions.
    mapping(address => mapping(address => uint256)) public escrowed;

    uint256 public platformFee = 50; // 0.5%
    /// @notice Platform fees accrued and not yet withdrawn.
    uint256 public platformFeesStored;

    event TraderRegistered(address indexed trader, string name, uint256 performanceFee);
    event TraderUpdated(address indexed trader, string name, uint256 performanceFee);
    event FollowerAdded(address indexed trader, address indexed follower, uint256 amount);
    event FollowerRemoved(address indexed trader, address indexed follower);
    event Deposited(address indexed follower, uint256 amount);
    event Withdrawn(address indexed follower, uint256 amount);
    event CopyTradeOpened(
        bytes32 indexed copyId, address indexed follower, address indexed trader, uint256 amount, uint256 entryPrice
    );
    event CopyTradeClosed(bytes32 indexed copyId, address indexed follower, uint256 profit, uint256 loss);
    event FeesPaid(address indexed trader, address indexed follower, uint256 amount);
    event PlatformFeesWithdrawn(address indexed to, uint256 amount);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    modifier onlyOwner() {
        require(msg.sender == owner, "TigerSwap: NOT_OWNER");
        _;
    }

    modifier onlyExecutor() {
        require(msg.sender == executor || msg.sender == owner, "TigerSwap: NOT_EXECUTOR");
        _;
    }

    uint256 private _locked = 1;

    modifier nonReentrant() {
        require(_locked == 1, "TigerSwap: REENTRANT");
        _locked = 2;
        _;
        _locked = 1;
    }

    constructor(address _paymentToken, address _priceOracle) {
        require(_paymentToken != address(0), "TigerSwap: ZERO_TOKEN");
        require(_priceOracle != address(0), "TigerSwap: ZERO_ORACLE");
        owner = msg.sender;
        paymentToken = _paymentToken;
        priceOracle = _priceOracle;
        executor = msg.sender;
        emit OwnershipTransferred(address(0), msg.sender);
    }

    // ============ Admin ============

    function setExecutor(address _executor) external onlyOwner {
        require(_executor != address(0), "TigerSwap: ZERO_ADDRESS");
        executor = _executor;
    }

    function setPriceOracle(address oracle) external onlyOwner {
        require(oracle != address(0), "TigerSwap: ZERO_ORACLE");
        priceOracle = oracle;
    }

    function setPriceAsset(address asset) external onlyOwner {
        require(asset != address(0), "TigerSwap: ZERO_ADDRESS");
        priceAsset = asset;
    }

    function setPlatformFee(uint256 _platformFee) external onlyOwner {
        require(_platformFee <= MAX_PLATFORM_FEE, "TigerSwap: FEE_TOO_HIGH");
        platformFee = _platformFee;
    }

    function commitTransferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "TigerSwap: ZERO_ADDRESS");
        futureOwner = newOwner;
    }

    function acceptTransferOwnership() external {
        require(msg.sender == futureOwner, "TigerSwap: NOT_FUTURE_OWNER");
        emit OwnershipTransferred(owner, futureOwner);
        owner = futureOwner;
        futureOwner = address(0);
    }

    function withdrawPlatformFees(address to, uint256 amount) external onlyOwner nonReentrant {
        require(to != address(0), "TigerSwap: ZERO_ADDRESS");
        require(amount > 0 && amount <= platformFeesStored, "TigerSwap: INSUFFICIENT_FEES");

        platformFeesStored -= amount;
        require(IERC20(paymentToken).transfer(to, amount), "TigerSwap: TRANSFER_FAILED");

        emit PlatformFeesWithdrawn(to, amount);
    }

    // ============ Trader registry ============

    function registerTrader(string memory name, uint256 performanceFee) external {
        require(!traders[msg.sender].isActive, "TigerSwap: ALREADY_REGISTERED");
        require(performanceFee <= MAX_PERFORMANCE_FEE, "TigerSwap: FEE_TOO_HIGH");

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

    function updateTrader(string memory name, uint256 performanceFee) external {
        Trader storage trader = traders[msg.sender];
        require(trader.isActive, "TigerSwap: NOT_REGISTERED");
        // The cap has to be enforced here too, otherwise a trader could raise
        // their fee to 100% after followers have already committed funds.
        require(performanceFee <= MAX_PERFORMANCE_FEE, "TigerSwap: FEE_TOO_HIGH");

        trader.name = name;
        trader.performanceFee = performanceFee;

        emit TraderUpdated(msg.sender, name, performanceFee);
    }

    function setMinFollowAmount(uint256 amount) external {
        Trader storage trader = traders[msg.sender];
        require(trader.isActive, "TigerSwap: NOT_REGISTERED");
        trader.minFollowAmount = amount;
    }

    // ============ Follower funds ============

    /// @notice Deposit the payment token. The balance is the follower's own.
    function deposit(uint256 amount) external nonReentrant {
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");
        require(IERC20(paymentToken).transferFrom(msg.sender, address(this), amount), "TigerSwap: TRANSFER_FAILED");

        balances[msg.sender][paymentToken] += amount;
        emit Deposited(msg.sender, amount);
    }

    /// @notice Withdraw any balance not currently escrowed in an open position.
    function withdraw(uint256 amount) external nonReentrant {
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");
        uint256 free = _freeBalance(msg.sender);
        require(amount <= free, "TigerSwap: INSUFFICIENT_BALANCE");

        balances[msg.sender][paymentToken] -= amount;
        require(IERC20(paymentToken).transfer(msg.sender, amount), "TigerSwap: TRANSFER_FAILED");

        emit Withdrawn(msg.sender, amount);
    }

    /// @notice Follow a trader. Deposits `amount` and records the follow.
    function followTrader(address trader, uint256 amount) external nonReentrant {
        Trader storage traderInfo = traders[trader];
        require(traderInfo.isActive, "TigerSwap: TRADER_NOT_ACTIVE");
        require(!isFollowing[msg.sender][trader], "TigerSwap: ALREADY_FOLLOWING");
        require(amount >= traderInfo.minFollowAmount, "TigerSwap: AMOUNT_TOO_LOW");

        require(IERC20(paymentToken).transferFrom(msg.sender, address(this), amount), "TigerSwap: TRANSFER_FAILED");
        balances[msg.sender][paymentToken] += amount;

        isFollowing[msg.sender][trader] = true;
        followerList[trader].push(msg.sender);
        traderInfo.followersCount++;

        emit Deposited(msg.sender, amount);
        emit FollowerAdded(trader, msg.sender, amount);
    }

    /// @notice Stop following a trader. The follower keeps their balance.
    function unfollowTrader(address trader) external nonReentrant {
        require(isFollowing[msg.sender][trader], "TigerSwap: NOT_FOLLOWING");

        isFollowing[msg.sender][trader] = false;

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

    // ============ Copy positions ============

    /**
     * @notice Escrow part of a follower's balance into a copy position.
     * @dev The entry price is read from the oracle, so a caller cannot pick it.
     *      Called by the executor, or by the follower for themselves.
     */
    function openCopyTrade(address follower, address trader, bytes32 originalPositionId, uint256 amount, bool isLong)
        external
        nonReentrant
        returns (bytes32 copyId)
    {
        require(isFollowing[follower][trader], "TigerSwap: NOT_FOLLOWING");
        require(msg.sender == follower || msg.sender == executor || msg.sender == owner, "TigerSwap: NOT_EXECUTOR");
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");
        require(amount <= _freeBalance(follower), "TigerSwap: INSUFFICIENT_BALANCE");

        uint256 entryPrice = _oraclePrice();

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

        // Move the principal out of the free balance so it cannot be withdrawn
        // or spent on another position while this one is open.
        escrowed[follower][paymentToken] += amount;

        allCopyPositions.push(copyId);

        emit CopyTradeOpened(copyId, follower, trader, amount, entryPrice);
    }

    /**
     * @notice Close a copy position at the oracle price.
     * @dev Only the follower may close. The escrowed principal is settled
     *      against the price move, and the result is credited to the follower's
     *      own balance. Losses can never exceed the escrowed principal.
     */
    function closeCopyTrade(bytes32 copyId) external nonReentrant {
        CopyPosition storage copyPos = copyPositions[copyId];
        require(!copyPos.isClosed, "TigerSwap: ALREADY_CLOSED");
        require(copyPos.follower == msg.sender, "TigerSwap: NOT_FOLLOWER");

        uint256 exitPrice = _oraclePrice();

        uint256 profit;
        uint256 loss;
        if (copyPos.isLong) {
            if (exitPrice > copyPos.entryPrice) {
                profit = copyPos.amount * (exitPrice - copyPos.entryPrice) / copyPos.entryPrice;
            } else if (exitPrice < copyPos.entryPrice) {
                loss = copyPos.amount * (copyPos.entryPrice - exitPrice) / copyPos.entryPrice;
            }
        } else {
            if (exitPrice < copyPos.entryPrice) {
                profit = copyPos.amount * (copyPos.entryPrice - exitPrice) / copyPos.entryPrice;
            } else if (exitPrice > copyPos.entryPrice) {
                loss = copyPos.amount * (exitPrice - copyPos.entryPrice) / copyPos.entryPrice;
            }
        }

        // A position cannot lose more than the principal committed to it.
        if (loss > copyPos.amount) {
            loss = copyPos.amount;
        }

        copyPos.isClosed = true;
        copyPos.profit = profit;

        address follower = copyPos.follower;
        address trader = copyPos.trader;
        uint256 principal = copyPos.amount;

        escrowed[follower][paymentToken] -= principal;

        // `balances` is the follower's total holding and `escrowed` is the part
        // committed to open positions, so closing only settles the P&L: the
        // principal was never removed from the balance when it was escrowed.
        if (profit > 0) {
            Trader storage traderInfo = traders[trader];
            uint256 traderFee = profit * traderInfo.performanceFee / BPS_DENOMINATOR;
            uint256 platformFeeAmount = profit * platformFee / BPS_DENOMINATOR;
            uint256 followerProfit = profit - traderFee - platformFeeAmount;

            balances[follower][paymentToken] += followerProfit;
            balances[trader][paymentToken] += traderFee;
            platformFeesStored += platformFeeAmount;

            traderInfo.totalProfit += profit;
            traderInfo.totalTrades++;

            emit FeesPaid(trader, follower, traderFee);
        } else {
            balances[follower][paymentToken] -= loss;
        }

        emit CopyTradeClosed(copyId, follower, profit, loss);
    }

    // ============ Views ============

    /// @notice Balance the follower may withdraw or commit right now.
    function freeBalance(address follower) external view returns (uint256) {
        return _freeBalance(follower);
    }

    function getFollowerCount(address trader) external view returns (uint256) {
        return traders[trader].followersCount;
    }

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

    /// @notice Top traders by cumulative profit, best first.
    function getTopTraders(address[] calldata candidates, uint256 limit) external view returns (Trader[] memory) {
        uint256 n = candidates.length < limit ? candidates.length : limit;
        Trader[] memory ranked = new Trader[](n);
        bool[] memory used = new bool[](candidates.length);

        for (uint256 k = 0; k < n; k++) {
            uint256 best = type(uint256).max;
            for (uint256 i = 0; i < candidates.length; i++) {
                if (used[i] || !traders[candidates[i]].isActive) continue;
                if (best == type(uint256).max || traders[candidates[i]].totalProfit > traders[candidates[best]].totalProfit)
                {
                    best = i;
                }
            }
            if (best == type(uint256).max) break;
            used[best] = true;
            ranked[k] = traders[candidates[best]];
        }

        return ranked;
    }

    // ============ Internals ============

    function _freeBalance(address follower) private view returns (uint256) {
        uint256 balance = balances[follower][paymentToken];
        uint256 committed = escrowed[follower][paymentToken];
        return balance > committed ? balance - committed : 0;
    }

    function _oraclePrice() private view returns (uint256 price) {
        price = ITigerSwapPriceOracle(priceOracle).getPrice(priceAsset);
        require(price > 0, "TigerSwap: INVALID_PRICE");
    }

}
