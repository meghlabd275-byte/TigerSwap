// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerSwapStable
 * @notice StableSwap - Curve-style invariant for stablecoin pools
 * @dev Implements StableSwap invariant (x^n * y^n = k)
 */
contract TigerSwapStable {
    // ============ Constants ============
    uint256 public constant N_COINS = 2; // Number of coins
    uint256 public constant PRECISION = 1e18;
    uint256 public constant MAX_FEE = 1e10;
    uint256 public constant MIN_RAMP_DURATION = 1 days;
    
    // ============ State Variables ============
    address[N_COINS] public coins;
    uint256[N_COINS] public balances;
    
    // Amplification coefficient (A)
    uint256 public A; // May change
    uint256 public A_precise; // Scaled up for precision
    
    // Fee parameters
    uint256 public fee;
    uint256 public admin_fee; // Fee for admin
    uint256 public default_fee = 5e7; // 0.5%
    uint256 public default_admin_fee = 5e9; // 50% of fee
    
    // Ramp parameters
    uint256 public initial_A;
    uint256 public future_A;
    uint256 public initial_A_time;
    uint256 public future_A_time;
    uint256 public RAMP_DURATION = 3 days;
    
    // Admin
    address public owner;
    address public future_owner;
    
    // Token
    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;
    
    // ============ Events ============
    event TokenExchange(
        address buyer,
        uint256 sold_id,
        uint256 tokens_sold,
        uint256 bought_id,
        uint256 tokens_bought
    );
    event AddLiquidity(
        address provider,
        uint256[2] token_amounts,
        uint256[2] fees,
        uint256 invariant,
        uint256 token_supply
    );
    event RemoveLiquidity(
        address provider,
        uint256[2] token_amounts,
        uint256[2] fees,
        uint256 token_supply
    );
    event RemoveLiquidityOne(
        address provider,
        uint256 token_amount,
        uint256 coin_amount,
        uint256[2] fees
    );
    event RemoveLiquidityImbalance(
        address provider,
        uint256[2] token_amounts,
        uint256[2] fees,
        uint256 token_supply
    );
    event RampA(uint256 old_A, uint256 new_A, uint256 initial_time, uint256 future_time);
    event StopRampA(uint256 A, uint256 time);
    
    // ============ Modifiers ============
    modifier onlyOwner() {
        require(msg.sender == owner, "Only owner");
        _;
    }
    
    // ============ Constructor ============
    constructor(
        address[2] memory _coins,
        uint256 _A,
        uint256 _fee,
        uint256 _admin_fee
    ) {
        for (uint256 i = 0; i < N_COINS; i++) {
            require(_coins[i] != address(0), "Invalid coin");
            coins[i] = _coins[i];
        }
        
        A = _A;
        A_precise = _A * PRECISION;
        fee = _fee;
        admin_fee = _admin_fee;
        owner = msg.sender;
        
        // Mint initial LP tokens
        uint256 supply = 0;
        if (totalSupply > 0) {
            supply = totalSupply;
        } else {
            // Initial mint - 1000 tokens
            supply = 1000 * 1e18;
            totalSupply = supply;
            balanceOf[msg.sender] = supply;
        }
        
        emit AddLiquidity(msg.sender, [uint256(0), 0], [uint256(0), 0], 0, supply);
    }
    
    // ============ Core Exchange Functions ============
    
    /**
     * @notice Exchange between two stablecoins
     * @param i Index of input coin
     * @param j Index of output coin
     * @param dx Amount of input coin
     * @param min_dy Minimum amount of output coin
     * @return Actual output amount
     */
    function exchange(
        int128 i,
        int128 j,
        uint256 dx,
        uint256 min_dy
    ) external returns (uint256) {
        require(i != j, "Same coin");
        require(i >= 0 && i < N_COINS, "Invalid i");
        require(j >= 0 && j < N_COINS, "Invalid j");
        require(dx > 0, "Invalid dx");
        
        // Get current balances
        uint256[N_COINS] memory rates = _getRates();
        uint256[N_COINS] memory xp = _xp(rates);
        
        // Calculate output
        uint256 x = xp[uint256(i)] + dx * rates[uint256(i)] / PRECISION;
        
        uint256 dy = _exchange(xp, uint256(i), uint256(j), x, rates);
        require(dy >= min_dy, "Slippage");
        
        // Update balances
        balances[uint256(i)] += dx;
        balances[uint256(j)] -= dy;
        
        // Transfer tokens
        IERC20(coins[uint256(i)]).transferFrom(msg.sender, address(this), dx);
        IERC20(coins[uint256(j)]).transfer(msg.sender, dy);
        
        emit TokenExchange(msg.sender, uint256(i), dx, uint256(j), dy);
        
        return dy;
    }
    
    /**
     * @notice Add liquidity to the pool
     * @param amounts Array of amounts to add [token0, token1]
     * @param min_mint Minimum LP tokens to receive
     * @return Amount of LP tokens minted
     */
    function add_liquidity(
        uint256[2] calldata amounts,
        uint256 min_mint
    ) external returns (uint256) {
        require(amounts[0] > 0 || amounts[1] > 0, "Invalid amounts");
        
        uint256[N_COINS] memory fees = [uint256(0), 0];
        uint256[2] memory deposit_amounts = amounts;
        
        // Get current state
        uint256 supply = totalSupply;
        uint256[N_COINS] memory rates = _getRates();
        
        if (supply > 0) {
            // Calculate invariant and fees
            uint256[N_COINS] memory xp = _xp(rates);
            uint256 d0 = _invariant(xp);
            
            for (uint256 i = 0; i < N_COINS; i++) {
                if (amounts[i] > 0) {
                    // Charge fee
                    fees[i] = amounts[i] * fee / MAX_FEE;
                    deposit_amounts[i] = amounts[i] - fees[i];
                }
            }
            
            // Update balances
            for (uint256 i = 0; i < N_COINS; i++) {
                balances[i] += deposit_amounts[i];
            }
            
            uint256[N_COINS] memory new_xp = _xp(rates);
            uint256 d1 = _invariant(new_xp);
            
            // Calculate tokens to mint
            uint256 mint = supply * (d1 - d0) / d0;
            require(mint >= min_mint, "Slippage");
            
            // Mint tokens
            _mint(msg.sender, mint);
            
            emit AddLiquidity(msg.sender, amounts, fees, d1, supply + mint);
            
            return mint;
        } else {
            // Initial deposit - no fees
            for (uint256 i = 0; i < N_COINS; i++) {
                balances[i] += amounts[i];
            }
            
            uint256 mint = _getD(_xp(rates));
            require(mint >= min_mint, "Slippage");
            
            _mint(msg.sender, mint);
            
            emit AddLiquidity(msg.sender, amounts, fees, mint, mint);
            
            return mint;
        }
    }
    
    /**
     * @notice Remove liquidity from the pool
     * @param amount Amount of LP tokens to burn
     * @param[2] min_amounts Minimum amounts to receive [token0, token1]
     * @return Actual amounts received
     */
    function remove_liquidity(
        uint256 amount,
        uint256[2] calldata min_amounts
    ) external returns (uint256[2] memory) {
        require(amount > 0, "Invalid amount");
        
        uint256[N_COINS] memory rates = _getRates();
        uint256[N_COINS] memory xp = _xp(rates);
        uint256 d0 = _invariant(xp);
        
        // Calculate amounts
        uint256[2] memory amounts;
        for (uint256 i = 0; i < N_COINS; i++) {
            amounts[i] = amount * xp[i] / d0;
            require(amounts[i] >= min_amounts[i], "Slippage");
        }
        
        // Burn tokens
        _burn(msg.sender, amount);
        
        // Update balances
        for (uint256 i = 0; i < N_COINS; i++) {
            balances[i] -= amounts[i];
        }
        
        // Transfer tokens
        for (uint256 i = 0; i < N_COINS; i++) {
            IERC20(coins[i]).transfer(msg.sender, amounts[i]);
        }
        
        emit RemoveLiquidity(msg.sender, amounts, [uint256(0), 0], totalSupply);
        
        return amounts;
    }
    
    /**
     * @notice Remove liquidity in one token
     * @param amount Amount of LP tokens to burn
     * @param i Index of token to receive
     * @param min_amount Minimum tokens to receive
     * @return Amount received
     */
    function remove_liquidity_one_coin(
        uint256 amount,
        int128 i,
        uint256 min_amount
    ) external returns (uint256) {
        require(amount > 0, "Invalid amount");
        require(i >= 0 && i < N_COINS, "Invalid i");
        
        // Calculate dy
        uint256[N_COINS] memory rates = _getRates();
        uint256[N_COINS] memory xp = _xp(rates);
        uint256 d0 = _invariant(xp);
        
        uint256[N_COINS] memory new_balances = balances;
        new_balances[uint256(i)] -= amount * xp[uint256(i)] / d0;
        
        uint256 dy = 0;
        uint256 dy_fee = 0;
        
        {
            uint256[N_COINS] memory new_xp = _xp_mem(rates, new_balances);
            uint256 d1 = _invariant(new_xp);
            
            dy = new_balances[uint256(i)] - _getYD(i, d1, new_xp);
            
            // Apply fees
            dy_fee = dy * fee / MAX_FEE;
            dy = (dy - dy_fee) * (MAX_FEE - admin_fee) / MAX_FEE;
        }
        
        require(dy >= min_amount, "Slippage");
        
        // Update state
        _burn(msg.sender, amount);
        balances[uint256(i)] -= (dy + dy_fee);
        
        // Transfer
        IERC20(coins[uint256(i)]).transfer(msg.sender, dy);
        
        emit RemoveLiquidityOne(msg.sender, amount, dy, [dy_fee, 0]);
        
        return dy;
    }
    
    // ============ A (Amplification) Management ============
    
    function ramp_A(uint256 _future_A, uint256 _future_time) external onlyOwner {
        require(block.timestamp >= initial_A_time + MIN_RAMP_DURATION, "Ramp not ready");
        require(_future_time >= block.timestamp + MIN_RAMP_DURATION, "Duration too short");
        
        uint256 current_A = A;
        require(_future_A > current_A, "A must increase");
        
        initial_A = current_A;
        future_A = _future_A;
        initial_A_time = block.timestamp;
        future_A_time = _future_time;
        
        emit RampA(current_A, _future_A, block.timestamp, _future_time);
    }
    
    function stop_ramp_A() external {
        require(future_A_time > block.timestamp, "Already stopped");
        
        uint256 current_A = _get_A();
        
        initial_A = current_A;
        future_A = current_A;
        initial_A_time = block.timestamp;
        future_A_time = block.timestamp;
        
        emit StopRampA(current_A, block.timestamp);
    }
    
    function _get_A() internal view returns (uint256) {
        if (block.timestamp >= future_A_time) {
            return future_A;
        }
        
        uint256 t = future_A_time - initial_A_time;
        uint256 w = block.timestamp - initial_A_time;
        
        uint256 A0 = initial_A * PRECISION;
        uint256 A1 = future_A * PRECISION;
        
        if (future_A > initial_A) {
            return A0 + (A1 - A0) * w / t;
        } else {
            return A0 - (A0 - A1) * w / t;
        }
    }
    
    // ============ Internal Functions ============
    
    function _xp(uint256[N_COINS] memory rates) internal view returns (uint256[N_COINS] memory) {
        uint256[N_COINS] memory xp = [
            rates[0] * balances[0] / PRECISION,
            rates[1] * balances[1] / PRECISION
        ];
        return xp;
    }
    
    function _xp_mem(uint256[N_COINS] memory rates, uint256[N_COINS] memory _balances) 
        internal pure returns (uint256[N_COINS] memory) {
        return [
            rates[0] * _balances[0] / PRECISION,
            rates[1] * _balances[1] / PRECISION
        ];
    }
    
    function _invariant(uint256[N_COINS] memory xp) internal view returns (uint256) {
        return _getD(xp);
    }
    
    function _getD(uint256[N_COINS] memory xp) internal view returns (uint256) {
        uint256 S = xp[0] + xp[1];
        if (S == 0) return 0;
        
        uint256 N = N_COINS * PRECISION;
        uint256 M_ant = A_precise / N;
        uint256 D = S;
        
        for (uint256 i = 0; i < 255; i++) {
            uint256 D_prev = D;
            D = (D * D + M_ant * S * N / xp[0] * N / xp[1]) / (2 * D + M_ant * N - N * N * PRECISION);
            
            if (D > D_prev + 1) {
                break;
            }
        }
        
        return D;
    }
    
    function _getYD(uint256 i, uint256 D, uint256[N_COINS] memory xp) internal pure returns (uint256) {
        require(i < N_COINS, "Invalid i");
        
        uint256 N = N_COINS * PRECISION;
        uint256 c = D * D / (xp[i] * N) * N;
        uint256 b = D + xp[i] * N / (A_precise / N);
        
        uint256 y = D;
        for (uint256 j = 0; j < 255; j++) {
            uint256 y_prev = y;
            y = (y * y + c) / (2 * y + b - D);
            
            if (y > y_prev + 1) {
                break;
            }
        }
        
        return y;
    }
    
    function _exchange(
        uint256[N_COINS] memory xp,
        uint256 i,
        uint256 j,
        uint256 x,
        uint256[N_COINS] memory rates
    ) internal view returns (uint256) {
        uint256 dy = _getYD(j, x, xp);
        
        // Apply fee
        uint256 dy_fee = dy * fee / MAX_FEE;
        dy = (dy - dy_fee) * (MAX_FEE - admin_fee) / MAX_FEE;
        
        return dy * PRECISION / rates[j];
    }
    
    function _getRates() internal view returns (uint256[N_COINS] memory) {
        return [PRECISION, PRECISION]; // Simplified - would use price oracles
    }
    
    // ============ Token Functions ============
    
    function _mint(address to, uint256 amount) internal {
        balanceOf[to] += amount;
        totalSupply += amount;
    }
    
    function _burn(address from, uint256 amount) internal {
        balanceOf[from] -= amount;
        totalSupply -= amount;
    }
    
    // ============ ERC20 Functions ============
    
    function transfer(address to, uint256 amount) external returns (bool) {
        balanceOf[msg.sender] -= amount;
        balanceOf[to] += amount;
        return true;
    }
    
    function transferFrom(address from, address to, uint256 amount) external returns (bool) {
        if (msg.sender != from) {
            allowance[from][msg.sender] -= amount;
        }
        balanceOf[from] -= amount;
        balanceOf[to] += amount;
        return true;
    }
    
    function approve(address spender, uint256 amount) external returns (bool) {
        allowance[msg.sender][spender] = amount;
        return true;
    }
    
    // ============ Admin Functions ============
    
    function commit_transfer_ownership(address new_owner) external onlyOwner {
        future_owner = new_owner;
    }
    
    function accept_transfer_ownership() external {
        require(msg.sender == future_owner, "Not future owner");
        owner = future_owner;
        future_owner = address(0);
    }
    
    function set_fee(uint256 new_fee) external onlyOwner {
        require(new_fee < MAX_FEE / 10, "Fee too high");
        fee = new_fee;
    }
    
    function set_admin_fee(uint256 new_admin_fee) external onlyOwner {
        require(new_admin_fee < MAX_FEE / 2, "Admin fee too high");
        admin_fee = new_admin_fee;
    }
}

// ============ Basic IERC20 ============
interface IERC20 {
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
}
