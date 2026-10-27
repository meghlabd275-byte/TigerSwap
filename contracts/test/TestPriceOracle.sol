// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @notice Oracle stub for tests. Only the price lookup is used by the
///         contracts under test; there is no median logic to fake here.
contract TestPriceOracle {
    mapping(address => uint256) public prices;

    function setPrice(address asset, uint256 price) external {
        prices[asset] = price;
    }

    function getPrice(address asset) external view returns (uint256) {
        uint256 price = prices[asset];
        require(price > 0, "TestPriceOracle: NO_PRICE");
        return price;
    }
}
