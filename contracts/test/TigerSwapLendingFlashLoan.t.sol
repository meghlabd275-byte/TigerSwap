// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapLending.sol";

contract LendERC20 {
    string public name;
    string public symbol;
    uint8 public decimals = 18;
    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);

    constructor(string memory _name, string memory _symbol) {
        name = _name;
        symbol = _symbol;
    }

    function mint(address to, uint256 amount) external {
        totalSupply += amount;
        balanceOf[to] += amount;
        emit Transfer(address(0), to, amount);
    }

    function approve(address spender, uint256 amount) external returns (bool) {
        allowance[msg.sender][spender] = amount;
        emit Approval(msg.sender, spender, amount);
        return true;
    }

    function transfer(address to, uint256 amount) external returns (bool) {
        balanceOf[msg.sender] -= amount;
        balanceOf[to] += amount;
        emit Transfer(msg.sender, to, amount);
        return true;
    }

    function transferFrom(address from, address to, uint256 amount) external returns (bool) {
        if (allowance[from][msg.sender] != type(uint256).max) {
            allowance[from][msg.sender] -= amount;
        }
        balanceOf[from] -= amount;
        balanceOf[to] += amount;
        emit Transfer(from, to, amount);
        return true;
    }
}

contract LendOracle is IPriceOracle {
    mapping(address => uint256) public prices;

    function setPrice(address token, uint256 price) external {
        prices[token] = price;
    }

    function getPrice(address token) external view returns (uint256) {
        return prices[token];
    }
}

/// @dev Honest receiver: repays the loan plus the fee out of a pre-funded balance.
contract HonestBorrower is IFlashLoanReceiver {
    LendERC20 public immutable token;
    TigerSwapLending public immutable pool;
    uint256 public lastAmount;
    uint256 public lastFee;
    bytes public lastData;
    /// @dev Amount withheld from the repayment; 0 means repay in full.
    uint256 public shortfall;

    constructor(LendERC20 _token, TigerSwapLending _pool) {
        token = _token;
        pool = _pool;
    }

    function fund(uint256 amount) external {
        token.mint(address(this), amount);
    }

    function setShortfall(uint256 amount) external {
        shortfall = amount;
    }

    function borrow(uint256 amount, bytes calldata data) external {
        pool.flashLoan(address(this), address(token), amount, data);
    }

    function onFlashLoan(address, address, uint256 amount, uint256 fee, bytes calldata data) external {
        lastAmount = amount;
        lastFee = fee;
        lastData = data;
        token.transfer(address(pool), amount + fee - shortfall);
    }
}

/// @dev Dishonest receiver: keeps the funds. The loan must revert.
contract ThiefBorrower is IFlashLoanReceiver {
    LendERC20 public immutable token;
    TigerSwapLending public immutable pool;

    constructor(LendERC20 _token, TigerSwapLending _pool) {
        token = _token;
        pool = _pool;
    }

    function borrow(uint256 amount) external {
        pool.flashLoan(address(this), address(token), amount, "");
    }

    function onFlashLoan(address, address, uint256, uint256, bytes calldata) external pure {
        // Deliberately repay nothing.
    }
}

/// @dev Receiver that tries to re-enter the pool mid-callback.
contract ReentrantBorrower is IFlashLoanReceiver {
    LendERC20 public immutable token;
    TigerSwapLending public immutable pool;
    bool public reentered;
    bytes public revertReason;

    constructor(LendERC20 _token, TigerSwapLending _pool) {
        token = _token;
        pool = _pool;
    }

    function fund(uint256 amount) external {
        token.mint(address(this), amount);
    }

    function borrow(uint256 amount) external {
        pool.flashLoan(address(this), address(token), amount, "");
    }

    function onFlashLoan(address, address, uint256 amount, uint256 fee, bytes calldata) external {
        // Attempt to deposit while the loan is outstanding.
        token.approve(address(pool), type(uint256).max);
        try pool.deposit(address(token), 1e18) {
            reentered = true;
        } catch (bytes memory reason) {
            revertReason = reason;
        }
        token.transfer(address(pool), amount + fee);
    }
}

contract TigerSwapLendingFlashLoanTest is Test {
    TigerSwapLending public pool;
    LendERC20 public rusd;
    LendOracle public oracle;

    HonestBorrower public honest;
    ThiefBorrower public thief;
    ReentrantBorrower public reentrant;

    address public supplier = address(0x51);

    uint256 constant WAD = 1e18;

    function setUp() public {
        rusd = new LendERC20("RUSD", "RUSD");
        oracle = new LendOracle();
        pool = new TigerSwapLending(address(oracle));

        oracle.setPrice(address(rusd), 1e18);

        pool.createMarket(address(rusd), 8e17, 85e16, 0, 1e18 / uint256(365 days) / 10, 1e18, 8e17, 5e16);

        rusd.mint(supplier, 1_000_000e18);
        vm.prank(supplier);
        rusd.approve(address(pool), type(uint256).max);
        vm.prank(supplier);
        pool.deposit(address(rusd), 1_000_000e18);

        honest = new HonestBorrower(rusd, pool);
        thief = new ThiefBorrower(rusd, pool);
        reentrant = new ReentrantBorrower(rusd, pool);
    }

    // ---- Happy path --------------------------------------------------------

    function testFlashLoanTransfersFundsAndCollectsFee() public {
        uint256 amount = 100_000e18;
        uint256 fee = amount * pool.flashLoanFee() / WAD;
        honest.fund(fee);

        uint256 poolBefore = rusd.balanceOf(address(pool));

        honest.borrow(amount, hex"1234");

        assertEq(honest.lastAmount(), amount, "receiver saw the principal");
        assertEq(honest.lastFee(), fee, "receiver saw the fee");
        assertEq(honest.lastData(), hex"1234", "calldata forwarded");
        // The pool keeps the fee.
        assertEq(rusd.balanceOf(address(pool)), poolBefore + fee, "pool earned the fee");
    }

    function testFeeAccruesToSuppliers() public {
        uint256 amount = 100_000e18;
        uint256 fee = amount * pool.flashLoanFee() / WAD;
        honest.fund(fee);

        uint256 indexBefore = _depositIndex();
        uint256 sharesBefore = _totalDepositShares();
        uint256 supplierValueBefore = pool.getDepositBalance(supplier, address(rusd));

        honest.borrow(amount, "");

        // totalDeposits is share-denominated, so the fee raises the deposit
        // index rather than the share count: the same shares are worth more.
        assertGt(_depositIndex(), indexBefore, "deposit index grew by the fee");
        assertEq(_totalDepositShares(), sharesBefore, "share count is unchanged");
        assertGt(pool.getDepositBalance(supplier, address(rusd)), supplierValueBefore, "supplier value grew");
    }

    /// @dev Reads the totalDeposits (share count) field of the RUSD market.
    function _totalDepositShares() internal view returns (uint256 totalDeposits) {
        (address token, uint256 deposits,,,,,,,,,,,,,,) = pool.markets(address(rusd));
        assertEq(token, address(rusd), "market is RUSD");
        return deposits;
    }

    /// @dev Reads the depositIndex field of the RUSD market.
    function _depositIndex() internal view returns (uint256 depositIndex) {
        (,,,, uint256 index,,,,,,,,,,,) = pool.markets(address(rusd));
        return index;
    }

    function testFeeIsSplitWithReserves() public {
        uint256 amount = 100_000e18;
        uint256 fee = amount * pool.flashLoanFee() / WAD;
        honest.fund(fee);

        honest.borrow(amount, "");

        uint256 reserves = _reserves();
        uint256 expectedReserves = fee * 1e17 / WAD; // default reserve factor 10%
        assertEq(reserves, expectedReserves, "reserves got their share");
    }

    /// @dev Reads the totalReserves field of the RUSD market.
    function _reserves() internal view returns (uint256 reserves) {
        (,,,,,,,,,,,,,, uint256 reserves,) = pool.markets(address(rusd));
        return reserves;
    }

    // ---- Repayment enforcement --------------------------------------------

    function testBorrowerThatKeepsFundsReverts() public {
        vm.expectRevert("TigerSwap: FLASH_LOAN_NOT_REPAID");
        thief.borrow(1000e18);
    }

    function testBorrowerPayingOneWeiLessThanOwedReverts() public {
        uint256 amount = 1000e18;
        uint256 fee = amount * pool.flashLoanFee() / WAD;
        honest.fund(amount + fee);
        honest.setShortfall(1);

        vm.expectRevert("TigerSwap: FLASH_LOAN_NOT_REPAID");
        honest.borrow(amount, "");
    }

    function testBorrowerPayingExactlyPrincipalReverts() public {
        // Repaying only the principal leaves the fee unpaid.
        uint256 amount = 1000e18;
        uint256 fee = amount * pool.flashLoanFee() / WAD;
        honest.fund(amount + fee);
        honest.setShortfall(fee);

        vm.expectRevert("TigerSwap: FLASH_LOAN_NOT_REPAID");
        honest.borrow(amount, "");
    }

    // ---- Limits ------------------------------------------------------------

    function testCannotBorrowMoreThanLiquidity() public {
        honest.fund(0);
        vm.expectRevert("TigerSwap: INSUFFICIENT_LIQUIDITY");
        honest.borrow(2_000_000e18, "");
    }

    function testCannotDrainTheWholeMarket() public {
        // 90% utilisation cap: exactly the full liquidity is refused.
        honest.fund(0);
        vm.expectRevert("TigerSwap: FLASH_LOAN_TOO_LARGE");
        honest.borrow(1_000_000e18, "");
    }

    function testBorrowRejectsZeroAmount() public {
        honest.fund(0);
        vm.expectRevert("TigerSwap: ZERO_AMOUNT");
        honest.borrow(0, "");
    }

    function testBorrowRejectsUnknownMarket() public {
        LendERC20 other = new LendERC20("OTHER", "OTHER");
        vm.expectRevert("TigerSwap: MARKET_NOT_ACTIVE");
        pool.flashLoan(address(honest), address(other), 1e18, "");
    }

    function testBorrowRejectsZeroReceiver() public {
        vm.expectRevert("TigerSwap: ZERO_RECEIVER");
        pool.flashLoan(address(0), address(rusd), 1e18, "");
    }

    // ---- Reentrancy --------------------------------------------------------

    function testCallbackCannotReenterThePool() public {
        uint256 amount = 100_000e18;
        uint256 fee = amount * pool.flashLoanFee() / WAD;
        reentrant.fund(fee);

        reentrant.borrow(amount);

        assertFalse(reentrant.reentered(), "reentrant deposit must not succeed");
        // The revert reason is the guard.
        assertTrue(reentrant.revertReason().length > 0, "deposit reverted");
    }

    // ---- Fee administration -----------------------------------------------

    function testOwnerCanChangeFee() public {
        pool.setFlashLoanFee(1e15);
        assertEq(pool.flashLoanFee(), 1e15, "fee updated");
    }

    function testFeeCannotExceedTenPercent() public {
        vm.expectRevert("TigerSwap: FEE_TOO_HIGH");
        pool.setFlashLoanFee(2e17);
    }

    function testOnlyOwnerCanChangeFee() public {
        vm.prank(supplier);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        pool.setFlashLoanFee(1e15);
    }

    // ---- Interaction with ordinary borrowing ------------------------------

    function testFlashLoanDoesNotCreateDebtForTheBorrower() public {
        uint256 amount = 100_000e18;
        uint256 fee = amount * pool.flashLoanFee() / WAD;
        honest.fund(fee);

        honest.borrow(amount, "");

        // The receiver owes nothing: the loan is repaid in the same call.
        assertEq(pool.getBorrowBalance(address(honest), address(rusd)), 0, "no residual debt");
    }

    function testPoolStaysSolventForSuppliers() public {
        uint256 amount = 100_000e18;
        uint256 fee = amount * pool.flashLoanFee() / WAD;
        honest.fund(fee);

        honest.borrow(amount, "");

        uint256 supplierValue = pool.getDepositBalance(supplier, address(rusd));
        assertGe(supplierValue, 1_000_000e18, "supplier can withdraw at least the principal");
        assertGe(rusd.balanceOf(address(pool)), supplierValue, "pool holds enough to pay suppliers");
    }
}
