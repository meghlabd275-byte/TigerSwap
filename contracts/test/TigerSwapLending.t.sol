// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapLending.sol";
import "../TigerSwapPriceOracle.sol";

contract MintableERC20 {
    string public name;
    string public symbol;
    uint8 public decimals = 18;
    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);

    constructor(string memory n, string memory s) {
        name = n;
        symbol = s;
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

contract LendingTest is Test {
    uint256 constant WAD = 1e18;

    TigerSwapLending lending;
    TigerSwapPriceOracle oracle;
    MintableERC20 rusd;
    MintableERC20 coll;

    address alice = address(0xA11CE);
    address bob = address(0xB0B);
    address liquidator = address(0x110);

    function setUp() public {
        oracle = new TigerSwapPriceOracle(1 hours, 1);
        lending = new TigerSwapLending(address(oracle));
        rusd = new MintableERC20("RUSD", "RUSD");
        coll = new MintableERC20("COLL", "COLL");

        // Price feeds: RUSD $1, COLL $2000.
        oracle.setReporter(address(this), true);
        oracle.submitPrice(address(rusd), 1e18);
        oracle.submitPrice(address(coll), 2000e18);

        // RUSD market: 0% base, 4% at kink, 100% at full utilisation.
        lending.createMarket(address(rusd), 0.8e18, 0.85e18, 0, 4e18, 75e18, 0.8e18, 0.05e18);
        // COLL market: usable as collateral, not borrowable here.
        lending.createMarket(address(coll), 0.7e18, 0.75e18, 0, 4e18, 75e18, 0.8e18, 0.05e18);

        rusd.mint(bob, 1_000_000e18);
        coll.mint(alice, 100e18);
        rusd.mint(liquidator, 1_000_000e18);

        vm.startPrank(bob);
        rusd.approve(address(lending), type(uint256).max);
        vm.stopPrank();

        vm.startPrank(alice);
        coll.approve(address(lending), type(uint256).max);
        rusd.approve(address(lending), type(uint256).max);
        vm.stopPrank();

        vm.startPrank(liquidator);
        rusd.approve(address(lending), type(uint256).max);
        vm.stopPrank();
    }

    function testDepositAndWithdraw() public {
        vm.startPrank(bob);
        lending.deposit(address(rusd), 100_000e18);
        assertEq(lending.getDepositBalance(bob, address(rusd)), 100_000e18);

        lending.withdraw(address(rusd), 40_000e18);
        assertEq(lending.getDepositBalance(bob, address(rusd)), 60_000e18);
        vm.stopPrank();

        assertEq(rusd.balanceOf(bob), 1_000_000e18 - 60_000e18);
    }

    function testBorrowWithinCollateral() public {
        vm.prank(bob);
        lending.deposit(address(rusd), 500_000e18);

        // Alice posts 10 COLL = $20,000 collateral, threshold 75% => $15,000.
        vm.startPrank(alice);
        lending.deposit(address(coll), 10e18);
        lending.borrow(address(rusd), 10_000e18);
        vm.stopPrank();

        assertEq(lending.getBorrowBalance(alice, address(rusd)), 10_000e18);
        assertEq(rusd.balanceOf(alice), 10_000e18);

        uint256 health = lending.getAccountHealth(alice);
        assertGt(health, WAD);
    }

    function testBorrowBeyondCollateralReverts() public {
        vm.prank(bob);
        lending.deposit(address(rusd), 500_000e18);

        vm.startPrank(alice);
        lending.deposit(address(coll), 1e18); // $2000 => $1500 borrow power
        vm.expectRevert(bytes("TigerSwap: INSUFFICIENT_COLLATERAL"));
        lending.borrow(address(rusd), 10_000e18);
        vm.stopPrank();
    }

    function testInterestAccruesToSupplier() public {
        vm.prank(bob);
        lending.deposit(address(rusd), 1_000_000e18);

        vm.startPrank(alice);
        lending.deposit(address(coll), 100e18);
        lending.borrow(address(rusd), 100_000e18);
        vm.stopPrank();

        uint256 before = lending.getDepositBalance(bob, address(rusd));
        vm.warp(block.timestamp + 365 days);
        uint256 afterBal = lending.getDepositBalance(bob, address(rusd));

        assertGt(afterBal, before);

        // The borrower owes more than they borrowed.
        assertGt(lending.getBorrowBalance(alice, address(rusd)), 100_000e18);
    }

    function testRepayReducesDebt() public {
        vm.prank(bob);
        lending.deposit(address(rusd), 500_000e18);

        vm.startPrank(alice);
        lending.deposit(address(coll), 10e18);
        lending.borrow(address(rusd), 5_000e18);
        lending.repay(address(rusd), 5_000e18);
        vm.stopPrank();

        assertEq(lending.getBorrowBalance(alice, address(rusd)), 0);
    }

    function testLiquidationSeizesCollateralForRepayment() public {
        vm.prank(bob);
        lending.deposit(address(rusd), 500_000e18);

        vm.startPrank(alice);
        lending.deposit(address(coll), 10e18);          // $20,000 collateral
        lending.borrow(address(rusd), 15_000e18);       // $15,000 debt, health ~1.33
        vm.stopPrank();

        // COLL drops to $1,000 => collateral $10,000 < debt $15,000.
        oracle.submitPrice(address(coll), 1000e18);

        assertLt(lending.getAccountHealth(alice), WAD);

        uint256 liquidatorRusdBefore = rusd.balanceOf(liquidator);
        uint256 liquidatorCollBefore = coll.balanceOf(liquidator);

        vm.prank(liquidator);
        lending.liquidate(alice, address(rusd), address(coll), 7_500e18);

        // The liquidator actually paid RUSD...
        assertEq(rusd.balanceOf(liquidator), liquidatorRusdBefore - 7_500e18);
        // ...and received COLL plus the bonus.
        uint256 seized = coll.balanceOf(liquidator) - liquidatorCollBefore;
        assertGt(seized, 7_500e18 * 1e18 / 1000e18); // > 7.5 COLL

        // Debt fell by the repaid amount, and the account improved.
        assertEq(lending.getBorrowBalance(alice, address(rusd)), 7_500e18);
    }

    function testLiquidationOfHealthyAccountReverts() public {
        vm.prank(bob);
        lending.deposit(address(rusd), 500_000e18);

        vm.startPrank(alice);
        lending.deposit(address(coll), 10e18);
        lending.borrow(address(rusd), 1_000e18);
        vm.stopPrank();

        vm.prank(liquidator);
        vm.expectRevert(bytes("TigerSwap: HEALTHY"));
        lending.liquidate(alice, address(rusd), address(coll), 500e18);
    }

    function testWithdrawThatBreaksHealthReverts() public {
        vm.prank(bob);
        lending.deposit(address(rusd), 500_000e18);

        vm.startPrank(alice);
        lending.deposit(address(coll), 10e18);
        lending.borrow(address(rusd), 14_000e18);

        vm.expectRevert(bytes("TigerSwap: WITHDRAW_WOULD_LIQUIDATE"));
        lending.withdraw(address(coll), 5e18);
        vm.stopPrank();
    }

    function testUtilisationAndRatesRespondToBorrowing() public {
        vm.prank(bob);
        lending.deposit(address(rusd), 100_000e18);
        assertEq(lending.getUtilisation(address(rusd)), 0);

        vm.startPrank(alice);
        lending.deposit(address(coll), 100e18);
        lending.borrow(address(rusd), 40_000e18);
        vm.stopPrank();

        assertEq(lending.getUtilisation(address(rusd)), 0.4e18);
        // Below the kink, so the rate is slope1 * utilisation.
        assertEq(lending.getBorrowRate(address(rusd)), 4e18 * 0.4e18 / WAD);
        assertGt(lending.getSupplyRate(address(rusd)), 0);
    }

    function testOracleUsesMedianAndRejectsStale() public {
        address r2 = address(0x2222);
        address r3 = address(0x3333);
        oracle.setReporter(r2, true);
        oracle.setReporter(r3, true);

        oracle.submitPrice(address(rusd), 1e18);
        oracle.submitPrice(address(r2), 1.02e18);
        oracle.submitPrice(address(r3), 0.98e18);

        // Median of {0.98, 1.00, 1.02} == 1.00.
        assertEq(oracle.getPrice(address(rusd)), 1e18);

        vm.warp(block.timestamp + 2 hours);
        // Every report is now stale.
        vm.expectRevert(bytes("TigerSwap: INSUFFICIENT_REPORTS"));
        oracle.getPrice(address(rusd));
    }
}
