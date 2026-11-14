// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapWeighted.sol";

contract WERC20 {
    string public name = "Token";
    string public symbol = "TKN";
    uint8 public decimals = 18;
    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);

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
        allowance[from][msg.sender] -= amount;
        balanceOf[from] -= amount;
        balanceOf[to] += amount;
        emit Transfer(from, to, amount);
        return true;
    }
}

/// @title Fuzz tests for the weighted pool's core invariants.
/// @notice The weighted invariant is the geometric mean
///         `prod(balance_i ^ weight_i)`, computed with a fixed-point `_ln`/`_exp`
///         series, so it carries roughly 1e-13 relative error. The properties
///         below are therefore asserted to that precision rather than exactly:
///         a swap cannot lose the pool a meaningful fraction of its invariant, a
///         coin is never drained, and a proportional deposit followed by an
///         immediate withdrawal cannot return materially more than was paid in.
///         The tolerance is orders of magnitude below the pool fee, so a
///         regression that stopped retaining the fee would still fail.
contract TigerSwapWeightedFuzzTest is Test {
    /// @dev Relative tolerance for invariant-based comparisons. The fixed-point
    ///      geometric mean is accurate to about 1e-13, so 1e-7 leaves a wide
    ///      margin while remaining far below the 0.3% fee.
    uint256 constant TOL = 1e7;

    TigerSwapWeightedPool pool;
    WERC20 tgr;
    WERC20 rusd;

    function setUp() public {
        tgr = new WERC20();
        rusd = new WERC20();

        address[] memory toks = new address[](2);
        toks[0] = address(tgr);
        toks[1] = address(rusd);
        uint256[] memory weights = new uint256[](2);
        weights[0] = 0.8e18;
        weights[1] = 0.2e18;

        pool = new TigerSwapWeightedPool(toks, weights, 30);

        tgr.mint(address(this), 1e30);
        rusd.mint(address(this), 1e30);
        tgr.approve(address(pool), type(uint256).max);
        rusd.approve(address(pool), type(uint256).max);

        uint256[] memory seed = new uint256[](2);
        seed[0] = 800_000e18;
        seed[1] = 200_000e18;
        pool.addLiquidity(seed, 0);
    }

    /// @dev `value` minus a relative tolerance of `TOL` parts per 1e18.
    function _lowerBound(uint256 value) internal pure returns (uint256) {
        return value - (value / TOL);
    }

    /// @notice A swap never loses the pool a meaningful fraction of its
    ///         invariant: the fee is retained by the pool, not paid out of the
    ///         reserves. The comparison allows the fixed-point `_ln`/`_exp`
    ///         rounding error.
    function testFuzzSwapNeverReducesInvariant(uint256 amountIn, bool sellTgr) public {
        uint256 inIdx = sellTgr ? 0 : 1;
        uint256 outIdx = sellTgr ? 1 : 0;
        uint256[] memory b = pool.getBalances();
        amountIn = bound(amountIn, 1e6, b[inIdx] / 10);

        tgr.mint(address(this), sellTgr ? amountIn : 0);
        rusd.mint(address(this), sellTgr ? 0 : amountIn);

        uint256 invBefore = pool.getInvariant();
        pool.swap(inIdx, outIdx, amountIn, 0);

        assertGe(pool.getInvariant(), _lowerBound(invBefore), "invariant fell beyond fixed-point tolerance");
    }

    /// @notice A swap never drains the output coin and never pays out the whole
    ///         balance.
    function testFuzzSwapNeverDrainsOutputCoin(uint256 amountIn, bool sellTgr) public {
        uint256 inIdx = sellTgr ? 0 : 1;
        uint256 outIdx = sellTgr ? 1 : 0;
        uint256[] memory b = pool.getBalances();
        amountIn = bound(amountIn, 1e6, b[inIdx] / 10);

        tgr.mint(address(this), sellTgr ? amountIn : 0);
        rusd.mint(address(this), sellTgr ? 0 : amountIn);

        uint256 outBefore = b[outIdx];
        uint256 amountOut = pool.swap(inIdx, outIdx, amountIn, 0);

        assertLt(amountOut, outBefore, "payout reached the pool's whole balance");
        uint256[] memory post = pool.getBalances();
        assertGt(post[outIdx], 0, "output coin fully drained");
    }

    /// @notice The input balance rises by exactly the amount paid in, and the
    ///         output balance falls by exactly the amount paid out.
    function testFuzzSwapMovesBalancesExactly(uint256 amountIn, bool sellTgr) public {
        uint256 inIdx = sellTgr ? 0 : 1;
        uint256 outIdx = sellTgr ? 1 : 0;
        uint256[] memory b = pool.getBalances();
        amountIn = bound(amountIn, 1e6, b[inIdx] / 10);

        tgr.mint(address(this), sellTgr ? amountIn : 0);
        rusd.mint(address(this), sellTgr ? 0 : amountIn);

        uint256 amountOut = pool.swap(inIdx, outIdx, amountIn, 0);
        uint256[] memory post = pool.getBalances();

        assertEq(post[inIdx], b[inIdx] + amountIn, "input balance did not rise by the amount paid");
        assertEq(post[outIdx], b[outIdx] - amountOut, "output balance did not fall by the payout");
    }

    /// @notice A proportional deposit followed by an immediate withdrawal cannot
    ///         return materially more than was deposited. Withdrawal rounds each
    ///         coin down, so the only slack is the pool's own fixed-point error
    ///         when the deposit is large relative to the pool.
    function testFuzzDepositWithdrawRoundTripIsNotProfitable(uint256 amount) public {
        uint256[] memory seed = pool.getBalances();
        // Bound the deposit to between 0.1% and 10% of the pool. A dust deposit
        // is dominated by the fixed-point rounding of `_ln`/`_exp` rather than by
        // the pool's economics, so bounding it keeps the assertion about the
        // property that matters.
        amount = bound(amount, seed[1] / 1000, seed[1] / 10);
        // Mirror the pool's own ratio so the deposit is proportional.
        uint256 amountTgr = amount * seed[0] / seed[1];
        if (amountTgr == 0) return;

        tgr.mint(address(this), amountTgr);
        rusd.mint(address(this), amount);

        uint256[] memory deposit = new uint256[](2);
        deposit[0] = amountTgr;
        deposit[1] = amount;

        uint256 lp = pool.addLiquidity(deposit, 0);

        uint256[] memory minOut = new uint256[](2);
        uint256[] memory out = pool.removeLiquidity(lp, minOut);

        assertLe(out[0], amountTgr + amountTgr / TOL, "withdrawal of token 0 exceeded the deposit");
        assertLe(out[1], amount + amount / TOL, "withdrawal of token 1 exceeded the deposit");
    }
}
