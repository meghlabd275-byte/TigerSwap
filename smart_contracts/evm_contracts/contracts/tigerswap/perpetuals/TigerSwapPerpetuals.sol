// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title TigerSwapPerpetuals
 * @notice Perpetual Futures Trading - Similar to GMX
 */
contract TigerSwapPerpetuals is ReentrancyGuard, Ownable {
    using SafeERC20 for IERC20;

    uint256 public constant PRECISION = 1e30;
    uint256 public constant BASIS_POINTS = 1e4;
    uint256 public constant MAX_LEVERAGE = 100e4;

    address public weth;
    address public wbtc;
    address public usdc;
    address public nativeToken;

    uint256 public poolAmount;
    uint256 public guaranteedUsd;
    uint256 public totalShares;

    mapping(bytes32 => Position) public positions;
    mapping(address => bytes32[]) public userPositionIds;

    uint256 public positionFee = 10;
    uint256 public liquidationFee = 50;
    bool public isPaused;

    mapping(address => address) public priceFeeds;

    event PositionOpened(address indexed account, bytes32 indexed positionId, address indexed collateralToken, address indexToken, bool isLong, uint256 size, uint256 collateral);
    event PositionModified(address indexed account, bytes32 indexed positionId, uint256 sizeDelta, uint256 collateralDelta);
    event PositionClosed(address indexed account, bytes32 indexed positionId, uint256 size, int256 pnl);
    event PositionLiquidated(address indexed account, bytes32 indexed positionId, address indexed liquidator, uint256 size);

    struct Position {
        address owner;
        address collateralToken;
        address indexToken;
        bool isLong;
        uint256 size;
        uint256 collateral;
        uint256 averagePrice;
        uint256 entryFundingRate;
        uint256 lastIncreasedTime;
    }

    constructor(address _weth, address _wbtc, address _usdc, address _nativeToken, address _owner) Ownable(_owner) {
        weth = _weth;
        wbtc = _wbtc;
        usdc = _usdc;
        nativeToken = _nativeToken;
    }

    function increasePosition(address _account, address _collateralToken, address _indexToken, uint256 _collateralDelta, uint256 _sizeDelta, bool _isLong) external nonReentrant returns (bytes32 positionId) {
        require(!isPaused, "Paused");

        uint256 price = _getPrice(_indexToken);
        require(price > 0, "Invalid price");

        positionId = _getPositionId(_account, _collateralToken, _indexToken, _isLong);
        Position storage position = positions[positionId];

        if (position.size == 0) {
            position.owner = _account;
            position.collateralToken = _collateralToken;
            position.indexToken = _indexToken;
            position.isLong = _isLong;
            position.entryFundingRate = 0;
            position.lastIncreasedTime = block.timestamp;
            userPositionIds[_account].push(positionId);
        }

        if (_collateralDelta > 0) {
            IERC20(_collateralToken).safeTransferFrom(_account, address(this), _collateralDelta);
            position.collateral += _collateralDelta;
        }

        if (_sizeDelta > 0) {
            require(position.collateral > 0, "No collateral");

            uint256 newSize = position.size + _sizeDelta;
            require(newSize <= position.collateral * MAX_LEVERAGE / BASIS_POINTS, "Max leverage");

            if (position.size == 0) {
                position.averagePrice = price;
            } else {
                position.averagePrice = (position.size * position.averagePrice + _sizeDelta * price) / (position.size + _sizeDelta);
            }

            position.size = newSize;
            position.lastIncreasedTime = block.timestamp;
        }

        emit PositionOpened(_account, positionId, _collateralToken, _indexToken, _isLong, position.size, position.collateral);
    }

    function decreasePosition(address _account, address _collateralToken, address _indexToken, uint256 _collateralDelta, uint256 _sizeDelta, bool _isLong, bool _close) external nonReentrant returns (bytes32 positionId) {
        require(!isPaused, "Paused");

        positionId = _getPositionId(_account, _collateralToken, _indexToken, _isLong);
        Position storage position = positions[positionId];

        require(position.size > 0, "No position");

        uint256 price = _getPrice(_indexToken);
        int256 pnl = _getPnL(position, price);

        if (_close || _sizeDelta >= position.size) {
            _sizeDelta = position.size;

            uint256 remainingCollateral = position.collateral;
            if (_collateralDelta > 0) remainingCollateral -= _collateralDelta;

            uint256 exitFee = (_sizeDelta * positionFee) / BASIS_POINTS;

            if (pnl > 0) remainingCollateral += uint256(pnl);
            else if (remainingCollateral > uint256(-pnl)) remainingCollateral -= uint256(-pnl);
            else remainingCollateral = 0;

            if (remainingCollateral > 0) {
                IERC20(_collateralToken).safeTransfer(_account, remainingCollateral - exitFee);
            }

            delete positions[positionId];
            emit PositionClosed(_account, positionId, position.size, pnl);
        } else {
            position.size -= _sizeDelta;
            if (_collateralDelta > 0) {
                position.collateral -= _collateralDelta;
                IERC20(_collateralToken).safeTransfer(_account, _collateralDelta);
            }
            emit PositionModified(_account, positionId, _sizeDelta, _collateralDelta);
        }

        guaranteedUsd -= _sizeDelta;
    }

    function liquidatePosition(address _account, address _collateralToken, address _indexToken, bool _isLong) external nonReentrant returns (bytes32 positionId) {
        require(!isPaused, "Paused");

        positionId = _getPositionId(_account, _collateralToken, _indexToken, _isLong);
        Position storage position = positions[positionId];

        require(position.size > 0, "No position");

        uint256 price = _getPrice(_indexToken);
        require(_isLiquidatable(position, price), "Not liquidatable");

        uint256 fee = (position.size * liquidationFee) / BASIS_POINTS;
        IERC20(_collateralToken).safeTransfer(msg.sender, fee / 2);

        delete positions[positionId];
        guaranteedUsd -= position.size;

        emit PositionLiquidated(_account, positionId, msg.sender, position.size);
    }

    function addLiquidity(address _token, uint256 _amount) external nonReentrant {
        require(_amount > 0, "Invalid amount");

        IERC20(_token).safeTransferFrom(msg.sender, address(this), _amount);

        uint256 share = totalShares == 0 ? _amount : (_amount * totalShares) / poolAmount;
        totalShares += share;
        poolAmount += _amount;
    }

    function removeLiquidity(address _token, uint256 _share) external nonReentrant {
        require(_share > 0 && _share <= totalShares, "Invalid share");

        uint256 amount = (poolAmount * _share) / totalShares;
        totalShares -= _share;
        poolAmount -= amount;

        IERC20(_token).safeTransfer(msg.sender, amount);
    }

    function _getPositionId(address _account, address _collateralToken, address _indexToken, bool _isLong) internal pure returns (bytes32) {
        return keccak256(abi.encodePacked(_account, _collateralToken, _indexToken, _isLong));
    }

    function _getPrice(address _token) internal view returns (uint256) {
        if (_token == weth) return 3500e30;
        if (_token == wbtc) return 95000e30;
        return 1e30;
    }

    function _getPnL(Position storage _position, uint256 _currentPrice) internal view returns (int256) {
        if (_position.size == 0) return 0;

        uint256 priceDelta = _position.averagePrice > _currentPrice ? _position.averagePrice - _currentPrice : _currentPrice - _position.averagePrice;
        uint256 pnl = (priceDelta * _position.size) / 1e30;

        return _position.isLong ? int256(pnl) : -int256(pnl);
    }

    function _isLiquidatable(Position storage _position, uint256 _currentPrice) internal view returns (bool) {
        int256 pnl = _getPnL(_position, _currentPrice);
        if (pnl < 0 && uint256(-pnl) >= _position.collateral) return true;

        uint256 leverage = (_position.size * BASIS_POINTS) / _position.collateral;
        if (leverage > MAX_LEVERAGE) return true;

        return false;
    }

    function getPosition(address _account, address _collateralToken, address _indexToken, bool _isLong) external view returns (Position memory) {
        return positions[_getPositionId(_account, _collateralToken, _indexToken, _isLong)];
    }

    function setPositionFee(uint256 _fee) external onlyOwner {
        positionFee = _fee;
    }

    function setLiquidationFee(uint256 _fee) external onlyOwner {
        liquidationFee = _fee;
    }

    function pause() external onlyOwner {
        isPaused = true;
    }

    function unpause() external onlyOwner {
        isPaused = false;
    }
}