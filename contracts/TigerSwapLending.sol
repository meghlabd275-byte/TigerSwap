// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";

/**
 * @title TigerSwapLending
 * @notice Isolated-pool money market: suppliers earn interest, borrowers post
 *         collateral and pay a variable rate. Interest accrues per second from a
 *         kinked utilisation curve, and under-collateralised debt is liquidated
 *         against real collateral with a bonus paid to the liquidator.
 */
contract TigerSwapLending {
    uint256 public constant WAD = 1e18;

    struct Market {
        address token;
        uint256 totalDeposits;        // supplied principal, scaled by depositIndex
        uint256 totalBorrows;         // outstanding principal, scaled by borrowIndex
        uint256 borrowIndex;          // cumulative borrow interest, WAD, starts at 1e18
        uint256 depositIndex;         // cumulative supply interest, WAD, starts at 1e18
        uint256 baseRate;             // per-second rate at 0% utilisation, WAD
        uint256 slope1;               // per-second additional rate at the kink, WAD
        uint256 slope2;               // per-second additional rate kink -> 100%, WAD
        uint256 kink;                 // utilisation where the curve bends, WAD
        uint256 collateralFactor;     // max borrow power, WAD
        uint256 liquidationThreshold; // health threshold, WAD
        uint256 liquidationBonus;     // extra collateral paid to liquidators, WAD
        uint256 lastAccrual;
        uint256 reserveFactor;        // protocol share of interest, WAD
        uint256 totalReserves;
        bool isActive;
    }

    /// @notice Index-scaled balance so interest compounds without per-user loops.
    struct Account {
        mapping(address => uint256) deposits;
        mapping(address => uint256) borrows;
    }

    address[] private _tokens;

    mapping(address => Market) public markets;
    mapping(address => Account) private accounts;
    address public owner;
    address public priceOracle;

    event MarketCreated(address indexed token, uint256 collateralFactor);
    event MarketUpdated(address indexed token);
    event Deposited(address indexed user, address indexed token, uint256 amount);
    event Withdrawn(address indexed user, address indexed token, uint256 amount);
    event Borrowed(address indexed user, address indexed token, uint256 amount);
    event Repaid(address indexed user, address indexed token, uint256 amount);
    event Liquidated(
        address indexed liquidator,
        address indexed user,
        address indexed debtToken,
        address collateralToken,
        uint256 debtRepaid,
        uint256 collateralSeized
    );
    event ReservesWithdrawn(address indexed token, uint256 amount);

    modifier onlyOwner() {
        require(msg.sender == owner, "TigerSwap: NOT_OWNER");
        _;
    }

    constructor(address _priceOracle) {
        require(_priceOracle != address(0), "TigerSwap: ZERO_ORACLE");
        owner = msg.sender;
        priceOracle = _priceOracle;
    }

    // ============ Admin ============

    function createMarket(
        address token,
        uint256 collateralFactor,
        uint256 liquidationThreshold,
        uint256 baseRate,
        uint256 slope1,
        uint256 slope2,
        uint256 kink,
        uint256 liquidationBonus
    ) external onlyOwner {
        require(token != address(0), "TigerSwap: ZERO_TOKEN");
        require(!markets[token].isActive, "TigerSwap: MARKET_EXISTS");
        require(collateralFactor <= WAD, "TigerSwap: CF_TOO_HIGH");
        require(liquidationThreshold >= collateralFactor && liquidationThreshold <= WAD, "TigerSwap: LT_INVALID");
        require(kink > 0 && kink < WAD, "TigerSwap: KINK_INVALID");

        markets[token] = Market({
            token: token,
            totalDeposits: 0,
            totalBorrows: 0,
            borrowIndex: WAD,
            depositIndex: WAD,
            baseRate: baseRate,
            slope1: slope1,
            slope2: slope2,
            kink: kink,
            collateralFactor: collateralFactor,
            liquidationThreshold: liquidationThreshold,
            liquidationBonus: liquidationBonus,
            lastAccrual: block.timestamp,
            reserveFactor: 0.1e18,
            totalReserves: 0,
            isActive: true
        });

        _tokens.push(token);
        emit MarketCreated(token, collateralFactor);
    }

    function setMarketParams(
        address token,
        uint256 collateralFactor,
        uint256 liquidationThreshold,
        uint256 liquidationBonus
    ) external onlyOwner {
        Market storage m = markets[token];
        require(m.isActive, "TigerSwap: MARKET_NOT_ACTIVE");
        require(liquidationThreshold >= collateralFactor && liquidationThreshold <= WAD, "TigerSwap: LT_INVALID");

        m.collateralFactor = collateralFactor;
        m.liquidationThreshold = liquidationThreshold;
        m.liquidationBonus = liquidationBonus;
        emit MarketUpdated(token);
    }

    function setReserveFactor(address token, uint256 reserveFactor) external onlyOwner {
        require(reserveFactor <= WAD, "TigerSwap: FACTOR_TOO_HIGH");
        markets[token].reserveFactor = reserveFactor;
    }

    function setPriceOracle(address _priceOracle) external onlyOwner {
        require(_priceOracle != address(0), "TigerSwap: ZERO_ORACLE");
        priceOracle = _priceOracle;
    }

    function withdrawReserves(address token, address to) external onlyOwner {
        Market storage m = markets[token];
        _accrue(m);
        uint256 amount = m.totalReserves;
        require(amount > 0, "TigerSwap: NO_RESERVES");
        m.totalReserves = 0;
        require(IERC20(token).transfer(to, amount), "TigerSwap: TRANSFER_FAILED");
        emit ReservesWithdrawn(token, amount);
    }

    // ============ Supply ============

    function deposit(address token, uint256 amount) external {
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");
        Market storage m = markets[token];
        require(m.isActive, "TigerSwap: MARKET_NOT_ACTIVE");
        _accrue(m);

        require(IERC20(token).transferFrom(msg.sender, address(this), amount), "TigerSwap: TRANSFER_FAILED");

        uint256 shares = amount * WAD / m.depositIndex;
        accounts[msg.sender].deposits[token] += shares;
        m.totalDeposits += shares;

        emit Deposited(msg.sender, token, amount);
    }

    function withdraw(address token, uint256 amount) external {
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");
        Market storage m = markets[token];
        _accrue(m);

        uint256 shares = _sharesForAmount(m.depositIndex, amount);
        require(accounts[msg.sender].deposits[token] >= shares, "TigerSwap: INSUFFICIENT_DEPOSIT");

        accounts[msg.sender].deposits[token] -= shares;
        m.totalDeposits -= shares;

        // A withdrawal must not push the account into liquidation.
        require(_isHealthy(msg.sender), "TigerSwap: WITHDRAW_WOULD_LIQUIDATE");

        require(IERC20(token).transfer(msg.sender, amount), "TigerSwap: TRANSFER_FAILED");
        emit Withdrawn(msg.sender, token, amount);
    }

    // ============ Borrow ============

    function borrow(address token, uint256 amount) external {
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");
        Market storage m = markets[token];
        require(m.isActive, "TigerSwap: MARKET_NOT_ACTIVE");
        _accrue(m);

        uint256 available = m.totalDeposits * m.depositIndex / WAD - m.totalBorrows * m.borrowIndex / WAD;
        require(available >= amount, "TigerSwap: INSUFFICIENT_LIQUIDITY");

        uint256 shares = _sharesForAmount(m.borrowIndex, amount);
        accounts[msg.sender].borrows[token] += shares;
        m.totalBorrows += shares;

        require(_isHealthy(msg.sender), "TigerSwap: INSUFFICIENT_COLLATERAL");

        require(IERC20(token).transfer(msg.sender, amount), "TigerSwap: TRANSFER_FAILED");
        emit Borrowed(msg.sender, token, amount);
    }

    function repay(address token, uint256 amount) external {
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");
        Market storage m = markets[token];
        _accrue(m);

        uint256 owed = _amountForShares(m.borrowIndex, accounts[msg.sender].borrows[token]);
        uint256 payAmount = amount > owed ? owed : amount;
        require(payAmount > 0, "TigerSwap: NO_DEBT");

        require(IERC20(token).transferFrom(msg.sender, address(this), payAmount), "TigerSwap: TRANSFER_FAILED");

        uint256 shares = _sharesForAmount(m.borrowIndex, payAmount);
        accounts[msg.sender].borrows[token] -= shares;
        m.totalBorrows -= shares;

        emit Repaid(msg.sender, token, payAmount);
    }

    // ============ Liquidation ============

    /// @notice Repay part of an unhealthy account's debt and seize collateral
    ///         plus a bonus. The liquidator funds the repayment.
    function liquidate(
        address user,
        address debtToken,
        address collateralToken,
        uint256 repayAmount
    ) external {
        require(repayAmount > 0, "TigerSwap: ZERO_AMOUNT");
        require(user != msg.sender, "TigerSwap: SELF_LIQUIDATE");
        require(debtToken != collateralToken, "TigerSwap: SAME_TOKEN");

        Market storage debtMarket = markets[debtToken];
        Market storage collMarket = markets[collateralToken];
        require(debtMarket.isActive && collMarket.isActive, "TigerSwap: MARKET_NOT_ACTIVE");
        _accrue(debtMarket);
        _accrue(collMarket);

        require(!_isHealthy(user), "TigerSwap: HEALTHY");

        uint256 debtOwed = _amountForShares(debtMarket.borrowIndex, accounts[user].borrows[debtToken]);
        require(debtOwed > 0, "TigerSwap: NO_DEBT");

        // Close factor caps a single liquidation at half the debt.
        uint256 maxRepay = debtOwed / 2;
        uint256 actualRepay = repayAmount > maxRepay ? maxRepay : repayAmount;
        require(actualRepay > 0, "TigerSwap: ZERO_AMOUNT");

        uint256 collPrice = _price(collateralToken);
        uint256 debtPrice = _price(debtToken);
        uint256 collateralValue = actualRepay * debtPrice / WAD;
        uint256 seizeValue = collateralValue * (WAD + collMarket.liquidationBonus) / WAD;
        uint256 collateralSeized = seizeValue * WAD / collPrice;

        uint256 userCollateral = _amountForShares(
            collMarket.depositIndex,
            accounts[user].deposits[collateralToken]
        );
        require(collateralSeized <= userCollateral, "TigerSwap: INSUFFICIENT_COLLATERAL");

        require(IERC20(debtToken).transferFrom(msg.sender, address(this), actualRepay), "TigerSwap: TRANSFER_FAILED");

        uint256 repayShares = _sharesForAmount(debtMarket.borrowIndex, actualRepay);
        accounts[user].borrows[debtToken] -= repayShares;
        debtMarket.totalBorrows -= repayShares;

        uint256 seizeShares = _sharesForAmount(collMarket.depositIndex, collateralSeized);
        accounts[user].deposits[collateralToken] -= seizeShares;
        collMarket.totalDeposits -= seizeShares;

        require(IERC20(collateralToken).transfer(msg.sender, collateralSeized), "TigerSwap: TRANSFER_FAILED");

        emit Liquidated(msg.sender, user, debtToken, collateralToken, actualRepay, collateralSeized);
    }

    // ============ Views ============

    /// @notice Health factor scaled by WAD; below 1e18 means liquidatable.
    function getAccountHealth(address user) external view returns (uint256) {
        (uint256 collateralValue, uint256 debtValue) = _accountValues(user);
        if (debtValue == 0) return type(uint256).max;
        return collateralValue * WAD / debtValue;
    }

    function getAccountLiquidity(address user) external view returns (uint256 collateralValue, uint256 debtValue) {
        return _accountValues(user);
    }

    function getBorrowBalance(address user, address token) external view returns (uint256) {
        return _amountForShares(_projectedBorrowIndex(markets[token]), accounts[user].borrows[token]);
    }

    function getDepositBalance(address user, address token) external view returns (uint256) {
        return _amountForShares(_projectedDepositIndex(markets[token]), accounts[user].deposits[token]);
    }

    function getUtilisation(address token) public view returns (uint256) {
        Market storage m = markets[token];
        uint256 deposits = m.totalDeposits * m.depositIndex / WAD;
        uint256 borrows = m.totalBorrows * m.borrowIndex / WAD;
        if (deposits == 0) return 0;
        return borrows * WAD / deposits;
    }

    /// @notice Current per-second borrow rate at the present utilisation.
    function getBorrowRate(address token) public view returns (uint256) {
        return _borrowRate(markets[token], getUtilisation(token));
    }

    /// @notice Current per-second supply rate at the present utilisation.
    function getSupplyRate(address token) public view returns (uint256) {
        Market storage m = markets[token];
        uint256 util = getUtilisation(token);
        return _borrowRate(m, util) * util / WAD * (WAD - m.reserveFactor) / WAD;
    }

    function getMarketInfo(address token) external view returns (
        uint256 totalDeposits,
        uint256 totalBorrows,
        uint256 borrowIndex,
        uint256 depositIndex,
        uint256 borrowRate,
        uint256 supplyRate,
        uint256 utilisation,
        bool isActive
    ) {
        Market storage m = markets[token];
        uint256 util = getUtilisation(token);
        uint256 currentBorrowRate = _borrowRate(m, util);
        return (
            m.totalDeposits * _projectedDepositIndex(m) / WAD,
            m.totalBorrows * _projectedBorrowIndex(m) / WAD,
            _projectedBorrowIndex(m),
            _projectedDepositIndex(m),
            currentBorrowRate,
            currentBorrowRate * util / WAD * (WAD - m.reserveFactor) / WAD,
            util,
            m.isActive
        );
    }

    /// @notice Every market this deployment serves, for indexers and health checks.
    function marketList() external view returns (address[] memory) {
        return _tokens;
    }

    // ============ Internals ============

    function _accrue(Market storage m) internal {
        if (m.lastAccrual == 0) {
            m.lastAccrual = block.timestamp;
            return;
        }
        uint256 elapsed = block.timestamp - m.lastAccrual;
        if (elapsed == 0) return;

        uint256 deposits = m.totalDeposits * m.depositIndex / WAD;
        uint256 borrows = m.totalBorrows * m.borrowIndex / WAD;

        if (borrows > 0 && deposits > 0) {
            uint256 util = borrows * WAD / deposits;
            uint256 interest = borrows * _borrowRate(m, util) * elapsed / WAD;

            if (interest > 0) {
                uint256 toReserves = interest * m.reserveFactor / WAD;
                m.totalReserves += toReserves;

                uint256 supplyInterest = interest - toReserves;
                m.borrowIndex += m.borrowIndex * interest / borrows;
                m.totalBorrows = (borrows + interest) * WAD / m.borrowIndex;
                m.depositIndex += m.depositIndex * supplyInterest / deposits;
                m.totalDeposits = (deposits + supplyInterest) * WAD / m.depositIndex;
            }
        }

        m.lastAccrual = block.timestamp;
    }

    function _borrowRate(Market storage m, uint256 util) internal view returns (uint256) {
        if (util <= m.kink) {
            return m.baseRate + m.slope1 * util / WAD;
        }
        uint256 excess = util - m.kink;
        return m.baseRate + m.slope1 * m.kink / WAD + m.slope2 * excess / (WAD - m.kink);
    }

    function _projectedBorrowIndex(Market storage m) internal view returns (uint256) {
        if (m.lastAccrual == 0) return m.borrowIndex;
        uint256 borrows = m.totalBorrows * m.borrowIndex / WAD;
        uint256 deposits = m.totalDeposits * m.depositIndex / WAD;
        if (borrows == 0 || deposits == 0) return m.borrowIndex;

        uint256 interest = borrows * _borrowRate(m, borrows * WAD / deposits)
            * (block.timestamp - m.lastAccrual) / WAD;
        return m.borrowIndex + m.borrowIndex * interest / borrows;
    }

    function _projectedDepositIndex(Market storage m) internal view returns (uint256) {
        if (m.lastAccrual == 0) return m.depositIndex;
        uint256 borrows = m.totalBorrows * m.borrowIndex / WAD;
        uint256 deposits = m.totalDeposits * m.depositIndex / WAD;
        if (borrows == 0 || deposits == 0) return m.depositIndex;

        uint256 interest = borrows * _borrowRate(m, borrows * WAD / deposits)
            * (block.timestamp - m.lastAccrual) / WAD;
        uint256 supplyInterest = interest * (WAD - m.reserveFactor) / WAD;
        return m.depositIndex + m.depositIndex * supplyInterest / deposits;
    }

    function _accountValues(address user)
        internal view returns (uint256 collateralValue, uint256 debtValue)
    {
        uint256 len = _tokens.length;
        for (uint256 i = 0; i < len; i++) {
            address token = _tokens[i];
            Market storage m = markets[token];

            uint256 borrowShares = accounts[user].borrows[token];
            if (borrowShares > 0) {
                uint256 amount = _amountForShares(_projectedBorrowIndex(m), borrowShares);
                debtValue += amount * _price(token) / WAD;
            }

            uint256 depositShares = accounts[user].deposits[token];
            if (depositShares > 0) {
                uint256 amount = _amountForShares(_projectedDepositIndex(m), depositShares);
                collateralValue += amount * _price(token) / WAD * m.liquidationThreshold / WAD;
            }
        }
    }

    function _isHealthy(address user) internal view returns (bool) {
        (uint256 collateralValue, uint256 debtValue) = _accountValues(user);
        return collateralValue >= debtValue;
    }

    function _price(address token) internal view returns (uint256) {
        uint256 p = IPriceOracle(priceOracle).getPrice(token);
        require(p > 0, "TigerSwap: NO_PRICE");
        return p;
    }

    function _sharesForAmount(uint256 index, uint256 amount) internal pure returns (uint256) {
        return amount * WAD / index;
    }

    function _amountForShares(uint256 index, uint256 shares) internal pure returns (uint256) {
        return shares * index / WAD;
    }
}

interface IPriceOracle {
    function getPrice(address token) external view returns (uint256);
}
