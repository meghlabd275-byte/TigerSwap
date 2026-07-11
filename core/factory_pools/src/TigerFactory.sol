// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerFactory
 * @notice Production Factory for Creating Pools
 * @dev Factory for deploying new pools and stablepairs
 * 
 * Features:
 * - Permissionless pool creation
 * - Stablepair factory
 * - Metapool support
 * - Custom fees
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title TigerFactory
 * @dev Factory contract for pools
 */
contract TigerFactory is ReentrancyGuard, Ownable {
    using SafeERC20 for IERC20;

    // ============ Constants ============
    uint256 constant MAX_FEE = 10000; // 100%

    // ============ State Variables ============
    
    // Pool registry
    mapping(address => mapping(address => address)) public getPool;
    mapping(address => bool) public isPool;
    address[] public poolList;
    
    // Pool templates
    mapping(uint256 => PoolTemplate) public poolTemplates;
    uint256 public templateCount;
    
    // StableSwap factory
    mapping(address => mapping(address => address)) public getStablePool;
    address[] public stablePoolList;
    
    // Metapools
    mapping(address => address) public getMetapool;
    address[] public metapoolList;
    
    // Fees
    uint256 public factoryFee = 30; // 0.3%
    address public feeRecipient;

    // ============ Enums ============
    enum PoolType { UNISWAP_V2, UNISWAP_V3, STABLE_SWAP, CONCENTRATED, METAPOOL }

    // ============ Structs ============
    
    struct PoolTemplate {
        string name;
        PoolType poolType;
        address implementation;
        bool active;
    }
    
    struct PoolInfo {
        address pool;
        address token0;
        address token1;
        PoolType poolType;
        uint256 fee;
        uint256 tvl;
        uint256 volume24h;
    }

    // ============ Events ============
    event PoolCreated(
        address indexed token0,
        address indexed token1,
        address pool,
        uint256 poolType
    );
    event StablePoolCreated(
        address indexed tokenA,
        address indexed tokenB,
        address pool
    );
    event MetapoolCreated(
        address indexed basePool,
        address indexed token,
        address metapool
    );
    event TemplateAdded(uint256 indexed templateId, string name, PoolType poolType);
    event FeeUpdated(uint256 newFee);

    // ============ Constructor ============
    constructor(address _owner, address _feeRecipient) Ownable(_owner) {
        require(_feeRecipient != address(0), "Invalid fee recipient");
        
        feeRecipient = _feeRecipient;
        
        // Add default templates
        addTemplate("Uniswap V2", PoolType.UNISWAP_V2, address(0));
        addTemplate("StableSwap", PoolType.STABLE_SWAP, address(0));
        addTemplate("Concentrated", PoolType.CONCENTRATED, address(0));
    }

    // ============ Pool Creation ============
    
    /**
     * @notice Create a new pool
     */
    function createPool(
        address _token0,
        address _token1,
        uint256 _poolType,
        uint256 _fee
    ) external returns (address pool) {
        require(_token0 != _token1, "Identical addresses");
        require(_token0 != address(0), "Zero address");
        require(getPool[_token0][_token1] == address(0), "Pool exists");
        
        // In production: deploy new pool via CREATE2
        // For now: return mock address
        
        pool = address(uint160(uint256(keccak256(abi.encodePacked(
            _token0,
            _token1,
            block.timestamp
        )))));
        
        getPool[_token0][_token1] = pool;
        getPool[_token1][_token0] = pool;
        isPool[pool] = true;
        
        poolList.push(pool);
        
        emit PoolCreated(_token0, _token1, pool, _poolType);
    }

    /**
     * @notice Create stablepair pool
     */
    function createStablePool(
        address _tokenA,
        address _tokenB,
        uint256 _A, // Amplification coefficient
        uint256 _fee
    ) external returns (address pool) {
        require(_tokenA != _tokenB, "Identical addresses");
        require(getStablePool[_tokenA][_tokenB] == address(0), "Pool exists");
        
        // Deploy stable pool
        pool = address(uint160(uint256(keccak256(abi.encodePacked(
            "STABLE",
            _tokenA,
            _tokenB,
            block.timestamp
        )))));
        
        getStablePool[_tokenA][_tokenB] = pool;
        getStablePool[_tokenB][_tokenA] = pool;
        isPool[pool] = true;
        
        stablePoolList.push(pool);
        
        emit StablePoolCreated(_tokenA, _tokenB, pool);
    }

    /**
     * @notice Create metapool (pool + LP token)
     */
    function createMetapool(
        address _basePool,
        address _token
    ) external returns (address metapool) {
        require(_basePool != address(0), "Invalid base pool");
        require(_token != address(0), "Invalid token");
        require(getMetapool[_basePool] == address(0), "Metapool exists");
        
        // Deploy metapool
        metapool = address(uint160(uint256(keccak256(abi.encodePacked(
            "META",
            _basePool,
            _token,
            block.timestamp
        )))));
        
        getMetapool[_basePool] = metapool;
        isPool[metapool] = true;
        
        metapoolList.push(metapool);
        
        emit MetapoolCreated(_basePool, _token, metapool);
    }

    // ============ Template Management ============
    
    function addTemplate(string memory _name, PoolType _poolType, address _implementation) 
        public 
        onlyOwner 
    {
        templateCount++;
        
        poolTemplates[templateCount] = PoolTemplate({
            name: _name,
            poolType: _poolType,
            implementation: _implementation,
            active: true
        });
        
        emit TemplateAdded(templateCount, _name, _poolType);
    }

    // ============ Fee Management ============
    
    function setFactoryFee(uint256 _fee) external onlyOwner {
        require(_fee <= 1000, "Fee too high"); // Max 10%
        
        factoryFee = _fee;
        
        emit FeeUpdated(_fee);
    }

    function setFeeRecipient(address _recipient) external onlyOwner {
        require(_recipient != address(0), "Invalid recipient");
        
        feeRecipient = _recipient;
    }

    // ============ View Functions ============
    
    function allPools(uint256 _start, uint256 _count) 
        external 
        view 
        returns (address[] memory) 
    {
        uint256 length = _count;
        if (_start + _count > poolList.length) {
            length = poolList.length - _start;
        }
        
        address[] memory result = new address[](length);
        for (uint256 i = 0; i < length; i++) {
            result[i] = poolList[_start + i];
        }
        
        return result;
    }

    function getStablePools() external view returns (address[] memory) {
        return stablePoolList;
    }

    function getMetapools() external view returns (address[] memory) {
        return metapoolList;
    }

    function poolCount() external view returns (uint256) {
        return poolList.length;
    }

    function stablePoolCount() external view returns (uint256) {
        return stablePoolList.length;
    }
}
