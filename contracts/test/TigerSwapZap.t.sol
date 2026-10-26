// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapZap.sol";
import "../TigerSwapRouter.sol";
import "../TigerSwapFactory.sol";

/// @dev Real ERC-20 with balances, allowances and supply.
contract ZapERC20 {
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

/// @dev Real wrapped-native implementation: deposit locks ETH and credits WETH,
/// withdraw burns WETH and returns ETH.
contract ZapWETH is ZapERC20 {
    constructor() ZapERC20("Wrapped Ether", "WETH") {}

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

    receive() external payable {
        totalSupply += msg.value;
        balanceOf[msg.sender] += msg.value;
        emit Transfer(address(0), msg.sender, msg.value);
    }
}

contract TigerSwapZapTest is Test {
    TigerSwapZap public zap;
    TigerSwapRouter public router;
    TigerSwapFactory public factory;
    ZapERC20 public tgr;
    ZapERC20 public rusd;
    ZapWETH public weth;

    address public user = address(0xBEEF);

    uint256 constant DEADLINE_OFFSET = 3600;

    function setUp() public {
        tgr = new ZapERC20("TigerSwap", "TGR");
        rusd = new ZapERC20("RUSD", "RUSD");
        weth = new ZapWETH();

        factory = new TigerSwapFactory();
        router = new TigerSwapRouter(address(factory), address(weth));
        zap = new TigerSwapZap(address(router), address(factory));

        tgr.mint(user, 1_000_000e18);
        rusd.mint(user, 1_000_000e18);
        vm.deal(user, 1_000 ether);
        vm.prank(user);
        weth.deposit{value: 200 ether}();

        vm.startPrank(user);
        weth.approve(address(router), type(uint256).max);
        weth.approve(address(zap), type(uint256).max);
        tgr.approve(address(router), type(uint256).max);
        rusd.approve(address(router), type(uint256).max);
        tgr.approve(address(zap), type(uint256).max);
        rusd.approve(address(zap), type(uint256).max);
        vm.stopPrank();
    }

    function _seedPool(address a, address b, uint256 amountA, uint256 amountB) internal returns (address pool) {
        vm.prank(user);
        router.addLiquidity(a, b, amountA, amountB, 0, 0, user, block.timestamp + DEADLINE_OFFSET);
        pool = factory.getPoolAddress(a, b);
    }

    // ---- Construction ------------------------------------------------------

    function testZapReadsWethFromRouter() public view {
        assertEq(zap.WETH(), address(weth), "WETH discovered from router");
    }

    function testConstructorRejectsZeroAddresses() public {
        vm.expectRevert("zero address");
        new TigerSwapZap(address(0), address(factory));
    }

    function testReceiveOnlyAcceptsFromWeth() public {
        vm.deal(address(this), 1 ether);
        (bool ok,) = address(zap).call{value: 1 ether}("");
        assertFalse(ok, "plain ETH transfer rejected");
    }

    // ---- Optimal swap amount ----------------------------------------------

    function testOptimalSwapIsHalfForBalancedPool() public view {
        // Balanced reserves: the optimal swap is half the input.
        uint256 amountIn = 100e18;
        uint256 swapAmount = zap._optimalSwapAmount(amountIn, 1000e18);
        // The closed form is fee-free, so it lands a few percent under half.
        assertApproxEqRel(swapAmount, amountIn / 2, 0.05e18, "balanced pool swaps ~half");
    }

    function testOptimalSwapIsBelowTotalInput() public view {
        uint256 amountIn = 100e18;
        uint256 swapAmount = zap._optimalSwapAmount(amountIn, 1000e18);
        assertLt(swapAmount, amountIn, "never swaps the entire balance");
    }

    function testOptimalSwapIncreasesWithReserveRatio() public view {
        uint256 amountIn = 100e18;
        uint256 low = zap._optimalSwapAmount(amountIn, 100e18);
        uint256 high = zap._optimalSwapAmount(amountIn, 10_000e18);
        assertLt(low, high, "deeper in-reserve needs a larger swap");
    }

    // ---- Zap in ------------------------------------------------------------

    function testZapInProducesLpFromSingleToken() public {
        _seedPool(address(tgr), address(rusd), 100_000e18, 100_000e18);

        uint256 tgrBefore = tgr.balanceOf(user);
        vm.prank(user);
        uint256 lp = zap.zapIn(address(tgr), address(rusd), 1000e18, 0, block.timestamp + DEADLINE_OFFSET);

        assertGt(lp, 0, "LP minted");
        uint256 spent = tgrBefore - tgr.balanceOf(user);
        assertLe(spent, 1000e18, "never spends more than requested");
        assertGt(spent, 995e18, "at most a small residual is refunded");

        address pool = factory.getPoolAddress(address(tgr), address(rusd));
        uint256 lpBefore = IERC20(pool).balanceOf(user) - lp;
        assertEq(IERC20(pool).balanceOf(user), lpBefore + lp, "user holds the zapped LP");
        assertEq(IERC20(address(tgr)).balanceOf(address(zap)), 0, "no TGR stranded");
        assertEq(IERC20(address(rusd)).balanceOf(address(zap)), 0, "no RUSD stranded");
    }

    function testZapInMatchesTwoSidedDepositValue() public {
        address pool = _seedPool(address(tgr), address(rusd), 100_000e18, 100_000e18);

        vm.prank(user);
        uint256 lpViaZap = zap.zapIn(address(tgr), address(rusd), 1000e18, 0, block.timestamp + DEADLINE_OFFSET);

        // The pool holds 100000e18 per side with 100000e18 shares, so a 1000e18
        // single-sided zap that ends up near 500/500 should mint about 500e18.
        assertApproxEqRel(lpViaZap, 500e18, 0.05e18, "zap mints the expected share");
        assertGt(lpViaZap, 450e18, "zap does not lose meaningful value");
        assertGt(IERC20(pool).totalSupply(), 0, "pool has supply");
    }

    function testZapInRejectsIdenticalTokens() public {
        vm.prank(user);
        vm.expectRevert("identical tokens");
        zap.zapIn(address(tgr), address(tgr), 100e18, 0, block.timestamp + DEADLINE_OFFSET);
    }

    function testZapInRejectsZeroAmount() public {
        vm.prank(user);
        vm.expectRevert("amount required");
        zap.zapIn(address(tgr), address(rusd), 0, 0, block.timestamp + DEADLINE_OFFSET);
    }

    function testZapInRejectsExpiredDeadline() public {
        _seedPool(address(tgr), address(rusd), 100_000e18, 100_000e18);
        vm.prank(user);
        vm.expectRevert("expired");
        zap.zapIn(address(tgr), address(rusd), 100e18, 0, block.timestamp - 1);
    }

    function testZapInRejectsMissingPool() public {
        vm.prank(user);
        vm.expectRevert("pool does not exist");
        zap.zapIn(address(tgr), address(rusd), 100e18, 0, block.timestamp + DEADLINE_OFFSET);
    }

    function testZapInEnforcesMinimumLp() public {
        _seedPool(address(tgr), address(rusd), 100_000e18, 100_000e18);
        vm.prank(user);
        vm.expectRevert("slippage exceeded");
        zap.zapIn(address(tgr), address(rusd), 1000e18, 1_000_000e18, block.timestamp + DEADLINE_OFFSET);
    }

    function testZapInWorksOnSkewedPool() public {
        _seedPool(address(tgr), address(rusd), 100_000e18, 400_000e18);

        vm.prank(user);
        uint256 lp = zap.zapIn(address(tgr), address(rusd), 5000e18, 0, block.timestamp + DEADLINE_OFFSET);

        assertGt(lp, 0, "LP minted on a skewed pool");
        assertEq(IERC20(address(tgr)).balanceOf(address(zap)), 0, "no residue");
        assertEq(IERC20(address(rusd)).balanceOf(address(zap)), 0, "no residue");
    }

    // ---- Zap in ETH --------------------------------------------------------

    function testZapInETHRoutesThroughWeth() public {
        _seedPool(address(weth), address(tgr), 100 ether, 100_000e18);

        uint256 ethBefore = user.balance;
        vm.prank(user);
        uint256 lp = zap.zapInETH{value: 10 ether}(address(tgr), 0, block.timestamp + DEADLINE_OFFSET);

        assertGt(lp, 0, "LP minted from ETH");
        uint256 ethSpent = ethBefore - user.balance;
        assertLe(ethSpent, 10 ether, "never spends more than sent");
        assertGt(ethSpent, 9.9 ether, "at most a small residual is refunded");
        assertEq(IERC20(address(weth)).balanceOf(address(zap)), 0, "no WETH stranded");
        assertEq(IERC20(address(tgr)).balanceOf(address(zap)), 0, "no TGR stranded");
    }

    function testZapInETHRejectsWethTarget() public {
        vm.prank(user);
        vm.expectRevert("use zapIn for WETH");
        zap.zapInETH(address(weth), 0, block.timestamp + DEADLINE_OFFSET);
    }

    function testZapInETHRejectsZeroValue() public {
        vm.prank(user);
        vm.expectRevert("value required");
        zap.zapInETH(address(tgr), 0, block.timestamp + DEADLINE_OFFSET);
    }

    // ---- Zap out -----------------------------------------------------------

    function testZapOutReturnsSingleToken() public {
        address pool = _seedPool(address(tgr), address(rusd), 100_000e18, 100_000e18);

        vm.prank(user);
        uint256 lp = zap.zapIn(address(tgr), address(rusd), 1000e18, 0, block.timestamp + DEADLINE_OFFSET);

        uint256 lpBefore = IERC20(pool).balanceOf(user);
        vm.startPrank(user);
        IERC20(pool).approve(address(zap), lp);
        uint256 rusdBefore = rusd.balanceOf(user);
        uint256 out = zap.zapOut(address(rusd), address(tgr), lp, 0, block.timestamp + DEADLINE_OFFSET);
        vm.stopPrank();

        assertGt(out, 0, "single token returned");
        assertEq(rusd.balanceOf(user) - rusdBefore, out, "payout delivered");
        assertEq(lpBefore - IERC20(pool).balanceOf(user), lp, "exactly the zapped LP consumed");
        assertEq(IERC20(address(tgr)).balanceOf(address(zap)), 0, "no TGR stranded");
        assertEq(IERC20(address(rusd)).balanceOf(address(zap)), 0, "no RUSD stranded");
    }

    function testZapOutEnforcesMinimumOut() public {
        address pool = _seedPool(address(tgr), address(rusd), 100_000e18, 100_000e18);
        vm.prank(user);
        uint256 lp = zap.zapIn(address(tgr), address(rusd), 1000e18, 0, block.timestamp + DEADLINE_OFFSET);

        vm.startPrank(user);
        IERC20(pool).approve(address(zap), lp);
        vm.expectRevert("slippage exceeded");
        zap.zapOut(address(rusd), address(tgr), lp, 1_000_000e18, block.timestamp + DEADLINE_OFFSET);
        vm.stopPrank();
    }

    function testZapOutRejectsIdenticalTokens() public {
        vm.prank(user);
        vm.expectRevert("identical tokens");
        zap.zapOut(address(tgr), address(tgr), 1e18, 0, block.timestamp + DEADLINE_OFFSET);
    }

    function testZapOutRejectsMissingPool() public {
        vm.prank(user);
        vm.expectRevert("pool does not exist");
        zap.zapOut(address(tgr), address(rusd), 1e18, 0, block.timestamp + DEADLINE_OFFSET);
    }

    // ---- Zap out ETH -------------------------------------------------------

    function testZapOutETHReturnsNative() public {
        address pool = _seedPool(address(weth), address(tgr), 100 ether, 100_000e18);

        vm.prank(user);
        uint256 lp = zap.zapInETH{value: 10 ether}(address(tgr), 0, block.timestamp + DEADLINE_OFFSET);

        vm.startPrank(user);
        IERC20(pool).approve(address(zap), lp);
        uint256 ethBefore = user.balance;
        uint256 out = zap.zapOutETH(address(tgr), lp, 0, block.timestamp + DEADLINE_OFFSET);
        vm.stopPrank();

        assertGt(out, 0, "ETH returned");
        assertEq(user.balance - ethBefore, out, "ETH delivered");
        assertEq(IERC20(address(weth)).balanceOf(address(zap)), 0, "no WETH stranded");
        assertEq(address(zap).balance, 0, "no ETH stranded");
    }

    function testZapOutETHRejectsWethToken() public {
        vm.prank(user);
        vm.expectRevert("use zapOut for WETH");
        zap.zapOutETH(address(weth), 1e18, 0, block.timestamp + DEADLINE_OFFSET);
    }

    // ---- Round trip --------------------------------------------------------

    function testZapRoundTripDoesNotGainValue() public {
        address pool = _seedPool(address(tgr), address(rusd), 100_000e18, 100_000e18);

        uint256 rusdBefore = rusd.balanceOf(user);

        vm.startPrank(user);
        uint256 lp = zap.zapIn(address(tgr), address(rusd), 1000e18, 0, block.timestamp + DEADLINE_OFFSET);
        IERC20(pool).approve(address(zap), lp);
        uint256 out = zap.zapOut(address(rusd), address(tgr), lp, 0, block.timestamp + DEADLINE_OFFSET);
        vm.stopPrank();

        // Round-tripping through a fee-charging AMM must lose a little, never gain.
        assertLt(rusd.balanceOf(user) - rusdBefore, 1000e18, "round trip cannot profit");
        assertGt(out, 950e18, "loss is only fees, not a large leak");
    }

    // ---- Admin -------------------------------------------------------------

    function testSweepRecoversStrayTokens() public {
        tgr.mint(address(zap), 500e18);

        zap.sweep(address(tgr), address(this), 500e18);
        assertEq(tgr.balanceOf(address(this)), 500e18, "stray tokens recovered");
    }

    function testOnlyOwnerCanSweep() public {
        tgr.mint(address(zap), 500e18);
        vm.prank(user);
        vm.expectRevert("not owner");
        zap.sweep(address(tgr), user, 500e18);
    }

    function testOwnershipTransfer() public {
        zap.transferOwnership(user);
        assertEq(zap.owner(), user, "ownership moved");
    }
}
