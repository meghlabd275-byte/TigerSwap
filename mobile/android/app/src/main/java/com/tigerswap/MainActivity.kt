package com.tigerswap

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.*
import androidx.compose.material.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.vectorResource
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.tigerswap.ui.theme.TigerSwapTheme

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            TigerSwapTheme {
                TigerSwapApp()
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun TigerSwapApp() {
    val navController = rememberNavController()
    val items = listOf(
        BottomNavItem("Swap", "swap"),
        BottomNavItem("Pool", "pool"),
        BottomNavItem("Portfolio", "portfolio"),
        BottomNavItem("Wallet", "wallet"),
        BottomNavItem("Settings", "settings")
    )

    Scaffold(
        bottomBar = {
            NavigationBar {
                val navBackStackEntry by navController.currentBackStackEntryAsState()
                val currentDestination = navBackStackEntry?.destination

                items.forEach { item ->
                    NavigationBarItem(
                        icon = { Icon(getIconFor(item.route), contentDescription = item.route) },
                        label = { Text(item.route) },
                        selected = currentDestination?.hierarchy?.any { it.route == item.route } == true,
                        onClick = {
                            navController.navigate(item.route) {
                                popUpTo(navController.graph.findStartDestination().id) {
                                    saveState = true
                                }
                                launchSingleTop = true
                                restoreState = true
                            }
                        }
                    )
                }
            }
        }
    ) { innerPadding ->
        NavHost(
            navController,
            startDestination = "swap",
            modifier = Modifier.padding(innerPadding)
        ) {
            composable("swap") { SwapScreen() }
            composable("pool") { PoolScreen() }
            composable("portfolio") { PortfolioScreen() }
            composable("wallet") { WalletScreen() }
            composable("settings") { SettingsScreen() }
        }
    }
}

data class BottomNavItem(val label: String, val route: String)

fun getIconFor(route: String): androidx.compose.ui.graphics.vector.ImageVector {
    return when (route) {
        "swap" -> androidx.compose.material.icons.Icons.Default.SwapHoriz
        "pool" -> androidx.compose.material.icons.Icons.Default.Pool
        "portfolio" -> androidx.compose.material.icons.Icons.Default.PieChart
        "wallet" -> androidx.compose.material.icons.Icons.Default.AccountBalanceWallet
        "settings" -> androidx.compose.material.icons.Icons.Default.Settings
        else -> androidx.compose.material.icons.Icons.Default.Home
    }
}

// ==================== SWAP SCREEN ====================
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SwapScreen() {
    var fromToken by remember { mutableStateOf("ETH") }
    var toToken by remember { mutableStateOf("USDC") }
    var fromAmount by remember { mutableStateOf("") }
    var toAmount by remember { mutableStateOf("") }
    var isLoading by remember { mutableStateOf(false) }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp)
    ) {
        // From Token Card
        Card(
            modifier = Modifier.fillMaxWidth()
        ) {
            Column(modifier = Modifier.padding(16.dp)) {
                Text("From", style = MaterialTheme.typography.labelMedium)
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween
                ) {
                    Text(fromToken, style = MaterialTheme.typography.titleLarge)
                    OutlinedTextField(
                        value = fromAmount,
                        onValueChange = { fromAmount = it },
                        modifier = Modifier.weight(1f),
                        placeholder = { Text("0.0") },
                        keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = androidx.compose.foundation.text.KeyboardType.Decimal)
                    )
                }
            }
        }

        // Swap Button
        IconButton(
            onClick = {
                val temp = fromToken
                fromToken = toToken
                toToken = temp
                fromAmount = ""
                toAmount = ""
            },
            modifier = Modifier.align(androidx.compose.ui.Alignment.CenterHorizontally)
        ) {
            Icon(
                imageVector = androidx.compose.material.icons.Icons.Default.SwapVert,
                contentDescription = "Swap"
            )
        }

        // To Token Card
        Card(
            modifier = Modifier.fillMaxWidth()
        ) {
            Column(modifier = Modifier.padding(16.dp)) {
                Text("To", style = MaterialTheme.typography.labelMedium)
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween
                ) {
                    Text(toToken, style = MaterialTheme.typography.titleLarge)
                    OutlinedTextField(
                        value = toAmount,
                        onValueChange = { toAmount = it },
                        modifier = Modifier.weight(1f),
                        placeholder = { Text("0.0") },
                        readOnly = true
                    )
                }
            }
        }

        Spacer(modifier = Modifier.height(24.dp))

        // Exchange Rate
        if (fromAmount.isNotEmpty() && toAmount.isNotEmpty()) {
            val from = fromAmount.toDoubleOrNull() ?: 0.0
            val to = toAmount.toDoubleOrNull() ?: 0.0
            val rate = if (from > 0) to / from else 0.0
            Text(
                "1 $fromToken = ${String.format("%.4f", rate)} $toToken",
                style = MaterialTheme.typography.bodySmall
            )
        }

        Spacer(modifier = Modifier.weight(1f))

        // Swap Button
        Button(
            onClick = { /* Execute swap */ },
            modifier = Modifier
                .fillMaxWidth()
                .height(56.dp),
            enabled = fromAmount.isNotEmpty() && !isLoading
        ) {
            if (isLoading) {
                CircularProgressIndicator(
                    modifier = Modifier.size(24.dp),
                    color = MaterialTheme.colorScheme.onPrimary
                )
            } else {
                Text("Swap", style = MaterialTheme.typography.titleMedium)
            }
        }
    }
}

// ==================== POOL SCREEN ====================
@Composable
fun PoolScreen() {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp)
    ) {
        // Search Bar
        OutlinedTextField(
            value = "",
            onValueChange = {},
            modifier = Modifier.fillMaxWidth(),
            placeholder = { Text("Search pools") },
            leadingIcon = { Icon(androidx.compose.material.icons.Icons.Default.Search, contentDescription = "Search") }
        )

        Spacer(modifier = Modifier.height(16.dp))

        // Pool List
        LazyColumn(
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            items(listOf(
                PoolData("ETH/USDC", "$1.2M", "$450K", "24.5%"),
                PoolData("WBTC/ETH", "$890K", "$320K", "18.2%"),
                PoolData("ETH/USDT", "$650K", "$210K", "15.8%"),
                PoolData("SOL/ETH", "$420K", "$150K", "22.1%")
            )) { pool ->
                PoolCard(pool)
            }
        }
    }
}

@Composable
fun PoolCard(pool: PoolData) {
    Card(
        modifier = Modifier.fillMaxWidth()
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Column {
                Text(pool.pair, style = MaterialTheme.typography.titleMedium)
                Text("TVL: ${pool.tvl}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            Column(horizontalAlignment = Alignment.End) {
                Text(pool.apr, style = MaterialTheme.typography.titleMedium, color = androidx.compose.ui.graphics.Color.Green)
                Text("Volume: ${pool.volume}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }
}

data class PoolData(val pair: String, val tvl: String, val volume: String, val apr: String)

// ==================== PORTFOLIO SCREEN ====================
@Composable
fun PortfolioScreen() {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp)
    ) {
        // Total Value Card
        Card(
            modifier = Modifier.fillMaxWidth(),
            colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.primaryContainer)
        ) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(24.dp),
                horizontalAlignment = Alignment.CenterHorizontally
            ) {
                Text("Total Value", style = MaterialTheme.typography.labelLarge)
                Text(
                    "$12,500.50",
                    style = MaterialTheme.typography.headlineLarge
                )
            }
        }

        Spacer(modifier = Modifier.height(24.dp))

        // Assets
        Text("Assets", style = MaterialTheme.typography.titleMedium)
        Spacer(modifier = Modifier.height(12.dp))

        LazyColumn(
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            items(listOf(
                AssetData("ETH", "2.5", "$6,250"),
                AssetData("USDC", "5,000", "$5,000"),
                AssetData("WETH", "0.5", "$1,250.50")
            )) { asset ->
                AssetCard(asset)
            }
        }
    }
}

@Composable
fun AssetCard(asset: AssetData) {
    Card(modifier = Modifier.fillMaxWidth()) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Row {
                Box(
                    modifier = Modifier
                        .size(40.dp)
                        .background(androidx.compose.ui.graphics.Color.Orange, shape = androidx.compose.foundation.shape.CircleShape),
                    contentAlignment = Alignment.Center
                ) {
                    Text(asset.symbol.first().toString(), color = androidx.compose.ui.graphics.Color.White)
                }
                Spacer(modifier = Modifier.width(12.dp))
                Column {
                    Text(asset.symbol, style = MaterialTheme.typography.titleMedium)
                    Text(asset.amount, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
            Text(asset.value, style = MaterialTheme.typography.titleMedium)
        }
    }
}

data class AssetData(val symbol: String, val amount: String, val value: String)

// ==================== WALLET SCREEN ====================
@Composable
fun WalletScreen() {
    var isConnected by remember { mutableStateOf(false) }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center
    ) {
        if (isConnected) {
            // Connected Wallet View
            Icon(
                imageVector = androidx.compose.material.icons.Icons.Default.AccountBalanceWallet,
                contentDescription = "Wallet",
                modifier = Modifier.size(80.dp),
                tint = MaterialTheme.colorScheme.primary
            )
            Spacer(modifier = Modifier.height(16.dp))
            Text("0x1234...5678", style = MaterialTheme.typography.bodyMedium)
            Spacer(modifier = Modifier.height(24.dp))
            Button(
                onClick = { isConnected = false },
                colors = ButtonDefaults.buttonColors(containerColor = androidx.compose.ui.graphics.Color.Red)
            ) {
                Text("Disconnect")
            }
        } else {
            // Connect Wallet View
            Icon(
                imageVector = androidx.compose.material.icons.Icons.Default.AccountBalanceWallet,
                contentDescription = "Wallet",
                modifier = Modifier.size(80.dp),
                tint = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(16.dp))
            Text("Connect your wallet", style = MaterialTheme.typography.titleLarge)
            Spacer(modifier = Modifier.height(24.dp))
            Button(
                onClick = { isConnected = true },
                modifier = Modifier
                    .fillMaxWidth()
                    .height(56.dp)
            ) {
                Text("Connect Wallet")
            }
        }
    }
}

// ==================== SETTINGS SCREEN ====================
@Composable
fun SettingsScreen() {
    var slippage by remember { mutableFloatStateOf(0.5f) }
    var selectedChain by remember { mutableStateOf("Ethereum") }

    LazyColumn(
        modifier = Modifier.fillMaxSize()
    ) {
        item {
            // Network Section
            ListItem(
                headlineContent = { Text("Network") },
                supportingContent = { Text(selectedChain) },
                leadingContent = { Icon(androidx.compose.material.icons.Icons.Default.Language, contentDescription = null) }
            )
        }

        item {
            // Slippage Section
            ListItem(
                headlineContent = { Text("Slippage Tolerance") },
                supportingContent = { Text("${(slippage * 100).toInt()}%") }
            )
            Slider(
                value = slippage,
                onValueChange = { slippage = it },
                valueRange = 0.01f..0.1f,
                modifier = Modifier.padding(horizontal = 16.dp)
            )
        }

        item {
            // Security Section
            ListItem(
                headlineContent = { Text("Biometric Auth") },
                trailingContent = { Switch(checked = true, onCheckedChange = {}) }
            )
        }

        item {
            ListItem(
                headlineContent = { Text("Push Notifications") },
                trailingContent = { Switch(checked = true, onCheckedChange = {}) }
            )
        }

        item {
            // About Section
            ListItem(
                headlineContent = { Text("Version") },
                supportingContent = { Text("1.0.0") }
            )
        }
    }
}
