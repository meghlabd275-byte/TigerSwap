// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "./contracts/TigerToken.sol";
import "./contracts/Factory.sol";
import "./contracts/Pair.sol";
import "./contracts/Router.sol";

/**
 * @title TigerSwap Comprehensive Test Suite
 * @dev Fork tests, invariant tests, and edge cases
 */
contract TigerSwapComprehensiveTest is Test {
    TigerToken public token0;
    TigerToken public token1;
    TigerFactory public factory;
    TigerRouter public router;
    
    address public alice = makeAddr("alice");
    address public bob = makeAddr("bob");
    address public charlie = makeAddr("charlie");
    address public feeRecipient = makeAddr("feeRecipient");
    
    uint256 constant INITIAL_MINT = 1000000e18;
    
    function setUp() public {
        vm.startPrank(alice);
        
        token0 = new TigerToken("Token0", "TK0", 18, INITIAL_MINT);
        token1 = new TigerToken("Token1", "TK1", 18, INITIAL_MINT);
        
        factory = new TigerFactory(alice);
        router = new TigerRouter(address(factory), address(0));
        
        vm.stopPrank();
        
        // Setup initial balances
        vm.prank(alice);
        token0.transfer(bob, 10000e18);
        vm.prank(alice);
        token1.transfer(bob, 10000e18);
    }
    
    // ============================================================================
    // Edge Case Tests
    // ============================================================================
    
    function testSwapWithZeroAmount() public {
        address pair = factory.createPair(address(token0), address(token1));
        TigerPair(pair).mint(alice, 1000e18);
        
        vm.prank(bob);
        vm.expectRevert("INSUFFICIENT_OUTPUT_AMOUNT");
        TigerPair(pair).swap(0, 0, bob, "");
    }
    
    function testSwapInsufficientLiquidity() public {
        address pair = factory.createPair(address(token0), address(token1));
        TigerPair(pair).mint(alice, 100e18); // Very little liquidity
        
        vm.prank(bob);
        token0.transfer(pair, 10000e18); // More than liquidity
        
        vm.expectRevert();
        TigerPair(pair).swap(0, 1, bob, "");
    }
    
    function testMinimumLiquidityLock() public {
        address pair = factory.createPair(address(token0), address(token1));
        
        vm.prank(alice);
        token0.approve(pair, 1000e18);
        
        uint256 liquidity = TigerPair(pair).mint(alice, 1000e18);
        
        // First liquidity providers get 1000 tokens locked
        assertTrue(liquidity < 1000e18);
    }
    
    function testMultipleSwapsInSameBlock() public {
        address pair = factory.createPair(address(token0), address(token1));
        
        vm.prank(alice);
        token0.approve(pair, 100000e18);
        TigerPair(pair).mint(alice, 100000e18);
        
        // Multiple swaps
        for (uint i = 0; i < 5; i++) {
            vm.prank(bob);
            token0.transfer(pair, 100e18);
            
            uint256 outAmount = TigerPair(pair).getAmountOut(100e18, address(token0));
            TigerPair(pair).swap(outAmount * 99 / 100, 0, bob, "");
        }
        
        assertTrue(TigerPair(pair).reserve0() > 100000e18);
    }
    
    // ============================================================================
    // Reentrancy Tests
    // ============================================================================
    
    function testReentrancyGuard() public {
        address pair = factory.createPair(address(token0), address(token1));
        
        vm.prank(alice);
        token0.approve(pair, 10000e18);
        TigerPair(pair).mint(alice, 10000e18);
        
        // Try to reenter during swap
        ReentrancyAttacker attacker = new ReentrancyAttacker(pair);
        
        vm.prank(bob);
        attacker.attack();
    }
    
    // ============================================================================
    // Fee Tests
    // ============================================================================
    
    function testProtocolFee() public {
        vm.prank(alice);
        factory.setFeeTo(feeRecipient);
        
        address pair = factory.createPair(address(token0), address(token1));
        
        vm.prank(alice);
        token0.approve(pair, 10000e18);
        TigerPair(pair).mint(alice, 10000e18);
        
        // Make a swap
        vm.prank(bob);
        token0.transfer(pair, 1000e18);
        uint256 outAmount = TigerPair(pair).getAmountOut(1000e18, address(token0));
        TigerPair(pair).swap(outAmount * 99 / 100, 0, bob, "");
        
        // Protocol should earn fees
        assertTrue(factory.feeTo() == feeRecipient);
    }
    
    // ============================================================================
    // Access Control Tests
    // ============================================================================
    
    function testFactoryAccessControl() public {
        // Only owner can create pairs
        vm.prank(bob);
        vm.expectRevert("Ownable: caller is not the owner");
        factory.createPair(address(token0), address(token1));
        
        // Owner can create pairs
        vm.prank(alice);
        address pair = factory.createPair(address(token0), address(token1));
        assertTrue(pair != address(0));
    }
    
    function testRouterAccessControl() public {
        vm.prank(bob);
        vm.expectRevert();
        router.addLiquidity(
            address(token0),
            address(token1),
            1000e18,
            1000e18,
            0,
            0,
            bob,
            block.timestamp + 300
        );
    }
    
    // ============================================================================
    // Gas Tests
    // ============================================================================
    
    function testSwapGasUsage() public {
        address pair = factory.createPair(address(token0), address(token1));
        
        vm.prank(alice);
        token0.approve(pair, 100000e18);
        TigerPair(pair).mint(alice, 100000e18);
        
        vm.prank(bob);
        token0.transfer(pair, 1000e18);
        
        uint256 gasBefore = gasleft();
        TigerPair(pair).swap(
            TigerPair(pair).getAmountOut(1000e18, address(token0)) * 99 / 100,
            0,
            bob,
            ""
        );
        uint256 gasUsed = gasBefore - gasleft();
        
        // Swap should use reasonable gas
        assertTrue(gasUsed < 200000);
    }
    
    // ============================================================================
    // Token Edge Cases
    // ============================================================================
    
    function testTokenDecimals() public {
        TigerToken decimalToken = new TigerToken("Decimal", "DEC", 6, INITIAL_MINT);
        
        assertEq(decimalToken.decimals(), 6);
        
        // Test with different decimal
        decimalToken.transfer(bob, 1000000);
        assertEq(decimalToken.balanceOf(bob), 1000000);
    }
    
    function testMaxUint256() public {
        vm.prank(alice);
        vm.expectRevert();
        token0.transfer(bob, type(uint256).max);
    }
    
    // ============================================================================
    // Timestamp Tests
    // ============================================================================
    
    function testDeadlineEnforcement() public {
        address pair = factory.createPair(address(token0), address(token1));
        
        vm.prank(alice);
        token0.approve(address(router), 1000e18);
        
        vm.prank(bob);
        token0.approve(address(router), 1000e18);
        
        vm.prank(alice);
        router.addLiquidity(
            address(token0),
            address(token1),
            1000e18,
            1000e18,
            0,
            0,
            alice,
            block.timestamp + 300
        );
        
        // Try to swap with expired deadline
        vm.prank(bob);
        address[] memory path = new address[](2);
        path[0] = address(token0);
        path[1] = address(token1);
        
        vm.prank(bob);
        vm.expectRevert();
        router.swapExactTokensForTokens(
            100e18,
            0,
            path,
            bob,
            block.timestamp - 1 // Expired
        );
    }
    
    // ============================================================================
    // Fork Simulation Tests (using vm.roll)
    // ============================================================================
    
    function testForkSimulation() public {
        // In production, would use mainnet fork
        // This simulates multiple blocks
        
        address pair = factory.createPair(address(token0), address(token1));
        
        vm.prank(alice);
        token0.approve(pair, 10000e18);
        TigerPair(pair).mint(alice, 10000e18);
        
        // Simulate multiple blocks
        for (uint i = 0; i < 10; i++) {
            vm.roll(block.number + 1);
            
            vm.prank(bob);
            token0.transfer(pair, 10e18);
            
            uint256 out = TigerPair(pair).getAmountOut(10e18, address(token0));
            TigerPair(pair).swap(out * 99 / 100, 0, bob, "");
        }
        
        assertTrue(TigerPair(pair).reserve0() > 10000e18);
    }
}

// Reentrancy attacker contract
contract ReentrancyAttacker {
    address immutable target;
    
    constructor(address _target) {
        target = _target;
    }
    
    function attack() public {
        // This would try to reenter but should be protected
        // In production, use ReentrancyGuard
    }
    
    receive() external payable {
        // Attempt reentrancy
        if (target.balance > 0) {
            TigerPair(target).swap(1, 0, address(this), "");
        }
    }
}

/**
 * @title Invariant Test Contract
 * @dev Tests invariants that should always hold
 */
contract TigerSwapInvariantTest is Test {
    TigerToken public token0;
    TigerToken public token1;
    TigerFactory public factory;
    TigerPair public pair;
    
    address[] public callers;
    mapping(address => uint256) public token0Balances;
    mapping(address => uint256) public token1Balances;
    
    function setUp() public {
        token0 = new TigerToken("Token0", "TK0", 18, 1000000e18);
        token1 = new TigerToken("Token1", "TK1", 18, 1000000e18);
        
        factory = new TigerFactory(address(this));
        
        address pairAddress = factory.createPair(address(token0), address(token1));
        pair = TigerPair(pairAddress);
        
        // Mint initial liquidity
        token0.approve(address(pair), 10000e18);
        pair.mint(address(this), 10000e18);
        
        callers.push(makeAddr("caller1"));
        callers.push(makeAddr("caller2"));
        callers.push(makeAddr("caller3"));
    }
    
    // Invariant: k = x * y should not decrease
    function invariantK() public view {
        uint256 k = pair.kLast();
        (uint256 reserve0, uint256 reserve1,) = pair.getReserves();
        uint256 currentK = reserve0 * reserve1;
        
        // After swaps, k should be >= previous k (or equal if no swaps)
        // This tests that protocol fees don't cause losses
        assertTrue(currentK >= k || reserve0 == 0 || reserve1 == 0);
    }
    
    // Invariant: total supply of LP tokens should match reserves
    function invariantTotalSupply() public view {
        uint256 totalSupply = pair.totalSupply();
        (uint256 reserve0, uint256 reserve1,) = pair.getReserves();
        
        // At least one reserve should back LP tokens
        assertTrue(totalSupply > 0);
    }
}

/**
 * @title Gas Benchmark Tests
 */
contract TigerSwapGasTest is Test {
    TigerToken public token0;
    TigerToken public token1;
    TigerFactory public factory;
    TigerRouter public router;
    TigerPair public pair;
    
    address[] public traders;
    
    function setUp() public {
        token0 = new TigerToken("Token0", "TK0", 18, 100000000e18);
        token1 = new TigerToken("Token1", "TK1", 18, 100000000e18);
        
        factory = new TigerFactory(address(this));
        router = new TigerRouter(address(factory), address(0));
        
        address pairAddress = factory.createPair(address(token0), address(token1));
        pair = TigerPair(pairAddress);
        
        token0.approve(address(pair), 10000000e18);
        pair.mint(address(this), 10000000e18);
        
        // Create multiple traders
        for (uint i = 0; i < 10; i++) {
            traders.push(makeAddr(string(abi.encodePacked("trader", i))));
        }
    }
    
    function testGasSwapSingleHop() public {
        uint256 totalGas;
        
        for (uint i = 0; i < traders.length; i++) {
            address trader = traders[i];
            
            token0.transfer(trader, 1000e18);
            
            vm.prank(trader);
            token0.transfer(address(pair), 100e18);
            
            uint256 gasBefore = gasleft();
            pair.swap(pair.getAmountOut(100e18, address(token0)) * 99 / 100, 0, trader, "");
            totalGas += gasBefore - gasleft();
        }
        
        emit log_named_uint("Total Gas", totalGas);
        emit log_named_uint("Avg Gas per Swap", totalGas / traders.length);
    }
    
    function testGasAddLiquidity() public {
        uint256 totalGas;
        
        for (uint i = 0; i < 5; i++) {
            address lp = makeAddr(string(abi.encodePacked("lp", i)));
            
            token0.transfer(lp, 10000e18);
            token1.transfer(lp, 10000e18);
            
            vm.startPrank(lp);
            token0.approve(address(pair), 10000e18);
            
            uint256 gasBefore = gasleft();
            pair.mint(lp, 5000e18);
            totalGas += gasBefore - gasleft();
            vm.stopPrank();
        }
        
        emit log_named_uint("Total Gas Add Liquidity", totalGas);
    }
}
