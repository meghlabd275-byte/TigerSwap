// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Script, console2} from "forge-std/Script.sol";

import {TigerSwapFactory} from "../TigerSwapFactory.sol";
import {TigerSwapRouter} from "../TigerSwapRouter.sol";
import {TigerSwapQuoter} from "../TigerSwapQuoter.sol";
import {TigerSwapPriceOracle} from "../TigerSwapPriceOracle.sol";
import {TigerSwapPerpetuals} from "../TigerSwapPerpetuals.sol";
import {TigerSwapLending} from "../TigerSwapLending.sol";
import {TigerSwapBridge} from "../TigerSwapBridge.sol";
import {TigerSwapAdvancedOrders} from "../TigerSwapAdvancedOrders.sol";
import {TigerSwapTWAMM} from "../TigerSwapTWAMM.sol";
import {TigerSwapPerpOrderBook} from "../TigerSwapPerpOrderBook.sol";

/**
 * @title Deploy
 * @notice Deploys the TigerSwap protocol for one chain and writes a per-chain
 *         address registry that the API reads at startup.
 *
 * Usage:
 *   forge script script/Deploy.s.sol --rpc-url $RPC --broadcast \
 *     --private-key $DEPLOYER_KEY
 *
 * Environment:
 *   TIGERSWAP_CHAIN_ID               chain id written into the registry (required)
 *   TIGERSWAP_WETH                   wrapped-native address for the router (required)
 *   TIGERSWAP_ORACLE_MAX_STALENESS   oracle staleness window in seconds (default 3600)
 *   TIGERSWAP_ORACLE_MIN_REPORTS     oracle quorum (default 1)
 *   TIGERSWAP_BRIDGE_CHAIN_ID        local bridge chain id (default = chain id)
 *   TIGERSWAP_PERPS_ORACLE           address allowed to push marks (default deployer)
 *
 * The registry is written to `deployments/<chainId>.json`. The Go API reads the
 * same JSON, so a deployed chain is dialled without hand-copying addresses into
 * environment variables.
 */
contract Deploy is Script {
    function run() external {
        uint256 chainId = vm.envUint("TIGERSWAP_CHAIN_ID");
        address weth = vm.envAddress("TIGERSWAP_WETH");
        require(weth != address(0), "Deploy: TIGERSWAP_WETH is required");

        uint256 maxStaleness = vm.envOr("TIGERSWAP_ORACLE_MAX_STALENESS", uint256(3600));
        uint256 minReports = vm.envOr("TIGERSWAP_ORACLE_MIN_REPORTS", uint256(1));
        uint256 bridgeChainId = vm.envOr("TIGERSWAP_BRIDGE_CHAIN_ID", chainId);
        address perpsOracle = vm.envOr("TIGERSWAP_PERPS_ORACLE", msg.sender);

        vm.startBroadcast();

        TigerSwapFactory factory = new TigerSwapFactory();
        TigerSwapRouter router = new TigerSwapRouter(address(factory), weth);
        TigerSwapQuoter quoter = new TigerSwapQuoter(address(factory));
        TigerSwapPriceOracle oracle = new TigerSwapPriceOracle(maxStaleness, minReports);
        TigerSwapPerpetuals perps = new TigerSwapPerpetuals();
        TigerSwapLending lending = new TigerSwapLending(address(oracle));
        TigerSwapBridge bridge = new TigerSwapBridge(bridgeChainId);

        // On-chain order execution. TigerSwapAdvancedOrders escrows the input of a
        // limit/stop/TWAP order and settles fills through the router, so a keeper
        // can only fill at the pool's real price. TigerSwapTWAMM executes a
        // long-term order in virtual slices against its own pool. Both read their
        // price from the same oracle the rest of the protocol uses.
        TigerSwapAdvancedOrders orders = new TigerSwapAdvancedOrders(address(oracle), address(router));

        // The on-chain perpetual order book: the derivatives settlement layer.
        // Traders margin a settlement token, rest price-time-priority limit
        // orders, and a taker crosses them in one transaction. Its oracle is the
        // keeper address, the same one that pushes perps prices.
        TigerSwapPerpOrderBook perpOrderBook = new TigerSwapPerpOrderBook(perpsOracle);

        // TigerSwapTWAMM is a per-pair pool, so it is deployed only when the pair
        // is configured. Without a pair there is nothing for a long-term order to
        // trade against, and the registry records the zero address.
        address twamm = address(0);
        address twammToken0 = vm.envOr("TIGERSWAP_TWAMM_TOKEN0", address(0));
        address twammToken1 = vm.envOr("TIGERSWAP_TWAMM_TOKEN1", address(0));
        if (twammToken0 != address(0) && twammToken1 != address(0)) {
            address twammFeeRecipient = vm.envOr("TIGERSWAP_TWAMM_FEE_RECIPIENT", msg.sender);
            uint256 twammFee = vm.envOr("TIGERSWAP_TWAMM_FEE", uint256(0));
            twamm = address(new TigerSwapTWAMM(twammToken0, twammToken1, twammFeeRecipient, twammFee));
        }

        // The perps oracle must be the address the API's keeper signs with, so it
        // is set from the environment rather than defaulting to the deployer.
        perps.setOracle(perpsOracle);

        vm.stopBroadcast();

        _seedMarkets(perps);
        _seedOrderBookMarkets(perpOrderBook);

        _write(chainId, address(factory), address(router), address(quoter), address(oracle), address(perps), address(lending), address(bridge), weth, address(orders), twamm, address(perpOrderBook));

        console2.log("TigerSwap deployed on chain", chainId);
        console2.log("  factory", address(factory));
        console2.log("  router ", address(router));
        console2.log("  quoter ", address(quoter));
        console2.log("  oracle ", address(oracle));
        console2.log("  perps  ", address(perps));
        console2.log("  orders ", address(orders));
        console2.log("  twamm  ", twamm);
        console2.log("  lending", address(lending));
        console2.log("  bridge ", address(bridge));
    }

    /// @notice Optionally create perpetual markets on the freshly deployed
    ///         contract. Each entry is `SYMBOL:SETTLEMENT_TOKEN:MAX_LEVERAGE`,
    ///         comma-separated, e.g. `BTC/RUSD:0xToken:20,ETH/RUSD:0xToken:20`.
    ///         Risk parameters use protocol defaults (maintenance 5%, liquidation
    ///         fee 1%, initial margin = 1/leverage), so a seeded market is
    ///         immediately tradable and can be tuned later by updateMarket.
    ///         Idempotent: a market that already exists is skipped, because
    ///         createMarket reverts on a duplicate.
    function _seedMarkets(TigerSwapPerpetuals perps) internal {
        string memory raw = vm.envOr("TIGERSWAP_PERPS_MARKETS", string(""));
        if (bytes(raw).length == 0) {
            return;
        }

        vm.startBroadcast();
        string[] memory entries = vm.split(raw, ",");
        for (uint256 i = 0; i < entries.length; i++) {
            string[] memory parts = vm.split(entries[i], ":");
            require(parts.length == 3, "Deploy: market entry must be SYMBOL:TOKEN:LEVERAGE");
            bytes32 marketId = keccak256(bytes(parts[0]));
            if (perps.getMarket(marketId).token != address(0)) {
                continue;
            }
            address token = vm.parseAddress(parts[1]);
            uint256 leverage = vm.parseUint(parts[2]);
            require(token != address(0), "Deploy: market token is required");
            require(leverage >= 1 && leverage <= 1001, "Deploy: leverage out of range");

            uint256 initialMargin = perps.RATIO_PRECISION() / leverage;
            perps.createMarket(
                marketId,
                token,
                leverage * perps.RATIO_PRECISION(),
                perps.RATIO_PRECISION() / 100, // 1% liquidation fee
                perps.RATIO_PRECISION() / 20, // 5% maintenance margin
                initialMargin
            );
            console2.log("seeded perps market", parts[0]);
        }
        vm.stopBroadcast();
    }

    /// @notice Optionally create perpetual order book markets. Each entry is
    ///         `SYMBOL:SETTLEMENT_TOKEN:ORACLE_ASSET:MAX_LEVERAGE`, comma-separated.
    ///         Risk parameters use protocol defaults (maintenance 5%, initial
    ///         margin = 1/leverage, taker 0.05%, maker 0.02%, lot 0.001 base
    ///         units). Idempotent: an existing market is skipped.
    function _seedOrderBookMarkets(TigerSwapPerpOrderBook book) internal {
        string memory raw = vm.envOr("TIGERSWAP_ORDERBOOK_MARKETS", string(""));
        if (bytes(raw).length == 0) {
            return;
        }

        vm.startBroadcast();
        string[] memory entries = vm.split(raw, ",");
        for (uint256 i = 0; i < entries.length; i++) {
            string[] memory parts = vm.split(entries[i], ":");
            require(parts.length == 4, "Deploy: order book entry must be SYMBOL:TOKEN:ORACLE_ASSET:LEVERAGE");
            bytes32 marketId = keccak256(bytes(parts[0]));
            if (book.getMarket(marketId).token != address(0)) {
                continue;
            }
            address token = vm.parseAddress(parts[1]);
            address oracleAsset = vm.parseAddress(parts[2]);
            uint256 leverage = vm.parseUint(parts[3]);
            require(token != address(0) && oracleAsset != address(0), "Deploy: token and oracle asset are required");
            require(leverage >= 1 && leverage <= 101, "Deploy: leverage out of range");

            uint256 wad = book.WAD();
            book.createMarket(
                marketId,
                token,
                oracleAsset,
                wad / 2000, // 0.05% taker fee
                wad / 5000, // 0.02% maker fee
                wad / leverage, // initial margin
                wad / 20, // 5% maintenance margin
                wad / 20, // 5% liquidation fee
                leverage * wad, // max leverage
                0, // uncapped open interest
                1e15, // min order size: 0.001 base units
                1e15 // lot size
            );
            book.setMarketActive(marketId, true);
            console2.log("seeded order book market", parts[0]);
        }
        vm.stopBroadcast();
    }

    function _write(
        uint256 chainId,
        address factory,
        address router,
        address quoter,
        address oracle,
        address perps,
        address lending,
        address bridge,
        address weth,
        address orders,
        address twamm,
        address perpOrderBook
    ) internal {
        string memory json = string.concat(
            "{\n",
            '  "chain_id": ', vm.toString(chainId), ",\n",
            '  "factory": "', vm.toString(factory), '",\n',
            '  "router": "', vm.toString(router), '",\n',
            '  "quoter": "', vm.toString(quoter), '",\n',
            '  "oracle": "', vm.toString(oracle), '",\n',
            '  "perps": "', vm.toString(perps), '",\n',
            '  "orders": "', vm.toString(orders), '",\n',
            '  "twamm": "', vm.toString(twamm), '",\n',
            '  "lending": "', vm.toString(lending), '",\n',
            '  "bridge": "', vm.toString(bridge), '",\n',
            '  "weth": "', vm.toString(weth), '",\n',
            '  "perp_order_book": "', vm.toString(perpOrderBook), '"\n',
            "}\n"
        );
        string memory path = string.concat("deployments/", vm.toString(chainId), ".json");
        vm.writeFile(path, json);
    }
}
