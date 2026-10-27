// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapPriceOracle.sol";

contract TigerSwapPriceOracleTest is Test {
    TigerSwapPriceOracle public oracle;

    address public asset = address(0xA55E7);
    address public r1 = address(0x1111);
    address public r2 = address(0x2222);
    address public r3 = address(0x3333);
    address public stranger = address(0x5BAD);

    uint256 constant STALENESS = 1 hours;

    function setUp() public {
        oracle = new TigerSwapPriceOracle(STALENESS, 3);
        oracle.setReporter(r1, true);
        oracle.setReporter(r2, true);
        oracle.setReporter(r3, true);
    }

    function _report(address reporter, uint256 price) internal {
        vm.prank(reporter);
        oracle.submitPrice(asset, price);
    }

    // ---- Construction and admin -------------------------------------------

    function testConstructorRejectsBadInput() public {
        vm.expectRevert("TigerSwap: MIN_REPORTS");
        new TigerSwapPriceOracle(STALENESS, 0);

        // Zero staleness would make every report stale, so it must be rejected.
        vm.expectRevert("TigerSwap: ZERO_STALENESS");
        new TigerSwapPriceOracle(0, 1);
    }

    function testOnlyOwnerCanManageReporters() public {
        vm.prank(stranger);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        oracle.setReporter(stranger, true);

        vm.prank(stranger);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        oracle.setMaxStaleness(1);

        vm.prank(stranger);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        oracle.setMinReports(1);
    }

    function testSetReporterRejectsZeroAddress() public {
        vm.expectRevert("TigerSwap: ZERO_ADDRESS");
        oracle.setReporter(address(0), true);
    }

    function testDisabledReporterIsRemovedFromTheList() public {
        assertEq(oracle.reporters(0), r1, "first reporter");

        oracle.setReporter(r1, false);
        assertFalse(oracle.isReporter(r1), "disabled");

        // Re-enabling must not create a duplicate entry.
        oracle.setReporter(r1, true);
        assertTrue(oracle.isReporter(r1), "re-enabled");

        uint256 count = 0;
        for (uint256 i = 0; ; i++) {
            (bool ok, bytes memory data) = address(oracle).staticcall(abi.encodeWithSignature("reporters(uint256)", i));
            if (!ok) break;
            if (abi.decode(data, (address)) == r1) count++;
        }
        assertEq(count, 1, "exactly one entry for r1");
    }

    function testSetMaxStalenessRejectsZero() public {
        vm.expectRevert("TigerSwap: ZERO_STALENESS");
        oracle.setMaxStaleness(0);
    }

    function testSetMinReportsRejectsZero() public {
        vm.expectRevert("TigerSwap: MIN_REPORTS");
        oracle.setMinReports(0);
    }

    function testTransferOwnershipRejectsZero() public {
        vm.expectRevert("TigerSwap: ZERO_ADDRESS");
        oracle.transferOwnership(address(0));
    }

    // ---- Reporting ---------------------------------------------------------

    function testOnlyReporterCanSubmit() public {
        vm.prank(stranger);
        vm.expectRevert("TigerSwap: NOT_REPORTER");
        oracle.submitPrice(asset, 100e18);
    }

    function testReporterRemovedCannotSubmit() public {
        oracle.setReporter(r1, false);

        vm.prank(r1);
        vm.expectRevert("TigerSwap: NOT_REPORTER");
        oracle.submitPrice(asset, 100e18);
    }

    function testSubmitRecordsTheReport() public {
        _report(r1, 100e18);

        (uint256 price, uint256 timestamp, bool active) = oracle.getReport(asset, r1);
        assertEq(price, 100e18, "price");
        assertEq(timestamp, block.timestamp, "timestamp");
        assertTrue(active, "active");
    }

    function testSubmitRejectsZeroPrice() public {
        vm.prank(r1);
        vm.expectRevert("TigerSwap: INVALID_PRICE");
        oracle.submitPrice(asset, 0);
    }

    // ---- Median ------------------------------------------------------------

    function testMedianOfOddCount() public {
        _report(r1, 100e18);
        _report(r2, 105e18);
        _report(r3, 110e18);

        assertEq(oracle.getPrice(asset), 105e18, "middle value");
    }

    function testMedianOfEvenCount() public {
        oracle.setMinReports(2);
        _report(r1, 100e18);
        _report(r2, 110e18);

        assertEq(oracle.getPrice(asset), 105e18, "average of the two middle values");
    }

    function testSingleOutlierCannotMoveTheMedian() public {
        _report(r1, 100e18);
        _report(r2, 101e18);
        _report(r3, 1_000_000e18); // wildly wrong

        assertEq(oracle.getPrice(asset), 101e18, "outlier rejected by the median");
    }

    function testMedianIsOrderIndependent() public {
        _report(r3, 110e18);
        _report(r1, 100e18);
        _report(r2, 105e18);

        assertEq(oracle.getPrice(asset), 105e18, "same median regardless of order");
    }

    function testRequiresEnoughReports() public {
        _report(r1, 100e18);
        _report(r2, 105e18);

        // minReports is 3.
        vm.expectRevert("TigerSwap: INSUFFICIENT_REPORTS");
        oracle.getPrice(asset);
    }

    function testNoReportsReverts() public {
        vm.expectRevert("TigerSwap: INSUFFICIENT_REPORTS");
        oracle.getPrice(asset);
    }

    // ---- Staleness ---------------------------------------------------------

    function testStaleReportsAreIgnored() public {
        _report(r1, 100e18);
        _report(r2, 105e18);
        _report(r3, 110e18);

        vm.warp(block.timestamp + STALENESS + 1);
        vm.expectRevert("TigerSwap: INSUFFICIENT_REPORTS");
        oracle.getPrice(asset);
    }

    function testFreshReportsAreUsedAfterTheStaleOnesExpire() public {
        _report(r1, 100e18);
        _report(r2, 105e18);
        _report(r3, 110e18);

        vm.warp(block.timestamp + STALENESS + 1);

        // Two reporters refresh; the third is still stale.
        _report(r1, 120e18);
        _report(r2, 121e18);

        vm.expectRevert("TigerSwap: INSUFFICIENT_REPORTS");
        oracle.getPrice(asset);

        oracle.setMinReports(2);
        assertEq(oracle.getPrice(asset), 120.5e18, "median of the two fresh reports");
    }

    function testLiveReportCountTracksFreshness() public {
        _report(r1, 100e18);
        _report(r2, 105e18);
        assertEq(oracle.liveReportCount(asset), 2, "two live");

        vm.warp(block.timestamp + STALENESS + 1);
        assertEq(oracle.liveReportCount(asset), 0, "all stale");
    }

    function testDisabledReporterIsExcludedFromTheMedian() public {
        _report(r1, 100e18);
        _report(r2, 105e18);
        _report(r3, 1_000_000e18);

        // Disabling the outlier's reporter leaves only two, below minReports.
        oracle.setReporter(r3, false);
        assertEq(oracle.liveReportCount(asset), 2, "outlier excluded");

        oracle.setMinReports(2);
        assertEq(oracle.getPrice(asset), 102.5e18, "median of the remaining two");
    }

    function testReportForOneAssetDoesNotAffectAnother() public {
        address other = address(0x07E7);

        _report(r1, 100e18);
        _report(r2, 105e18);
        _report(r3, 110e18);

        vm.prank(r1);
        oracle.submitPrice(other, 7e18);

        assertEq(oracle.getPrice(asset), 105e18, "asset unaffected");
        assertEq(oracle.liveReportCount(other), 1, "other asset has one report");
    }

    function testPriceUpdatesWhenReportersRefresh() public {
        _report(r1, 100e18);
        _report(r2, 105e18);
        _report(r3, 110e18);
        assertEq(oracle.getPrice(asset), 105e18, "initial median");

        _report(r1, 200e18);
        _report(r2, 205e18);
        _report(r3, 210e18);
        assertEq(oracle.getPrice(asset), 205e18, "median moved");
    }
}
