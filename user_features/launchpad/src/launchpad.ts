/**
 * TigerSwap Launchpad - Token Launch Platform
 * IDO/IEO/INO Platform for token launches
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

import { ethers } from 'ethers';

// ============================================================================
// Types
// ============================================================================

export interface LaunchpadConfig {
  factoryAddress: string;
  routerAddress: string;
  tokenAddress: string;
  owner: string;
}

export interface IFO {
  id: string;
  name: string;
  description: string;
  
  // Token info
  tokenAddress: string;
  tokenSymbol: string;
  tokenName: string;
  tokenDecimals: number;
  tokenLogo: string;
  
  // Offering info
  offeringToken: string; // Token accepted (USDT, etc.)
  totalOfferingAmount: string;
  pricePerToken: string;
  minBuyAmount: string;
  maxBuyAmount: string;
  
  // Schedule
  startTime: number;
  endTime: number;
  claimTime: number;
  
  // Status
  status: IFOStatus;
  
  // Metrics
  totalRaised: string;
  totalParticipants: number;
  
  // Pool
  hardCap: string;
  softCap: string;
}

export enum IFOStatus {
  PENDING = 'pending',
  ACTIVE = 'active',
  ENDED = 'ended',
  CLAIMABLE = 'claimable',
  CANCELLED = 'cancelled'
}

export interface IFOPool {
  poolId: number;
  allocation: string;
  minDeposit: string;
  maxDeposit: string;
  requiredNFT?: string;
}

export interface UserParticipation {
  ifoId: string;
  userAddress: string;
  poolId: number;
  amountDeposited: string;
  amountClaimed: string;
  hasClaimed: boolean;
  timestamp: number;
}

// ============================================================================
// Launchpad Contract ABI
// ============================================================================

export const LAUNCHPAD_ABI = [
  // IFO Functions
  'function createIFO(address token, address offeringToken, uint256 offeringAmount, uint256 pricePerToken, uint256 startTime, uint256 endTime, uint256 claimTime) returns (address)',
  'function deposit(address ifo, uint256 amount) returns (bool)',
  'function claim(address ifo) returns (bool)',
  'function pendingReward(address ifo, address user) view returns (uint256)',
  
  // View Functions
  'function getIFO(address ifo) view returns (tuple(address token, address offeringToken, uint256 totalOfferingAmount, uint256 pricePerToken, uint256 startTime, uint256 endTime, uint256 claimTime, uint256 totalRaised, uint256 totalParticipants))',
  'function getUserInfo(address ifo, address user) view returns (uint256 amount, uint256 claimed, bool hasClaimed))',
  'function getAllIFOs() view returns (address[])',
  
  // Events
  'event IFOCreated(address indexed ifo, address indexed token, address offeringToken, uint256 startTime, uint256 endTime)',
  'event Deposit(address indexed user, address indexed ifo, uint256 amount)',
  'event Claim(address indexed user, address indexed ifo, uint256 amount)',
];

export const TOKEN_ABI = [
  'function balanceOf(address account) view returns (uint256)',
  'function transfer(address to, uint256 amount) returns (bool)',
  'function approve(address spender, uint256 amount) returns (bool)',
  'function allowance(address owner, address spender) view returns (uint256)',
];

// ============================================================================
// Launchpad Client
// ============================================================================

export class LaunchpadClient {
  private provider: ethers.Provider;
  private signer: ethers.Signer | null;
  private contract: ethers.Contract;
  
  constructor(
    provider: ethers.Provider,
    config: LaunchpadConfig,
    signer?: ethers.Signer
  ) {
    this.provider = provider;
    this.signer = signer || null;
    this.contract = new ethers.Contract(
      config.factoryAddress,
      LAUNCHPAD_ABI,
      signer || provider
    );
  }
  
  // ========================================================================
  // IFO Management
  // ========================================================================
  
  /**
   * Create a new IFO
   */
  async createIFO(ifo: Omit<IFO, 'id' | 'status' | 'totalRaised' | 'totalParticipants'>): Promise<string> {
    if (!this.signer) {
      throw new Error('Signer required for creating IFO');
    }
    
    const tx = await this.contract.createIFO(
      ifo.tokenAddress,
      ifo.offeringToken,
      ifo.totalOfferingAmount,
      ifo.pricePerToken,
      ifo.startTime,
      ifo.endTime,
      ifo.claimTime
    );
    
    const receipt = await tx.wait();
    
    // Extract IFO address from event
    const event = receipt.logs.find((log: any) => 
      log.eventName === 'IFOCreated'
    );
    
    return event?.args?.ifo || '';
  }
  
  /**
   * Get all IFO addresses
   */
  async getAllIFOs(): Promise<string[]> {
    return this.contract.getAllIFOs();
  }
  
  /**
   * Get IFO details
   */
  async getIFO(ifoAddress: string): Promise<IFO> {
    const data = await this.contract.getIFO(ifoAddress);
    const status = this.getIFOStatus(data);
    
    return {
      id: ifoAddress,
      name: '', // Would need to fetch from metadata
      description: '',
      tokenAddress: data.token,
      tokenSymbol: '',
      tokenName: '',
      tokenDecimals: 18,
      tokenLogo: '',
      offeringToken: data.offeringToken,
      totalOfferingAmount: data.totalOfferingAmount.toString(),
      pricePerToken: data.pricePerToken.toString(),
      minBuyAmount: '0',
      maxBuyAmount: '0',
      startTime: Number(data.startTime),
      endTime: Number(data.endTime),
      claimTime: Number(data.claimTime),
      status,
      totalRaised: data.totalRaised.toString(),
      totalParticipants: Number(data.totalParticipants),
      hardCap: data.totalOfferingAmount.toString(),
      softCap: '0',
    };
  }
  
  /**
   * Get user's participation in IFO
   */
  async getUserParticipation(ifoAddress: string, userAddress: string): Promise<UserParticipation> {
    const data = await this.contract.getUserInfo(ifoAddress, userAddress);
    
    return {
      ifoId: ifoAddress,
      userAddress,
      poolId: 0,
      amountDeposited: data.amount.toString(),
      amountClaimed: data.claimed.toString(),
      hasClaimed: data.hasClaimed,
      timestamp: Date.now(),
    };
  }
  
  // ========================================================================
  // User Actions
  // ========================================================================
  
  /**
   * Deposit funds to IFO
   */
  async deposit(ifoAddress: string, amount: string): Promise<ethers.TransactionResponse> {
    if (!this.signer) {
      throw new Error('Signer required for deposit');
    }
    
    // Approve tokens first
    const token = new ethers.Contract(
      await this.getOfferingToken(ifoAddress),
      TOKEN_ABI,
      this.signer
    );
    
    const signerAddress = await this.signer.getAddress();
    const allowance = await token.allowance(signerAddress, ifoAddress);
    
    if (allowance < amount) {
      const approveTx = await token.approve(ifoAddress, ethers.MaxUint256);
      await approveTx.wait();
    }
    
    // Deposit
    return this.contract.deposit(ifoAddress, amount);
  }
  
  /**
   * Claim tokens from IFO
   */
  async claim(ifoAddress: string): Promise<ethers.TransactionResponse> {
    if (!this.signer) {
      throw new Error('Signer required for claiming');
    }
    
    return this.contract.claim(ifoAddress);
  }
  
  /**
   * Get pending reward to claim
   */
  async getPendingReward(ifoAddress: string, userAddress: string): Promise<string> {
    return this.contract.pendingReward(ifoAddress, userAddress);
  }
  
  // ========================================================================
  // Helpers
  // ========================================================================
  
  private async getOfferingToken(ifoAddress: string): Promise<string> {
    const data = await this.contract.getIFO(ifoAddress);
    return data.offeringToken;
  }
  
  private getIFOStatus(data: any): IFOStatus {
    const now = Math.floor(Date.now() / 1000);
    
    if (now < Number(data.startTime)) {
      return IFOStatus.PENDING;
    }
    if (now >= Number(data.startTime) && now < Number(data.endTime)) {
      return IFOStatus.ACTIVE;
    }
    if (now >= Number(data.endTime) && now < Number(data.claimTime)) {
      return IFOStatus.ENDED;
    }
    if (now >= Number(data.claimTime)) {
      return IFOStatus.CLAIMABLE;
    }
    
    return IFOStatus.PENDING;
  }
  
  // ========================================================================
  // Calculations
  // ========================================================================
  
  /**
   * Calculate tokens to receive based on deposit
   */
  calculateTokensReceived(depositAmount: string, pricePerToken: string): string {
    const deposit = BigInt(depositAmount);
    const price = BigInt(pricePerToken);
    
    // tokens = deposit / price * 10^decimals
    const decimals = BigInt(1e18);
    return ((deposit * decimals) / price).toString();
  }
  
  /**
   * Calculate required deposit for desired tokens
   */
  calculateRequiredDeposit(tokensWanted: string, pricePerToken: string): string {
    const tokens = BigInt(tokensWanted);
    const price = BigInt(pricePerToken);
    
    // deposit = tokens * price / 10^decimals
    const decimals = BigInt(1e18);
    return ((tokens * price) / decimals).toString();
  }
}

// ============================================================================
// Factory
// ============================================================================

export function createLaunchpadClient(
  provider: ethers.Provider,
  config: LaunchpadConfig,
  signer?: ethers.Signer
): LaunchpadClient {
  return new LaunchpadClient(provider, config, signer);
}

// ============================================================================
// Utility Functions
// ============================================================================

export function formatIFODate(timestamp: number): string {
  return new Date(timestamp * 1000).toLocaleString();
}

export function getIFOStatusLabel(status: IFOStatus): string {
  const labels: Record<IFOStatus, string> = {
    [IFOStatus.PENDING]: 'Upcoming',
    [IFOStatus.ACTIVE]: 'Live',
    [IFOStatus.ENDED]: 'Ended',
    [IFOStatus.CLAIMABLE]: 'Claimable',
    [IFOStatus.CANCELLED]: 'Cancelled',
  };
  
  return labels[status] || 'Unknown';
}

export function calculateAllocationPercentage(
  userDeposit: string,
  totalRaised: string,
  poolAllocation: string
): string {
  if (totalRaised === '0' || poolAllocation === '0') {
    return '0';
  }
  
  const user = BigInt(userDeposit);
  const total = BigInt(totalRaised);
  const allocation = BigInt(poolAllocation);
  
  // percentage = (user / min(total, allocation)) * 100
  const denominator = total < allocation ? total : allocation;
  const percentage = (user * BigInt(1000000)) / denominator;
  
  return (Number(percentage) / 10000).toFixed(2);
}
