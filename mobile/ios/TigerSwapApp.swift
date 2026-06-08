//
//  TigerSwapApp.swift
//  TigerSwap iOS Native App
//
//  A native iOS implementation of TigerSwap DEX
//

import SwiftUI

@main
struct TigerSwapApp: App {
    @StateObject private var appState = AppState()
    
    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(appState)
        }
    }
}

// MARK: - App State
@MainActor
class AppState: ObservableObject {
    @Published var isConnected: Bool = false
    @Published var currentWallet: Wallet?
    @Published var selectedChain: Chain = .ethereum
    @Published var portfolio: Portfolio = Portfolio()
    
    let walletManager = WalletManager()
    let swapEngine = SwapEngine()
    let portfolioTracker = PortfolioTracker()
    
    init() {
        // Initialize wallet
        walletManager.delegate = self
    }
}

// MARK: - Wallet Manager Delegate
extension AppState: WalletManagerDelegate {
    func walletDidConnect(_ wallet: Wallet) {
        isConnected = true
        currentWallet = wallet
        Task {
            await loadPortfolio()
        }
    }
    
    func walletDidDisconnect() {
        isConnected = false
        currentWallet = nil
    }
    
    private func loadPortfolio() async {
        guard let wallet = currentWallet else { return }
        portfolio = await portfolioTracker.fetchPortfolio(wallet: wallet)
    }
}

// MARK: - Content View
struct ContentView: View {
    @EnvironmentObject var appState: AppState
    
    var body: some View {
        TabView {
            // Swap Tab
            SwapView()
                .tabItem {
                    Label("Swap", systemImage: "arrow.triangle.2.circlepath")
                }
            
            // Pool Tab
            PoolView()
                .tabItem {
                    Label("Pool", systemImage: "drop.fill")
                }
            
            // Portfolio Tab
            PortfolioView()
                .tabItem {
                    Label("Portfolio", systemImage: "chart.pie.fill")
                }
            
            // Wallet Tab
            WalletView()
                .tabItem {
                    Label("Wallet", systemImage: "wallet.pass.fill")
                }
            
            // Settings Tab
            SettingsView()
                .tabItem {
                    Label("Settings", systemImage: "gearshape.fill")
                }
        }
        .tint(.orange)
    }
}

// MARK: - Swap View
struct SwapView: View {
    @EnvironmentObject var appState: AppState
    @State private var fromToken: Token = Token.usdc
    @State private var toToken: Token = Token.weth
    @State private var fromAmount: String = ""
    @State private var toAmount: String = ""
    @State private var isLoading: Bool = false
    @State private var showSettings: Bool = false
    
    var body: some View {
        NavigationStack {
            VStack(spacing: 20) {
                // From Token Card
                TokenCard(
                    title: "From",
                    token: $fromToken,
                    amount: $fromAmount,
                    isSelectable: true
                )
                
                // Swap Button
                Button(action: swapTokens) {
                    Image(systemName: "arrow.up.arrow.down.circle.fill")
                        .font(.system(size: 32))
                        .foregroundColor(.orange)
                }
                
                // To Token Card
                TokenCard(
                    title: "To",
                    token: $toToken,
                    amount: $toAmount,
                    isSelectable: true
                )
                
                // Exchange Rate
                if !toAmount.isEmpty && !fromAmount.isEmpty {
                    ExchangeRateView(from: fromToken, to: toToken, fromAmount: fromAmount, toAmount: toAmount)
                }
                
                Spacer()
                
                // Swap Button
                Button(action: executeSwap) {
                    Text("Swap")
                        .font(.headline)
                        .foregroundColor(.white)
                        .frame(maxWidth: .infinity)
                        .padding()
                        .background(fromAmount.isEmpty ? Color.gray : Color.orange)
                        .cornerRadius(12)
                }
                .disabled(fromAmount.isEmpty || isLoading)
                
                if isLoading {
                    ProgressView()
                }
            }
            .padding()
            .navigationTitle("TigerSwap")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .navigationBarTrailing) {
                    Button(action: { showSettings = true }) {
                        Image(systemName: "gearshape")
                    }
                }
            }
            .sheet(isPresented: $showSettings) {
                SettingsView()
            }
            .onChange(of: fromAmount) { _ in
                Task { await calculateOutput() }
            }
        }
    }
    
    private func swapTokens() {
        let temp = fromToken
        fromToken = toToken
        toToken = temp
        fromAmount = ""
        toAmount = ""
    }
    
    private func calculateOutput() async {
        guard !fromAmount.isEmpty,
              let amount = Double(fromAmount) else { return }
        
        isLoading = true
        let quote = await appState.swapEngine.getQuote(from: fromToken, to: toToken, amount: amount)
        toAmount = String(format: "%.6f", quote.outputAmount)
        isLoading = false
    }
    
    private func executeSwap() async {
        guard let amount = Double(fromAmount) else { return }
        
        isLoading = true
        let result = await appState.swapEngine.executeSwap(
            from: fromToken,
            to: toToken,
            amount: amount,
            slippage: 50
        )
        
        if result.success {
            toAmount = ""
            fromAmount = ""
            await appState.loadPortfolio()
        }
        isLoading = false
    }
}

// MARK: - Token Card
struct TokenCard: View {
    let title: String
    @Binding var token: Token
    @Binding var amount: String
    let isSelectable: Bool
    
    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title)
                .font(.caption)
                .foregroundColor(.gray)
            
            HStack {
                Button(action: {}) {
                    HStack {
                        AsyncImage(url: URL(string: token.logoURL)) { image in
                            image.resizable().frame(width: 32, height: 32)
                        } placeholder: {
                            Circle().fill(Color.gray.opacity(0.3)).frame(width: 32, height: 32)
                        }
                        Text(token.symbol)
                            .font(.headline)
                        Image(systemName: "chevron.down")
                            .font(.caption)
                    }
                }
                
                Spacer()
                
                TextField("0.0", text: $amount)
                    .font(.title2)
                    .multilineTextAlignment(.trailing)
                    .keyboardType(.decimalPad)
            }
        }
        .padding()
        .background(Color(.systemGray6))
        .cornerRadius(12)
    }
}

// MARK: - Exchange Rate View
struct ExchangeRateView: View {
    let from: Token
    let to: Token
    let fromAmount: String
    let toAmount: String
    
    var rate: Double {
        guard let f = Double(fromAmount), let t = Double(toAmount), f > 0 else { return 0 }
        return t / f
    }
    
    var body: some View {
        HStack {
            Text("1 \(from.symbol) = \(String(format: "%.4f", rate)) \(to.symbol)")
                .font(.caption)
                .foregroundColor(.gray)
            Spacer()
        }
    }
}

// MARK: - Pool View
struct PoolView: View {
    @EnvironmentObject var appState: AppState
    @State private var selectedTab: PoolTab = .all
    
    enum PoolTab: String, CaseIterable {
        case all = "All"
        case myPools = "My Pools"
        case explore = "Explore"
    }
    
    var body: some View {
        NavigationStack {
            VStack {
                Picker("Tab", selection: $selectedTab) {
                    ForEach(PoolTab.allCases, id: \.self) { tab in
                        Text(tab.rawValue).tag(tab)
                    }
                }
                .pickerStyle(.segmented)
                .padding()
                
                ScrollView {
                    LazyVStack(spacing: 12) {
                        ForEach(mockPools) { pool in
                            PoolRow(pool: pool)
                        }
                    }
                    .padding()
                }
            }
            .navigationTitle("Pool")
            .toolbar {
                ToolbarItem(placement: .navigationBarTrailing) {
                    Button(action: {}) {
                        Image(systemName: "plus.circle")
                    }
                }
            }
        }
    }
    
    var mockPools: [PoolInfo] {
        [
            PoolInfo(token0: .eth, token1: .usdc, tvl: "$1.2M", volume: "$450K", apr: "24.5%"),
            PoolInfo(token0: .btc, token1: .eth, tvl: "$890K", volume: "$320K", apr: "18.2%"),
            PoolInfo(token0: .eth, token1: .usdt, tvl: "$650K", volume: "$210K", apr: "15.8%")
        ]
    }
}

struct PoolRow: View {
    let pool: PoolInfo
    
    var body: some View {
        HStack {
            // Token pair icons
            ZStack {
                Image(systemName: "circle.fill")
                    .foregroundColor(.orange)
                    .frame(width: 40, height: 40)
                Image(systemName: "circle.fill")
                    .foregroundColor(.blue)
                    .frame(width: 40, height: 40)
                    .offset(x: 10)
            }
            .frame(width: 60)
            
            VStack(alignment: .leading) {
                Text("\(pool.token0.symbol)/\(pool.token1.symbol)")
                    .font(.headline)
                Text("TVL: \(pool.tvl)")
                    .font(.caption)
                    .foregroundColor(.gray)
            }
            
            Spacer()
            
            VStack(alignment: .trailing) {
                Text(pool.apr)
                    .font(.headline)
                    .foregroundColor(.green)
                Text("Volume: \(pool.volume)")
                    .font(.caption)
                    .foregroundColor(.gray)
            }
        }
        .padding()
        .background(Color(.systemGray6))
        .cornerRadius(12)
    }
}

struct PoolInfo: Identifiable {
    let id = UUID()
    let token0: Token
    let token1: Token
    let tvl: String
    let volume: String
    let apr: String
}

// MARK: - Portfolio View
struct PortfolioView: View {
    @EnvironmentObject var appState: AppState
    
    var body: some View {
        NavigationStack {
            VStack(spacing: 20) {
                // Total Value Card
                VStack {
                    Text("Total Value")
                        .font(.caption)
                        .foregroundColor(.gray)
                    Text("$\(String(format: "%.2f", appState.portfolio.totalValue))")
                        .font(.system(size: 36, weight: .bold))
                }
                .padding()
                
                // Asset List
                ScrollView {
                    LazyVStack(spacing: 12) {
                        ForEach(appState.portfolio.balances) { balance in
                            AssetRow(balance: balance)
                        }
                    }
                }
            }
            .padding()
            .navigationTitle("Portfolio")
        }
    }
}

struct AssetRow: View {
    let balance: TokenBalance
    
    var body: some View {
        HStack {
            Image(systemName: "circle.fill")
                .foregroundColor(.orange)
                .frame(width: 40, height: 40)
            
            VStack(alignment: .leading) {
                Text(balance.token.symbol)
                    .font(.headline)
                Text(balance.token.name)
                    .font(.caption)
                    .foregroundColor(.gray)
            }
            
            Spacer()
            
            VStack(alignment: .trailing) {
                Text(String(format: "%.4f", balance.balance))
                    .font(.headline)
                Text("$\(String(format: "%.2f", balance.valueUSD))")
                    .font(.caption)
                    .foregroundColor(.gray)
            }
        }
        .padding()
        .background(Color(.systemGray6))
        .cornerRadius(12)
    }
}

// MARK: - Wallet View
struct WalletView: View {
    @EnvironmentObject var appState: AppState
    
    var body: some View {
        NavigationStack {
            VStack(spacing: 20) {
                if appState.isConnected, let wallet = appState.currentWallet {
                    // Connected Wallet
                    VStack {
                        Image(systemName: "wallet.pass.fill")
                            .font(.system(size: 60))
                            .foregroundColor(.orange)
                        
                        Text(wallet.address)
                            .font(.caption)
                            .foregroundColor(.gray)
                        
                        Button("Disconnect") {
                            appState.walletManager.disconnect()
                        }
                        .foregroundColor(.red)
                    }
                    .padding()
                } else {
                    // Connect Wallet
                    VStack(spacing: 16) {
                        Image(systemName: "wallet.pass")
                            .font(.system(size: 60))
                            .foregroundColor(.gray)
                        
                        Text("Connect your wallet")
                            .font(.title2)
                        
                        Button(action: connectWallet) {
                            Text("Connect Wallet")
                                .font(.headline)
                                .foregroundColor(.white)
                                .frame(maxWidth: .infinity)
                                .padding()
                                .background(Color.orange)
                                .cornerRadius(12)
                        }
                    }
                    .padding()
                }
            }
            .navigationTitle("Wallet")
        }
    }
    
    private func connectWallet() {
        appState.walletManager.connect()
    }
}

// MARK: - Settings View
struct SettingsView: View {
    @EnvironmentObject var appState: AppState
    @State private var slippageTolerance: Double = 0.5
    @State private var selectedChain: Chain = .ethereum
    @State private var showNetworks: Bool = false
    
    var body: some View {
        List {
            Section("Network") {
                Button(action: { showNetworks = true }) {
                    HStack {
                        Text("Network")
                        Spacer()
                        Text(selectedChain.name)
                            .foregroundColor(.gray)
                    }
                }
            }
            
            Section("Trading") {
                HStack {
                    Text("Slippage Tolerance")
                    Spacer()
                    Text("\(Int(slippageTolerance * 100))%")
                        .foregroundColor(.gray)
                }
                Slider(value: $slippageTolerance, in: 0.01...0.1, step: 0.01)
                
                Toggle("Expert Mode", isOn: .constant(false))
            }
            
            Section("Security") {
                Toggle("Biometric Auth", isOn: .constant(true))
                Toggle("Push Notifications", isOn: .constant(true))
            }
            
            Section("About") {
                HStack {
                    Text("Version")
                    Spacer()
                    Text("1.0.0")
                        .foregroundColor(.gray)
                }
                
                Link("Documentation", destination: URL(string: "https://docs.tigerswap.com")!)
                
                Link("Support", destination: URL(string: "https://support.tigerswap.com")!)
            }
        }
        .navigationTitle("Settings")
        .sheet(isPresented: $showNetworks) {
            NetworkSelectionView(selectedChain: $selectedChain)
        }
    }
}

struct NetworkSelectionView: View {
    @Binding var selectedChain: Chain
    
    var body: some View {
        NavigationStack {
            List(Chain.allCases) { chain in
                Button(action: { selectedChain = chain }) {
                    HStack {
                        Text(chain.name)
                        Spacer()
                        if chain == selectedChain {
                            Image(systemName: "checkmark")
                                .foregroundColor(.orange)
                        }
                    }
                }
            }
            .navigationTitle("Select Network")
            .navigationBarItems(trailing: Button("Done") {})
        }
    }
}

// MARK: - Models
struct Wallet {
    let address: String
    let chain: Chain
    let type: WalletType
    
    enum WalletType {
        case metamask
        case walletconnect
        case hardware
        case privateKey
    }
}

struct Token: Identifiable, Hashable {
    let id: String
    let symbol: String
    let name: String
    let decimals: Int
    let logoURL: String
    let chain: Chain
    
    static let eth = Token(id: "0xeth", symbol: "ETH", name: "Ethereum", decimals: 18, logoURL: "", chain: .ethereum)
    static let usdc = Token(id: "0xusdc", symbol: "USDC", name: "USD Coin", decimals: 6, logoURL: "", chain: .ethereum)
    static let usdt = Token(id: "0xusdt", symbol: "USDT", name: "Tether", decimals: 6, logoURL: "", chain: .ethereum)
    static let weth = Token(id: "0xweth", symbol: "WETH", name: "Wrapped Ether", decimals: 18, logoURL: "", chain: .ethereum)
    static let btc = Token(id: "0xbtc", symbol: "WBTC", name: "Wrapped Bitcoin", decimals: 8, logoURL: "", chain: .ethereum)
}

struct TokenBalance: Identifiable {
    let id = UUID()
    let token: Token
    let balance: Double
    let valueUSD: Double
}

struct Portfolio {
    var totalValue: Double = 0
    var balances: [TokenBalance] = []
}

enum Chain: String, CaseIterable, Identifiable {
    case ethereum = "Ethereum"
    case polygon = "Polygon"
    case arbitrum = "Arbitrum"
    case optimism = "Optimism"
    case base = "Base"
    case bsc = "BNB Chain"
    case avalanche = "Avalanche"
    case solana = "Solana"
    
    var id: String { rawValue }
    var name: String { rawValue }
}

// MARK: - Engine Protocols
protocol WalletManagerDelegate: AnyObject {
    func walletDidConnect(_ wallet: Wallet)
    func walletDidDisconnect()
}

class WalletManager {
    weak var delegate: WalletManagerDelegate?
    
    func connect() {
        // Simulate wallet connection
        let wallet = Wallet(address: "0x1234...5678", chain: .ethereum, type: .metamask)
        delegate?.walletDidConnect(wallet)
    }
    
    func disconnect() {
        delegate?.walletDidDisconnect()
    }
}

class SwapEngine {
    func getQuote(from: Token, to: Token, amount: Double) async -> QuoteResult {
        // Simulate quote calculation
        let outputAmount = amount * 2500 // Mock rate
        return QuoteResult(outputAmount: outputAmount, priceImpact: 0.1)
    }
    
    func executeSwap(from: Token, to: Token, amount: Double, slippage: Double) async -> SwapResult {
        return SwapResult(success: true, txHash: "0xabcd...1234")
    }
}

class PortfolioTracker {
    func fetchPortfolio(wallet: Wallet) async -> Portfolio {
        return Portfolio(
            totalValue: 12500.50,
            balances: [
                TokenBalance(token: .eth, balance: 2.5, valueUSD: 6250),
                TokenBalance(token: .usdc, balance: 5000, valueUSD: 5000),
                TokenBalance(token: .weth, balance: 0.5, valueUSD: 1250.50)
            ]
        )
    }
}

struct QuoteResult {
    let outputAmount: Double
    let priceImpact: Double
}

struct SwapResult {
    let success: Bool
    let txHash: String
}

#Preview {
    ContentView()
        .environmentObject(AppState())
}
