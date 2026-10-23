// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapRouter.sol";
import "../TigerSwapFactory.sol";

/// @dev Minimal ERC-20 used by the AMM tests. Not a mock of TigerSwap logic:
/// it is a real token implementation that stores balances and allowances.
contract TestERC20 {
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

contract TigerSwapRouterTest is Test {
    TigerSwapRouter public router;
    TigerSwapFactory public factory;
    TestERC20 public tokenA;
    TestERC20 public tokenB;
    TestERC20 public tokenC;
    TestERC20 public WETH;

    address public user1 = address(0x1);
    address public user2 = address(0x2);

    function setUp() public {
        tokenA = new TestERC20("Token A", "TKA");
        tokenB = new TestERC20("Token B", "TKB");
        tokenC = new TestERC20("Token C", "TKC");
        WETH = new TestERC20("Wrapped Ether", "WETH");

        factory = new TigerSwapFactory();
        router = new TigerSwapRouter(address(factory), address(WETH));

        tokenA.mint(user1, 1_000_000e18);
        tokenB.mint(user1, 1_000_000e18);
        tokenC.mint(user1, 1_000_000e18);

        vm.startPrank(user1);
        tokenA.approve(address(router), type(uint256).max);
        tokenB.approve(address(router), type(uint256).max);
        tokenC.approve(address(router), type(uint256).max);
        vm.stopPrank();
    }

    function _addLiquidity(address a, address b, uint256 amountA, uint256 amountB) internal {
        vm.prank(user1);
        router.addLiquidity(a, b, amountA, amountB, 0, 0, user1, block.timestamp + 60);
    }

    function testAddLiquidityCreatesPoolAndMintsLp() public {
        _addLiquidity(address(tokenA), address(tokenB), 1000e18, 1000e18);

        address pool = factory.getPoolAddress(address(tokenA), address(tokenB));
        assertTrue(pool != address(0));

        uint256 lp = ILpToken(pool).balanceOf(user1);
        assertGt(lp, 0);

        // Initial liquidity is locked forever.
        assertEq(ILpToken(pool).balanceOf(address(0)), 1000);
        assertEq(ILpToken(pool).totalSupply(), lp + 1000);
    }

    function testGetReservesReflectsDeposits() public {
        _addLiquidity(address(tokenA), address(tokenB), 1000e18, 1000e18);
        address pool = factory.getPoolAddress(address(tokenA), address(tokenB));

        (uint112 r0, uint112 r1, ) = ILpToken(pool).getReserves();
        assertEq(uint256(r0), 1000e18);
        assertEq(uint256(r1), 1000e18);
    }

    function testGetAmountsOutAppliesFee() public {
        _addLiquidity(address(tokenA), address(tokenB), 1000e18, 1000e18);

        address[] memory path = new address[](2);
        path[0] = address(tokenA);
        path[1] = address(tokenB);

        uint256[] memory amounts = router.getAmountsOut(100e18, path);
        // Constant product with a 0.3% fee on a balanced 1000e18/1000e18 pool.
        uint256 amountInWithFee = 100e18 * 9970;
        uint256 expected = (amountInWithFee * 1000e18) / (1000e18 * 10000 + amountInWithFee);
        assertEq(amounts[1], expected);
        assertLt(amounts[1], 100e18);
    }

    function testGetAmountsOutRevertsWithoutPool() public {
        address[] memory path = new address[](2);
        path[0] = address(tokenA);
        path[1] = address(tokenB);

        vm.expectRevert(bytes("TigerSwap: POOL_NOT_FOUND"));
        router.getAmountsOut(1e18, path);
    }

    function testGetAmountsInMatchesOut() public {
        _addLiquidity(address(tokenA), address(tokenB), 1000e18, 1000e18);

        address[] memory path = new address[](2);
        path[0] = address(tokenA);
        path[1] = address(tokenB);

        uint256[] memory outAmounts = router.getAmountsOut(100e18, path);
        uint256[] memory inAmounts = router.getAmountsIn(outAmounts[1], path);

        // The input needed to get exactly the output is within a rounding unit.
        assertApproxEqAbs(inAmounts[0], 100e18, 1e6);
    }

    function testSwapMovesTokensAndKeepsReserves() public {
        _addLiquidity(address(tokenA), address(tokenB), 1000e18, 1000e18);
        address pool = factory.getPoolAddress(address(tokenA), address(tokenB));

        uint256 balanceBefore = tokenB.balanceOf(user2);

        address[] memory path = new address[](2);
        path[0] = address(tokenA);
        path[1] = address(tokenB);

        vm.prank(user1);
        uint256[] memory amounts = router.swapExactTokensForTokens(
            100e18, 0, path, user2, block.timestamp + 60
        );

        uint256 received = tokenB.balanceOf(user2) - balanceBefore;
        assertEq(received, amounts[1]);
        assertGt(received, 0);

        // The input side grew by exactly the amount in; the output side shrank
        // by exactly the amount out. Which slot is which depends on address
        // ordering, so resolve it rather than assuming.
        (uint112 r0, uint112 r1, ) = ILpToken(pool).getReserves();
        (uint256 reserveA, uint256 reserveB) = address(tokenA) < address(tokenB)
            ? (uint256(r0), uint256(r1))
            : (uint256(r1), uint256(r0));

        assertEq(reserveA, 1100e18);
        assertEq(reserveB, 1000e18 - received);
        assertEq(tokenA.balanceOf(pool), reserveA);
        assertEq(tokenB.balanceOf(pool), reserveB);
        // Constant product must not decrease.
        assertGe(reserveA * reserveB, 1000e18 * 1000e18);
    }

    function testSwapRevertsOnInsufficientOutput() public {
        _addLiquidity(address(tokenA), address(tokenB), 1000e18, 1000e18);

        address[] memory path = new address[](2);
        path[0] = address(tokenA);
        path[1] = address(tokenB);

        vm.prank(user1);
        vm.expectRevert(bytes("TigerSwap: INSUFFICIENT_OUTPUT_AMOUNT"));
        router.swapExactTokensForTokens(100e18, 1000e18, path, user2, block.timestamp + 60);
    }

    function testMultiHopSwap() public {
        _addLiquidity(address(tokenA), address(tokenB), 1000e18, 1000e18);
        _addLiquidity(address(tokenB), address(tokenC), 1000e18, 1000e18);

        address[] memory path = new address[](3);
        path[0] = address(tokenA);
        path[1] = address(tokenB);
        path[2] = address(tokenC);

        uint256 balanceBefore = tokenC.balanceOf(user2);
        vm.prank(user1);
        router.swapExactTokensForTokens(100e18, 0, path, user2, block.timestamp + 60);

        assertGt(tokenC.balanceOf(user2) - balanceBefore, 0);
    }

    function testRemoveLiquidityReturnsTokens() public {
        _addLiquidity(address(tokenA), address(tokenB), 1000e18, 1000e18);
        address pool = factory.getPoolAddress(address(tokenA), address(tokenB));

        uint256 lp = ILpToken(pool).balanceOf(user1);
        uint256 aBefore = tokenA.balanceOf(user1);
        uint256 bBefore = tokenB.balanceOf(user1);

        vm.startPrank(user1);
        ILpToken(pool).approve(address(router), lp / 2);
        router.removeLiquidity(address(tokenA), address(tokenB), lp / 2, 0, 0, user1, block.timestamp + 60);
        vm.stopPrank();

        assertGt(tokenA.balanceOf(user1) - aBefore, 0);
        assertGt(tokenB.balanceOf(user1) - bBefore, 0);
    }

    function testGetAmountsInRevertsWithoutPool() public {
        address[] memory path = new address[](2);
        path[0] = address(tokenA);
        path[1] = address(tokenB);

        vm.expectRevert(bytes("TigerSwap: POOL_NOT_FOUND"));
        router.getAmountsIn(1e18, path);
    }

    receive() external payable {}
}

interface ILpToken {
    function balanceOf(address account) external view returns (uint256);
    function totalSupply() external view returns (uint256);
    function approve(address spender, uint256 value) external returns (bool);
    function getReserves() external view returns (uint112 reserve0, uint112 reserve1, uint32 blockTimestampLast);
}
