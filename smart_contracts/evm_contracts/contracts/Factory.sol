// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./Pair.sol";

/**
 * @title TigerSwapFactory
 * @notice Factory contract for creating liquidity pools (Pairs)
 * @dev Core contract that maintains registry of all pairs and fee parameters
 */
contract TigerSwapFactory {
    using SafeMath for uint256;

    // Fee recipient - protocol fees go here
    address public feeTo;
    address public feeToSetter;

    // All pairs (token0 < token1 by address comparison)
    mapping(address => mapping(address => address)) public getPair;
    address[] public allPairs;

    // Fee configuration (in basis points)
    uint256 public constant PROTOCOL_FEE_BPS = 25; // 0.25% of each swap
    uint256 public constant LP_REWARDS_BPS = 5;   // 0.05% to LP providers
    
    // Token ordering - always store tokens in ascending address order
    mapping(address => address) public tokenOrder;

    // Events
    event PairCreated(address indexed token0, address indexed token1, address pair, uint256);
    event FeeToUpdated(address indexed newFeeTo);
    event FeeToSetterUpdated(address indexed newSetter);

    constructor(address _feeToSetter) {
        feeToSetter = _feeToSetter;
        feeTo = address(0);
    }

    /**
     * @notice Returns the number of pairs created
     */
    function allPairsLength() external view returns (uint256) {
        return allPairs.length;
    }

    /**
     * @notice Creates a new liquidity pool for token0/token1 pair
     * @param _token0 First token (must be lower address)
     * @param _token1 Second token (must be higher address)
     */
    function createPair(address _token0, address _token1) external returns (address pair) {
        require(_token0 != _token1, "TigerSwap: IDENTICAL_ADDRESSES");
        
        // Ensure consistent ordering - always token0 < token1
        (address token0, address token1) = _token0 < _token1 
            ? (_token0, _token1) 
            : (_token1, _token0);

        require(token0 != address(0), "TigerSwap: ZERO_ADDRESS");
        require(getPair[token0][token1] == address(0), "TigerSwap: PAIR_EXISTS");

        // Create new pair contract
        bytes memory bytecode = type(TigerSwapPair).creationCode;
        bytes32 salt = keccak256(abi.encodePacked(token0, token1));
        assembly {
            pair := create2(0, add(bytecode, 32), mload(bytecode), salt)
        }

        // Initialize the pair
        TigerSwapPair(pair).initialize(token0, token1);

        // Register the pair
        getPair[token0][token1] = pair;
        getPair[token1][token0] = pair; // Reverse mapping
        allPairs.push(pair);

        emit PairCreated(token0, token1, pair, allPairs.length);
    }

    /**
     * @notice Updates the protocol fee recipient address
     */
    function setFeeTo(address _feeTo) external {
        require(msg.sender == feeToSetter, "TigerSwap: FORBIDDEN");
        feeTo = _feeTo;
        emit FeeToUpdated(_feeTo);
    }

    /**
     * @notice Updates the fee setter (governance)
     */
    function setFeeToSetter(address _feeToSetter) external {
        require(msg.sender == feeToSetter, "TigerSwap: FORBIDDEN");
        feeToSetter = _feeToSetter;
        emit FeeToSetterUpdated(_feeToSetter);
    }

    /**
     * @notice Returns pair address for two tokens
     */
    function getPairAddress(address _tokenA, address _tokenB) external view returns (address) {
        (address token0, address token1) = _tokenA < _tokenB 
            ? (_tokenA, _tokenB) 
            : (_tokenB, _tokenA);
        return getPair[token0][token1];
    }
}