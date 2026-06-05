// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerFactory
 * @notice Factory contract for creating liquidity pools
 */
contract TigerFactory {
    bytes32 public constant INIT_CODE_HASH = keccak256(abi.encodePacked(type(TigerPair).creationCode));

    address public feeTo;
    address public feeToSetter;
    address public router;

    mapping(address => mapping(address => address)) public getPair;
    mapping(address => address[]) public allPairs;

    event PairCreated(address indexed token0, address indexed token1, address pair, uint256);

    constructor(address _feeToSetter) {
        feeToSetter = _feeToSetter;
    }

    function setFeeTo(address _feeTo) external {
        require(msg.sender == feeToSetter, "TigerFactory: FORBIDDEN");
        feeTo = _feeTo;
    }

    function setFeeToSetter(address _feeToSetter) external {
        require(msg.sender == feeToSetter, "TigerFactory: FORBIDDEN");
        feeToSetter = _feeToSetter;
    }

    function setRouter(address _router) external {
        require(msg.sender == feeToSetter, "TigerFactory: FORBIDDEN");
        router = _router;
    }

    function createPair(address tokenA, address tokenB) external returns (address pair) {
        require(tokenA != tokenB, "TigerFactory: IDENTICAL_ADDRESSES");
        (address token0, address token1) = tokenA < tokenB ? (tokenA, tokenB) : (tokenB, tokenA);
        require(token0 != address(0), "TigerFactory: ZERO_ADDRESS");
        require(getPair[token0][token1] == address(0), "TigerFactory: PAIR_EXISTS");

        bytes memory bytecode = type(TigerPair).creationCode;
        bytes32 salt = keccak256(abi.encodePacked(token0, token1));
        assembly {
            pair := create2(0, add(bytecode, 32), mload(bytecode), salt)
        }

        ITigerPair(pair).initialize(token0, token1);
        getPair[token0][token1] = pair;
        getPair[token1][token0] = pair;
        allPairs[token0].push(pair);
        emit PairCreated(token0, token1, pair, allPairs[token0].length);
    }

    function getPair(address tokenA, address tokenB) external view returns (address) {
        return getPair[tokenA][tokenB];
    }

    function allPairsLength() external view returns (uint256) {
        return allPairs[msg.sender].length;
    }
}

interface ITigerPair {
    function initialize(address, address) external;
}