// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapVault.sol";
import "../TigerSwapRouter.sol";
import "../TigerSwapFactory.sol";

contract VaultERC20 {
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

contract VaultWETH is VaultERC20 {
    constructor() VaultERC20("Wrapped Ether", "WETH") {}

    function deposit() external payable {
        totalSupply += msg.value;
        balanceOf[msg.sender] += msg.value;
        emit Transfer(address(0), msg.sender, msg.value);
    }

    function withdraw(uint256 amount) external {
        balanceOf[msg.sender] -= amount;
        totalSupply -= amount;
        emit Transfer(msg.sender, address(0), amount);
        (bool ok,) = msg.sender.call{value: amount}("");
        require(ok, "eth transfer failed");
    }
}

contract TigerSwapVaultTest is Test {
    TigerSwapVault public vault;
    TigerSwapRouter public router;
    TigerSwapFactory public factory;
    VaultERC20 public tgr;
    VaultERC20 public reward;
    VaultWETH public weth;

    address public alice = address(0xA11CE);
    address public bob = address(0xB0B);
    address public keeper = address(0x4EE1);

    function setUp() public {
        tgr = new VaultERC20("TigerSwap", "TGR");
        reward = new VaultERC20("Reward", "RWD");
        weth = new VaultWETH();

        factory = new TigerSwapFactory();
        router = new TigerSwapRouter(address(factory), address(weth));
        vault = new TigerSwapVault(address(tgr), address(router), address(factory), "Tiger TGR Vault", "vtTGR");

        // A reward/TGR pool so harvest has somewhere to swap.
        reward.mint(address(this), 10_000_000e18);
        tgr.mint(address(this), 10_000_000e18);
        reward.approve(address(router), type(uint256).max);
        tgr.approve(address(router), type(uint256).max);
        router.addLiquidity(address(reward), address(tgr), 5_000_000e18, 5_000_000e18, 0, 0, address(this), block.timestamp + 3600);

        tgr.mint(alice, 1_000_000e18);
        tgr.mint(bob, 1_000_000e18);
        reward.mint(address(vault), 100_000e18);

        vm.startPrank(alice);
        tgr.approve(address(vault), type(uint256).max);
        vm.stopPrank();
        vm.startPrank(bob);
        tgr.approve(address(vault), type(uint256).max);
        vm.stopPrank();

        vault.setKeeper(keeper, true);
    }

    // ---- Construction ------------------------------------------------------

    function testVaultRecordsMetadata() public view {
        assertEq(vault.name(), "Tiger TGR Vault", "name");
        assertEq(vault.symbol(), "vtTGR", "symbol");
        assertEq(vault.decimals(), 18, "decimals");
        assertEq(address(vault.asset()), address(tgr), "asset");
    }

    function testConstructorRejectsZeroAddresses() public {
        vm.expectRevert("zero address");
        new TigerSwapVault(address(0), address(router), address(factory), "n", "s");
    }

    // ---- Deposits ----------------------------------------------------------

    function testFirstDepositMintsSharesMinusLockedMinimum() public {
        vm.prank(alice);
        uint256 shares = vault.deposit(10_000e18, alice);

        assertEq(shares, 10_000e18 - 1000, "shares exclude the locked minimum");
        assertEq(vault.totalSupply(), 10_000e18, "supply includes the locked minimum");
        assertEq(vault.totalAssets(), 10_000e18, "assets held");
        assertEq(vault.balanceOf(alice), shares, "alice holds shares");
    }

    function testDepositRejectsZero() public {
        vm.prank(alice);
        vm.expectRevert("amount required");
        vault.deposit(0, alice);
    }

    function testDepositRejectsZeroReceiver() public {
        vm.prank(alice);
        vm.expectRevert("zero receiver");
        vault.deposit(1000e18, address(0));
    }

    function testDepositMintsProportionalSharesForLaterDepositor() public {
        vm.prank(alice);
        vault.deposit(10_000e18, alice);

        vm.prank(bob);
        uint256 shares = vault.deposit(5_000e18, bob);

        // Bob put in half of Alice's amount at the same share price.
        assertApproxEqRel(shares, vault.balanceOf(alice) / 2, 0.001e18, "proportional shares");
    }

    // ---- Withdrawals -------------------------------------------------------

    function testWithdrawReturnsAssets() public {
        vm.prank(alice);
        vault.deposit(10_000e18, alice);

        uint256 before = tgr.balanceOf(alice);
        vm.prank(alice);
        vault.withdraw(4_000e18, alice, alice);

        assertEq(tgr.balanceOf(alice) - before, 4_000e18, "assets returned");
        assertEq(vault.totalAssets(), 6_000e18, "assets reduced");
    }

    function testRedeemReturnsProportionalAssets() public {
        vm.prank(alice);
        uint256 shares = vault.deposit(10_000e18, alice);

        uint256 before = tgr.balanceOf(alice);
        vm.prank(alice);
        uint256 assets = vault.redeem(shares, alice, alice);

        assertEq(tgr.balanceOf(alice) - before, assets, "assets delivered");
        assertApproxEqRel(assets, 10_000e18 - 1000, 0.001e18, "full principal back");
    }

    function testWithdrawRejectsInsufficientAssets() public {
        vm.prank(alice);
        vault.deposit(10_000e18, alice);

        vm.prank(alice);
        vm.expectRevert("insufficient assets");
        vault.withdraw(1_000_000e18, alice, alice);
    }

    function testWithdrawOnBehalfRequiresAllowance() public {
        vm.prank(alice);
        vault.deposit(10_000e18, alice);

        vm.prank(bob);
        vm.expectRevert("insufficient allowance");
        vault.withdraw(1_000e18, bob, alice);
    }

    function testWithdrawOnBehalfWithAllowanceSucceeds() public {
        vm.prank(alice);
        vault.deposit(10_000e18, alice);

        vm.prank(alice);
        vault.approve(bob, type(uint256).max);

        uint256 before = tgr.balanceOf(bob);
        vm.prank(bob);
        vault.withdraw(1_000e18, bob, alice);

        assertEq(tgr.balanceOf(bob) - before, 1_000e18, "bob received the withdrawal");
    }

    // ---- Compounding -------------------------------------------------------

    function testHarvestRaisesSharePriceWithoutMinting() public {
        vm.prank(alice);
        vault.deposit(100_000e18, alice);

        uint256 supplyBefore = vault.totalSupply();
        uint256 aliceShares = vault.balanceOf(alice);
        uint256 assetsBefore = vault.totalAssets();

        address[] memory path = new address[](2);
        path[0] = address(reward);
        path[1] = address(tgr);

        vm.prank(keeper);
        uint256 added = vault.harvest(address(reward), 100_000e18, path, 0);

        assertGt(added, 0, "assets added");
        assertEq(vault.totalSupply(), supplyBefore, "no new shares minted");
        assertEq(vault.balanceOf(alice), aliceShares, "holder share count unchanged");
        assertGt(vault.totalAssets(), assetsBefore, "total assets grew");
    }

    function testHarvestIncreasesRedeemableValue() public {
        vm.prank(alice);
        vault.deposit(100_000e18, alice);

        uint256 shares = vault.balanceOf(alice);
        uint256 redeemBefore = vault.previewRedeem(shares);

        address[] memory path = new address[](2);
        path[0] = address(reward);
        path[1] = address(tgr);

        vm.prank(keeper);
        vault.harvest(address(reward), 100_000e18, path, 0);

        uint256 redeemAfter = vault.previewRedeem(shares);
        assertGt(redeemAfter, redeemBefore, "each share is worth more after harvest");
    }

    function testHarvestIsProportionalAcrossDepositors() public {
        vm.prank(alice);
        vault.deposit(100_000e18, alice);
        vm.prank(bob);
        vault.deposit(100_000e18, bob);

        uint256 aliceShares = vault.balanceOf(alice);
        uint256 bobShares = vault.balanceOf(bob);

        address[] memory path = new address[](2);
        path[0] = address(reward);
        path[1] = address(tgr);
        vm.prank(keeper);
        vault.harvest(address(reward), 100_000e18, path, 0);

        uint256 aliceValue = vault.previewRedeem(aliceShares);
        uint256 bobValue = vault.previewRedeem(bobShares);
        assertApproxEqRel(aliceValue, bobValue, 0.001e18, "gains split proportionally");
    }

    function testHarvestRejectsNonKeeper() public {
        address[] memory path = new address[](2);
        path[0] = address(reward);
        path[1] = address(tgr);

        vm.prank(alice);
        vm.expectRevert("not keeper");
        vault.harvest(address(reward), 1e18, path, 0);
    }

    function testHarvestRejectsRewardEqualToAsset() public {
        address[] memory path = new address[](2);
        path[0] = address(tgr);
        path[1] = address(tgr);

        vm.prank(keeper);
        vm.expectRevert("reward is the asset");
        vault.harvest(address(tgr), 1e18, path, 0);
    }

    function testHarvestRejectsWrongPathEndpoints() public {
        address[] memory path = new address[](2);
        path[0] = address(tgr);
        path[1] = address(reward);

        vm.prank(keeper);
        vm.expectRevert("path start mismatch");
        vault.harvest(address(reward), 1e18, path, 0);
    }

    function testHarvestRejectsShortPath() public {
        address[] memory path = new address[](1);
        path[0] = address(reward);

        vm.prank(keeper);
        vm.expectRevert("bad path");
        vault.harvest(address(reward), 1e18, path, 0);
    }

    // ---- Share token -------------------------------------------------------

    function testShareTransfers() public {
        vm.prank(alice);
        vault.deposit(10_000e18, alice);

        uint256 shares = vault.balanceOf(alice) / 2;
        vm.prank(alice);
        vault.transfer(bob, shares);

        assertEq(vault.balanceOf(bob), shares, "shares transferred");
    }

    function testShareTransferRejectsInsufficientBalance() public {
        vm.prank(bob);
        vm.expectRevert("insufficient balance");
        vault.transfer(alice, 1e18);
    }

    function testApproveAndTransferFrom() public {
        vm.prank(alice);
        vault.deposit(10_000e18, alice);

        uint256 shares = vault.balanceOf(alice) / 2;
        vm.prank(alice);
        vault.approve(bob, shares);
        vm.prank(bob);
        vault.transferFrom(alice, bob, shares);

        assertEq(vault.balanceOf(bob), shares, "transferred via allowance");
    }

    function testTransferFromRejectsInsufficientAllowance() public {
        vm.prank(alice);
        vault.deposit(10_000e18, alice);

        vm.prank(bob);
        vm.expectRevert("insufficient allowance");
        vault.transferFrom(alice, bob, 1e18);
    }

    // ---- Admin -------------------------------------------------------------

    function testKeeperCanBeRevoked() public {
        vault.setKeeper(keeper, false);

        address[] memory path = new address[](2);
        path[0] = address(reward);
        path[1] = address(tgr);

        vm.prank(keeper);
        vm.expectRevert("not keeper");
        vault.harvest(address(reward), 1e18, path, 0);
    }

    function testCannotSweepTheAsset() public {
        vm.prank(alice);
        vault.deposit(10_000e18, alice);

        vm.expectRevert("cannot sweep the asset");
        vault.sweep(address(tgr), address(this), 1e18);
    }

    function testSweepRescuesOtherTokens() public {
        reward.mint(address(vault), 500e18);
        uint256 before = reward.balanceOf(address(this));
        vault.sweep(address(reward), address(this), 500e18);
        assertEq(reward.balanceOf(address(this)) - before, 500e18, "reward rescued");
    }

    function testOwnershipTwoStep() public {
        vault.transferOwnership(bob);
        assertEq(vault.owner(), address(this), "not yet transferred");
        vm.prank(bob);
        vault.acceptOwnership();
        assertEq(vault.owner(), bob, "transferred");
    }

    function testOnlyOwnerCanSetKeeper() public {
        vm.prank(alice);
        vm.expectRevert("not owner");
        vault.setKeeper(alice, true);
    }

    // ---- Preview functions -------------------------------------------------

    function testPreviewDepositMatchesActual() public {
        vm.prank(alice);
        vault.deposit(10_000e18, alice);

        uint256 predicted = vault.previewDeposit(5_000e18);
        vm.prank(bob);
        uint256 actual = vault.deposit(5_000e18, bob);

        assertEq(predicted, actual, "preview matches execution");
    }

    function testPreviewRedeemMatchesActual() public {
        vm.prank(alice);
        vault.deposit(10_000e18, alice);

        uint256 shares = vault.balanceOf(alice);
        uint256 predicted = vault.previewRedeem(shares);

        vm.prank(alice);
        uint256 actual = vault.redeem(shares, alice, alice);

        assertEq(predicted, actual, "preview matches execution");
    }

    function testPreviewWithdrawRoundsSharesUp() public {
        vm.prank(alice);
        vault.deposit(10_000e18, alice);

        uint256 shares = vault.previewWithdraw(1e18);
        // Rounding up means the share cost covers the requested assets.
        uint256 covered = (shares * vault.totalAssets()) / vault.totalSupply();
        assertGe(covered, 1e18, "shares round up to cover the withdrawal");
    }

    function testEmptyVaultPreviews() public view {
        // The first deposit mints assets minus the permanently locked minimum.
        assertEq(vault.previewDeposit(1000e18), 1000e18 - 1000, "first deposit excludes the minimum");
        assertEq(vault.previewRedeem(1000e18), 0, "no assets without supply");
        assertEq(vault.previewWithdraw(1000e18), 0, "no shares without supply");
    }
}
