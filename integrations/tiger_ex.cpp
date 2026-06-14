// TigerEX Trading Engine - Main Entry Point
// Ultra-Low Latency C++ Implementation

#include "tiger_ex_trading_engine.hpp"
#include <cassert>

using namespace TigerEX;

int main() {
    std::cout << "=== TigerEX C++ Trading Engine ===" << std::endl;
    
    // Create and initialize engine
    TradingEngine engine;
    engine.initialize();
    
    // Get statistics
    std::cout << "\nPlatform Statistics:" << std::endl;
    auto stats = engine.get_stats();
    std::cout << "  EVM Chains: " << stats["totalEvmChains"] << std::endl;
    std::cout << "  Non-EVM Chains: " << stats["totalNonEvmChains"] << std::endl;
    std::cout << "  Tokens: " << stats["totalTokens"] << std::endl;
    std::cout << "  DEX Pools: " << stats["totalPools"] << std::endl;
    std::cout << "  Farms: " << stats["totalFarms"] << std::endl;
    std::cout << "  Bridges: " << stats["totalBridges"] << std::endl;
    std::cout << "  Initialized: " << stats["initialized"] << std::endl;
    
    // Test swap calculation
    std::cout << "\nSwap Test (100 USDT -> TGR):" << std::endl;
    auto swap_result = engine.calculate_swap("USDT", "TGR", 100000000); // 100 USDT
    if (swap_result) {
        std::cout << "  Input: " << swap_result->amount_in << " USDT" << std::endl;
        std::cout << "  Output: " << swap_result->amount_out << " TGR" << std::endl;
        std::cout << "  Fee: " << swap_result->fee << " TGR" << std::endl;
        std::cout << "  Path: ";
        for (size_t i = 0; i < swap_result->path.size(); i++) {
            std::cout << swap_result->path[i];
            if (i < swap_result->path.size() - 1) std::cout << " -> ";
        }
        std::cout << std::endl;
    }
    
    // Test bridge calculation
    std::cout << "\nBridge Test (ETH -> BSC):" << std::endl;
    auto bridge_result = engine.calculate_bridge("ethereum", "bsc", 1000000000000000000); // 1 ETH
    if (bridge_result) {
        std::cout << "  Sent: " << bridge_result->amount_sent << " ETH" << std::endl;
        std::cout << "  Received: " << bridge_result->amount_received << " ETH" << std::endl;
        std::cout << "  Fee: " << bridge_result->fee << " ETH" << std::endl;
        std::cout << "  Time: " << bridge_result->estimated_time_ms << "ms" << std::endl;
    }
    
    // Test chain search
    std::cout << "\nChain Search (Polygon):" << std::endl;
    auto chains = engine.search_chains("Polygon");
    std::cout << "  Found: " << chains.size() << " chains" << std::endl;
    for (const auto& chain : chains) {
        std::cout << "    - " << chain.name << " (" << chain.symbol << ") - Chain ID: " << chain.chain_id << std::endl;
    }
    
    // Test fee collection
    std::cout << "\nFee Collection Test:" << std::endl;
    engine.collect_fee(1000000, "swap");
    engine.collect_fee(500000, "bridge");
    engine.collect_fee(300000, "exchange");
    engine.collect_fee(100000, "wallet");
    std::cout << "  Total Fees: " << engine.get_total_fees() << std::endl;
    
    // Test dynamic chain addition
    std::cout << "\nDynamic Chain Addition:" << std::endl;
    ChainConfig new_chain = {
        "new_chain",
        "New Blockchain",
        "NEW",
        ChainCategory::EVM,
        ChainStatus::Active,
        99999,
        {"https://rpc.newchain.com"},
        {"https://scan.newchain.com"},
        {"New Coin", "NEW", 18},
        2.0,
        true
    };
    if (engine.add_evm_chain(new_chain)) {
        std::cout << "  Added: " << new_chain.name << std::endl;
    }
    
    // Test dynamic pool creation
    std::cout << "\nDynamic Pool Creation:" << std::endl;
    if (engine.create_pool("NEW", "USDT", 30)) {
        std::cout << "  Created pool: NEW-USDT" << std::endl;
    }
    
    // Test dynamic farm creation
    std::cout << "\nDynamic Farm Creation:" << std::endl;
    if (engine.create_farm("NEW-USDT", "TGR", 30)) {
        std::cout << "  Created farm: NEW-USDT (30% APY)" << std::endl;
    }
    
    // Test dynamic bridge addition
    std::cout << "\nDynamic Bridge Addition:" << std::endl;
    if (engine.add_bridge("new_chain", "ethereum", "NEW", 10, 600000)) {
        std::cout << "  Created bridge: new_chain -> ethereum" << std::endl;
    }
    
    // Final stats
    std::cout << "\nFinal Platform Statistics:" << std::endl;
    stats = engine.get_stats();
    std::cout << "  EVM Chains: " << stats["totalEvmChains"] << std::endl;
    std::cout << "  DEX Pools: " << stats["totalPools"] << std::endl;
    std::cout << "  Farms: " << stats["totalFarms"] << std::endl;
    std::cout << "  Bridges: " << stats["totalBridges"] << std::endl;
    
    std::cout << "\n=== Test Complete ===" << std::endl;
    
    return 0;
}