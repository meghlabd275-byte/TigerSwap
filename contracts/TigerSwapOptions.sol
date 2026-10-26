// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";
import "./TigerSwapOptionMath.sol";

/**
 * @title TigerSwapOptions
 * @notice Self-contained European option AMM, cash settled.
 *
 * Each option series is a fully collateralised pool. Liquidity providers
 * deposit the settlement asset (RUSD) and are the counterparty to every trade;
 * in return they earn the premiums buyers pay. Buyers get a long position that
 * pays intrinsic value at expiry.
 *
 * Collateral is locked per trade so a series can never sell more exposure than
 * it can settle:
 *   - a call locks size * spot, the cost of delivering the underlying
 *   - a put locks size * strike, the worst case payout
 *
 * Pricing is Black-Scholes from `TigerSwapOptionMath`. Implied volatility is
 * not a fixed constant: it rises with the share of collateral already sold,
 * which is how the pool charges more for the risk it is carrying.
 */
contract TigerSwapOptions {
    using TigerSwapOptionMath for uint256;

    IERC20 public immutable collateralToken;
    address public owner;
    address public oracle;

    uint256 public constant WAD = 1e18;
    /// @dev Cap on implied volatility after skew, 500%.
    uint256 public constant MAX_IV = 5e18;
    /// @dev Skew added to base IV when the pool is fully utilised, 200%.
    uint256 public constant SKEW = 2e18;

    struct Series {
        uint256 strike;
        uint256 expiry;
        bool isCall;
        uint256 baseIV;
        uint256 markPrice;
        uint256 collateral;
        uint256 lockedCollateral;
        uint256 totalLongSize;
        uint256 premiumCollected;
        bool settled;
        uint256 settlementPrice;
    }

    Series[] public series;

    /// @dev Long position size held by each trader per series.
    mapping(uint256 => mapping(address => uint256)) public longPositions;
    /// @dev Premium the pool has set aside for LPs of each series, withdrawable after settlement.
    mapping(uint256 => uint256) public lpClaims;
    /// @dev Share of the claimable pool each LP owns, per series.
    mapping(uint256 => mapping(address => uint256)) public lpShares;
    /// @dev Total shares issued for a series.
    mapping(uint256 => uint256) public totalLpShares;
    /// @dev Principal each LP deposited and has not yet withdrawn, per series.
    mapping(uint256 => mapping(address => uint256)) public lpPrincipal;

    event SeriesCreated(uint256 indexed seriesId, uint256 strike, uint256 expiry, bool isCall, uint256 baseIV);
    event MarkPriceUpdated(uint256 indexed seriesId, uint256 markPrice);
    event CollateralAdded(uint256 indexed seriesId, address indexed provider, uint256 amount);
    event CollateralRemoved(uint256 indexed seriesId, address indexed provider, uint256 amount);
    event OptionBought(
        uint256 indexed seriesId,
        address indexed buyer,
        uint256 size,
        uint256 premium,
        uint256 impliedVolatility,
        uint256 lockedCollateral
    );
    event SeriesSettled(uint256 indexed seriesId, uint256 settlementPrice);
    event Exercised(uint256 indexed seriesId, address indexed holder, uint256 size, uint256 payout);
    event LpWithdrawn(uint256 indexed seriesId, address indexed provider, uint256 amount);
    event OracleUpdated(address indexed oracle);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    modifier onlyOracle() {
        require(msg.sender == oracle || msg.sender == owner, "not oracle");
        _;
    }

    uint256 private _status = 1;

    modifier nonReentrant() {
        require(_status == 1, "reentrant call");
        _status = 2;
        _;
        _status = 1;
    }

    constructor(address _collateralToken) {
        require(_collateralToken != address(0), "zero address");
        collateralToken = IERC20(_collateralToken);
        owner = msg.sender;
        oracle = msg.sender;
        emit OwnershipTransferred(address(0), msg.sender);
        emit OracleUpdated(msg.sender);
    }

    // ---- Series administration --------------------------------------------

    /// @notice Create a new option series. Admin only.
    function createSeries(uint256 _strike, uint256 _expiry, bool _isCall, uint256 _baseIV, uint256 _markPrice)
        external
        onlyOwner
        returns (uint256 seriesId)
    {
        require(_strike > 0, "strike required");
        require(_expiry > block.timestamp, "expiry must be in the future");
        require(_baseIV > 0 && _baseIV <= MAX_IV, "base IV out of range");
        require(_markPrice > 0, "mark price required");

        series.push(
            Series({
                strike: _strike,
                expiry: _expiry,
                isCall: _isCall,
                baseIV: _baseIV,
                markPrice: _markPrice,
                collateral: 0,
                lockedCollateral: 0,
                totalLongSize: 0,
                premiumCollected: 0,
                settled: false,
                settlementPrice: 0
            })
        );
        seriesId = series.length - 1;

        emit SeriesCreated(seriesId, _strike, _expiry, _isCall, _baseIV);
    }

    /// @notice Update the mark price used for pricing and collateral checks.
    function setMarkPrice(uint256 _seriesId, uint256 _markPrice) external onlyOracle {
        Series storage s = _requireSeries(_seriesId);
        require(!s.settled, "series already settled");
        require(_markPrice > 0, "mark price required");
        s.markPrice = _markPrice;
        emit MarkPriceUpdated(_seriesId, _markPrice);
    }

    /// @notice Record the settlement price once the series has expired.
    function settleSeries(uint256 _seriesId, uint256 _settlementPrice) external onlyOracle {
        Series storage s = _requireSeries(_seriesId);
        require(!s.settled, "already settled");
        require(block.timestamp >= s.expiry, "series not expired");
        require(_settlementPrice > 0, "settlement price required");

        s.settled = true;
        s.settlementPrice = _settlementPrice;
        // Everything still held becomes claimable by LPs once holders exercise.
        lpClaims[_seriesId] = s.collateral;
        emit SeriesSettled(_seriesId, _settlementPrice);
    }

    // ---- Liquidity ---------------------------------------------------------

    /// @notice Deposit settlement collateral to underwrite the series.
    /// @dev Deposits are recorded per LP. Shares are priced against the pool's
    ///      free collateral, so a new deposit cannot dilute the principal other
    ///      LPs already have at risk, and an LP can only ever withdraw their own.
    function addCollateral(uint256 _seriesId, uint256 _amount) external nonReentrant {
        Series storage s = _requireSeries(_seriesId);
        require(!s.settled, "series already settled");
        require(_amount > 0, "amount required");

        require(collateralToken.transferFrom(msg.sender, address(this), _amount), "transfer in failed");

        uint256 free = s.collateral > s.lockedCollateral ? s.collateral - s.lockedCollateral : 0;
        // When the pool is fully committed every existing share is momentarily
        // worth nothing, so a new deposit mints at par rather than at an
        // unbounded ratio.
        uint256 shares = (totalLpShares[_seriesId] == 0 || free == 0)
            ? _amount
            : _amount * totalLpShares[_seriesId] / free;
        require(shares > 0, "amount too small");

        s.collateral += _amount;
        totalLpShares[_seriesId] += shares;
        lpShares[_seriesId][msg.sender] += shares;
        lpPrincipal[_seriesId][msg.sender] += _amount;

        emit CollateralAdded(_seriesId, msg.sender, _amount);
    }

    /// @notice Withdraw collateral that is not backing open positions.
    /// @dev Bounded by the caller's own principal and by the pool's free
    ///      collateral, and it burns the matching shares.
    function removeCollateral(uint256 _seriesId, uint256 _amount) external nonReentrant {
        Series storage s = _requireSeries(_seriesId);
        require(!s.settled, "series already settled");
        require(_amount > 0, "amount required");
        require(_amount <= s.collateral - s.lockedCollateral, "collateral is locked");
        require(_amount <= lpPrincipal[_seriesId][msg.sender], "exceeds your principal");

        uint256 free = s.collateral - s.lockedCollateral;
        uint256 burn = _amount * totalLpShares[_seriesId] / free;
        require(burn <= lpShares[_seriesId][msg.sender], "exceeds your share");

        s.collateral -= _amount;
        totalLpShares[_seriesId] -= burn;
        lpShares[_seriesId][msg.sender] -= burn;
        lpPrincipal[_seriesId][msg.sender] -= _amount;
        require(collateralToken.transfer(msg.sender, _amount), "transfer out failed");

        emit CollateralRemoved(_seriesId, msg.sender, _amount);
    }

    // ---- Pricing -----------------------------------------------------------

    /// @notice Implied volatility for the series, skewed by how much of the
    /// collateral has already been sold.
    function impliedVolatility(uint256 _seriesId) public view returns (uint256) {
        Series storage s = _requireSeries(_seriesId);
        if (s.collateral == 0) {
            return s.baseIV;
        }
        uint256 utilization = s.lockedCollateral * WAD / s.collateral;
        if (utilization > WAD) {
            utilization = WAD;
        }
        uint256 iv = s.baseIV + SKEW * utilization / WAD;
        return iv > MAX_IV ? MAX_IV : iv;
    }

    /// @notice Premium for buying `_size` of the series, in collateral units.
    function premium(uint256 _seriesId, uint256 _size) public view returns (uint256) {
        Series storage s = _requireSeries(_seriesId);
        require(!s.settled, "series already settled");
        if (block.timestamp >= s.expiry) {
            return 0;
        }
        uint256 iv = impliedVolatility(_seriesId);
        uint256 unitPrice = TigerSwapOptionMath.blackScholes(
            s.markPrice,
            s.strike,
            s.expiry - block.timestamp,
            riskFreeRate,
            iv,
            s.isCall
        );
        return unitPrice * _size / WAD;
    }

    /// @notice Annual risk-free rate used for pricing, 5%.
    uint256 public riskFreeRate = 5e16;

    /// @notice Collateral the pool must lock to underwrite `_size`.
    function requiredCollateral(uint256 _seriesId, uint256 _size) public view returns (uint256) {
        Series storage s = _requireSeries(_seriesId);
        uint256 perUnit = s.isCall ? s.markPrice : s.strike;
        return perUnit * _size / WAD;
    }

    // ---- Trading -----------------------------------------------------------

    /// @notice Buy a long option position. The premium is paid to the pool.
    function buy(uint256 _seriesId, uint256 _size, uint256 _maxPremium)
        external
        nonReentrant
        returns (uint256 paid)
    {
        Series storage s = _requireSeries(_seriesId);
        require(!s.settled, "series already settled");
        require(block.timestamp < s.expiry, "series expired");
        require(_size > 0, "size required");

        paid = premium(_seriesId, _size);
        require(paid > 0, "premium is zero");
        require(paid <= _maxPremium, "premium above limit");

        uint256 lock = requiredCollateral(_seriesId, _size);
        require(s.lockedCollateral + lock <= s.collateral, "insufficient pool collateral");

        require(collateralToken.transferFrom(msg.sender, address(this), paid), "premium transfer failed");

        s.premiumCollected += paid;
        // The premium is now pool money, so it joins the collateral the LPs
        // own. Without this the tokens would sit in the contract unaccounted
        // for and could never be withdrawn.
        s.collateral += paid;
        s.lockedCollateral += lock;
        s.totalLongSize += _size;
        longPositions[_seriesId][msg.sender] += _size;

        emit OptionBought(_seriesId, msg.sender, _size, paid, impliedVolatility(_seriesId), lock);
    }

    // ---- Settlement --------------------------------------------------------

    /// @notice Intrinsic payout for `_size` at the settlement price.
    function payoutAt(uint256 _seriesId, uint256 _size) public view returns (uint256) {
        Series storage s = _requireSeries(_seriesId);
        require(s.settled, "series not settled");
        uint256 perUnit = s.isCall
            ? (s.settlementPrice > s.strike ? s.settlementPrice - s.strike : 0)
            : (s.strike > s.settlementPrice ? s.strike - s.settlementPrice : 0);
        return perUnit * _size / WAD;
    }

    /// @notice Claim the intrinsic payout on an expired long position.
    function exercise(uint256 _seriesId) external nonReentrant returns (uint256 payout) {
        Series storage s = _requireSeries(_seriesId);
        require(s.settled, "series not settled");

        uint256 size = longPositions[_seriesId][msg.sender];
        require(size > 0, "no position");

        longPositions[_seriesId][msg.sender] = 0;
        s.totalLongSize -= size;

        payout = payoutAt(_seriesId, size);
        if (payout > 0) {
            require(payout <= s.collateral, "pool cannot cover payout");
            s.collateral -= payout;
            require(collateralToken.transfer(msg.sender, payout), "payout transfer failed");
        }

        // Release the collateral that backed this position.
        uint256 lock = requiredCollateral(_seriesId, size);
        s.lockedCollateral = s.lockedCollateral > lock ? s.lockedCollateral - lock : 0;
        if (s.collateral < s.lockedCollateral) {
            s.lockedCollateral = s.collateral;
        }
        lpClaims[_seriesId] = s.collateral;

        emit Exercised(_seriesId, msg.sender, size, payout);
    }

    /// @notice Withdraw the caller's share of the pool's remaining collateral.
    /// @dev Collateral still backing unexercised positions is reserved, so an
    ///      LP cannot withdraw funds an option holder is owed.
    function withdrawLp(uint256 _seriesId, uint256 _amount) external nonReentrant {
        Series storage s = _requireSeries(_seriesId);
        require(s.settled, "series not settled");
        require(_amount > 0, "amount required");
        require(_amount <= lpClaims[_seriesId], "exceeds claimable collateral");

        uint256 reserved = s.lockedCollateral > lpClaims[_seriesId] ? lpClaims[_seriesId] : s.lockedCollateral;
        require(_amount <= lpClaims[_seriesId] - reserved, "collateral is reserved");

        uint256 totalShares = totalLpShares[_seriesId];
        require(totalShares > 0, "no liquidity providers");
        uint256 burn = _amount * totalShares / lpClaims[_seriesId];
        require(burn <= lpShares[_seriesId][msg.sender], "exceeds your share");

        lpClaims[_seriesId] -= _amount;
        s.collateral -= _amount;
        totalLpShares[_seriesId] = totalShares - burn;
        lpShares[_seriesId][msg.sender] -= burn;
        require(collateralToken.transfer(msg.sender, _amount), "transfer out failed");

        emit LpWithdrawn(_seriesId, msg.sender, _amount);
    }

    // ---- Views -------------------------------------------------------------

    function seriesCount() external view returns (uint256) {
        return series.length;
    }

    /// @notice Full series snapshot for the frontend and the admin API.
    function getSeries(uint256 _seriesId)
        external
        view
        returns (
            uint256 strike,
            uint256 expiry,
            bool isCall,
            uint256 baseIV,
            uint256 markPrice,
            uint256 collateral,
            uint256 lockedCollateral,
            uint256 totalLongSize,
            uint256 premiumCollected,
            bool settled,
            uint256 settlementPrice
        )
    {
        Series storage s = _requireSeries(_seriesId);
        return (
            s.strike,
            s.expiry,
            s.isCall,
            s.baseIV,
            s.markPrice,
            s.collateral,
            s.lockedCollateral,
            s.totalLongSize,
            s.premiumCollected,
            s.settled,
            s.settlementPrice
        );
    }

    function _requireSeries(uint256 _seriesId) private view returns (Series storage) {
        require(_seriesId < series.length, "unknown series");
        return series[_seriesId];
    }

    // ---- Admin -------------------------------------------------------------

    function setRiskFreeRate(uint256 _rate) external onlyOwner {
        require(_rate <= 1e18, "rate too high");
        riskFreeRate = _rate;
    }

    function setOracle(address _oracle) external onlyOwner {
        require(_oracle != address(0), "zero address");
        oracle = _oracle;
        emit OracleUpdated(_oracle);
    }

    function transferOwnership(address _newOwner) external onlyOwner {
        require(_newOwner != address(0), "zero address");
        owner = _newOwner;
        emit OwnershipTransferred(msg.sender, _newOwner);
    }
}
