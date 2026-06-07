import React, { useState, useEffect } from 'react';
import {
  StyleSheet,
  Text,
  View,
  TouchableOpacity,
  ScrollView,
  TextInput,
  Alert,
  SafeAreaView,
  StatusBar,
  Image,
} from 'react-native';
import { NavigationContainer } from '@react-navigation/native';
import { createStackNavigator } from '@react-navigation/stack';
import * as SecureStore from 'expo-secure-store';
import * as Crypto from 'expo-crypto';

// ============================================================================
// Types
// ============================================================================

interface Wallet {
  id: string;
  name: string;
  address: string;
  chain: string;
  balance: number;
}

interface Token {
  symbol: string;
  name: string;
  balance: number;
  value: number;
  price: number;
}

interface Transaction {
  id: string;
  hash: string;
  type: 'send' | 'receive' | 'swap';
  amount: number;
  token: string;
  status: 'pending' | 'confirmed' | 'failed';
  timestamp: number;
}

// ============================================================================
// API Service
// ============================================================================

const API_BASE = 'https://api.tigerswap.io';

class APIService {
  private baseUrl: string;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
  }

  async get<T>(endpoint: string): Promise<T> {
    // Simulated API response
    return {} as T;
  }

  async post<T>(endpoint: string, data: any): Promise<T> {
    // Simulated API response
    return {} as T;
  }
}

const api = new APIService(API_BASE);

// ============================================================================
// Wallet Service
// ============================================================================

class WalletService {
  async createWallet(name: string, chain: string): Promise<Wallet> {
    // Generate random address
    const address = '0x' + Array(40).fill(0).map(() => 
      Math.floor(Math.random() * 16).toString(16)
    ).join('');

    return {
      id: Crypto.randomUUID(),
      name,
      address,
      chain,
      balance: 0,
    };
  }

  async importWallet(mnemonic: string, chain: string): Promise<Wallet> {
    // Import from mnemonic
    return this.createWallet('Imported Wallet', chain);
  }

  async getBalance(address: string): Promise<number> {
    // Get balance from RPC
    return 0;
  }

  async sendTransaction(to: string, amount: number): Promise<string> {
    // Send transaction
    return '0x' + Array(64).fill(0).map(() => 
      Math.floor(Math.random() * 16).toString(16)
    ).join('');
  }

  async signMessage(message: string, privateKey: string): Promise<string> {
    // Sign message
    return 'signature';
  }
}

const walletService = new WalletService();

// ============================================================================
// Storage Service
// ============================================================================

class StorageService {
  async set(key: string, value: string): Promise<void> {
    await SecureStore.setItemAsync(key, value);
  }

  async get(key: string): Promise<string | null> {
    return SecureStore.getItemAsync(key);
  }

  async delete(key: string): Promise<void> {
    await SecureStore.deleteItemAsync(key);
  }

  async has(key: string): Promise<boolean> {
    const value = await SecureStore.getItemAsync(key);
    return value !== null;
  }
}

const storage = new StorageService();

// ============================================================================
// Screens
// ============================================================================

// Home Screen
function HomeScreen({ navigation }: any) {
  const [wallets, setWallets] = useState<Wallet[]>([]);
  const [tokens, setTokens] = useState<Token[]>([]);
  const [transactions, setTransactions] = useState<Transaction[]>([]);

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    // Load wallets
    const walletData = await storage.get('wallets');
    if (walletData) {
      setWallets(JSON.parse(walletData));
    }

    // Load tokens
    setTokens([
      { symbol: 'ETH', name: 'Ethereum', balance: 1.5, value: 3000, price: 2000 },
      { symbol: 'USDC', name: 'USD Coin', balance: 5000, value: 5000, price: 1 },
      { symbol: 'BTC', name: 'Bitcoin', balance: 0.1, value: 5000, price: 50000 },
    ]);

    // Load transactions
    setTransactions([
      {
        id: '1',
        hash: '0x123',
        type: 'send',
        amount: 0.5,
        token: 'ETH',
        status: 'confirmed',
        timestamp: Date.now() - 3600000,
      },
      {
        id: '2',
        hash: '0x456',
        type: 'swap',
        amount: 1000,
        token: 'USDC',
        status: 'confirmed',
        timestamp: Date.now() - 7200000,
      },
    ]);
  };

  const totalValue = tokens.reduce((sum, t) => sum + t.value, 0);

  return (
    <SafeAreaView style={styles.container}>
      <StatusBar barStyle="dark-content" />
      
      <ScrollView>
        {/* Header */}
        <View style={styles.header}>
          <Text style={styles.logo}>TigerSwap</Text>
          <TouchableOpacity style={styles.settingsButton}>
            <Text>⚙️</Text>
          </TouchableOpacity>
        </View>

        {/* Balance Card */}
        <View style={styles.balanceCard}>
          <Text style={styles.balanceLabel}>Total Balance</Text>
          <Text style={styles.balanceAmount}>${totalValue.toLocaleString()}</Text>
          <Text style={styles.balanceChange}>+$150.00 (2.5%)</Text>
        </View>

        {/* Actions */}
        <View style={styles.actions}>
          <TouchableOpacity 
            style={styles.actionButton}
            onPress={() => navigation.navigate('Send')}
          >
            <Text style={styles.actionIcon}>📤</Text>
            <Text style={styles.actionText}>Send</Text>
          </TouchableOpacity>
          
          <TouchableOpacity 
            style={styles.actionButton}
            onPress={() => navigation.navigate('Receive')}
          >
            <Text style={styles.actionIcon}>📥</Text>
            <Text style={styles.actionText}>Receive</Text>
          </TouchableOpacity>
          
          <TouchableOpacity 
            style={styles.actionButton}
            onPress={() => navigation.navigate('Swap')}
          >
            <Text style={styles.actionIcon}>🔄</Text>
            <Text style={styles.actionText}>Swap</Text>
          </TouchableOpacity>
          
          <TouchableOpacity 
            style={styles.actionButton}
            onPress={() => navigation.navigate('Buy')}
          >
            <Text style={styles.actionIcon}>💳</Text>
            <Text style={styles.actionText}>Buy</Text>
          </TouchableOpacity>
        </View>

        {/* Tokens */}
        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Assets</Text>
          {tokens.map((token, index) => (
            <View key={index} style={styles.tokenItem}>
              <View style={styles.tokenIcon}>
                <Text>{token.symbol[0]}</Text>
              </View>
              <View style={styles.tokenInfo}>
                <Text style={styles.tokenSymbol}>{token.symbol}</Text>
                <Text style={styles.tokenName}>{token.name}</Text>
              </View>
              <View style={styles.tokenValue}>
                <Text style={styles.tokenBalance}>{token.balance}</Text>
                <Text style={styles.tokenDollar}>${token.value.toLocaleString()}</Text>
              </View>
            </View>
          ))}
        </View>

        {/* Transactions */}
        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Recent Activity</Text>
          {transactions.map((tx, index) => (
            <View key={index} style={styles.txItem}>
              <View style={[styles.txIcon, tx.type === 'send' && styles.txSend]}>
                <Text>{tx.type === 'send' ? '📤' : tx.type === 'receive' ? '📥' : '🔄'}</Text>
              </View>
              <View style={styles.txInfo}>
                <Text style={styles.txType}>
                  {tx.type === 'send' ? 'Sent' : tx.type === 'receive' ? 'Received' : 'Swapped'}
                </Text>
                <Text style={styles.txToken}>
                  {tx.amount} {tx.token}
                </Text>
              </View>
              <View style={styles.txStatus}>
                <Text style={[
                  styles.txStatusText,
                  tx.status === 'confirmed' && styles.txConfirmed,
                  tx.status === 'pending' && styles.txPending,
                ]}>
                  {tx.status}
                </Text>
              </View>
            </View>
          ))}
        </View>
      </ScrollView>

      {/* Bottom Tab Bar */}
      <View style={styles.tabBar}>
        <TouchableOpacity style={styles.tabItem}>
          <Text style={styles.tabIcon}>🏠</Text>
          <Text style={styles.tabText}>Home</Text>
        </TouchableOpacity>
        <TouchableOpacity style={styles.tabItem}>
          <Text style={styles.tabIcon}>📊</Text>
          <Text style={styles.tabText}>Trade</Text>
        </TouchableOpacity>
        <TouchableOpacity style={styles.tabItem}>
          <Text style={styles.tabIcon}>📷</Text>
          <Text style={styles.tabText}>Scan</Text>
        </TouchableOpacity>
        <TouchableOpacity style={styles.tabItem}>
          <Text style={styles.tabIcon}>👤</Text>
          <Text style={styles.tabText}>Wallet</Text>
        </TouchableOpacity>
      </View>
    </SafeAreaView>
  );
}

// Send Screen
function SendScreen({ navigation }: any) {
  const [recipient, setRecipient] = useState('');
  const [amount, setAmount] = useState('');

  const handleSend = async () => {
    if (!recipient || !amount) {
      Alert.alert('Error', 'Please fill in all fields');
      return;
    }

    Alert.alert('Confirm', `Send ${amount} ETH to ${recipient}?`, [
      { text: 'Cancel', style: 'cancel' },
      { text: 'Confirm', onPress: async () => {
        Alert.alert('Success', 'Transaction sent!');
        navigation.goBack();
      }},
    ]);
  };

  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.header}>
        <TouchableOpacity onPress={() => navigation.goBack()}>
          <Text>← Back</Text>
        </TouchableOpacity>
        <Text style={styles.screenTitle}>Send</Text>
        <View />
      </View>

      <View style={styles.form}>
        <Text style={styles.label}>Recipient Address</Text>
        <TextInput
          style={styles.input}
          value={recipient}
          onChangeText={setRecipient}
          placeholder="0x..."
          placeholderTextColor="#999"
        />

        <Text style={styles.label}>Amount</Text>
        <View style={styles.amountInput}>
          <TextInput
            style={[styles.input, { flex: 1 }]}
            value={amount}
            onChangeText={setAmount}
            placeholder="0.00"
            placeholderTextColor="#999"
            keyboardType="decimal-pad"
          />
          <Text style={styles.maxButton}>MAX</Text>
        </View>

        <TouchableOpacity style={styles.primaryButton} onPress={handleSend}>
          <Text style={styles.primaryButtonText}>Send</Text>
        </TouchableOpacity>
      </View>
    </SafeAreaView>
  );
}

// Receive Screen
function ReceiveScreen({ navigation }: any) {
  const address = '0x1234567890abcdef1234567890abcdef12345678';

  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.header}>
        <TouchableOpacity onPress={() => navigation.goBack()}>
          <Text>← Back</Text>
        </TouchableOpacity>
        <Text style={styles.screenTitle}>Receive</Text>
        <View />
      </View>

      <View style={styles.receiveContent}>
        <View style={styles.qrCode}>
          <Text style={styles.qrPlaceholder}>QR Code</Text>
        </View>
        
        <Text style={styles.addressLabel}>Your Address</Text>
        <Text style={styles.address}>{address}</Text>
        
        <TouchableOpacity style={styles.copyButton}>
          <Text style={styles.copyButtonText}>Copy Address</Text>
        </TouchableOpacity>
      </View>
    </SafeAreaView>
  );
}

// Swap Screen
function SwapScreen({ navigation }: any) {
  const [fromToken, setFromToken] = useState('ETH');
  const [toToken, setToToken] = useState('USDC');
  const [amount, setAmount] = useState('');

  const handleSwap = async () => {
    if (!amount) {
      Alert.alert('Error', 'Please enter an amount');
      return;
    }

    Alert.alert('Confirm', `Swap ${amount} ${fromToken} to ${toToken}?`, [
      { text: 'Cancel', style: 'cancel' },
      { text: 'Confirm', onPress: () => {
        Alert.alert('Success', 'Swap completed!');
        navigation.goBack();
      }},
    ]);
  };

  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.header}>
        <TouchableOpacity onPress={() => navigation.goBack()}>
          <Text>← Back</Text>
        </TouchableOpacity>
        <Text style={styles.screenTitle}>Swap</Text>
        <View />
      </View>

      <View style={styles.form}>
        <Text style={styles.label}>From</Text>
        <View style={styles.tokenSelector}>
          <Text style={styles.tokenValue}>{fromToken}</Text>
        </View>

        <Text style={styles.label}>To</Text>
        <View style={styles.tokenSelector}>
          <Text style={styles.tokenValue}>{toToken}</Text>
        </View>

        <Text style={styles.label}>Amount</Text>
        <TextInput
          style={styles.input}
          value={amount}
          onChangeText={setAmount}
          placeholder="0.00"
          placeholderTextColor="#999"
          keyboardType="decimal-pad"
        />

        <TouchableOpacity style={styles.primaryButton} onPress={handleSwap}>
          <Text style={styles.primaryButtonText}>Swap</Text>
        </TouchableOpacity>
      </View>
    </SafeAreaView>
  );
}

// Buy Screen
function BuyScreen({ navigation }: any) {
  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.header}>
        <TouchableOpacity onPress={() => navigation.goBack()}>
          <Text>← Back</Text>
        </TouchableOpacity>
        <Text style={styles.screenTitle}>Buy Crypto</Text>
        <View />
      </View>

      <View style={styles.buyContent}>
        <Text style={styles.comingSoon}>Coming Soon</Text>
        <Text style={styles.comingSoonText}>
          Buy crypto with card or bank transfer
        </Text>
      </View>
    </SafeAreaView>
  );
}

// ============================================================================
// Navigation
// ============================================================================

const Stack = createStackNavigator();

export default function App() {
  return (
    <NavigationContainer>
      <Stack.Navigator screenOptions={{ headerShown: false }}>
        <Stack.Screen name="Home" component={HomeScreen} />
        <Stack.Screen name="Send" component={SendScreen} />
        <Stack.Screen name="Receive" component={ReceiveScreen} />
        <Stack.Screen name="Swap" component={SwapScreen} />
        <Stack.Screen name="Buy" component={BuyScreen} />
      </Stack.Navigator>
    </NavigationContainer>
  );
}

// ============================================================================
// Styles
// ============================================================================

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#fff',
  },
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: 20,
    paddingTop: 50,
  },
  logo: {
    fontSize: 24,
    fontWeight: 'bold',
    color: '#FF6B00',
  },
  settingsButton: {
    padding: 10,
  },
  balanceCard: {
    backgroundColor: '#FF6B00',
    margin: 20,
    padding: 30,
    borderRadius: 20,
  },
  balanceLabel: {
    color: 'rgba(255,255,255,0.8)',
    fontSize: 14,
  },
  balanceAmount: {
    color: '#fff',
    fontSize: 36,
    fontWeight: 'bold',
    marginVertical: 10,
  },
  balanceChange: {
    color: 'rgba(255,255,255,0.8)',
    fontSize: 14,
  },
  actions: {
    flexDirection: 'row',
    justifyContent: 'space-around',
    padding: 20,
  },
  actionButton: {
    alignItems: 'center',
  },
  actionIcon: {
    fontSize: 28,
    marginBottom: 8,
  },
  actionText: {
    fontSize: 12,
    color: '#333',
  },
  section: {
    padding: 20,
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: 'bold',
    marginBottom: 15,
  },
  tokenItem: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 15,
    borderBottomWidth: 1,
    borderBottomColor: '#eee',
  },
  tokenIcon: {
    width: 40,
    height: 40,
    borderRadius: 20,
    backgroundColor: '#FF6B00',
    justifyContent: 'center',
    alignItems: 'center',
  },
  tokenInfo: {
    flex: 1,
    marginLeft: 15,
  },
  tokenSymbol: {
    fontSize: 16,
    fontWeight: 'bold',
  },
  tokenName: {
    fontSize: 12,
    color: '#666',
  },
  tokenValue: {
    alignItems: 'flex-end',
  },
  tokenBalance: {
    fontSize: 16,
    fontWeight: 'bold',
  },
  tokenDollar: {
    fontSize: 12,
    color: '#666',
  },
  txItem: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 15,
    borderBottomWidth: 1,
    borderBottomColor: '#eee',
  },
  txIcon: {
    width: 36,
    height: 36,
    borderRadius: 18,
    backgroundColor: '#e0ffe0',
    justifyContent: 'center',
    alignItems: 'center',
  },
  txSend: {
    backgroundColor: '#ffe0e0',
  },
  txInfo: {
    flex: 1,
    marginLeft: 15,
  },
  txType: {
    fontSize: 14,
    fontWeight: 'bold',
  },
  txToken: {
    fontSize: 12,
    color: '#666',
  },
  txStatus: {},
  txStatusText: {
    fontSize: 12,
    color: '#666',
  },
  txConfirmed: {
    color: '#00aa00',
  },
  txPending: {
    color: '#ffaa00',
  },
  tabBar: {
    flexDirection: 'row',
    justifyContent: 'space-around',
    padding: 15,
    borderTopWidth: 1,
    borderTopColor: '#eee',
  },
  tabItem: {
    alignItems: 'center',
  },
  tabIcon: {
    fontSize: 20,
    marginBottom: 4,
  },
  tabText: {
    fontSize: 10,
    color: '#666',
  },
  screenTitle: {
    fontSize: 18,
    fontWeight: 'bold',
  },
  form: {
    padding: 20,
  },
  label: {
    fontSize: 14,
    fontWeight: 'bold',
    marginBottom: 8,
    marginTop: 16,
  },
  input: {
    backgroundColor: '#f5f5f5',
    padding: 15,
    borderRadius: 10,
    fontSize: 16,
  },
  amountInput: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  maxButton: {
    padding: 15,
    color: '#FF6B00',
    fontWeight: 'bold',
  },
  primaryButton: {
    backgroundColor: '#FF6B00',
    padding: 18,
    borderRadius: 10,
    alignItems: 'center',
    marginTop: 30,
  },
  primaryButtonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: 'bold',
  },
  receiveContent: {
    padding: 30,
    alignItems: 'center',
  },
  qrCode: {
    width: 200,
    height: 200,
    backgroundColor: '#f5f5f5',
    justifyContent: 'center',
    alignItems: 'center',
    marginBottom: 30,
  },
  qrPlaceholder: {
    fontSize: 14,
    color: '#999',
  },
  addressLabel: {
    fontSize: 14,
    color: '#666',
    marginBottom: 10,
  },
  address: {
    fontSize: 12,
    color: '#333',
    marginBottom: 20,
    textAlign: 'center',
  },
  copyButton: {
    backgroundColor: '#FF6B00',
    padding: 15,
    borderRadius: 10,
    paddingHorizontal: 40,
  },
  copyButtonText: {
    color: '#fff',
    fontWeight: 'bold',
  },
  tokenSelector: {
    backgroundColor: '#f5f5f5',
    padding: 15,
    borderRadius: 10,
    flexDirection: 'row',
    justifyContent: 'space-between',
  },
  tokenValue: {
    fontSize: 16,
    fontWeight: 'bold',
  },
  buyContent: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  comingSoon: {
    fontSize: 24,
    fontWeight: 'bold',
    marginBottom: 10,
  },
  comingSoonText: {
    fontSize: 14,
    color: '#666',
  },
});