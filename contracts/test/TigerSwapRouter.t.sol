// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapRouter.sol";
import "../TigerSwapFactory.sol";
import "@openzeppelin/contracts/mocks/ERC20Mock.sol";

contract TigerSwapRouterTest is Test {
    TigerSwapRouter public router;
    TigerSwapFactory public factory;
    ERC20Mock public tokenA;
    ERC20Mock public tokenB;
    ERC20Mock public WETH;

    address public user1 = address(0x1);
    address public user2 = address(0x2);

    uint256 constant MINIMUM_LIQUIDITY = 1000;

    function setUp() public {
        // Deploy mock tokens
        tokenA = new ERC20Mock("Token A", "TKA", 18);
        tokenB = new ERC20Mock("Token B", "TKB", 18);
        WETH = new ERC20Mock("Wrapped Ether", "WETH", 18);

        // Deploy factory
        factory = new TigerSwapFactory(address(this));

        // Deploy router
        router = new TigerSwapFactory(address(factory)).getRouter();
    }

    function testAddLiquidity() public {
        tokenA.mint(user1, 1000e18);
        tokenB.mint(user1, 1000e18);

        tokenA.transfer(address(router), 1000e18);
        tokenB.transfer(address(router), 1000e18);

        (uint256 amountA, uint256 amountB, uint256 liquidity) = router.addLiquidity(
            address(tokenA),
            address(tokenB),
            1000e18,
            1000e18,
            900e18,
            900e18,
            user1,
            block.timestamp + 60
        );

        assertEq(amountA, 1000e18);
        assertEq(amountB, 1000e18);
        assertGt(liquidity, 0);
    }

    function testRemoveLiquidity() public {
        // First add liquidity
        testAddLiquidity();

        // Get the pair address
        address pair = factory.getPair(address(tokenA), address(tokenB));
        ERC20Mock(pair).transfer(address(router), 100e18);

        (uint256 amountA, uint256 amountB) = router.removeLiquidity(
            address(tokenA),
            address(tokenB),
            100e18,
            90e18,
            90e18,
            user1,
            block.timestamp + 60
        );

        assertGt(amountA, 0);
        assertGt(amountB, 0);
    }

    function testSwapExactTokensForTokens() public {
        // Add liquidity first
        testAddLiquidity();

        // User swaps
        tokenA.mint(user2, 100e18);
        vm.startPrank(user2);
        tokenA.transfer(address(router), 100e18);

        uint256[] memory amounts = router.swapExactTokensForTokens(
            100e18,
            90e18,
            _getPath(address(tokenA), address(tokenB)),
            user2,
            block.timestamp + 60
        );
        vm.stopPrank();

        // Verify swap occurred
        assertGt(amounts[amounts.length - 1], 0);
    }

    function testSwapTokensForExactTokens() public {
        // Add liquidity first
        testAddLiquidity();

        // User swaps
        tokenA.mint(user2, 1000e18);
        vm.startPrank(user2);
        tokenA.transfer(address(router), 1000e18);

        uint256[] memory amounts = router.swapTokensForExactTokens(
            100e18,
            1000e18,
            _getPath(address(tokenA), address(tokenB)),
            user2,
            block.timestamp + 60
        );
        vm.stopPrank();

        // Verify swap occurred
        assertGt(amounts[0], 0);
    }

    function testQuote() public {
        (uint256 amountB) = router.quote(
            1000e18,
            1000e18,
            1000e18
        );

        assertEq(amountB, 1000e18);
    }

    function testGetAmountsOut() public {
        uint256[] memory amounts = router.getAmountsOut(
            1000e18,
            _getPath(address(tokenA), address(tokenB))
        );

        assertGt(amounts[amounts.length - 1], 0);
    }

    function testGetAmountsIn() public {
        uint256[] memory amounts = router.getAmountsIn(
            1000e18,
            _getPath(address(tokenA), address(tokenB))
        );

        assertGt(amounts[0], 0);
    }

    function testPairExists() public {
        address pair = factory.createPair(address(tokenA), address(tokenB));
        assertTrue(pair != address(0));
    }

    function testLiquidityPoolCreated() public {
        address pair = factory.createPair(address(tokenA), address(tokenB));
        
        // Verify pair has liquidity (after initial mint)
        assertTrue(IERC20(pair).totalSupply() >= MINIMUM_LIQUIDITY);
    }

    function testMultiHopSwap() public {
        // Create a third token for multi-hop
        ERC20Mock tokenC = new ERC20Mock("Token C", "TKC", 18);
        
        // Create pairs
        factory.createPair(address(tokenA), address(tokenB));
        factory.createPair(address(tokenB), address(tokenC));

        // Add liquidity to both pairs
        tokenA.mint(address(this), 1000e18);
        tokenB.mint(address(this), 1000e18);
        tokenC.mint(address(this), 1000e18);

        tokenA.transfer(address(router), 500e18);
        tokenB.transfer(address(router), 500e18);
        tokenC.transfer(address(router), 500e18);

        router.addLiquidity(
            address(tokenA),
            address(tokenB),
            500e18,
            500e18,
            400e18,
            400e18,
            user1,
            block.timestamp + 60
        );

        router.addLiquidity(
            address(tokenB),
            address(tokenC),
            500e18,
            500e18,
            400e18,
            400e18,
            user1,
            block.timestamp + 60
        );

        // Swap A -> C (multi-hop through B)
        tokenA.mint(user2, 100e18);
        vm.startPrank(user2);
        tokenA.transfer(address(router), 100e18);

        uint256[] memory amounts = router.swapExactTokensForTokens(
            100e18,
            50e18,
            _getPath(address(tokenA), address(tokenB), address(tokenC)),
            user2,
            block.timestamp + 60
        );
        vm.stopPrank();

        assertGt(amounts[amounts.length - 1], 0);
    }

    // Helper functions
    function _getPath(address tokenA, address tokenB) internal pure returns (address[] memory) {
        address[] memory path = new address[](2);
        path[0] = tokenA;
        path[1] = tokenB;
        return path;
    }

    function _getPath(address tokenA, address tokenB, address tokenC) internal pure returns (address[] memory) {
        address[] memory path = new address[](3);
        path[0] = tokenA;
        path[1] = tokenB;
        path[2] = tokenC;
        return path;
    }

    receive() external payable {}
}

interface IERC20 {
    function totalSupply() external view returns (uint256);
    function balanceOf(address account) external view returns (uint256);
    function transfer(address to, uint256 amount) external returns (bool);
    function allowance(address owner, address spender) external view returns (uint256);
    function approve(address spender, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
}
