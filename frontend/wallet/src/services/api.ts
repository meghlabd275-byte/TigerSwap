import axios, { AxiosInstance, AxiosError } from 'axios';
import { Address, Hash, Hex, keccak256, toHex } from 'viem';
import type { 
  Token, 
  TokenBalance, 
  GasPrice, 
  GasEstimate,
  UserOperation,
  UserOperationReceipt,
  FiatQuote,
  FiatProvider,
  NFT,
  DApp,
  Notification,
  Proposal,
  SecurityAlert,
  SwapIntent,
} from '@/types/wallet';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1';

class ApiService {
  private client: AxiosInstance;

  constructor() {
    this.client = axios.create({
      baseURL: API_BASE_URL,
      timeout: 30000,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    this.client.interceptors.request.use((config) => {
      const token = typeof window !== 'undefined' 
        ? localStorage.getItem('auth_token')
        : null;
      if (token) {
        config.headers.Authorization = `Bearer ${token}`;
      }
      return config;
    });

    this.client.interceptors.response.use(
      (response) => response,
      (error: AxiosError) => {
        if (error.response?.status === 401) {
          if (typeof window !== 'undefined') {
            localStorage.removeItem('auth_token');
            window.location.href = '/login';
          }
        }
        return Promise.reject(error);
      }
    );
  }

  // Wallet Operations
  async createSmartAccount(owner: Address, entryPoint: Address, factory: Address): Promise<{ address: Address; transactionHash: Hash }> {
    const response = await this.client.post('/wallet/smart-account/create', {
      owner,
      entryPoint,
      factory,
    });
    return response.data;
  }

  async getWalletAddress(entryPoint: Address, factory: Address, owner: Address, salt: bigint): Promise<Address> {
    const response = await this.client.get('/wallet/address', {
      params: { entryPoint, factory, owner, salt: salt.toString() },
    });
    return response.data.address;
  }

  // Token Operations
  async getTokens(chainId: number): Promise<Token[]> {
    const response = await this.client.get(`/tokens`, {
      params: { chainId },
    });
    return response.data.tokens;
  }

  async getTokenBalances(address: Address, chainId: number): Promise<TokenBalance[]> {
    const response = await this.client.get(`/wallet/${address}/balances`, {
      params: { chainId },
    });
    return response.data.balances;
  }

  async getTokenPrice(address: Address): Promise<number> {
    const response = await this.client.get(`/tokens/${address}/price`);
    return response.data.price;
  }

  // User Operation (ERC-4337)
  async sendUserOperation(userOp: UserOperation, entryPoint: Address): Promise<{ hash: Hash }> {
    const response = await this.client.post('/user-operation/send', {
      userOp,
      entryPoint,
    });
    return response.data;
  }

  async getUserOperationReceipt(userOpHash: Hash): Promise<UserOperationReceipt | null> {
    const response = await this.client.get(`/user-operation/${userOpHash}/receipt`);
    return response.data.receipt;
  }

  async simulateUserOperation(userOp: UserOperation, entryPoint: Address): Promise<{ success: boolean; data?: any }> {
    const response = await this.client.post('/user-operation/simulate', {
      userOp,
      entryPoint,
    });
    return response.data;
  }

  // Gas Operations
  async getGasPrice(chainId: number): Promise<GasPrice> {
    const response = await this.client.get(`/gas/price`, {
      params: { chainId },
    });
    return response.data;
  }

  async estimateGas(
    from: Address,
    to: Address,
    data: Hex,
    value?: bigint,
    chainId?: number
  ): Promise<GasEstimate> {
    const response = await this.client.post('/gas/estimate', {
      from,
      to,
      data,
      value: value?.toString(),
      chainId,
    });
    return response.data;
  }

  // Transaction History
  async getTransactionHistory(address: Address, chainId: number, limit = 50, offset = 0) {
    const response = await this.client.get(`/wallet/${address}/transactions`, {
      params: { chainId, limit, offset },
    });
    return response.data;
  }

  // Swap Operations
  async getSwapQuote(
    fromToken: Address,
    toToken: Address,
    amount: bigint,
    chainId: number
  ): Promise<{ toAmount: bigint; path: Address[]; gas: bigint }> {
    const response = await this.client.get('/swap/quote', {
      params: {
        fromToken,
        toToken,
        amount: amount.toString(),
        chainId,
      },
    });
    return response.data;
  }

  async executeSwap(
    fromToken: Address,
    toToken: Address,
    fromAmount: bigint,
    minToAmount: bigint,
    recipient: Address,
    chainId: number
  ): Promise<{ hash: Hash }> {
    const response = await this.client.post('/swap/execute', {
      fromToken,
      toToken,
      fromAmount: fromAmount.toString(),
      minToAmount: minToAmount.toString(),
      recipient,
      chainId,
    });
    return response.data;
  }

  // Intent-based swap
  async createSwapIntent(intent: Omit<SwapIntent, 'id' | 'status'>): Promise<SwapIntent> {
    const response = await this.client.post('/intent/swap/create', intent);
    return response.data;
  }

  async fillSwapIntent(intentId: string, data: Hex): Promise<{ hash: Hash }> {
    const response = await this.client.post(`/intent/swap/${intentId}/fill`, { data });
    return response.data;
  }

  // NFT Operations
  async getNFTs(address: Address, chainId: number): Promise<NFT[]> {
    const response = await this.client.get(`/wallet/${address}/nfts`, {
      params: { chainId },
    });
    return response.data.nfts;
  }

  async getNFTCollections(chainId: number): Promise<any[]> {
    const response = await this.client.get('/nft/collections', {
      params: { chainId },
    });
    return response.data.collections;
  }

  // Fiat On-Ramp
  async getFiatProviders(): Promise<FiatProvider[]> {
    const response = await this.client.get('/fiat/providers');
    return response.data.providers;
  }

  async getFiatQuote(
    providerId: string,
    fromCurrency: string,
    toCurrency: string,
    fromAmount: number
  ): Promise<FiatQuote> {
    const response = await this.client.get(`/fiat/${providerId}/quote`, {
      params: { fromCurrency, toCurrency, fromAmount },
    });
    return response.data;
  }

  async createFiatOrder(
    providerId: string,
    quoteId: string,
    paymentMethodId: string,
    walletAddress: Address
  ): Promise<{ orderId: string; paymentUrl: string }> {
    const response = await this.client.post(`/fiat/${providerId}/order`, {
      quoteId,
      paymentMethodId,
      walletAddress,
    });
    return response.data;
  }

  // DApp Store
  async getDApps(category?: string, chainId?: number): Promise<DApp[]> {
    const response = await this.client.get('/dapps', {
      params: { category, chainId },
    });
    return response.data.dapps;
  }

  async submitDApp(dapp: Partial<DApp>): Promise<DApp> {
    const response = await this.client.post('/dapps/submit', dapp);
    return response.data;
  }

  // Notifications
  async getNotifications(): Promise<Notification[]> {
    const response = await this.client.get('/notifications');
    return response.data.notifications;
  }

  async markNotificationRead(id: string): Promise<void> {
    await this.client.patch(`/notifications/${id}/read`);
  }

  async registerPushToken(token: string, deviceType: string): Promise<void> {
    await this.client.post('/notifications/push/register', { token, deviceType });
  }

  // Governance
  async getProposals(): Promise<Proposal[]> {
    const response = await this.client.get('/governance/proposals');
    return response.data.proposals;
  }

  async getProposal(id: string): Promise<Proposal> {
    const response = await this.client.get(`/governance/proposals/${id}`);
    return response.data.proposal;
  }

  async castVote(proposalId: string, choice: 'FOR' | 'AGAINST' | 'ABSTAIN', weight: bigint): Promise<void> {
    await this.client.post(`/governance/proposals/${proposalId}/vote`, {
      choice,
      weight: weight.toString(),
    });
  }

  async delegateVotes(delegatee: Address): Promise<void> {
    await this.client.post('/governance/delegate', { delegatee });
  }

  // Security
  async checkAddressSecurity(address: Address): Promise<SecurityAlert[]> {
    const response = await this.client.get(`/security/check/${address}`);
    return response.data.alerts;
  }

  async checkApprovalSecurity(address: Address, spender: Address): Promise<SecurityAlert[]> {
    const response = await this.client.post('/security/check-approval', {
      owner: address,
      spender,
    });
    return response.data.alerts;
  }

  async simulateTransaction(
    from: Address,
    to: Address,
    data: Hex,
    value?: bigint
  ): Promise<{ success: boolean; logs: any[]; gasUsed: bigint }> {
    const response = await this.client.post('/security/simulate', {
      from,
      to,
      data,
      value: value?.toString(),
    });
    return response.data;
  }

  // MPC & Social Login
  async initiateSocialLogin(provider: 'google' | 'apple' | 'email', idToken?: string): Promise<{ authUrl: string }> {
    const response = await this.client.post('/auth/social/init', {
      provider,
      idToken,
    });
    return response.data;
  }

  async getMPCKeyShares(walletAddress: Address): Promise<any[]> {
    const response = await this.client.get(`/mpc/${walletAddress}/shares`);
    return response.data.shares;
  }

  async initiateKeyRecovery(walletAddress: Address, guardianAddresses: Address[]): Promise<{ recoveryId: string }> {
    const response = await this.client.post('/mpc/recovery/init', {
      walletAddress,
      guardians: guardianAddresses,
    });
    return response.data;
  }

  async completeKeyRecovery(recoveryId: string, guardianSignatures: Hex[]): Promise<{ newKeyShare: Hex }> {
    const response = await this.client.post(`/mpc/recovery/${recoveryId}/complete`, {
      guardianSignatures,
    });
    return response.data;
  }

  // Social Recovery
  async setupSocialRecovery(
    walletAddress: Address,
    guardians: Address[],
    threshold: number,
    delayPeriod: number
  ): Promise<void> {
    await this.client.post('/social-recovery/setup', {
      walletAddress,
      guardians,
      threshold,
      delayPeriod,
    });
  }

  async initiateSocialRecovery(
    walletAddress: Address,
    newOwner: Address,
    guardianSignatures: Hex[]
  ): Promise<{ recoveryId: string; executeAfter: number }> {
    const response = await this.client.post('/social-recovery/initiate', {
      walletAddress,
      newOwner,
      guardianSignatures,
    });
    return response.data;
  }

  async executeSocialRecovery(recoveryId: string): Promise<void> {
    await this.client.post(`/social-recovery/${recoveryId}/execute`);
  }

  // Cross-Chain Bridge
  async getBridgeQuote(
    fromChain: number,
    toChain: number,
    fromToken: Address,
    toToken: Address,
    amount: bigint
  ): Promise<{ toAmount: bigint; bridgeFee: bigint; estimatedTime: number }> {
    const response = await this.client.get('/bridge/quote', {
      params: {
        fromChain,
        toChain,
        fromToken,
        toToken,
        amount: amount.toString(),
      },
    });
    return response.data;
  }

  async executeBridge(
    fromChain: number,
    toChain: number,
    fromToken: Address,
    toToken: Address,
    amount: bigint,
    minToAmount: bigint,
    recipient: Address
  ): Promise<{ hash: Hash; bridgeId: string }> {
    const response = await this.client.post('/bridge/execute', {
      fromChain,
      toChain,
      fromToken,
      toToken,
      amount: amount.toString(),
      minToAmount: minToAmount.toString(),
      recipient,
    });
    return response.data;
  }

  // Device Sync
  async getSyncDevices(): Promise<any[]> {
    const response = await this.client.get('/sync/devices');
    return response.data.devices;
  }

  async requestSync(): Promise<{ syncId: string }> {
    const response = await this.client.post('/sync/request');
    return response.data;
  }

  async approveSyncDevice(deviceId: string): Promise<void> {
    await this.client.post(`/sync/device/${deviceId}/approve`);
  }

  // MEV Protection
  async sendProtectedTransaction(
    to: Address,
    data: Hex,
    value?: bigint,
    maxFeePerGas?: bigint,
    maxPriorityFeePerGas?: bigint
  ): Promise<{ hash: Hash }> {
    const response = await this.client.post('/mev/protected-tx', {
      to,
      data,
      value: value?.toString(),
      maxFeePerGas: maxFeePerGas?.toString(),
      maxPriorityFeePerGas: maxPriorityFeePerGas?.toString(),
    });
    return response.data;
  }
}

export const apiService = new ApiService();
export default apiService;
