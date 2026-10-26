// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";

/**
 * @title TigerSwapWeightedPool
 * @notice Weighted pools with an arbitrary number of tokens, Balancer-style.
 *
 * The invariant is the weighted geometric mean of the balances:
 *   invariant = prod(balance_i ^ weight_i) / prod(weight_i ^ weight_i)
 *
 * Swaps solve that invariant for the unknown output balance using Newton's
 * method, then apply the pool fee. Weights are fixed at construction and must
 * sum to 1e18, so a 80/20 TGR/RUSD pool is weights [0.8e18, 0.2e18].
 */
contract TigerSwapWeightedPool {
    uint256 public constant WEIGHT_PRECISION = 1e18;
    uint256 public constant MIN_WEIGHT = 1e16; // 1%
    uint256 public constant MAX_TOKENS = 8;
    uint256 public constant NEWTON_ITERATIONS = 32;

    IERC20[MAX_TOKENS] public tokens;
    uint256 public tokenCount;
    uint256 public feeBps; // basis points, e.g. 30 = 0.3%

    uint256[MAX_TOKENS] public weight; // scaled by WEIGHT_PRECISION
    uint256[MAX_TOKENS] public balance; // token balances held by the pool

    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    address public owner;

    event Swap(address indexed sender, uint256 tokenIn, uint256 tokenOut, uint256 amountIn, uint256 amountOut);
    event LiquidityAdded(address indexed provider, uint256[] amounts, uint256 lpMinted);
    event LiquidityRemoved(address indexed provider, uint256[] amounts, uint256 lpBurned);
    event FeeUpdated(uint256 oldFeeBps, uint256 newFeeBps);
    event Approval(address indexed owner, address indexed spender, uint256 value);
    event Transfer(address indexed from, address indexed to, uint256 value);

    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    uint256 private _status = 1;

    modifier nonReentrant() {
        require(_status == 1, "reentrant call");
        _status = 2;
        _;
        _status = 1;
    }

    constructor(address[] memory _tokens, uint256[] memory _weights, uint256 _feeBps) {
        require(_tokens.length >= 2, "need at least two tokens");
        require(_tokens.length <= MAX_TOKENS, "too many tokens");
        require(_tokens.length == _weights.length, "length mismatch");
        require(_feeBps <= 1000, "fee too high"); // capped at 10%

        uint256 weightSum;
        for (uint256 i = 0; i < _tokens.length; i++) {
            require(_tokens[i] != address(0), "zero token");
            require(_weights[i] >= MIN_WEIGHT, "weight below minimum");
            // Reject duplicate tokens, which would corrupt the invariant.
            for (uint256 j = 0; j < i; j++) {
                require(_tokens[j] != _tokens[i], "duplicate token");
            }
            tokens[i] = IERC20(_tokens[i]);
            weight[i] = _weights[i];
            weightSum += _weights[i];
        }
        require(weightSum == WEIGHT_PRECISION, "weights must sum to 1e18");

        tokenCount = _tokens.length;
        feeBps = _feeBps;
        owner = msg.sender;
    }

    // ---- Invariant ---------------------------------------------------------

    uint256 internal constant LN_1E18 = 41446531673892822312; // ln(1e18) * 1e18
    uint256 internal constant LN_2 = 693147180559945309; // ln(2) * 1e18

    /// @notice The pool invariant: the weighted geometric mean of the balances,
    /// prod(balance_i ^ weight_i). It is constant along a swap, which is all the
    /// AMM requires. The usual Balancer normalising constant prod(w_i ^ w_i) is
    /// a per-pool constant that cancels in every ratio we use, so it is omitted.
    function getInvariant() external view returns (uint256) {
        return _invariant(_balancesArray());
    }

    function _balancesArray() internal view returns (uint256[] memory b) {
        b = new uint256[](tokenCount);
        for (uint256 i = 0; i < tokenCount; i++) {
            b[i] = balance[i];
        }
    }

    function _invariant(uint256[] memory b) internal view returns (uint256) {
        int256 logSum = 0;
        for (uint256 i = 0; i < tokenCount; i++) {
            require(b[i] > 0, "zero balance");
            logSum += (_ln(b[i]) * int256(weight[i])) / int256(WEIGHT_PRECISION);
        }
        require(logSum > 0, "invariant underflow");
        return _exp(logSum);
    }

    /// @notice ln(x) * 1e18 for any x > 0, as a signed value.
    /// @dev Normalises x into [1e18, 2e18) by halving or doubling, then uses the
    /// atanh series ln(m) = 2 * atanh((m-1)/(m+1)) around m = 1.
    function _ln(uint256 x) internal pure returns (int256) {
        require(x > 0, "ln of zero");

        int256 n = 0;
        if (x < WEIGHT_PRECISION) {
            while (x < WEIGHT_PRECISION) {
                x *= 2;
                n -= 1;
            }
        } else {
            while (x >= 2 * WEIGHT_PRECISION) {
                x /= 2;
                n += 1;
            }
        }

        // x is now in [1e18, 2e18); ln(x) = ln(x/1e18) + ln(1e18).
        int256 z = (int256(x - WEIGHT_PRECISION) * int256(WEIGHT_PRECISION)) / int256(x + WEIGHT_PRECISION);
        int256 zSquared = (z * z) / int256(WEIGHT_PRECISION);

        int256 series = 0;
        int256 term = z;
        for (uint256 i = 1; i <= 21; i += 2) {
            series += (2 * term) / int256(i);
            term = (term * zSquared) / int256(WEIGHT_PRECISION);
        }

        return series + int256(LN_1E18) + n * int256(LN_2);
    }

    /// @notice e^x * 1e18 for x >= 0.
    /// @dev Splits x into n * ln2 + remainder so the series stays small, then
    /// scales by 2^n.
    function _exp(int256 x) internal pure returns (uint256) {
        require(x >= 0, "exp of negative");
        int256 ln2 = int256(LN_2);
        int256 n = x / ln2;
        require(n < 200, "exp overflow");
        int256 remainder = x - n * ln2;

        int256 result = int256(WEIGHT_PRECISION);
        int256 term = int256(WEIGHT_PRECISION);
        for (uint256 i = 1; i <= 24; i++) {
            term = (term * remainder) / (int256(WEIGHT_PRECISION) * int256(i));
            result += term;
        }
        require(result > 0, "exp underflow");
        return uint256(result) << uint256(n);
    }

    /// @notice (a * b) / denominator with a 512-bit intermediate, so share maths
    /// on large invariants cannot overflow.
    function _mulDiv(uint256 a, uint256 b, uint256 denominator) internal pure returns (uint256 result) {
        require(denominator > 0, "division by zero");
        unchecked {
            uint256 prod0;
            uint256 prod1;
            assembly {
                let mm := mulmod(a, b, not(0))
                prod0 := mul(a, b)
                prod1 := sub(sub(mm, prod0), lt(mm, prod0))
            }

            if (prod1 == 0) {
                return prod0 / denominator;
            }

            require(denominator > prod1, "mulDiv overflow");

            uint256 remainder;
            assembly {
                remainder := mulmod(a, b, denominator)
                prod1 := sub(prod1, gt(remainder, prod0))
                prod0 := sub(prod0, remainder)
            }

            uint256 twos = denominator & (~denominator + 1);
            assembly {
                denominator := div(denominator, twos)
                prod0 := div(prod0, twos)
                twos := add(div(sub(0, twos), twos), 1)
            }

            prod0 |= prod1 * twos;

            uint256 inverse = (3 * denominator) ^ 2;
            inverse *= 2 - denominator * inverse;
            inverse *= 2 - denominator * inverse;
            inverse *= 2 - denominator * inverse;
            inverse *= 2 - denominator * inverse;
            inverse *= 2 - denominator * inverse;
            inverse *= 2 - denominator * inverse;

            result = prod0 * inverse;
        }
    }

    // ---- Swap --------------------------------------------------------------

    /// @notice Swap `_amountIn` of `_tokenIn` for `_tokenOut`.
    function swap(uint256 _tokenIn, uint256 _tokenOut, uint256 _amountIn, uint256 _minAmountOut)
        external
        nonReentrant
        returns (uint256 amountOut)
    {
        require(_tokenIn < tokenCount && _tokenOut < tokenCount, "bad index");
        require(_tokenIn != _tokenOut, "same token");
        require(_amountIn > 0, "amount required");

        uint256[] memory b = _balancesArray();
        uint256 invBefore = _invariant(b);

        b[_tokenIn] += _amountIn;
        // Solve for the output balance that restores the invariant.
        uint256 targetOut = _solveOutGivenIn(b, _tokenOut, invBefore);
        require(targetOut < b[_tokenOut], "insufficient liquidity");

        uint256 grossOut = b[_tokenOut] - targetOut;
        amountOut = (grossOut * (10000 - feeBps)) / 10000;
        require(amountOut >= _minAmountOut, "slippage exceeded");
        require(amountOut < b[_tokenOut], "output exceeds balance");

        balance[_tokenIn] = b[_tokenIn];
        balance[_tokenOut] = b[_tokenOut] - amountOut;

        require(tokens[_tokenIn].transferFrom(msg.sender, address(this), _amountIn), "transfer in failed");
        require(tokens[_tokenOut].transfer(msg.sender, amountOut), "transfer out failed");

        emit Swap(msg.sender, _tokenIn, _tokenOut, _amountIn, amountOut);
    }

    /// @dev Find the balance of `_outIndex` that restores `_invariantTarget`
    /// given every other balance, by bisection on the monotone invariant.
    function _solveOutGivenIn(uint256[] memory b, uint256 _outIndex, uint256 _invariantTarget)
        internal
        view
        returns (uint256)
    {
        uint256 low = 1;
        uint256 high = b[_outIndex];
        uint256[] memory probe = new uint256[](tokenCount);

        for (uint256 i = 0; i < NEWTON_ITERATIONS; i++) {
            uint256 mid = (low + high) / 2;
            for (uint256 j = 0; j < tokenCount; j++) {
                probe[j] = b[j];
            }
            probe[_outIndex] = mid;
            uint256 inv = _invariant(probe);
            // The invariant increases with the remaining out-balance. If the
            // probe invariant is too high the remaining balance is too large,
            // so the output would be too small; shrink the upper bound.
            if (inv > _invariantTarget) {
                high = mid;
            } else {
                low = mid;
            }
            if (high - low <= 1) {
                break;
            }
        }
        return low;
    }

    // ---- Liquidity ---------------------------------------------------------

    /// @notice Add liquidity. The first deposit sets the share supply from the
    /// weighted invariant; later deposits are proportional to the invariant.
    function addLiquidity(uint256[] memory _amounts, uint256 _minLp)
        external
        nonReentrant
        returns (uint256 lpMinted)
    {
        require(_amounts.length == tokenCount, "length mismatch");

        uint256[] memory b = _balancesArray();
        bool firstDeposit = totalSupply == 0;

        for (uint256 i = 0; i < tokenCount; i++) {
            require(_amounts[i] > 0, "zero amount");
            b[i] += _amounts[i];
        }

        uint256 invAfter = _invariant(b);
        if (firstDeposit) {
            lpMinted = invAfter;
        } else {
            uint256 invBefore = _invariant(_balancesArray());
            require(invAfter > invBefore, "invariant must grow");
            lpMinted = _mulDiv(totalSupply, invAfter - invBefore, invBefore);
        }
        require(lpMinted > 0, "no liquidity minted");
        require(lpMinted >= _minLp, "slippage exceeded");

        for (uint256 i = 0; i < tokenCount; i++) {
            balance[i] = b[i];
            require(tokens[i].transferFrom(msg.sender, address(this), _amounts[i]), "transfer in failed");
        }

        totalSupply += lpMinted;
        balanceOf[msg.sender] += lpMinted;

        emit LiquidityAdded(msg.sender, _amounts, lpMinted);
        emit Transfer(address(0), msg.sender, lpMinted);
    }

    /// @notice Burn LP shares and withdraw the proportional share of balances.
    function removeLiquidity(uint256 _lpAmount, uint256[] memory _minAmounts)
        external
        nonReentrant
        returns (uint256[] memory amounts)
    {
        require(_lpAmount > 0, "amount required");
        require(balanceOf[msg.sender] >= _lpAmount, "insufficient shares");
        require(_minAmounts.length == tokenCount, "length mismatch");

        amounts = new uint256[](tokenCount);
        for (uint256 i = 0; i < tokenCount; i++) {
            uint256 out = _mulDiv(balance[i], _lpAmount, totalSupply);
            require(out >= _minAmounts[i], "slippage exceeded");
            require(out > 0, "nothing to withdraw");
            amounts[i] = out;
            balance[i] -= out;
            require(tokens[i].transfer(msg.sender, out), "transfer out failed");
        }

        balanceOf[msg.sender] -= _lpAmount;
        totalSupply -= _lpAmount;

        emit LiquidityRemoved(msg.sender, amounts, _lpAmount);
        emit Transfer(msg.sender, address(0), _lpAmount);
    }

    // ---- ERC20 -------------------------------------------------------------

    function approve(address _spender, uint256 _amount) external returns (bool) {
        allowance[msg.sender][_spender] = _amount;
        emit Approval(msg.sender, _spender, _amount);
        return true;
    }

    function transfer(address _to, uint256 _amount) external returns (bool) {
        require(balanceOf[msg.sender] >= _amount, "insufficient balance");
        balanceOf[msg.sender] -= _amount;
        balanceOf[_to] += _amount;
        emit Transfer(msg.sender, _to, _amount);
        return true;
    }

    function transferFrom(address _from, address _to, uint256 _amount) external returns (bool) {
        require(balanceOf[_from] >= _amount, "insufficient balance");
        require(allowance[_from][msg.sender] >= _amount, "insufficient allowance");
        allowance[_from][msg.sender] -= _amount;
        balanceOf[_from] -= _amount;
        balanceOf[_to] += _amount;
        emit Transfer(_from, _to, _amount);
        return true;
    }

    // ---- Views -------------------------------------------------------------

    function getBalances() external view returns (uint256[] memory) {
        return _balancesArray();
    }

    function getWeights() external view returns (uint256[] memory w) {
        w = new uint256[](tokenCount);
        for (uint256 i = 0; i < tokenCount; i++) {
            w[i] = weight[i];
        }
    }

    /// @notice Spot price of `_tokenIn` in units of `_tokenOut`, scaled by 1e18.
    function spotPrice(uint256 _tokenIn, uint256 _tokenOut) external view returns (uint256) {
        require(_tokenIn < tokenCount && _tokenOut < tokenCount, "bad index");
        require(balance[_tokenIn] > 0 && balance[_tokenOut] > 0, "empty pool");
        // Marginal price of one tokenIn in tokenOut units:
        //   price = (b_out / w_out) / (b_in / w_in)
        // which is the derivative d(out)/d(in) of the swap at zero size.
        return (balance[_tokenOut] * weight[_tokenIn] * 1e18) / (weight[_tokenOut] * balance[_tokenIn]);
    }

    function setFee(uint256 _feeBps) external onlyOwner {
        require(_feeBps <= 1000, "fee too high");
        uint256 old = feeBps;
        feeBps = _feeBps;
        emit FeeUpdated(old, _feeBps);
    }
}
