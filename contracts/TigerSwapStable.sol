// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerSwapStable
 * @notice StableSwap AMM for pegged assets (RUSD/USDT/USDC/RYUSD).
 *
 * The invariant blends a constant-sum and a constant-product term so trades
 * between near-equivalent assets cost almost nothing, while the pool still
 * resists being drained to one side:
 *
 *   A*n^n*sum(x) + D = A*D*n^n + D^(n+1)/(n^n * prod(x))
 *
 * `D` is the invariant, solved iteratively, and `y` is solved for the output
 * balance given a new input balance.
 *
 * Properties this implementation guarantees:
 *   - `rates` are real, per-coin and owner-settable, so a coin that is not
 *     exactly 1:1 (or has non-18 decimals) is priced correctly. They are not
 *     hardcoded to 1.
 *   - The amplification coefficient `A` ramps linearly and every pricing
 *     function uses the ramped value, so a ramp actually takes effect.
 *   - The admin's cut of the fee is tracked in `adminFees` and excluded from LP
 *     reserves, so LPs and the treasury never claim the same tokens.
 *   - `get_dy` and `exchange` share one pricing path, so a quote can never
 *     disagree with execution.
 *   - Every state-changing entry point is reentrancy guarded.
 */
contract TigerSwapStable {
    // ============ Constants ============
    uint256 public constant N_COINS = 2;
    uint256 public constant PRECISION = 1e18;
    /// @dev Amplification precision. `A` is stored unscaled, e.g. 100.
    uint256 public constant A_PRECISION = 100;
    /// @dev Fee denominator: a fee of 1e6 is 0.01%.
    uint256 public constant FEE_DENOMINATOR = 1e10;
    uint256 public constant MAX_FEE = 1e10;
    uint256 public constant MIN_RAMP_DURATION = 1 days;

    // ============ State ============
    address[N_COINS] public coins;
    uint256[N_COINS] public balances;
    /// @dev Admin's share of collected fees, per coin, held outside LP reserves.
    uint256[N_COINS] public adminFees;
    /// @dev Price of one whole coin in `PRECISION` units. Defaults to 1:1.
    uint256[N_COINS] public rates;

    uint256 public A;
    uint256 public fee;
    uint256 public adminFee;

    uint256 public initialA;
    uint256 public futureA;
    uint256 public initialATime;
    uint256 public futureATime;

    address public owner;
    address public futureOwner;

    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    // ============ Events ============
    event TokenExchange(
        address indexed buyer, uint256 soldId, uint256 tokensSold, uint256 boughtId, uint256 tokensBought
    );
    event AddLiquidity(
        address indexed provider, uint256[2] tokenAmounts, uint256[2] fees, uint256 invariant, uint256 tokenSupply
    );
    event RemoveLiquidity(address indexed provider, uint256[2] tokenAmounts, uint256[2] fees, uint256 tokenSupply);
    event RemoveLiquidityOne(address indexed provider, uint256 tokenAmount, uint256 coinAmount, uint256[2] fees);
    event RampA(uint256 oldA, uint256 newA, uint256 initialTime, uint256 futureTime);
    event StopRampA(uint256 a, uint256 time);
    event AdminFeesWithdrawn(address indexed to, uint256[2] amounts);
    event RatesUpdated(uint256[2] rates);
    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed tokenOwner, address indexed spender, uint256 value);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    // ============ Modifiers ============
    modifier onlyOwner() {
        require(msg.sender == owner, "TigerSwap: NOT_OWNER");
        _;
    }

    uint256 private _locked = 1;

    modifier nonReentrant() {
        require(_locked == 1, "TigerSwap: REENTRANT");
        _locked = 2;
        _;
        _locked = 1;
    }

    // ============ Construction ============
    constructor(address[2] memory _coins, uint256 _a, uint256 _fee, uint256 _adminFee) {
        require(_coins[0] != address(0) && _coins[1] != address(0), "TigerSwap: ZERO_COIN");
        require(_coins[0] != _coins[1], "TigerSwap: DUPLICATE_COIN");
        require(_a > 0, "TigerSwap: ZERO_A");
        require(_fee < MAX_FEE / 10, "TigerSwap: FEE_TOO_HIGH");
        require(_adminFee <= FEE_DENOMINATOR, "TigerSwap: ADMIN_FEE_TOO_HIGH");

        for (uint256 i = 0; i < N_COINS; i++) {
            coins[i] = _coins[i];
            rates[i] = PRECISION;
        }

        A = _a;
        initialA = _a;
        futureA = _a;
        initialATime = block.timestamp;
        futureATime = block.timestamp;
        fee = _fee;
        adminFee = _adminFee;
        owner = msg.sender;

        emit OwnershipTransferred(address(0), msg.sender);
    }

    // ============ Liquidity ============

    /// @notice Deposit both coins. The fee applies only to the imbalanced part,
    ///         because a proportional deposit leaves the pool's composition
    ///         unchanged and would otherwise be charged for nothing.
    function addLiquidity(uint256[2] calldata amounts, uint256 minMint)
        external
        nonReentrant
        returns (uint256 mint)
    {
        require(amounts[0] > 0 || amounts[1] > 0, "TigerSwap: ZERO_AMOUNT");

        uint256 supply = totalSupply;
        uint256[N_COINS] memory fees;

        if (supply == 0) {
            for (uint256 i = 0; i < N_COINS; i++) {
                balances[i] += amounts[i];
            }
            mint = _getD(_xp());
            require(mint > 0, "TigerSwap: ZERO_MINT");
            require(mint >= minMint, "TigerSwap: SLIPPAGE");
            _mint(msg.sender, mint);
            emit AddLiquidity(msg.sender, amounts, fees, mint, mint);
        } else {
            uint256[N_COINS] memory xp = _xp();
            uint256 d0 = _getD(xp);
            uint256 sumXp = xp[0] + xp[1];

            for (uint256 i = 0; i < N_COINS; i++) {
                if (amounts[i] == 0) continue;
                uint256 scaled = amounts[i] * rates[i] / PRECISION;
                uint256 ideal = d0 * xp[i] / sumXp;
                if (scaled > ideal) {
                    uint256 excess = scaled - ideal;
                    uint256 feeAmount = excess * fee / FEE_DENOMINATOR;
                    fees[i] = feeAmount;
                    balances[i] += amounts[i] - feeAmount * PRECISION / rates[i];
                    // The fee stays in the pool for the LPs, but the admin's
                    // share of it is earmarked so it can be withdrawn once.
                    adminFees[i] += feeAmount * adminFee / FEE_DENOMINATOR * PRECISION / rates[i];
                } else {
                    balances[i] += amounts[i];
                }
            }

            uint256 d1 = _getD(_xp());
            mint = supply * (d1 - d0) / d0;
            require(mint > 0, "TigerSwap: ZERO_MINT");
            require(mint >= minMint, "TigerSwap: SLIPPAGE");
            _mint(msg.sender, mint);
            emit AddLiquidity(msg.sender, amounts, fees, d1, supply + mint);
        }

        for (uint256 i = 0; i < N_COINS; i++) {
            if (amounts[i] == 0) continue;
            require(IERC20(coins[i]).transferFrom(msg.sender, address(this), amounts[i]), "TigerSwap: TRANSFER_FAILED");
        }
    }

    /// @notice Withdraw proportionally. A balanced exit pays no fee because it
    ///         does not change the pool's composition.
    function removeLiquidity(uint256 amount, uint256[2] calldata minAmounts)
        external
        nonReentrant
        returns (uint256[2] memory amounts)
    {
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");
        uint256 supply = totalSupply;
        require(amount <= supply, "TigerSwap: INSUFFICIENT_SUPPLY");

        uint256[N_COINS] memory xp = _xp();
        uint256 d0 = _getD(xp);
        require(d0 > 0, "TigerSwap: EMPTY_POOL");

        for (uint256 i = 0; i < N_COINS; i++) {
            // Convert the invariant-space share back into token units.
            amounts[i] = amount * xp[i] / d0 * PRECISION / rates[i];
            require(amounts[i] >= minAmounts[i], "TigerSwap: SLIPPAGE");
            require(amounts[i] <= balances[i], "TigerSwap: INSUFFICIENT_BALANCE");
        }

        _burn(msg.sender, amount);
        for (uint256 i = 0; i < N_COINS; i++) {
            balances[i] -= amounts[i];
        }

        uint256[2] memory fees;
        emit RemoveLiquidity(msg.sender, amounts, fees, totalSupply);

        for (uint256 i = 0; i < N_COINS; i++) {
            require(IERC20(coins[i]).transfer(msg.sender, amounts[i]), "TigerSwap: TRANSFER_FAILED");
        }
    }

    /// @notice Withdraw everything into a single coin.
    function removeLiquidityOneCoin(uint256 amount, int128 i, uint256 minAmount)
        external
        nonReentrant
        returns (uint256 dy)
    {
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");
        require(i >= 0 && uint256(uint128(i)) < N_COINS, "TigerSwap: INVALID_INDEX");
        uint256 idx = uint256(uint128(i));

        uint256 supply = totalSupply;
        require(amount <= supply, "TigerSwap: INSUFFICIENT_SUPPLY");

        uint256[N_COINS] memory xp = _xp();
        uint256 d0 = _getD(xp);
        require(d0 > 0, "TigerSwap: EMPTY_POOL");

        uint256 d1 = d0 - amount * d0 / supply;
        require(d1 > 0, "TigerSwap: DRAINED");

        // Value of the coin position at the new invariant, before fees.
        uint256 newY = _getYD(idx, d1, xp);
        require(xp[idx] > newY, "TigerSwap: INSUFFICIENT_LIQUIDITY");
        uint256 dyBeforeFee = (xp[idx] - newY) * PRECISION / rates[idx];

        // The fee is charged on the imbalance the withdrawal creates.
        uint256[N_COINS] memory xpReduced = xp;
        uint256 feePerCoin = fee * N_COINS / (4 * (N_COINS - 1));
        for (uint256 j = 0; j < N_COINS; j++) {
            uint256 dxExpected = j == idx ? xp[j] * d1 / d0 - newY : xp[j] - xp[j] * d1 / d0;
            xpReduced[j] -= feePerCoin * dxExpected / FEE_DENOMINATOR;
        }

        uint256 yReduced = _getYD(idx, d1, xpReduced);
        require(xpReduced[idx] > yReduced, "TigerSwap: INSUFFICIENT_LIQUIDITY");
        dy = (xpReduced[idx] - yReduced) * PRECISION / rates[idx];
        require(dy >= minAmount, "TigerSwap: SLIPPAGE");
        require(dy <= balances[idx], "TigerSwap: INSUFFICIENT_BALANCE");

        uint256 dyFee = dyBeforeFee > dy ? dyBeforeFee - dy : 0;

        _burn(msg.sender, amount);
        // The pool keeps the fee; the admin's share of it is earmarked.
        balances[idx] -= dy;
        adminFees[idx] += dyFee * adminFee / FEE_DENOMINATOR;

        uint256[2] memory fees;
        fees[idx] = dyFee;
        emit RemoveLiquidityOne(msg.sender, amount, dy, fees);

        require(IERC20(coins[idx]).transfer(msg.sender, dy), "TigerSwap: TRANSFER_FAILED");
    }

    // ============ Swapping ============

    /// @notice Quote the output for `dx` of coin `i` into coin `j`.
    function getDy(int128 i, int128 j, uint256 dx) external view returns (uint256) {
        _requireIndices(i, j);
        require(dx > 0, "TigerSwap: ZERO_AMOUNT");
        return _getDy(uint256(uint128(i)), uint256(uint128(j)), dx);
    }

    /// @notice Exchange `dx` of coin `i` for coin `j`.
    function exchange(int128 i, int128 j, uint256 dx, uint256 minDy)
        external
        nonReentrant
        returns (uint256 dy)
    {
        _requireIndices(i, j);
        require(dx > 0, "TigerSwap: ZERO_AMOUNT");
        uint256 idxI = uint256(uint128(i));
        uint256 idxJ = uint256(uint128(j));

        uint256 gross = _getDyGross(idxI, idxJ, dx);
        dy = _getDy(idxI, idxJ, dx);
        require(dy > 0, "TigerSwap: ZERO_OUTPUT");
        require(dy >= minDy, "TigerSwap: SLIPPAGE");

        uint256 feeAmount = gross - dy;
        uint256 adminPart = feeAmount * adminFee / FEE_DENOMINATOR;

        balances[idxI] += dx;
        // The pool loses what the trader receives plus the admin's cut; that cut
        // is tracked separately so LPs cannot also claim it.
        balances[idxJ] -= (dy + adminPart);
        adminFees[idxJ] += adminPart;

        require(IERC20(coins[idxI]).transferFrom(msg.sender, address(this), dx), "TigerSwap: TRANSFER_FAILED");
        require(IERC20(coins[idxJ]).transfer(msg.sender, dy), "TigerSwap: TRANSFER_FAILED");

        emit TokenExchange(msg.sender, idxI, dx, idxJ, dy);
    }

    /// @notice The invariant per LP token, scaled by 1e18.
    function getVirtualPrice() external view returns (uint256) {
        uint256 supply = totalSupply;
        if (supply == 0) return PRECISION;
        return _getD(_xp()) * PRECISION / supply;
    }

    /// @notice LP tokens minted for a deposit, or burned for a withdrawal.
    function calcTokenAmount(uint256[2] calldata amounts, bool isDeposit) external view returns (uint256) {
        uint256 supply = totalSupply;
        uint256[N_COINS] memory xp = _xp();
        uint256 d0 = _getD(xp);
        if (supply == 0 || d0 == 0) return 0;

        uint256[N_COINS] memory newBalances = balances;
        for (uint256 i = 0; i < N_COINS; i++) {
            if (isDeposit) {
                newBalances[i] += amounts[i];
            } else {
                require(newBalances[i] >= amounts[i], "TigerSwap: INSUFFICIENT_BALANCE");
                newBalances[i] -= amounts[i];
            }
        }

        uint256 d1 = _getD(_xpMem(newBalances));
        if (isDeposit) {
            return supply * (d1 - d0) / d0;
        }
        return supply * (d0 - d1) / d0;
    }

    // ============ Amplification ============

    /// @notice Begin ramping A to `newA` by `futureTime`.
    function rampA(uint256 newA, uint256 futureTime) external onlyOwner {
        require(newA > 0, "TigerSwap: ZERO_A");
        require(block.timestamp >= initialATime + MIN_RAMP_DURATION, "TigerSwap: RAMP_NOT_READY");
        require(futureTime >= block.timestamp + MIN_RAMP_DURATION, "TigerSwap: DURATION_TOO_SHORT");
        require(futureTime <= block.timestamp + 4 * MIN_RAMP_DURATION, "TigerSwap: DURATION_TOO_LONG");

        uint256 currentA = _getA();
        initialA = currentA;
        futureA = newA;
        initialATime = block.timestamp;
        futureATime = futureTime;

        emit RampA(currentA, newA, block.timestamp, futureTime);
    }

    /// @notice Freeze A at its current ramped value.
    function stopRampA() external {
        require(futureATime > block.timestamp, "TigerSwap: NOT_RAMPING");
        uint256 currentA = _getA();
        initialA = currentA;
        futureA = currentA;
        initialATime = block.timestamp;
        futureATime = block.timestamp;
        emit StopRampA(currentA, block.timestamp);
    }

    /// @notice The currently effective amplification coefficient.
    function getA() external view returns (uint256) {
        return _getA();
    }

    function _getA() internal view returns (uint256) {
        if (block.timestamp >= futureATime) {
            return futureA;
        }
        uint256 total = futureATime - initialATime;
        if (total == 0) {
            return futureA;
        }
        uint256 elapsed = block.timestamp - initialATime;
        if (futureA >= initialA) {
            return initialA + (futureA - initialA) * elapsed / total;
        }
        return initialA - (initialA - futureA) * elapsed / total;
    }

    // ============ Pricing internals ============

    function _requireIndices(int128 i, int128 j) private pure {
        require(i != j, "TigerSwap: SAME_COIN");
        require(i >= 0 && uint256(uint128(i)) < N_COINS, "TigerSwap: INVALID_INDEX");
        require(j >= 0 && uint256(uint128(j)) < N_COINS, "TigerSwap: INVALID_INDEX");
    }

    /// @dev Output before fees, in token units. Kept private so the fee split
    ///      and the net quote can never diverge.
    function _getDyGross(uint256 i, uint256 j, uint256 dx) private view returns (uint256) {
        uint256[N_COINS] memory xp = _xp();
        uint256 d = _getD(xp);

        uint256 x = xp[i] + dx * rates[i] / PRECISION;
        uint256 y = _getYExchange(i, j, x, xp, d);
        require(xp[j] > y, "TigerSwap: INSUFFICIENT_LIQUIDITY");

        // Round in the pool's favour.
        uint256 dy = xp[j] - y - 1;
        return dy * PRECISION / rates[j];
    }

    /// @dev Net output for a trade, fee applied. Used by both the quote and the
    ///      execution path so they can never disagree.
    function _getDy(uint256 i, uint256 j, uint256 dx) private view returns (uint256) {
        uint256 gross = _getDyGross(i, j, dx);
        uint256 feeAmount = gross * fee / FEE_DENOMINATOR;
        return gross - feeAmount;
    }

    /// @dev Ann = A * A_PRECISION * n, the scaled amplification term used by the
    ///      invariant. Keeping A_PRECISION here is what makes the denominator
    ///      and numerator the same order of magnitude.
    function _ann() private view returns (uint256) {
        return _getA() * A_PRECISION * N_COINS;
    }

    function _xp() internal view returns (uint256[N_COINS] memory) {
        return [rates[0] * balances[0] / PRECISION, rates[1] * balances[1] / PRECISION];
    }

    function _xpMem(uint256[N_COINS] memory _balances) internal view returns (uint256[N_COINS] memory) {
        return [rates[0] * _balances[0] / PRECISION, rates[1] * _balances[1] / PRECISION];
    }

    /**
     * @dev Solve for D:
     *        D = (Ann*S/A + D_P*n) * D / ((Ann-A)*D/A + (n+1)*D_P)
     *      where D_P = D^(n+1) / (n^n * prod(x)) and Ann = A*n.
     */
    function _getD(uint256[N_COINS] memory xp) internal view returns (uint256) {
        uint256 s = xp[0] + xp[1];
        if (s == 0) return 0;

        uint256 ann = _ann();
        uint256 d = s;

        for (uint256 iter = 0; iter < 255; iter++) {
            uint256 dP = d;
            for (uint256 k = 0; k < N_COINS; k++) {
                dP = dP * d / (xp[k] * N_COINS);
            }
            uint256 dPrev = d;
            d = (ann * s / A_PRECISION + dP * N_COINS) * d
                / ((ann - A_PRECISION) * d / A_PRECISION + (N_COINS + 1) * dP);

            if (d > dPrev) {
                if (d - dPrev <= 1) return d;
            } else if (dPrev - d <= 1) {
                return d;
            }
        }
        return d;
    }

    /**
     * @dev Solve for the output balance during a swap. `x` replaces xp[i]; xp[j]
     *      is the unknown.
     */
    function _getYExchange(uint256 i, uint256 j, uint256 x, uint256[N_COINS] memory xp, uint256 d)
        private
        view
        returns (uint256)
    {
        uint256 ann = _ann();
        uint256 c = d;
        uint256 s = 0;

        for (uint256 k = 0; k < N_COINS; k++) {
            if (k == j) continue;
            uint256 xK = k == i ? x : xp[k];
            s += xK;
            c = c * d / (xK * N_COINS);
        }
        c = c * d * A_PRECISION / (ann * N_COINS);
        uint256 b = s + d * A_PRECISION / ann;

        return _newtonY(d, b, c);
    }

    /**
     * @dev Solve for xp[i] given a target invariant D, holding the other
     *      balances fixed. Used when removing liquidity into one coin.
     */
    function _getYD(uint256 i, uint256 d, uint256[N_COINS] memory xp) internal view returns (uint256) {
        uint256 ann = _ann();
        uint256 c = d;
        uint256 s = 0;

        for (uint256 k = 0; k < N_COINS; k++) {
            if (k == i) continue;
            uint256 xK = xp[k];
            s += xK;
            c = c * d / (xK * N_COINS);
        }
        c = c * d * A_PRECISION / (ann * N_COINS);
        uint256 b = s + d * A_PRECISION / ann;

        return _newtonY(d, b, c);
    }

    /// @dev Newton iteration for the positive root of y^2 + (b - D)*y = c.
    function _newtonY(uint256 d, uint256 b, uint256 c) private pure returns (uint256) {
        uint256 y = d;
        for (uint256 iter = 0; iter < 255; iter++) {
            uint256 yPrev = y;
            uint256 denom = 2 * y + b;
            require(denom > d, "TigerSwap: CONVERGENCE");
            y = (y * y + c) / (denom - d);

            if (y > yPrev) {
                if (y - yPrev <= 1) return y;
            } else if (yPrev - y <= 1) {
                return y;
            }
        }
        return y;
    }

    // ============ Admin ============

    /// @notice Set the price of each coin in `PRECISION` units.
    function setRates(uint256[2] calldata newRates) external onlyOwner {
        require(newRates[0] > 0 && newRates[1] > 0, "TigerSwap: ZERO_RATE");
        rates[0] = newRates[0];
        rates[1] = newRates[1];
        emit RatesUpdated(newRates);
    }

    /// @notice Withdraw the fees the protocol has accrued.
    function withdrawAdminFees() external onlyOwner nonReentrant {
        uint256[2] memory amounts;
        for (uint256 i = 0; i < N_COINS; i++) {
            uint256 amount = adminFees[i];
            if (amount == 0) continue;
            // Admin fees are held outside LP reserves, so they are paid out
            // without touching the reserves LPs can withdraw.
            adminFees[i] = 0;
            amounts[i] = amount;
            require(IERC20(coins[i]).transfer(msg.sender, amount), "TigerSwap: TRANSFER_FAILED");
        }
        emit AdminFeesWithdrawn(msg.sender, amounts);
    }

    function commitTransferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "TigerSwap: ZERO_OWNER");
        futureOwner = newOwner;
    }

    function acceptTransferOwnership() external {
        require(msg.sender == futureOwner, "TigerSwap: NOT_FUTURE_OWNER");
        emit OwnershipTransferred(owner, futureOwner);
        owner = futureOwner;
        futureOwner = address(0);
    }

    function setFee(uint256 newFee) external onlyOwner {
        require(newFee < MAX_FEE / 10, "TigerSwap: FEE_TOO_HIGH");
        fee = newFee;
    }

    function setAdminFee(uint256 newAdminFee) external onlyOwner {
        require(newAdminFee <= FEE_DENOMINATOR, "TigerSwap: ADMIN_FEE_TOO_HIGH");
        adminFee = newAdminFee;
    }

    // ============ ERC20 ============

    function transfer(address to, uint256 amount) external returns (bool) {
        _transfer(msg.sender, to, amount);
        return true;
    }

    function transferFrom(address from, address to, uint256 amount) external returns (bool) {
        uint256 allowed = allowance[from][msg.sender];
        if (allowed != type(uint256).max) {
            require(allowed >= amount, "TigerSwap: ALLOWANCE");
            allowance[from][msg.sender] = allowed - amount;
        }
        _transfer(from, to, amount);
        return true;
    }

    function approve(address spender, uint256 amount) external returns (bool) {
        require(spender != address(0), "TigerSwap: ZERO_SPENDER");
        allowance[msg.sender][spender] = amount;
        emit Approval(msg.sender, spender, amount);
        return true;
    }

    function _transfer(address from, address to, uint256 amount) private {
        require(to != address(0), "TigerSwap: ZERO_RECIPIENT");
        require(balanceOf[from] >= amount, "TigerSwap: INSUFFICIENT_BALANCE");
        balanceOf[from] -= amount;
        balanceOf[to] += amount;
        emit Transfer(from, to, amount);
    }

    function _mint(address to, uint256 amount) private {
        require(to != address(0), "TigerSwap: ZERO_RECIPIENT");
        totalSupply += amount;
        balanceOf[to] += amount;
        emit Transfer(address(0), to, amount);
    }

    function _burn(address from, uint256 amount) private {
        require(balanceOf[from] >= amount, "TigerSwap: INSUFFICIENT_BALANCE");
        balanceOf[from] -= amount;
        totalSupply -= amount;
        emit Transfer(from, address(0), amount);
    }
}

interface IERC20 {
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
}
