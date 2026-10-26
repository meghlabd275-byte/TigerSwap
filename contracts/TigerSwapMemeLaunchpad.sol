// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";

/**
 * @title TigerSwapMemeToken
 * @notice Minimal ERC20 whose supply is minted once at deployment. Only the
 *         launchpad that created it may burn, which is used to retire the
 *         unsold remainder at graduation.
 */
contract TigerSwapMemeToken {
    string public name;
    string public symbol;
    uint8 public constant decimals = 18;

    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    address public immutable launchpad;

    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);

    modifier onlyLaunchpad() {
        require(msg.sender == launchpad, "TigerSwap: NOT_LAUNCHPAD");
        _;
    }

    constructor(string memory _name, string memory _symbol, uint256 _supply, address _launchpad) {
        require(_launchpad != address(0), "TigerSwap: ZERO_LAUNCHPAD");
        require(_supply > 0, "TigerSwap: ZERO_SUPPLY");
        name = _name;
        symbol = _symbol;
        launchpad = _launchpad;
        totalSupply = _supply;
        balanceOf[_launchpad] = _supply;
        emit Transfer(address(0), _launchpad, _supply);
    }

    function approve(address spender, uint256 value) external returns (bool) {
        allowance[msg.sender][spender] = value;
        emit Approval(msg.sender, spender, value);
        return true;
    }

    function transfer(address to, uint256 value) external returns (bool) {
        _transfer(msg.sender, to, value);
        return true;
    }

    function transferFrom(address from, address to, uint256 value) external returns (bool) {
        uint256 allowed = allowance[from][msg.sender];
        if (allowed != type(uint256).max) {
            require(allowed >= value, "TigerSwap: ALLOWANCE");
            allowance[from][msg.sender] = allowed - value;
        }
        _transfer(from, to, value);
        return true;
    }

    function burn(uint256 value) external onlyLaunchpad {
        require(balanceOf[msg.sender] >= value, "TigerSwap: BALANCE");
        balanceOf[msg.sender] -= value;
        totalSupply -= value;
        emit Transfer(msg.sender, address(0), value);
    }

    function _transfer(address from, address to, uint256 value) private {
        require(to != address(0), "TigerSwap: ZERO_RECIPIENT");
        require(balanceOf[from] >= value, "TigerSwap: BALANCE");
        balanceOf[from] -= value;
        balanceOf[to] += value;
        emit Transfer(from, to, value);
    }
}

/**
 * @title TigerSwapMemeLaunchpad
 * @notice Fair-launch bonding curve for new tokens. A creator opens a launch,
 *         buyers trade against a constant-product curve held on virtual
 *         reserves, and once enough real quote has been raised the curve
 *         graduates: a real TigerSwap pool is created and seeded at exactly the
 *         curve's final price, so the transition is continuous for holders.
 *
 * No external launchpad is involved. Pricing, the curve, the graduation and the
 * pool seeding are all computed here.
 */
contract TigerSwapMemeLaunchpad {
    uint256 public constant WAD = 1e18;
    uint256 public constant FEE_DENOMINATOR = 10_000;
    /// @dev Total fee charged on each trade, 1%.
    uint256 public constant TRADE_FEE_BPS = 100;
    /// @dev Curve is seeded with these virtual reserves, which set the opening price.
    uint256 public constant VIRTUAL_QUOTE = 30_000e18;
    uint256 public constant VIRTUAL_TOKEN = 1_000_000_000e18;
    /// @dev Real quote raised before a curve graduates.
    uint256 public constant GRADUATION_TARGET = 100_000e18;
    /// @dev LP tokens are minted here, locking the graduated pool forever.
    address public constant LP_LOCK = address(0x000000000000000000000000000000000000dEaD);

    IERC20 public immutable quoteToken;
    ITigerSwapFactory public immutable factory;
    address public owner;
    address public treasury;
    bool public paused;

    struct Launch {
        address token;
        address creator;
        uint256 creatorFeeBps;
        uint256 virtualQuote;
        uint256 virtualToken;
        uint256 realQuote;
        uint256 tokensSold;
        uint256 creatorFees;
        bool graduated;
        bool active;
    }

    Launch[] public launches;
    /// @dev Launch id for a token, so a token can only belong to one curve.
    mapping(address => uint256) public launchIdOf;
    mapping(address => bool) public isLaunchToken;

    event LaunchCreated(uint256 indexed launchId, address indexed token, address indexed creator, string name, string symbol);
    event Bought(uint256 indexed launchId, address indexed buyer, uint256 quoteIn, uint256 tokensOut, uint256 fee);
    event Sold(uint256 indexed launchId, address indexed seller, uint256 tokensIn, uint256 quoteOut, uint256 fee);
    event Graduated(uint256 indexed launchId, address pool, uint256 quoteSeeded, uint256 tokensSeeded, uint256 tokensBurned);
    event CreatorFeesClaimed(uint256 indexed launchId, address indexed creator, uint256 amount);
    event TreasuryUpdated(address indexed treasury);
    event PausedSet(bool paused);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    modifier onlyOwner() {
        require(msg.sender == owner, "TigerSwap: NOT_OWNER");
        _;
    }

    modifier whenNotPaused() {
        require(!paused, "TigerSwap: PAUSED");
        _;
    }

    uint256 private _locked = 1;

    modifier nonReentrant() {
        require(_locked == 1, "TigerSwap: REENTRANT");
        _locked = 2;
        _;
        _locked = 1;
    }

    constructor(address _quoteToken, address _factory, address _treasury) {
        require(_quoteToken != address(0), "TigerSwap: ZERO_QUOTE");
        require(_factory != address(0), "TigerSwap: ZERO_FACTORY");
        require(_treasury != address(0), "TigerSwap: ZERO_TREASURY");
        quoteToken = IERC20(_quoteToken);
        factory = ITigerSwapFactory(_factory);
        owner = msg.sender;
        treasury = _treasury;
        emit OwnershipTransferred(address(0), msg.sender);
        emit TreasuryUpdated(_treasury);
    }

    // ============ Admin ============

    function setTreasury(address _treasury) external onlyOwner {
        require(_treasury != address(0), "TigerSwap: ZERO_TREASURY");
        treasury = _treasury;
        emit TreasuryUpdated(_treasury);
    }

    function setPaused(bool _paused) external onlyOwner {
        paused = _paused;
        emit PausedSet(_paused);
    }

    function transferOwnership(address _newOwner) external onlyOwner {
        require(_newOwner != address(0), "TigerSwap: ZERO_OWNER");
        emit OwnershipTransferred(owner, _newOwner);
        owner = _newOwner;
    }

    // ============ Launch lifecycle ============

    /// @notice Open a new bonding curve and mint its token.
    /// @param creatorFeeBps Share of each trade fee paid to the creator, in bps.
    function createLaunch(string calldata name, string calldata symbol, uint256 creatorFeeBps)
        external
        whenNotPaused
        returns (uint256 launchId, address token)
    {
        require(bytes(name).length > 0 && bytes(symbol).length > 0, "TigerSwap: EMPTY_METADATA");
        // The creator's cut is a share of the trade fee, so the denominator is
        // FEE_DENOMINATOR, not the fee itself.
        require(creatorFeeBps <= FEE_DENOMINATOR, "TigerSwap: FEE_TOO_HIGH");

        TigerSwapMemeToken deployed = new TigerSwapMemeToken(name, symbol, VIRTUAL_TOKEN, address(this));
        token = address(deployed);

        launches.push(
            Launch({
                token: token,
                creator: msg.sender,
                creatorFeeBps: creatorFeeBps,
                virtualQuote: VIRTUAL_QUOTE,
                virtualToken: VIRTUAL_TOKEN,
                realQuote: 0,
                tokensSold: 0,
                creatorFees: 0,
                graduated: false,
                active: true
            })
        );
        launchId = launches.length - 1;
        launchIdOf[token] = launchId;
        isLaunchToken[token] = true;

        emit LaunchCreated(launchId, token, msg.sender, name, symbol);
    }

    /// @notice Buy tokens from the curve.
    /// @param quoteIn Quote spent, fee included.
    /// @param minTokensOut Slippage floor.
    function buy(uint256 launchId, uint256 quoteIn, uint256 minTokensOut)
        external
        whenNotPaused
        nonReentrant
        returns (uint256 tokensOut)
    {
        Launch storage l = _requireTradingLaunch(launchId);
        require(quoteIn > 0, "TigerSwap: ZERO_AMOUNT");

        uint256 fee = quoteIn * TRADE_FEE_BPS / FEE_DENOMINATOR;
        uint256 netQuote = quoteIn - fee;

        uint256 quoteReserve = l.virtualQuote + l.realQuote;
        uint256 tokenReserve = l.virtualToken - l.tokensSold;
        uint256 k = quoteReserve * tokenReserve;

        uint256 newQuoteReserve = quoteReserve + netQuote;
        uint256 newTokenReserve = k / newQuoteReserve;
        tokensOut = tokenReserve - newTokenReserve;
        require(tokensOut > 0, "TigerSwap: ZERO_OUTPUT");
        require(tokensOut >= minTokensOut, "TigerSwap: SLIPPAGE");

        require(quoteToken.transferFrom(msg.sender, address(this), quoteIn), "TigerSwap: TRANSFER_FAILED");
        require(TigerSwapMemeToken(l.token).transfer(msg.sender, tokensOut), "TigerSwap: TRANSFER_FAILED");

        l.realQuote += netQuote;
        l.tokensSold += tokensOut;
        _splitFee(l, fee);

        emit Bought(launchId, msg.sender, quoteIn, tokensOut, fee);

        // Graduation is checked after the trade so the buyer is filled at the
        // curve price they were quoted.
        if (l.realQuote >= GRADUATION_TARGET) {
            _graduate(launchId);
        }
    }

    /// @notice Sell tokens back to the curve before graduation.
    function sell(uint256 launchId, uint256 tokenIn, uint256 minQuoteOut)
        external
        whenNotPaused
        nonReentrant
        returns (uint256 quoteOut)
    {
        Launch storage l = _requireTradingLaunch(launchId);
        require(tokenIn > 0, "TigerSwap: ZERO_AMOUNT");

        uint256 quoteReserve = l.virtualQuote + l.realQuote;
        uint256 tokenReserve = l.virtualToken - l.tokensSold;
        uint256 k = quoteReserve * tokenReserve;

        uint256 newTokenReserve = tokenReserve + tokenIn;
        uint256 newQuoteReserve = k / newTokenReserve;
        require(quoteReserve > newQuoteReserve, "TigerSwap: CURVE_UNDERFLOW");
        uint256 grossQuote = quoteReserve - newQuoteReserve;

        uint256 fee = grossQuote * TRADE_FEE_BPS / FEE_DENOMINATOR;
        quoteOut = grossQuote - fee;
        require(quoteOut > 0, "TigerSwap: ZERO_OUTPUT");
        require(quoteOut >= minQuoteOut, "TigerSwap: SLIPPAGE");
        require(quoteOut <= l.realQuote, "TigerSwap: INSUFFICIENT_RESERVE");

        require(TigerSwapMemeToken(l.token).transferFrom(msg.sender, address(this), tokenIn), "TigerSwap: TRANSFER_FAILED");
        require(quoteToken.transfer(msg.sender, quoteOut), "TigerSwap: TRANSFER_FAILED");

        l.realQuote -= quoteOut;
        l.tokensSold -= tokenIn;
        _splitFee(l, fee);

        emit Sold(launchId, msg.sender, tokenIn, quoteOut, fee);
    }

    /// @notice Anyone may graduate a curve that has reached its target.
    function graduate(uint256 launchId) external whenNotPaused nonReentrant {
        Launch storage l = launches[launchId];
        require(l.active, "TigerSwap: UNKNOWN_LAUNCH");
        require(!l.graduated, "TigerSwap: ALREADY_GRADUATED");
        require(l.realQuote >= GRADUATION_TARGET, "TigerSwap: TARGET_NOT_REACHED");
        _graduate(launchId);
    }

    function claimCreatorFees(uint256 launchId) external nonReentrant returns (uint256 amount) {
        Launch storage l = launches[launchId];
        require(l.active, "TigerSwap: UNKNOWN_LAUNCH");
        require(msg.sender == l.creator, "TigerSwap: NOT_CREATOR");
        amount = l.creatorFees;
        require(amount > 0, "TigerSwap: NOTHING_TO_CLAIM");
        l.creatorFees = 0;
        require(quoteToken.transfer(msg.sender, amount), "TigerSwap: TRANSFER_FAILED");
        emit CreatorFeesClaimed(launchId, msg.sender, amount);
    }

    // ============ Graduation ============

    /// @dev Creates the real pool and seeds it at the curve's final price.
    ///
    ///      The curve price is quoteReserve / tokenReserve. To keep it, the pool
    ///      is seeded with all the real quote raised and exactly
    ///      realQuote * tokenReserve / quoteReserve tokens. The tokens that only
    ///      ever existed on the virtual side are burned, so nothing is handed to
    ///      the creator or the treasury at graduation.
    function _graduate(uint256 launchId) private {
        Launch storage l = launches[launchId];
        uint256 quoteReserve = l.virtualQuote + l.realQuote;
        uint256 tokenReserve = l.virtualToken - l.tokensSold;

        uint256 quoteSeeded = l.realQuote;
        uint256 tokensSeeded = quoteSeeded * tokenReserve / quoteReserve;
        require(quoteSeeded > 0 && tokensSeeded > 0, "TigerSwap: NOTHING_TO_SEED");

        address token = l.token;
        address pool = factory.createPool(token, address(quoteToken));
        require(pool != address(0), "TigerSwap: POOL_CREATION_FAILED");

        require(quoteToken.transfer(pool, quoteSeeded), "TigerSwap: TRANSFER_FAILED");
        require(TigerSwapMemeToken(token).transfer(pool, tokensSeeded), "TigerSwap: TRANSFER_FAILED");
        ITigerSwapPool(pool).mint(LP_LOCK);

        uint256 remainder = TigerSwapMemeToken(token).balanceOf(address(this));
        if (remainder > 0) {
            TigerSwapMemeToken(token).burn(remainder);
        }

        l.graduated = true;
        l.realQuote = 0;
        l.tokensSold = 0;

        emit Graduated(launchId, pool, quoteSeeded, tokensSeeded, remainder);
    }

    // ============ Views ============

    /// @notice Quote needed to receive `tokenOut` from the curve, fee included.
    function quoteForTokens(uint256 launchId, uint256 tokenOut) external view returns (uint256 quoteIn) {
        Launch storage l = _requireLaunch(launchId);
        uint256 quoteReserve = l.virtualQuote + l.realQuote;
        uint256 tokenReserve = l.virtualToken - l.tokensSold;
        require(tokenOut < tokenReserve, "TigerSwap: INSUFFICIENT_TOKEN_RESERVE");
        uint256 k = quoteReserve * tokenReserve;
        uint256 newTokenReserve = tokenReserve - tokenOut;
        uint256 newQuoteReserve = k / newTokenReserve;
        uint256 netQuote = newQuoteReserve - quoteReserve;
        quoteIn = netQuote * FEE_DENOMINATOR / (FEE_DENOMINATOR - TRADE_FEE_BPS);
    }

    /// @notice Tokens received for `quoteIn`, fee included.
    function tokensForQuote(uint256 launchId, uint256 quoteIn) external view returns (uint256 tokensOut) {
        Launch storage l = _requireLaunch(launchId);
        uint256 fee = quoteIn * TRADE_FEE_BPS / FEE_DENOMINATOR;
        uint256 netQuote = quoteIn - fee;
        uint256 quoteReserve = l.virtualQuote + l.realQuote;
        uint256 tokenReserve = l.virtualToken - l.tokensSold;
        uint256 k = quoteReserve * tokenReserve;
        uint256 newTokenReserve = k / (quoteReserve + netQuote);
        tokensOut = tokenReserve - newTokenReserve;
    }

    /// @notice Spot price of the curve, quote per whole token.
    function spotPrice(uint256 launchId) external view returns (uint256 price) {
        Launch storage l = _requireLaunch(launchId);
        uint256 quoteReserve = l.virtualQuote + l.realQuote;
        uint256 tokenReserve = l.virtualToken - l.tokensSold;
        if (tokenReserve == 0) return 0;
        price = quoteReserve * WAD / tokenReserve;
    }

    function launchCount() external view returns (uint256) {
        return launches.length;
    }

    function getLaunch(uint256 launchId)
        external
        view
        returns (
            address token,
            address creator,
            uint256 creatorFeeBps,
            uint256 realQuote,
            uint256 tokensSold,
            uint256 creatorFees,
            uint256 price,
            bool graduated,
            bool active
        )
    {
        Launch storage l = _requireLaunch(launchId);
        token = l.token;
        creator = l.creator;
        creatorFeeBps = l.creatorFeeBps;
        realQuote = l.realQuote;
        tokensSold = l.tokensSold;
        creatorFees = l.creatorFees;
        uint256 quoteReserve = l.virtualQuote + l.realQuote;
        uint256 tokenReserve = l.virtualToken - l.tokensSold;
        price = tokenReserve == 0 ? 0 : quoteReserve * WAD / tokenReserve;
        graduated = l.graduated;
        active = l.active;
    }

    // ============ Internals ============

    function _splitFee(Launch storage l, uint256 fee) private {
        if (fee == 0) return;
        uint256 creatorCut = fee * l.creatorFeeBps / FEE_DENOMINATOR;
        if (creatorCut > 0) {
            l.creatorFees += creatorCut;
        }
        uint256 treasuryCut = fee - creatorCut;
        if (treasuryCut > 0) {
            require(quoteToken.transfer(treasury, treasuryCut), "TigerSwap: TRANSFER_FAILED");
        }
    }

    function _requireLaunch(uint256 launchId) private view returns (Launch storage l) {
        require(launchId < launches.length, "TigerSwap: UNKNOWN_LAUNCH");
        l = launches[launchId];
    }

    function _requireTradingLaunch(uint256 launchId) private view returns (Launch storage l) {
        l = _requireLaunch(launchId);
        require(!l.graduated, "TigerSwap: GRADUATED");
    }
}
