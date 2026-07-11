// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerSwap Test Suite
 * @notice Comprehensive tests for TigerSwap contracts
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

import "forge-std/Test.sol";
import "../src/TigerSwap.sol";
import "../src/TigerPoolV3.sol";

/**
 * @title TigerSwapCoreTest
 * @dev Core functionality tests
 */
contract TigerSwapCoreTest is Test {
    TigerSwap public tigerSwap;
    TigerPoolV3 public pool;
    
    address public user1 = address(0x1);
    address public user2 = address(0x2);
    address public user3 = address(0x3);
    
    address public token0;
    address public token1;
    
    uint256 constant INITIAL_LIQUIDITY = 1000000e18;
    
    function setUp() public {
        // Deploy contracts
        tigerSwap = new TigerSwap();
        
        // Create mock tokens
        token0 = address(new MockERC20("Token A", "TKNA", 18));
        token1 = address(new MockERC20("Token B", "TKNB", 18));
        
        // Mint tokens to users
        MockERC20(token0).mint(user1, INITIAL_LIQUIDITY);
        MockERC20(token0).mint(user2, INITIAL_LIQUIDITY);
        MockERC20(token1).mint(user1, INITIAL_LIQUIDITY);
        MockERC20(token1).mint(user2, INITIAL_LIQUIDITY);
    }
    
    function testAddLiquidity() public {
        vm.startPrank(user1);
        
        // Approve tokens
        MockERC20(token0).approve(address(tigerSwap), INITIAL_LIQUIDITY);
        MockERC20(token1).approve(address(tigerSwap), INITIAL_LIQUIDITY);
        
        // Add liquidity
        (uint256 amount0, uint256 amount1, uint256 liquidity) = tigerSwap.addLiquidity(
            token0,
            token1,
            1000e18,
            1000e18,
            0,
            0
        );
        
        assertGt(liquidity, 0);
        assertEq(amount0, 1000e18);
        assertEq(amount1, 1000e18);
        
        vm.stopPrank();
    }
    
    function testSwap() public {
        // Setup liquidity first
        vm.startPrank(user1);
        MockERC20(token0).approve(address(tigerSwap), INITIAL_LIQUIDITY);
        MockERC20(token1).approve(address(tigerSwap), INITIAL_LIQUIDITY);
        
        tigerSwap.addLiquidity(token0, token1, 10000e18, 10000e18, 0, 0);
        vm.stopPrank();
        
        // Perform swap
        vm.startPrank(user2);
        MockERC20(token0).approve(address(tigerSwap), 1000e18);
        
        uint256 amountOut = tigerSwap.swapExactTokenForToken(
            token0,
            token1,
            1000e18,
            1
        );
        
        assertGt(amountOut, 0);
        
        vm.stopPrank();
    }
    
    function testRemoveLiquidity() public {
        // Add liquidity first
        vm.startPrank(user1);
        MockERC20(token0).approve(address(tigerSwap), INITIAL_LIQUIDITY);
        MockERC20(token1).approve(address(tigerSwap), INITIAL_LIQUIDITY);
        
        (,, uint256 liquidity) = tigerSwap.addLiquidity(
            token0,
            token1,
            1000e18,
            1000e18,
            0,
            0
        );
        
        // Approve LP tokens for removal
        tigerSwap.approve(address(tigerSwap), liquidity);
        
        // Remove liquidity
        (uint256 amount0, uint256 amount1) = tigerSwap.removeLiquidity(
            token0,
            token1,
            liquidity,
            0,
            0
        );
        
        assertEq(amount0, 1000e18);
        assertEq(amount1, 1000e18);
        
        vm.stopPrank();
    }
    
    function testSwapWithSlippage() public {
        // Setup
        vm.startPrank(user1);
        MockERC20(token0).approve(address(tigerSwap), INITIAL_LIQUIDITY);
        MockERC20(token1).approve(address(tigerSwap), INITIAL_LIQUIDITY);
        
        tigerSwap.addLiquidity(token0, token1, 10000e18, 10000e18, 0, 0);
        vm.stopPrank();
        
        // Swap with high slippage tolerance should succeed
        vm.startPrank(user2);
        MockERC20(token0).approve(address(tigerSwap), 1000e18);
        
        uint256 amountOut = tigerSwap.swapExactTokenForToken(
            token0,
            token1,
            1000e18,
            1 // Very low min - should work
        );
        
        assertGt(amountOut, 0);
        
        vm.stopPrank();
    }
    
    function testSwapRevertsOnInsufficientOutput() public {
        // Setup
        vm.startPrank(user1);
        MockERC20(token0).approve(address(tigerSwap), INITIAL_LIQUIDITY);
        MockERC20(token1).approve(address(tigerSwap), INITIAL_LIQUIDITY);
        
        tigerSwap.addLiquidity(token0, token1, 10000e18, 10000e18, 0, 0);
        vm.stopPrank();
        
        // Swap with zero min should fail
        vm.startPrank(user2);
        MockERC20(token0).approve(address(tigerSwap), 1000e18);
        
        vm.expectRevert("Insufficient output amount");
        tigerSwap.swapExactTokenForToken(
            token0,
            token1,
            1000e18,
            type(uint256).max // Impossible min
        );
        
        vm.stopPrank();
    }
    
    function testDoubleAddLiquidity() public {
        vm.startPrank(user1);
        
        MockERC20(token0).approve(address(tigerSwap), INITIAL_LIQUIDITY);
        MockERC20(token1).approve(address(tigerSwap), INITIAL_LIQUIDITY);
        
        // First add
        (,, uint256 liquidity1) = tigerSwap.addLiquidity(
            token0,
            token1,
            1000e18,
            1000e18,
            0,
            0
        );
        
        // Second add
        (,, uint256 liquidity2) = tigerSwap.addLiquidity(
            token0,
            token1,
            1000e18,
            1000e18,
            0,
            0
        );
        
        assertGt(liquidity1, 0);
        assertGt(liquidity2, 0);
        assertGt(liquidity2, liquidity1);
        
        vm.stopPrank();
    }
}

/**
 * @title MockERC20
 * @dev Mock ERC20 token for testing
 */
contract MockERC20 is ERC20 {
    uint8 private _decimals;
    
    constructor(
        string memory _name,
        string memory _symbol,
        uint8 __decimals
    ) ERC20(_name, _symbol) {
        _decimals = __decimals;
    }
    
    function decimals() public view override returns (uint8) {
        return _decimals;
    }
    
    function mint(address to, uint256 amount) external {
        _mint(to, amount);
    }
    
    function burn(address from, uint256 amount) external {
        _burn(from, amount);
    }
}
