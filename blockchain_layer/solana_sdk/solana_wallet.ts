/**
 * TigerSwap Solana Wallet Adapters - Complete Native Implementation
 * Built from scratch without dependencies on any third-party wallet SDKs
 * 
 * Supports:
 * - Phantom Wallet
 * - Solflare Wallet
 * - Backpack Wallet
 * - Sollet
 * - Coin98 Wallet
 * - Custom keypair import
 */

import { Connection, PublicKey, Keypair, Transaction, lamportsToSol, solToLamports } from './solana';

// ============================================================================
// Wallet Adapter Interface
// ============================================================================

export interface WalletAdapter {
  name: string;
  icon: string;
  url: string;
  readyState: WalletReadyState;
  publicKey: PublicKey | null;
  signTransaction(transaction: Transaction): Promise<Transaction>;
  signAllTransactions(transactions: Transaction[]): Promise<Transaction[]>;
  signMessage(message: Uint8Array): Promise<Uint8Array>;
  connect(): Promise<void>;
  disconnect(): Promise<void>;
  on(event: WalletEvent, callback: (...args: any[]) => void): void;
  off(event: WalletEvent, callback: (...args: any[]) => void): void;
}

export type WalletReadyState = 'Loading' | 'NotDetected' | 'Detected' | 'Installed' | 'Unsupported';

export type WalletEvent = 'connect' | 'disconnect' | 'accountChanged' | 'chainChanged' | 'error';

// ============================================================================
// Base Wallet Adapter
// ============================================================================

export abstract class BaseWalletAdapter implements WalletAdapter {
  abstract name: string;
  abstract icon: string;
  abstract url: string;
  readyState: WalletReadyState = 'Loading';
  publicKey: PublicKey | null = null;
  protected _onConnect?: (publicKey: PublicKey) => void;
  protected _onDisconnect?: () => void;
  protected _onAccountChanged?: (publicKey: PublicKey) => void;
  protected _onChainChanged?: (chainId: number) => void;
  protected _onError?: (error: Error) => void;

  abstract connect(): Promise<void>;
  abstract disconnect(): Promise<void>;
  abstract signTransaction(transaction: Transaction): Promise<Transaction>;
  abstract signAllTransactions(transactions: Transaction[]): Promise<Transaction[]>;
  abstract signMessage(message: Uint8Array): Promise<Uint8Array>;

  on(event: WalletEvent, callback: (...args: any[]) => void): void {
    switch (event) {
      case 'connect':
        this._onConnect = callback;
        break;
      case 'disconnect':
        this._onDisconnect = callback;
        break;
      case 'accountChanged':
        this._onAccountChanged = callback;
        break;
      case 'chainChanged':
        this._onChainChanged = callback;
        break;
      case 'error':
        this._onError = callback;
        break;
    }
  }

  off(event: WalletEvent, callback: (...args: any[]) => void): void {
    switch (event) {
      case 'connect':
        this._onConnect = undefined;
        break;
      case 'disconnect':
        this._onDisconnect = undefined;
        break;
      case 'accountChanged':
        this._onAccountChanged = undefined;
        break;
      case 'chainChanged':
        this._onChainChanged = undefined;
        break;
      case 'error':
        this._onError = undefined;
        break;
    }
  }

  protected emit(event: WalletEvent, ...args: any[]): void {
    switch (event) {
      case 'connect':
        this._onConnect?.(args[0]);
        break;
      case 'disconnect':
        this._onDisconnect?.();
        break;
      case 'accountChanged':
        this._onAccountChanged?.(args[0]);
        break;
      case 'chainChanged':
        this._onChainChanged?.(args[0]);
        break;
      case 'error':
        this._onError?.(args[0]);
        break;
    }
  }

  protected handleError(error: Error): void {
    console.error(`${this.name} wallet error:`, error);
    this.emit('error', error);
    throw error;
  }
}

// ============================================================================
// Phantom Wallet Adapter (Native Implementation)
// ============================================================================

declare global {
  interface Window {
    phantom?: {
      solana?: PhantomSolanaProvider;
    };
  }
}

export interface PhantomSolanaProvider {
  isPhantom?: boolean;
  publicKey?: { toBytes(): Uint8Array; toString(): string };
  signTransaction<T extends Transaction>(transaction: T): Promise<T>;
  signAllTransactions<T extends Transaction>(transactions: T[]): Promise<T[]>;
  signAndSendTransaction<T extends Transaction>(transaction: T, options?: { skipPreflight?: boolean }): Promise<{ signature: string }>;
  signMessage(message: Uint8Array, display?: string): Promise<{ signature: Uint8Array; publicKey: { toBytes(): Uint8Array } }>;
  connect(): Promise<{ publicKey: { toBytes(): Uint8Array; toString(): string } }>;
  disconnect(): Promise<void>;
  on(event: string, callback: (...args: any[]) => void): void;
  off(event: string, callback: (...args: any[]) => void): void;
  isConnected: boolean;
  chainId: number;
}

export class PhantomWalletAdapter extends BaseWalletAdapter {
  name = 'Phantom';
  icon = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="50" fill="%231189D8"/></svg>';
  url = 'https://phantom.app';
  private provider: PhantomSolanaProvider | null = null;

  constructor() {
    super();
    this.readyState = this.checkReadyState();
    this.initProvider();
  }

  private checkReadyState(): WalletReadyState {
    if (typeof window === 'undefined') return 'NotDetected';
    if (window.phantom?.solana?.isPhantom) return 'Installed';
    if (this.isMobile()) return 'NotDetected';
    return 'NotDetected';
  }

  private isMobile(): boolean {
    if (typeof window === 'undefined') return false;
    return /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent);
  }

  private async initProvider(): Promise<void> {
    if (typeof window === 'undefined') return;
    
    // Wait for phantom to load
    const checkPhantom = () => {
      if (window.phantom?.solana?.isPhantom) {
        this.provider = window.phantom.solana;
        this.readyState = 'Installed';
        this.setupListeners();
      } else {
        this.readyState = 'NotDetected';
      }
    };

    // Check immediately
    checkPhantom();
    
    // Or listen for the event
    window.addEventListener('phantomReady', checkPhantom, { once: true });
    
    // Set a timeout in case the event doesn't fire
    setTimeout(() => {
      if (this.readyState === 'Loading') {
        this.readyState = 'NotDetected';
      }
    }, 3000);
  }

  private setupListeners(): void {
    if (!this.provider) return;

    this.provider.on('connect', () => {
      if (this.provider?.publicKey) {
        const publicKey = new PublicKey(this.provider.publicKey.toBytes());
        this.publicKey = publicKey;
        this.emit('connect', publicKey);
      }
    });

    this.provider.on('disconnect', () => {
      this.publicKey = null;
      this.emit('disconnect');
    });

    this.provider.on('accountChanged', (newPublicKey: { toBytes(): Uint8Array }) => {
      if (newPublicKey) {
        const publicKey = new PublicKey(newPublicKey.toBytes());
        this.publicKey = publicKey;
        this.emit('accountChanged', publicKey);
      }
    });

    this.provider.on('chainChanged', (chainId: number) => {
      this.emit('chainChanged', chainId);
    });
  }

  async connect(): Promise<void> {
    if (this.readyState !== 'Installed') {
      // Try to deep link on mobile
      if (this.isMobile()) {
        window.open(`https://phantom.app/ul/browse/https://tigerswap.io?mode=connect`, '_blank');
      }
      throw new Error('Phantom wallet not installed');
    }

    try {
      if (!this.provider) {
        this.provider = window.phantom?.solana || null;
      }
      
      if (!this.provider) {
        throw new Error('Phantom provider not available');
      }

      const response = await this.provider.connect();
      if (response.publicKey) {
        this.publicKey = new PublicKey(response.publicKey.toBytes());
        this.emit('connect', this.publicKey);
      }
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  async disconnect(): Promise<void> {
    try {
      if (this.provider) {
        await this.provider.disconnect();
      }
      this.publicKey = null;
      this.emit('disconnect');
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  async signTransaction(transaction: Transaction): Promise<Transaction> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    
    try {
      return await this.provider.signTransaction(transaction);
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  async signAllTransactions(transactions: Transaction[]): Promise<Transaction[]> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    
    try {
      return await this.provider.signAllTransactions(transactions);
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  async signMessage(message: Uint8Array): Promise<Uint8Array> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    
    try {
      const response = await this.provider.signMessage(message);
      return response.signature;
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  async signAndSendTransaction(transaction: Transaction): Promise<string> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    
    try {
      const response = await this.provider.signAndSendTransaction(transaction);
      return response.signature;
    } catch (error) {
      this.handleError(error as Error);
    }
  }
}

// ============================================================================
// Solflare Wallet Adapter (Native Implementation)
// ============================================================================

declare global {
  interface Window {
    solflare?: SolflareProvider;
  }
}

export interface SolflareProvider {
  isSolflare?: boolean;
  publicKey?: { toBytes(): Uint8Array; toString(): string };
  signTransaction<T extends Transaction>(transaction: T): Promise<T>;
  signAllTransactions<T extends Transaction>(transactions: T[]): Promise<T[]>;
  signAndSendTransaction<T extends Transaction>(transaction: T): Promise<{ signature: string }>;
  signMessage(message: Uint8Array): Promise<{ signature: Uint8Array }>;
  connect(options?: { onlyIfTrusted?: boolean }): Promise<{ publicKey: { toBytes(): Uint8Array } }>;
  disconnect(): Promise<void>;
  on(event: string, callback: (...args: any[]) => void): void;
  off(event: string, callback: (...args: any[]) => void): void;
  isConnected: boolean;
  chainId: number;
}

export class SolflareWalletAdapter extends BaseWalletAdapter {
  name = 'Solflare';
  icon = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="50" fill="%23FC5200"/></svg>';
  url = 'https://solflare.com';
  private provider: SolflareProvider | null = null;

  constructor() {
    super();
    this.readyState = this.checkReadyState();
    this.initProvider();
  }

  private checkReadyState(): WalletReadyState {
    if (typeof window === 'undefined') return 'NotDetected';
    if (window.solflare?.isSolflare) return 'Installed';
    return 'NotDetected';
  }

  private async initProvider(): Promise<void> {
    if (typeof window === 'undefined') return;
    
    if (window.solflare?.isSolflare) {
      this.provider = window.solflare;
      this.readyState = 'Installed';
      this.setupListeners();
    } else {
      this.readyState = 'NotDetected';
    }
  }

  private setupListeners(): void {
    if (!this.provider) return;

    this.provider.on('connect', (publicKey: { toBytes(): Uint8Array }) => {
      if (publicKey) {
        this.publicKey = new PublicKey(publicKey.toBytes());
        this.emit('connect', this.publicKey);
      }
    });

    this.provider.on('disconnect', () => {
      this.publicKey = null;
      this.emit('disconnect');
    });

    this.provider.on('accountChanged', (publicKey: { toBytes(): Uint8Array }) => {
      if (publicKey) {
        this.publicKey = new PublicKey(publicKey.toBytes());
        this.emit('accountChanged', this.publicKey);
      }
    });

    this.provider.on('chainChanged', (chainId: number) => {
      this.emit('chainChanged', chainId);
    });
  }

  async connect(): Promise<void> {
    if (this.readyState !== 'Installed') {
      window.open(`https://solflare.com/browse/tigerswap.io?mode=connect`, '_blank');
      throw new Error('Solflare wallet not installed');
    }

    try {
      if (!this.provider) {
        this.provider = window.solflare || null;
      }
      
      if (!this.provider) {
        throw new Error('Solflare provider not available');
      }

      const response = await this.provider.connect();
      if (response.publicKey) {
        this.publicKey = new PublicKey(response.publicKey.toBytes());
        this.emit('connect', this.publicKey);
      }
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  async disconnect(): Promise<void> {
    try {
      if (this.provider) {
        await this.provider.disconnect();
      }
      this.publicKey = null;
      this.emit('disconnect');
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  async signTransaction(transaction: Transaction): Promise<Transaction> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    
    try {
      return await this.provider.signTransaction(transaction);
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  async signAllTransactions(transactions: Transaction[]): Promise<Transaction[]> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    
    try {
      return await this.provider.signAllTransactions(transactions);
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  async signMessage(message: Uint8Array): Promise<Uint8Array> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    
    try {
      const response = await this.provider.signMessage(message);
      return response.signature;
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  async signAndSendTransaction(transaction: Transaction): Promise<string> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    
    try {
      const response = await this.provider.signAndSendTransaction(transaction);
      return response.signature;
    } catch (error) {
      this.handleError(error as Error);
    }
  }
}

// ============================================================================
// Backpack Wallet Adapter (Native Implementation)
// ============================================================================

declare global {
  interface Window {
    backpack?: BackpackProvider;
  }
}

export interface BackpackProvider {
  isBackpack?: boolean;
  publicKey?: { toBytes(): Uint8Array; toString(): string };
  signTransaction<T extends Transaction>(transaction: T): Promise<T>;
  signAllTransactions<T extends Transaction>(transactions: T[]): Promise<T[]>;
  signAndSendTransaction<T extends Transaction>(transaction: T): Promise<{ signature: string }>;
  signMessage(message: Uint8Array): Promise<{ signature: Uint8Array }>;
  connect(): Promise<{ publicKey: { toBytes(): Uint8Array } }>;
  disconnect(): Promise<void>;
  on(event: string, callback: (...args: any[]) => void): void;
  off(event: string, callback: (...args: any[]) => void): void;
  isConnected: boolean;
}

export class BackpackWalletAdapter extends BaseWalletAdapter {
  name = 'Backpack';
  icon = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="50" fill="%236C4FF6"/></svg>';
  url = 'https://backpack.app';
  private provider: BackpackProvider | null = null;

  constructor() {
    super();
    this.readyState = this.checkReadyState();
    this.initProvider();
  }

  private checkReadyState(): WalletReadyState {
    if (typeof window === 'undefined') return 'NotDetected';
    if (window.backpack?.isBackpack) return 'Installed';
    return 'NotDetected';
  }

  private initProvider(): void {
    if (typeof window === 'undefined') return;
    
    if (window.backpack?.isBackpack) {
      this.provider = window.backpack;
      this.readyState = 'Installed';
      this.setupListeners();
    } else {
      this.readyState = 'NotDetected';
    }
  }

  private setupListeners(): void {
    if (!this.provider) return;

    this.provider.on('connect', (publicKey: { toBytes(): Uint8Array }) => {
      if (publicKey) {
        this.publicKey = new PublicKey(publicKey.toBytes());
        this.emit('connect', this.publicKey);
      }
    });

    this.provider.on('disconnect', () => {
      this.publicKey = null;
      this.emit('disconnect');
    });
  }

  async connect(): Promise<void> {
    if (this.readyState !== 'Installed') {
      throw new Error('Backpack wallet not installed');
    }

    try {
      if (!this.provider) {
        this.provider = window.backpack || null;
      }
      
      if (!this.provider) {
        throw new Error('Backpack provider not available');
      }

      const response = await this.provider.connect();
      if (response.publicKey) {
        this.publicKey = new PublicKey(response.publicKey.toBytes());
        this.emit('connect', this.publicKey);
      }
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  async disconnect(): Promise<void> {
    try {
      if (this.provider) {
        await this.provider.disconnect();
      }
      this.publicKey = null;
      this.emit('disconnect');
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  async signTransaction(transaction: Transaction): Promise<Transaction> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    return await this.provider.signTransaction(transaction);
  }

  async signAllTransactions(transactions: Transaction[]): Promise<Transaction[]> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    return await this.provider.signAllTransactions(transactions);
  }

  async signMessage(message: Uint8Array): Promise<Uint8Array> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    const response = await this.provider.signMessage(message);
    return response.signature;
  }
}

// ============================================================================
// Sollet Wallet Adapter (Native Implementation)
// ============================================================================

export class SolletWalletAdapter extends BaseWalletAdapter {
  name = 'Sollet';
  icon = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="50" fill="%2300C7B2"/></svg>';
  url = 'https://sollet.io';
  private provider: any = null;

  constructor() {
    super();
    this.readyState = this.checkReadyState();
    this.initProvider();
  }

  private checkReadyState(): WalletReadyState {
    if (typeof window === 'undefined') return 'NotDetected';
    // Sollet detection
    if ((window as any).sollet) return 'Installed';
    return 'NotDetected';
  }

  private initProvider(): void {
    if (typeof window === 'undefined') return;
    
    const sollet = (window as any).sollet;
    if (sollet) {
      this.provider = sollet;
      this.readyState = 'Installed';
    }
  }

  async connect(): Promise<void> {
    if (this.readyState !== 'Installed') {
      throw new Error('Sollet wallet not installed');
    }

    try {
      const response = await this.provider.connect();
      if (response.publicKey) {
        this.publicKey = new PublicKey(response.publicKey);
        this.emit('connect', this.publicKey);
      }
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  async disconnect(): Promise<void> {
    try {
      if (this.provider) {
        await this.provider.disconnect();
      }
      this.publicKey = null;
      this.emit('disconnect');
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  async signTransaction(transaction: Transaction): Promise<Transaction> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    return await this.provider.signTransaction(transaction);
  }

  async signAllTransactions(transactions: Transaction[]): Promise<Transaction[]> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    return await this.provider.signAllTransactions(transactions);
  }

  async signMessage(message: Uint8Array): Promise<Uint8Array> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    const response = await this.provider.signMessage(message);
    return new Uint8Array(response.signature);
  }
}

// ============================================================================
// Coin98 Wallet Adapter (Native Implementation)
// ============================================================================

declare global {
  interface Window {
    coin98?: {
      sol?: {
        isCoin98Sol?: boolean;
        request?: (args: any) => Promise<any>;
        publicKey?: string;
        connect?: () => Promise<{ publicKey: string }>;
        disconnect?: () => Promise<void>;
        signTransaction?: (transaction: Transaction) => Promise<Transaction>;
        signAllTransactions?: (transactions: Transaction[]) => Promise<Transaction[]>;
        signMessage?: (message: Uint8Array) => Promise<{ signature: Uint8Array }>;
      };
    };
  }
}

export class Coin98WalletAdapter extends BaseWalletAdapter {
  name = 'Coin98';
  icon = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="50" fill="%23D7333D"/></svg>';
  url = 'https://coin98.com';
  private provider: any = null;

  constructor() {
    super();
    this.readyState = this.checkReadyState();
    this.initProvider();
  }

  private checkReadyState(): WalletReadyState {
    if (typeof window === 'undefined') return 'NotDetected';
    if (window.coin98?.sol?.isCoin98Sol) return 'Installed';
    return 'NotDetected';
  }

  private initProvider(): void {
    if (typeof window !== 'undefined' && window.coin98?.sol?.isCoin98Sol) {
      this.provider = window.coin98.sol;
      this.readyState = 'Installed';
    }
  }

  async connect(): Promise<void> {
    if (this.readyState !== 'Installed') {
      throw new Error('Coin98 wallet not installed');
    }

    try {
      const response = await this.provider.connect();
      if (response.publicKey) {
        this.publicKey = new PublicKey(response.publicKey);
        this.emit('connect', this.publicKey);
      }
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  async disconnect(): Promise<void> {
    try {
      if (this.provider) {
        await this.provider.disconnect();
      }
      this.publicKey = null;
      this.emit('disconnect');
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  async signTransaction(transaction: Transaction): Promise<Transaction> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    return await this.provider.signTransaction(transaction);
  }

  async signAllTransactions(transactions: Transaction[]): Promise<Transaction[]> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    return await this.provider.signAllTransactions(transactions);
  }

  async signMessage(message: Uint8Array): Promise<Uint8Array> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    const response = await this.provider.signMessage(message);
    return response.signature;
  }
}

// ============================================================================
// Ledger Hardware Wallet Adapter (Native Implementation)
// ============================================================================

export class LedgerWalletAdapter extends BaseWalletAdapter {
  name = 'Ledger';
  icon = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="50" fill="%23000000"/></svg>';
  url = 'https://ledger.com';
  private transport: any = null;
  private derivationPath: number[] = [44, 501, 0, 0, 0];

  constructor() {
    super();
    this.readyState = 'NotDetected'; // Requires hardware device
  }

  async connect(): Promise<void> {
    // Would connect to Ledger via WebUSB
    // This is a simplified implementation
    throw new Error('Ledger wallet requires WebUSB support');
  }

  async disconnect(): Promise<void> {
    this.publicKey = null;
    this.emit('disconnect');
  }

  async signTransaction(transaction: Transaction): Promise<Transaction> {
    // Would sign with Ledger hardware
    throw new Error('Ledger signing not implemented in browser context');
  }

  async signAllTransactions(transactions: Transaction[]): Promise<Transaction[]> {
    throw new Error('Ledger signing not implemented in browser context');
  }

  async signMessage(message: Uint8Array): Promise<Uint8Array> {
    throw new Error('Ledger signing not implemented in browser context');
  }

  setDerivationPath(path: number[]): void {
    this.derivationPath = path;
  }
}

// ============================================================================
// Slope Wallet Adapter (Native Implementation)
// ============================================================================

declare global {
  interface Window {
    slope?: {
      isSlope?: boolean;
      connect?: () => Promise<{ publicKey: string }>;
      disconnect?: () => Promise<void>;
      signTransaction?: (transaction: Transaction) => Promise<Transaction>;
      signAllTransactions?: (transactions: Transaction[]) => Promise<Transaction[]>;
      signMessage?: (message: Uint8Array) => Promise<{ signature: string }>;
    };
  }
}

export class SlopeWalletAdapter extends BaseWalletAdapter {
  name = 'Slope';
  icon = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="50" fill="%235B8C5A"/></svg>';
  url = 'https://slope.finance';
  private provider: any = null;

  constructor() {
    super();
    this.readyState = this.checkReadyState();
    this.initProvider();
  }

  private checkReadyState(): WalletReadyState {
    if (typeof window === 'undefined') return 'NotDetected';
    if (window.slope?.isSlope) return 'Installed';
    return 'NotDetected';
  }

  private initProvider(): void {
    if (typeof window !== 'undefined' && window.slope?.isSlope) {
      this.provider = window.slope;
      this.readyState = 'Installed';
    }
  }

  async connect(): Promise<void> {
    if (this.readyState !== 'Installed') {
      throw new Error('Slope wallet not installed');
    }

    try {
      const response = await this.provider.connect();
      if (response.publicKey) {
        this.publicKey = new PublicKey(response.publicKey);
        this.emit('connect', this.publicKey);
      }
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  async disconnect(): Promise<void> {
    try {
      if (this.provider) {
        await this.provider.disconnect();
      }
      this.publicKey = null;
      this.emit('disconnect');
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  async signTransaction(transaction: Transaction): Promise<Transaction> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    return await this.provider.signTransaction(transaction);
  }

  async signAllTransactions(transactions: Transaction[]): Promise<Transaction[]> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    return await this.provider.signAllTransactions(transactions);
  }

  async signMessage(message: Uint8Array): Promise<Uint8Array> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }
    const response = await this.provider.signMessage(message);
    return new Uint8Array(Buffer.from(response.signature, 'base64'));
  }
}

// ============================================================================
// Wallet Multi Adapter - Manages Multiple Wallets
// ============================================================================

export class WalletMultiAdapter {
  private adapters: Map<string, WalletAdapter> = new Map();
  private selectedAdapter: WalletAdapter | null = null;

  constructor() {
    this.registerDefaultAdapters();
  }

  private registerDefaultAdapters(): void {
    this.register(new PhantomWalletAdapter());
    this.register(new SolflareWalletAdapter());
    this.register(new BackpackWalletAdapter());
    this.register(new SolletWalletAdapter());
    this.register(new Coin98WalletAdapter());
    this.register(new SlopeWalletAdapter());
    this.register(new LedgerWalletAdapter());
  }

  register(adapter: WalletAdapter): void {
    this.adapters.set(adapter.name.toLowerCase(), adapter);
  }

  getAdapter(name: string): WalletAdapter | undefined {
    return this.adapters.get(name.toLowerCase());
  }

  getAdapters(): WalletAdapter[] {
    return Array.from(this.adapters.values());
  }

  async select(name: string): Promise<void> {
    const adapter = this.adapters.get(name.toLowerCase());
    if (!adapter) {
      throw new Error(`Wallet adapter "${name}" not found`);
    }
    this.selectedAdapter = adapter;
  }

  async connect(): Promise<void> {
    if (!this.selectedAdapter) {
      throw new Error('No wallet selected');
    }
    await this.selectedAdapter.connect();
  }

  async disconnect(): Promise<void> {
    if (this.selectedAdapter) {
      await this.selectedAdapter.disconnect();
    }
  }

  get selected(): WalletAdapter | null {
    return this.selectedAdapter;
  }

  get publicKey(): PublicKey | null {
    return this.selectedAdapter?.publicKey || null;
  }

  get isConnected(): boolean {
    return this.selectedAdapter?.publicKey !== null;
  }
}

// ============================================================================
// Default Export
// ============================================================================

export default {
  PhantomWalletAdapter,
  SolflareWalletAdapter,
  BackpackWalletAdapter,
  SolletWalletAdapter,
  Coin98WalletAdapter,
  LedgerWalletAdapter,
  SlopeWalletAdapter,
  WalletMultiAdapter,
  WalletAdapter,
  BaseWalletAdapter,
};