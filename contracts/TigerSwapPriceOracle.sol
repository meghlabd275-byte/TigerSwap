// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";

/**
 * @title TigerSwapPriceOracle
 * @notice Median-of-N price oracle. Each reporter is an independent source
 *         (a TigerSwap node, a Chainlink feed adapter, a Pyth adapter, or a
 *         TWAP reader). The oracle returns the median of the active reports,
 *         which makes a single bad or stale feed unable to move the price.
 */
contract TigerSwapPriceOracle {
    uint256 public constant WAD = 1e18;

    /// @notice A price submission. `timestamp` is when the source last updated.
    struct Report {
        uint256 price;      // WAD-scaled price
        uint256 timestamp;  // block timestamp of the update
        bool active;
    }

    address public owner;
    uint256 public maxStaleness;   // seconds before a report is ignored
    uint256 public minReports;     // minimum live reports required to answer

    mapping(address => bool) public isReporter;
    address[] public reporters;

    /// @dev asset => reporter => report
    mapping(address => mapping(address => Report)) private _reports;

    event ReporterUpdated(address indexed reporter, bool allowed);
    event PriceSubmitted(address indexed asset, address indexed reporter, uint256 price);
    event MaxStalenessUpdated(uint256 maxStaleness);
    event MinReportsUpdated(uint256 minReports);

    modifier onlyOwner() {
        require(msg.sender == owner, "TigerSwap: NOT_OWNER");
        _;
    }

    modifier onlyReporter() {
        require(isReporter[msg.sender], "TigerSwap: NOT_REPORTER");
        _;
    }

    constructor(uint256 _maxStaleness, uint256 _minReports) {
        require(_minReports >= 1, "TigerSwap: MIN_REPORTS");
        require(_maxStaleness > 0, "TigerSwap: ZERO_STALENESS");
        owner = msg.sender;
        maxStaleness = _maxStaleness;
        minReports = _minReports;
    }

    // ============ Admin ============

    function setReporter(address reporter, bool allowed) external onlyOwner {
        require(reporter != address(0), "TigerSwap: ZERO_ADDRESS");
        if (allowed && !isReporter[reporter]) {
            reporters.push(reporter);
        } else if (!allowed && isReporter[reporter]) {
            // Remove the entry so a later re-enable does not add a duplicate.
            for (uint256 i = 0; i < reporters.length; i++) {
                if (reporters[i] == reporter) {
                    reporters[i] = reporters[reporters.length - 1];
                    reporters.pop();
                    break;
                }
            }
        }
        isReporter[reporter] = allowed;
        emit ReporterUpdated(reporter, allowed);
    }

    function setMaxStaleness(uint256 _maxStaleness) external onlyOwner {
        require(_maxStaleness > 0, "TigerSwap: ZERO_STALENESS");
        maxStaleness = _maxStaleness;
        emit MaxStalenessUpdated(_maxStaleness);
    }

    function setMinReports(uint256 _minReports) external onlyOwner {
        require(_minReports >= 1, "TigerSwap: MIN_REPORTS");
        minReports = _minReports;
        emit MinReportsUpdated(_minReports);
    }

    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "TigerSwap: ZERO_ADDRESS");
        owner = newOwner;
    }

    // ============ Reporting ============

    /// @notice Submit a price for `asset`.
    function submitPrice(address asset, uint256 price) external onlyReporter {
        require(asset != address(0), "TigerSwap: ZERO_ASSET");
        require(price > 0, "TigerSwap: INVALID_PRICE");
        _reports[asset][msg.sender] = Report({price: price, timestamp: block.timestamp, active: true});
        emit PriceSubmitted(asset, msg.sender, price);
    }

    // ============ Reads ============

    /// @notice Median price across live reporters. Reverts when there are fewer
    ///         than `minReports` fresh reports, so a caller can never act on a
    ///         stale or thin market.
    function getPrice(address asset) external view returns (uint256) {
        uint256[] memory live = _livePrices(asset);
        require(live.length >= minReports, "TigerSwap: INSUFFICIENT_REPORTS");

        _sort(live);
        uint256 n = live.length;
        if (n % 2 == 1) {
            return live[n / 2];
        }
        return (live[n / 2 - 1] + live[n / 2]) / 2;
    }

    /// @notice Number of reports that are currently fresh for `asset`.
    function liveReportCount(address asset) external view returns (uint256) {
        return _livePrices(asset).length;
    }

    /// @notice The raw report a reporter last submitted for `asset`.
    function getReport(address asset, address reporter) external view returns (uint256, uint256, bool) {
        Report storage r = _reports[asset][reporter];
        return (r.price, r.timestamp, r.active);
    }

    function _livePrices(address asset) internal view returns (uint256[] memory) {
        uint256 count = 0;
        uint256 len = reporters.length;
        uint256[] memory buffer = new uint256[](len);

        for (uint256 i = 0; i < len; i++) {
            address reporter = reporters[i];
            if (!isReporter[reporter]) continue;
            Report storage r = _reports[asset][reporter];
            if (!r.active || r.price == 0) continue;
            if (block.timestamp - r.timestamp > maxStaleness) continue;
            buffer[count] = r.price;
            count++;
        }

        uint256[] memory live = new uint256[](count);
        for (uint256 i = 0; i < count; i++) {
            live[i] = buffer[i];
        }
        return live;
    }

    /// @dev Insertion sort: reporter counts are small (single digits), so this is
    ///      cheaper than a general sort and avoids extra allocation.
    function _sort(uint256[] memory values) internal pure {
        uint256 n = values.length;
        for (uint256 i = 1; i < n; i++) {
            uint256 key = values[i];
            uint256 j = i;
            while (j > 0 && values[j - 1] > key) {
                values[j] = values[j - 1];
                j--;
            }
            values[j] = key;
        }
    }
}
