import { Address, Hash, Hex } from 'viem';

// Wallet Types
export interface Wallet {
  id: string;
  address: Address;
  type: WalletType;
  name?: string;
  createdAt: number;
  updatedAt: number;
}

export type WalletType = 'EOA' | 'SMART_ACCOUNT' | 'MPC' | 'Hardware';

export interface SmartAccount extends Wallet {
  type: 'SMART_ACCOUNT';
  entryPoint: Address;
  factory: Address;
  implementation: Address;
  owners: Address[];
  threshold: number;
  nonce: number;
}

export interface MPCKeyShare {
  id: string;
  walletAddress: Address;
  shareIndex: number;
  encryptedShare: string;
  createdAt: number;
}

export interface Guardian {
  address: Address;
  name?: string;
  type: GuardianType;
  weight: number;
}

export type GuardianType = 'EOA' | 'CONTRACT' | 'EMAIL' | 'PHONE';

export interface SocialRecoveryConfig {
  walletAddress: Address;
  guardians: Guardian[];
  threshold: number;
  delayPeriod: number;
  isActive: boolean;
}

// Transaction Types
export interface Transaction {
  id: string;
  hash: Hash;
  from: Address;
  to: Address;
  value: bigint;
  data: Hex;
  nonce: number;
  gasLimit: bigint;
  gasPrice: bigint;
  maxFeePerGas: bigint;
  maxPriorityFeePerGas: bigint;
  chainId: number;
  status: TransactionStatus;
  timestamp: number;
}

export type TransactionStatus = 'pending' | 'confirmed' | 'failed' | 'dropped';

// User Operation (ERC-4337)
export interface UserOperation {
  sender: Address;
  nonce: bigint;
  initCode: Hex;
  callData: Hex;
  callGasLimit: bigint;
  verificationGasLimit: bigint;
  preVerificationGas: bigint;
  maxFeePerGas: bigint;
  maxPriorityFeePerGas: bigint;
  paymasterAndData: Hex;
  signature: Hex;
}

export interface UserOperationReceipt {
  userOpHash: Hash;
  entryPoint: Address;
  sender: Address;
  nonce: bigint;
  success: boolean;
  actualGasUsed: bigint;
  logs: any[];
  transactionHash: Hash;
}

// Token Types
export interface Token {
  address: Address;
  chainId: number;
  name: string;
  symbol: string;
  decimals: number;
  logoURI?: string;
  price?: number;
  marketCap?: number;
  volume24h?: number;
}

export interface TokenBalance {
  token: Token;
  balance: bigint;
  balanceUSD: number;
  price: number;
}

// NFT Types
export interface NFT {
  id: string;
  contractAddress: Address;
  tokenId: bigint;
  owner: Address;
  uri: string;
  name?: string;
  description?: string;
  image?: string;
  attributes?: NFTAttribute[];
  collection: NFTCollection;
}

export interface NFTCollection {
  address: Address;
  name: string;
  symbol?: string;
  floorPrice?: number;
  totalSupply: number;
  image?: string;
}

export interface NFTAttribute {
  trait_type: string;
  value: string | number;
}

// DApp Types
export interface DApp {
  id: string;
  name: string;
  description: string;
  url: string;
  logo: string;
  category: DAppCategory;
  chains: number[];
  verified: boolean;
  rating: number;
  visits: number;
}

export type DAppCategory = 
  | 'defi' 
  | 'nft' 
  | 'games' 
  | 'social' 
  | 'tools' 
  | 'bridge' 
  | 'staking'
  | 'other';

// Fiat On-Ramp Types
export interface FiatProvider {
  id: string;
  name: string;
  logo: string;
  supportedCurrencies: string[];
  supportedPaymentMethods: PaymentMethod[];
  fees: number;
  limits: {
    min: number;
    max: number;
  };
}

export interface PaymentMethod {
  id: string;
  name: string;
  type: PaymentType;
  icon?: string;
}

export type PaymentType = 
  | 'CARD'
  | 'BANK_TRANSFER'
  | 'APPLE_PAY'
  | 'GOOGLE_PAY'
  | 'SEPA'
  | 'FPS'
  | 'PIX'
  | 'UPI';

export interface FiatQuote {
  provider: FiatProvider;
  fromAmount: number;
  fromCurrency: string;
  toAmount: number;
  toCurrency: string;
  rate: number;
  fee: number;
  expiresAt: Date;
}

// Notification Types
export interface Notification {
  id: string;
  type: NotificationType;
  title: string;
  message: string;
  data?: Record<string, any>;
  read: boolean;
  timestamp: number;
}

export type NotificationType = 
  | 'TRANSACTION'
  | 'PRICE_ALERT'
  | 'AIRDROP'
  | 'GAS_ALERT'
  | 'SECURITY'
  | 'SYSTEM';

// Governance Types
export interface Proposal {
  id: string;
  title: string;
  description: string;
  proposer: Address;
  status: ProposalStatus;
  votesFor: bigint;
  votesAgainst: bigint;
  votesAbstain: bigint;
  startBlock: number;
  endBlock: number;
  executionData?: Hex;
}

export type ProposalStatus = 
  | 'PENDING'
  | 'ACTIVE'
  | 'CANCELLED'
  | 'DEFEATED'
  | 'SUCCEEDED'
  | 'EXECUTED'
  | 'EXPIRED';

export interface Vote {
  proposalId: string;
  voter: Address;
  choice: VoteChoice;
  weight: bigint;
  timestamp: number;
}

export type VoteChoice = 'FOR' | 'AGAINST' | 'ABSTAIN';

// Security Types
export interface SecurityAlert {
  id: string;
  type: SecurityAlertType;
  severity: SecuritySeverity;
  title: string;
  description: string;
  recommendation?: string;
  timestamp: number;
}

export type SecurityAlertType = 
  | 'HONEYPOT'
  | 'SANDWICH'
  | 'INFINITE_APPROVAL'
  | 'PERMIT2_VULNERABILITY'
  | 'AAVE_HEALTH'
  | 'PHISHING';

export type SecuritySeverity = 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';

// Multi-Device Sync Types
export interface SyncDevice {
  id: string;
  name: string;
  type: DeviceType;
  lastSeen: number;
  trusted: boolean;
}

export type DeviceType = 'DESKTOP' | 'MOBILE' | 'TABLET' | 'BROWSER_EXTENSION';

// Gas Types
export interface GasPrice {
  slow: bigint;
  standard: bigint;
  fast: bigint;
  baseFee: bigint;
}

export interface GasEstimate {
  gasLimit: bigint;
  gasPrice: bigint;
  totalCost: bigint;
  usdCost: number;
}

// Intent Types
export interface SwapIntent {
  id: string;
  owner: Address;
  fromToken: Address;
  toToken: Address;
  fromAmount: bigint;
  minToAmount: bigint;
  fillDeadLine: number;
  status: IntentStatus;
}

export type IntentStatus = 'OPEN' | 'FILLED' | 'EXPIRED' | 'CANCELLED';
