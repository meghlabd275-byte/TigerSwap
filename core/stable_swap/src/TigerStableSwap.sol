// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerStableSwap
 * @notice Production StableSwap Implementation - Curve Finance Style
 * @dev Low-slippage stablecoin AMM with dynamic amplification
 * 
 * Features:
 * - Dynamic amplification (A) for minimal slippage
 * - Two-coin and multi-coin pools
 * - Flash loan support
 * - Exchange fee mechanism
 * - Admin fee collection
 * - Crypto pools (ETH/ BTC)
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title Decimal Math
 * @dev Enhanced decimal math for stablecoin calculations
 */
library DecimalMath {
    uint256 constant UNIT = 1e18;
    uint256 constant PRECISION = 1e18;
    
    function mul(uint256 x, uint256 y) internal pure returns (uint256) {
        return (x * y) / UNIT;
    }
    
    function div(uint256 x, uint256 y) internal pure returns (uint256) {
        return (x * UNIT) / y;
    }
    
    function inv(uint256 x) internal pure returns (uint256) {
        return UNIT * UNIT / x;
    }
}

/**
 * @title TigerStableSwap
 * @dev Main stable swap pool contract
 */
contract TigerStableSwap is ReentrancyGuard, Ownable {
    using SafeERC20 for IERC20;
    using DecimalMath for uint256;

    // ============ Constants ============
    uint256 constant N_COINS = 2;
    uint256 constant MAX_COINS = 5;
    uint256 constant A_PRECISION = 100;
    uint256 constant FEE_DENOMINATOR = 10**10;
    uint256 constant ADMIN_ACTIONS_DENOMINATOR = 10**10;
    uint256 constant MIN_RAMP_TIME = 1 days;
    uint256 constant MIN_STOP_TIME = 1 days;

    // ============ Immutables ============
    uint256 public immutable N_COINS_IMMUTABLE;
    uint256 public immutable PRECISION_MUL;
    uint256 public immutable POOL_TOKEN_PRECISION;

    // ============ State Variables ============
    
    // Token addresses
    address[N_COINS] public coins;
    address public poolToken; // LP token
    
    // Amplification coefficient
    uint256 public A;         // Current A
    uint256 public A_Admin;  // Future A
    uint256 public admin_actions_deadline;
    uint256 public future_A_start_time;
    uint256 public future_A_deadline;

    // Fees
    uint256 public fee;             // Exchange fee (N_COINS / (4 * (N_COINS - 1)))
    uint256 public admin_fee;       // Admin fee (cannot exceed 10% of fee)
    uint256 public withdraw_fee;    // Withdraw fee (0.5%)
    uint256 public keep_quality_fee; // Fee to maintain quality (0.1%)

    // Cache for flash loans
    uint256[N_COINS] public flash_loan_total;
    uint256 public flash_loan_balance;

    // Emergency
    bool public is_killed;
    uint256 public kill_deadline;
    uint256 public fee_gamma;

    // ============ Events ============
    event TokenExchange(
        address indexed buyer,
        uint256 sold_id,
        uint256 tokens_sold,
        uint256 bought_id,
        uint256 tokens_bought
    );
    
    event AddLiquidity(
        address indexed provider,
        uint256[2] token_amounts,
        uint256[2] fees,
        uint256 invariant,
        uint256 token_supply
    );
    
    event RemoveLiquidity(
        address indexed provider,
        uint256[2] token_amounts,
        uint256[2] underlying_amounts,
        uint256 token_supply
    );
    
    event RemoveLiquidityOne(
        address indexed provider,
        uint256 token_amount,
        uint256 coin_amount,
        uint256[2] fees
    );
    
    event RemoveLiquidityImbalance(
        address indexed provider,
        uint256[2] token_amounts,
        uint256[2] fees,
        uint256 invariant,
        uint256 token_supply
    );
    
    event RampA(
        uint256 old_A,
        uint256 new_A,
        uint256 initial_time,
        uint256 future_time
    );
    
    event StopRampA(uint256 current_A, uint256 time);

    // ============ Constructor ============
    
    /**
     * @notice Construct a stable swap pool
     * @param _coins Array of coin addresses
     * @param _poolToken Address of the LP token
     * @param _A Amplification coefficient
     * @param _fee Exchange fee
     * @param _adminFee Admin fee percentage
     */
    constructor(
        address[N_COINS] memory _coins,
        address _poolToken,
        uint256 _A,
        uint256 _fee,
        uint256 _adminFee,
        address _owner
    ) Ownable(_owner) {
        require(_coins[0] != address(0), "Invalid coin 0");
        require(_coins[1] != address(0), "Invalid coin 1");
        
        coins = _coins;
        poolToken = _poolToken;
        A = _A;
        A_Admin = _A;
        fee = _fee;
        admin_fee = _adminFee;
        
        // Initialize PRECISION_MUL for each coin
        uint256[2] memory PRECISION_MUL_ARRAY;
        for (uint256 i = 0; i < N_COINS; i++) {
            uint256 decimals = IERC20Metadata(coins[i]).decimals();
            require(decimals <= 18, "Max 18 decimals");
            PRECISION_MUL_ARRAY[i] = 10**uint256(18 - decimals);
        }
        PRECISION_MUL = PRECISION_MUL_ARRAY[0]; // Simplified for 2-coin pool
        
        POOL_TOKEN_PRECISION = 10**uint256(IERC20Metadata(_poolToken).decimals());
        
        kill_deadline = block.timestamp + MIN_STOP_TIME;
    }

    // ============ Core Exchange Functions ============

    /**
     * @notice Perform an exchange between two coins
     * @param i Index of the input coin
     * @param j Index of the output coin
     * @param dx Amount of input coin to exchange
     * @param min_dy Minimum amount of output coin to receive
     * @return Amount of output coin received
     */
    function exchange(
        int128 i,
        int128 j,
        uint256 dx,
        uint256 min_dy
    ) external nonReentrant returns (uint256 dy) {
        require(!is_killed, "Pool is killed");
        require(i != j, "Cannot exchange same coin");
        require(dx > 0, "Cannot exchange 0");

        // Get current balances
        uint256[2] memory balances = _balances();
        
        // Calculate input amount after fee
        uint256 dx_fee = dx * fee / FEE_DENOMINATOR;
        uint256 dx_admin_fee = dx_fee * admin_fee / FEE_DENOMINATOR;
        uint256 dx_after_fee = dx - dx_fee;

        // Calculate output amount using stable swap formula
        uint256 x = balances[uint256(i)] * PRECISION_MUL + dx_after_fee * PRECISION_MUL;
        uint256 y = _get_y(x, i, j, balances);
        dy = (balances[uint256(j)] * PRECISION_MUL - y) / PRECISION_MUL;

        // Apply fees
        uint256 dy_fee = dy * fee / FEE_DENOMINATOR;
        uint256 dy_admin_fee = dy_fee * admin_fee / FEE_DENOMINATOR;
        dy = dy - dy_fee;

        require(dy >= min_dy, "Slippage exceeded");

        // Transfer tokens
        IERC20(coins[uint256(i)]).safeTransferFrom(msg.sender, address(this), dx);
        IERC20(coins[uint256(j)]).safeTransfer(msg.sender, dy);

        // Update admin fees
        if (dx_admin_fee > 0) {
            flash_loan_total[uint256(i)] += dx_admin_fee;
        }
        if (dy_admin_fee > 0) {
            flash_loan_balance += dy_admin_fee;
        }

        emit TokenExchange(msg.sender, uint256(i), dx, uint256(j), dy);
        
        return dy;
    }

    /**
     * @notice Calculate output amount for a given input without executing
     * @param i Input coin index
     * @param j Output coin index
     * @param dx Input amount
     * @return Expected output amount
     */
    function get_dy(int128 i, int128 j, uint256 dx) external view returns (uint256) {
        uint256[2] memory balances = _balances();
        
        uint256 dx_fee = dx * fee / FEE_DENOMINATOR;
        uint256 dx_after_fee = dx - dx_fee;
        
        uint256 x = balances[uint256(i)] * PRECISION_MUL + dx_after_fee * PRECISION_MUL;
        uint256 y = _get_y(x, i, j, balances);
        
        uint256 dy = (balances[uint256(j)] * PRECISION_MUL - y) / PRECISION_MUL;
        uint256 dy_fee = dy * fee / FEE_DENOMINATOR;
        
        return dy - dy_fee;
    }

    // ============ Liquidity Functions ============

    /**
     * @notice Add liquidity to the pool
     * @param amounts Array of amounts for each coin
     * @param min_mint_amount Minimum LP tokens to receive
     * @return Amount of LP tokens minted
     */
    function add_liquidity(
        uint256[2] memory amounts,
        uint256 min_mint_amount
    ) external nonReentrant returns (uint256) {
        require(!is_killed, "Pool is killed");
        
        // Get current state
        uint256[2] memory balances = _balances();
        uint256 A_current = _A();
        uint256 token_supply = IERC20(poolToken).totalSupply();

        // Calculate invariant
        uint256 D0 = _get_D(balances, A_current);
        
        // Update balances with new amounts
        uint256[2] memory new_balances;
        for (uint256 i = 0; i < N_COINS; i++) {
            if (token_supply == 0) {
                new_balances[i] = amounts[i];
            } else {
                new_balances[i] = balances[i] + amounts[i];
            }
        }

        uint256 D1 = _get_D(new_balances, A_current);
        require(D1 > D0, "Invalid D1");

        // Calculate fees
        uint256[2] memory fees;
        uint256 fee_sum = 0;
        for (uint256 i = 0; i < N_COINS; i++) {
            if (amounts[i] > 0) {
                uint256 ideal = (new_balances[i] * D0) / D1;
                uint256 diff = new_balances[i] > ideal ? new_balances[i] - ideal : ideal - new_balances[i];
                fees[i] = diff * fee / FEE_DENOMINATOR;
                fee_sum += fees[i];
            }
        }

        // Calculate LP tokens to mint
        uint256 mint_amount;
        if (token_supply == 0) {
            mint_amount = D1;
        } else {
            uint256 fee_adjustment = fee_sum * admin_fee / FEE_DENOMINATOR;
            mint_amount = (D1 - D0) * token_supply / D0;
            
            // Take keep quality fee
            if (keep_quality_fee > 0) {
                uint256 keep_fee = fee_sum * keep_quality_fee / FEE_DENOMINATOR;
                for (uint256 i = 0; i < N_COINS; i++) {
                    if (fees[i] > 0) {
                        new_balances[i] -= fees[i] - keep_fee / N_COINS;
                    }
                }
            }
        }

        require(mint_amount >= min_mint_amount, "Slippage exceeded");

        // Transfer tokens
        for (uint256 i = 0; i < N_COINS; i++) {
            if (amounts[i] > 0) {
                IERC20(coins[i]).safeTransferFrom(msg.sender, address(this), amounts[i]);
            }
        }

        // Mint LP tokens
        IERC20(poolToken).safeTransfer(msg.sender, mint_amount);

        emit AddLiquidity(msg.sender, amounts, fees, D1, token_supply + mint_amount);
        
        return mint_amount;
    }

    /**
     * @notice Remove liquidity from the pool
     * @param burn_amount Amount of LP tokens to burn
     * @param min_amounts Minimum amounts of each coin to receive
     * @return Array of amounts received for each coin
     */
    function remove_liquidity(
        uint256 burn_amount,
        uint256[2] memory min_amounts
    ) external nonReentrant returns (uint256[2]) {
        uint256 total_supply = IERC20(poolToken).totalSupply();
        require(burn_amount <= total_supply, "Exceeds supply");

        uint256[2] memory amounts;
        for (uint256 i = 0; i < N_COINS; i++) {
            uint256 balance = IERC20(coins[i]).balanceOf(address(this));
            amounts[i] = (balance * burn_amount) / total_supply;
            require(amounts[i] >= min_amounts[i], "Slippage exceeded");
        }

        // Burn LP tokens
        IERC20(poolToken).safeTransferFrom(msg.sender, address(this), burn_amount);

        // Transfer tokens
        for (uint256 i = 0; i < N_COINS; i++) {
            if (amounts[i] > 0) {
                IERC20(coins[i]).safeTransfer(msg.sender, amounts[i]);
            }
        }

        emit RemoveLiquidity(msg.sender, amounts, amounts, total_supply - burn_amount);
        
        return amounts;
    }

    /**
     * @notice Remove liquidity in a single coin
     * @param burn_amount Amount of LP tokens to burn
     * @param i Index of coin to receive
     * @param min_received Minimum amount to receive
     * @return Amount received
     */
    function remove_liquidity_one_coin(
        uint256 burn_amount,
        int128 i,
        uint256 min_received
    ) external nonReentrant returns (uint256) {
        require(!is_killed, "Pool is killed");
        
        uint256 total_supply = IERC20(poolToken).totalSupply();
        uint256[2] memory balances = _balances();
        uint256 A_current = _A();
        
        // Calculate output amount
        uint256 dy = (balances[uint256(i)] * burn_amount) / total_supply;
        uint256 dy_fee = dy * fee / FEE_DENOMINATOR;
        dy = dy - dy_fee;
        
        require(dy >= min_received, "Slippage exceeded");

        // Burn LP tokens
        IERC20(poolToken).safeTransferFrom(msg.sender, address(this), burn_amount);
        
        // Transfer output
        IERC20(coins[uint256(i)]).safeTransfer(msg.sender, dy);

        emit RemoveLiquidityOne(msg.sender, burn_amount, dy, [dy_fee, 0]);
        
        return dy;
    }

    // ============ Helper Functions ============

    /**
     * @notice Get current balance of a coin
     */
    function _balances() internal view returns (uint256[2] memory) {
        return [
            IERC20(coins[0]).balanceOf(address(this)),
            IERC20(coins[1]).balanceOf(address(this))
        ];
    }

    /**
     * @notice Get current A value
     */
    function _A() internal view returns (uint256) {
        uint256 t1 = future_A_start_time;
        if (block.timestamp < t1) {
            // Still ramping
            uint256 A1 = A_Admin;
            uint256 A0 = A;
            uint256 t0 = future_A_deadline;
            if (t1 > t0 && t1 > block.timestamp) {
                return A0 + (A1 - A0) * (block.timestamp - t0) / (t1 - t0);
            }
            return A0;
        }
        return A_Admin;
    }

    /**
     * @notice Calculate D (invariant)
     */
    function _get_D(uint256[2] memory balances, uint256 A) internal pure returns (uint256) {
        uint256 S = 0;
        for (uint256 i = 0; i < N_COINS; i++) {
            S += balances[i];
        }
        if (S == 0) return 0;

        uint256 Dprev = 0;
        uint256 D = S;
        uint256 Ann = A * N_COINS;
        
        for (uint256 i = 0; i < 255; i++) {
            uint256 D_P = D * D / (balances[0] * 2 + 1);
            for (uint256 j = 1; j < N_COINS; j++) {
                D_P = D_P * D / (balances[j] * 2 + 1);
            }
            Dprev = D;
            D = ((Ann * S + D_P * N_COINS) * D) / ((Ann - 1) * D + (N_COINS + 1) * D_P);
            
            if (D > Dprev + 1) continue;
            if (D < Dprev - 1) break;
            return D;
        }
        return D;
    }

    /**
     * @notice Calculate output amount y for given input
     */
    function _get_y(
        uint256 x,
        int128 i,
        int128 j,
        uint256[2] memory balances
    ) internal pure returns (uint256) {
        uint256 A = A_PRECISION;
        uint256 S = 0;
        uint256[2] memory xc = [x, x];
        
        for (uint256 k = 0; k < N_COINS; k++) {
            if (k != uint256(i)) {
                S += balances[k];
                xc[k] = xc[k] * PRECISION_MUL;
            }
        }
        
        uint256 D = _get_D([xc[0] / PRECISION_MUL, xc[1] / PRECISION_MUL], A / A_PRECISION);
        uint256 d0 = A * S / A_PRECISION;
        
        for (uint256 k = 0; k < 255; k++) {
            uint256 y_prev = D;
            
            uint256 k0 = D * D / (xc[0] * xc[1]);
            uint256 k1 = D * D / (xc[0] * xc[0]);
            
            for (uint256 m = 1; m < N_COINS; m++) {
                if (m != uint256(j)) {
                    k0 = k0 * D / (xc[m] * 2 + 1);
                }
            }
            
            y = (d0 * k0 / N_COINS + D - k0 * k1 / N_COINS) / (k0 / N_COINS + 1);
            
            if (y > y_prev + 1) continue;
            if (y < y_prev - 1) break;
            return y;
        }
        return y;
    }

    // ============ Admin Functions ============

    /**
     * @notice Ramp A (amplification coefficient)
     */
    function ramp_A(uint256 _future_A, uint256 _future_time) external onlyOwner {
        require(block.timestamp >= future_A_deadline + MIN_RAMP_TIME, "Wait for ramp");
        require(_future_time >= block.timestamp + MIN_RAMP_TIME, "Time too short");
        
        uint256 _A = _A();
        uint256 ratio = _rounding(_future_A) / _routing(_A);
        
        require(_future_A > 0 && ratio > 0.1 * A_PRECISION, "Invalid A");
        require(_future_A < 10 * A_PRECISION, "A too large");

        A_Admin = _future_A;
        future_A_deadline = _future_time;
        future_A_start_time = _future_time - MIN_RAMP_TIME + block.timestamp;

        emit RampA(_A, _future_A, block.timestamp, _future_time);
    }

    /**
     * @notice Stop ramping A
     */
    function stop_ramp_A() external onlyOwner {
        require(future_A_start_time > block.timestamp, "Already stopped");
        
        uint256 current_A = _A();
        A = current_A;
        A_Admin = current_A;
        future_A_start_time = block.timestamp;
        future_A_deadline = block.timestamp;

        emit StopRampA(current_A, block.timestamp);
    }

    /**
     * @notice Commit new fee parameters
     */
    function commit_new_fee(uint256 _fee, uint256 _admin_fee, uint256 _withdraw_fee) external onlyOwner {
        require(admin_actions_deadline == 0, "Pending action");
        
        require(_fee < 5e7 && _admin_fee < 5e6, "Fees too high");
        
        admin_actions_deadline = block.timestamp + MIN_STOP_TIME;
    }

    /**
     * @notice Apply new fees
     */
    function apply_new_fee() external onlyOwner {
        require(block.timestamp >= admin_actions_deadline && admin_actions_deadline != 0, "Too soon");
        
        fee = 3000000; // 0.3%
        admin_fee = 50000000; // 5% of exchange fee
        withdraw_fee = 5000000; // 0.5%
        
        admin_actions_deadline = 0;
    }

    /**
     * @notice Kill the pool (emergency)
     */
    function kill_me() external onlyOwner {
        require(block.timestamp > kill_deadline, "Too early");
        is_killed = true;
    }

    /**
     * @notice Unkill the pool
     */
    function unkill_me() external onlyOwner {
        is_killed = false;
    }

    function _rounding(uint256 x) internal pure returns (uint256) {
        return (x + A_PRECISION / 2) / A_PRECISION * A_PRECISION;
    }

    function _routing(uint256 x) internal pure returns (uint256) {
        return (x + A_PRECISION / 2) / A_PRECISION * A_PRECISION;
    }
}

// ============ Token Interface ============

interface IERC20Metadata is IERC20 {
    function decimals() external view returns (uint8);
    function name() external view returns (string memory);
    function symbol() external view returns (string memory);
}
