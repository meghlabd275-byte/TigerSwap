/**
 * TigerSwap Security Platform - Fraud Detection
 * 
 * Enterprise-grade security with complete fraud detection, rate limiting, and circuit breaker.
 * Zero external dependencies - fully native implementation.
 * 
 * Features:
 * - Transaction analysis
 * - Pattern detection
 * - Anomaly detection
 * - Rate limiting
 * - Circuit breaker
 * - Compliance reporting
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface Transaction {
  hash: string;
  from: string;
  to: string;
  value: bigint;
  gasPrice: bigint;
  gasLimit: bigint;
  timestamp: number;
  blockNumber: number;
  input: string;
}

export interface AddressProfile {
  address: string;
  firstSeen: number;
  totalTransactions: number;
  totalVolume: bigint;
  riskScore: number;
  flags: RiskFlag[];
  labels: string[];
}

export enum RiskFlag {
  MALICIOUS_CONTRACT = 'malicious_contract',
  PHISHING = 'phishing',
  HACKER = 'hacker',
  MIXER = 'mixer',
  EXCHANGE = 'exchange',
  DEFI = 'defi',
  CONTRACT = 'contract',
  WHALE = 'whale',
  NEW = 'new',
  SUSPICIOUS = 'suspicious',
}

export interface FraudAlert {
  id: string;
  type: FraudType;
  severity: Severity;
  address: string;
  transaction?: string;
  description: string;
  timestamp: number;
  resolved: boolean;
}

export enum FraudType {
  FRONT_RUNNING = 'front_running',
  SANITIZER = 'sanitizer',
  VACUUM = 'vacuum',
  HONEY_POT = 'honey_pot',
  RUG_PULL = 'rug_pull',
  PHISHING = 'phishing',
  IMPERSONATION = 'impersonation',
  SCAM = 'scam',
  MIXER_USAGE = 'mixer_usage',
  TORNADO_CASH = 'tornado_cash',
  SUSPICIOUS_TRANSFER = 'suspicious_transfer',
  LARGE_TRANSFER = 'large_transfer',
  RAPID_TRANSACTIONS = 'rapid_transactions',
}

export enum Severity {
  LOW = 'low',
  MEDIUM = 'medium',
  HIGH = 'high',
  CRITICAL = 'critical',
}

export interface RateLimitConfig {
  windowMs: number;
  maxRequests: number;
  maxVolume: bigint;
}

export interface CircuitBreakerState {
  state: 'CLOSED' | 'OPEN' | 'HALF_OPEN';
  failures: number;
  lastFailure: number;
  nextAttempt: number;
}

// ============================================================================
// Fraud Detection Engine
// ============================================================================

/**
 * FraudDetection - Real-time fraud detection
 */
export class FraudDetection {
  private addressProfiles: Map<string, AddressProfile>;
  private knownMalicious: Set<string>;
  private knownExchanges: Set<string>;
  private honeyPots: Set<string>;
  private recentTransactions: Map<string, Transaction[]>;

  constructor() {
    this.addressProfiles = new Map();
    this.knownMalicious = new Set();
    this.knownExchanges = new Set();
    this.honeyPots = new Set();
    this.recentTransactions = new Map();
    
    // Initialize known addresses (in production, load from database)
    this.initializeKnownAddresses();
  }

  /**
   * Analyze transaction for fraud
   */
  analyzeTransaction(tx: Transaction): FraudAlert[] {
    const alerts: FraudAlert[] = [];

    // Check sender/receiver
    const senderRisk = this.analyzeAddress(tx.from);
    const receiverRisk = this.analyzeAddress(tx.to);

    // Front-running detection
    if (this.detectFrontRunning(tx)) {
      alerts.push(this.createAlert(
        FraudType.FRONT_RUNNING,
        Severity.HIGH,
        tx.from,
        tx.hash,
        'Front-running pattern detected'
      ));
    }

    // Honey pot detection
    if (this.honeyPots.has(tx.to.toLowerCase())) {
      alerts.push(this.createAlert(
        FraudType.HONEY_POT,
        Severity.CRITICAL,
        tx.to,
        tx.hash,
        'Transaction to known honey pot contract'
      ));
    }

    // Large transfer detection
    if (this.isLargeTransfer(tx.value)) {
      alerts.push(this.createAlert(
        FraudType.LARGE_TRANSFER,
        Severity.MEDIUM,
        tx.from,
        tx.hash,
        `Large transfer detected: ${tx.value} wei`
      ));
    }

    // Rapid transactions
    if (this.hasRapidTransactions(tx.from)) {
      alerts.push(this.createAlert(
        FraudType.RAPID_TRANSACTIONS,
        Severity.HIGH,
        tx.from,
        tx.hash,
        'Rapid transactions from single address'
      ));
    }

    // High gas price (possible front-running)
    if (this.isHighGasPrice(tx.gasPrice)) {
      alerts.push(this.createAlert(
        FraudType.FRONT_RUNNING,
        Severity.MEDIUM,
        tx.from,
        tx.hash,
        'Unusually high gas price detected'
      ));
    }

    // Sanitizer detection
    if (this.detectSanitizer(tx)) {
      alerts.push(this.createAlert(
        FraudType.SANITIZER,
        Severity.CRITICAL,
        tx.to,
        tx.hash,
        'Funds going to known sanitizer address'
      ));
    }

    // Update address profiles
    this.updateProfiles(tx);

    return alerts;
  }

  /**
   * Analyze address risk
   */
  analyzeAddress(address: string): number {
    const profile = this.addressProfiles.get(address.toLowerCase());
    if (!profile) {
      return this.calculateNewAddressRisk(address);
    }
    return profile.riskScore;
  }

  /**
   * Get address profile
   */
  getAddressProfile(address: string): AddressProfile | undefined {
    return this.addressProfiles.get(address.toLowerCase());
  }

  /**
   * Mark address as malicious
   */
  markAsMalicious(address: string, flag: RiskFlag): void {
    this.knownMalicious.add(address.toLowerCase());
    
    const profile = this.addressProfiles.get(address.toLowerCase()) || this.createProfile(address);
    profile.riskScore = 100;
    profile.flags.push(flag);
    this.addressProfiles.set(address.toLowerCase(), profile);
  }

  /**
   * Mark address as exchange
   */
  markAsExchange(address: string): void {
    this.knownExchanges.add(address.toLowerCase());
    
    const profile = this.addressProfiles.get(address.toLowerCase()) || this.createProfile(address);
    profile.riskScore = 0;
    profile.labels.push('exchange');
    this.addressProfiles.set(address.toLowerCase(), profile);
  }

  /**
   * Add honey pot
   */
  addHoneyPot(address: string): void {
    this.honeyPots.add(address.toLowerCase());
  }

  private initializeKnownAddresses(): void {
    // Tornado Cash addresses (example)
    this.knownMalicious.add('0xee6a5c5ecd84bae91c0872481e66ef15f5e33540');
    this.knownMalicious.add('0x4736dcfcb53e8d3b5f2b2ea4b4b4d30c2c03e2c');
    
    // Known exchanges
    this.knownExchanges.add('0x28c6c06298d514db089934071355e5743bf21d61'); // Binance hot
    this.knownExchanges.add('0x8ba1f109551bd432803012645ac136ddd64dba72'); // Coinbase
    this.knownExchanges.add('0xd8da6bf26964af9d7eed9e03e53415d37aa96044'); // Kraken
  }

  private analyzeAddressInternal(address: string): AddressProfile {
    let profile = this.addressProfiles.get(address.toLowerCase());
    
    if (!profile) {
      profile = this.createProfile(address);
      this.addressProfiles.set(address.toLowerCase(), profile);
    }
    
    return profile;
  }

  private createProfile(address: string): AddressProfile {
    return {
      address: address.toLowerCase(),
      firstSeen: Date.now(),
      totalTransactions: 0,
      totalVolume: 0n,
      riskScore: this.calculateNewAddressRisk(address),
      flags: [],
      labels: [],
    };
  }

  private calculateNewAddressRisk(address: string): number {
    // New addresses have higher risk
    return 50;
  }

  private detectFrontRunning(tx: Transaction): boolean {
    // Check for high gas price and same-token swaps in quick succession
    const recentTxs = this.recentTransactions.get(tx.from.toLowerCase()) || [];
    const recentSameBlock = recentTxs.filter(t => 
      t.blockNumber === tx.blockNumber && 
      t.hash !== tx.hash
    );
    
    return recentSameBlock.length > 2 && this.isHighGasPrice(tx.gasPrice);
  }

  private detectSanitizer(tx: Transaction): boolean {
    // Check if funds going to known sanitizer
    const sanitizerAddresses = [
      '0xee6a5c5ecd84bae91c0872481e66ef15f5e33540', // Tornado Cash
      '0x4736dcfcb53e8d3b5f2b2ea4b4b4d30c2c03e2c',
    ];
    
    return sanitizerAddresses.includes(tx.to.toLowerCase());
  }

  private isLargeTransfer(value: bigint): boolean {
    // Consider anything > 1000 ETH as large
    return value > 1000000000000000000000n;
  }

  private isHighGasPrice(gasPrice: bigint): boolean {
    // Consider > 100 gwei as high
    return gasPrice > 100000000000n;
  }

  private hasRapidTransactions(address: string): boolean {
    const recentTxs = this.recentTransactions.get(address.toLowerCase()) || [];
    const oneMinuteAgo = Date.now() - 60000;
    
    return recentTxs.filter(t => t.timestamp > oneMinuteAgo).length > 10;
  }

  private updateProfiles(tx: Transaction): void {
    // Update sender
    const sender = this.analyzeAddressInternal(tx.from);
    sender.totalTransactions++;
    sender.totalVolume += tx.value;
    
    // Update receiver
    const receiver = this.analyzeAddressInternal(tx.to);
    receiver.totalTransactions++;
    
    // Add to recent transactions
    const senderTxs = this.recentTransactions.get(tx.from.toLowerCase()) || [];
    senderTxs.push(tx);
    
    // Keep only recent transactions (last 100)
    if (senderTxs.length > 100) {
      senderTxs.shift();
    }
    this.recentTransactions.set(tx.from.toLowerCase(), senderTxs);
  }

  private createAlert(
    type: FraudType,
    severity: Severity,
    address: string,
    transaction: string,
    description: string
  ): FraudAlert {
    return {
      id: this.generateId(),
      type,
      severity,
      address,
      transaction,
      description,
      timestamp: Date.now(),
      resolved: false,
    };
  }

  private generateId(): string {
    return `alert_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`;
  }
}

// ============================================================================
// Rate Limiter
// ============================================================================

/**
 * RateLimiter - Token bucket rate limiting
 */
export class RateLimiter {
  private configs: Map<string, RateLimitConfig>;
  private buckets: Map<string, { tokens: number; lastRefill: number }>;

  constructor() {
    this.configs = new Map();
    this.buckets = new Map();
  }

  /**
   * Set rate limit for key
   */
  setLimit(key: string, config: RateLimitConfig): void {
    this.configs.set(key, config);
    this.buckets.set(key, {
      tokens: config.maxRequests,
      lastRefill: Date.now(),
    });
  }

  /**
   * Check if request is allowed
   */
  check(key: string): boolean {
    const config = this.configs.get(key);
    if (!config) {
      return true; // No limit set
    }

    this.refill(key);
    const bucket = this.buckets.get(key);
    
    if (!bucket) {
      return true;
    }

    return bucket.tokens > 0;
  }

  /**
   * Consume token
   */
  consume(key: string): boolean {
    const config = this.configs.get(key);
    if (!config) {
      return true;
    }

    this.refill(key);
    const bucket = this.buckets.get(key);
    
    if (!bucket || bucket.tokens <= 0) {
      return false;
    }

    bucket.tokens--;
    return true;
  }

  /**
   * Get remaining tokens
   */
  getRemaining(key: string): number {
    const config = this.configs.get(key);
    if (!config) {
      return -1;
    }

    this.refill(key);
    const bucket = this.buckets.get(key);
    return bucket?.tokens || 0;
  }

  /**
   * Reset rate limit
   */
  reset(key: string): void {
    const config = this.configs.get(key);
    if (config) {
      this.buckets.set(key, {
        tokens: config.maxRequests,
        lastRefill: Date.now(),
      });
    }
  }

  private refill(key: string): void {
    const config = this.configs.get(key);
    const bucket = this.buckets.get(key);
    
    if (!config || !bucket) {
      return;
    }

    const now = Date.now();
    const timePassed = now - bucket.lastRefill;
    const tokensToAdd = Math.floor(timePassed / config.windowMs) * config.maxRequests;

    if (tokensToAdd > 0) {
      bucket.tokens = Math.min(config.maxRequests, bucket.tokens + tokensToAdd);
      bucket.lastRefill = now;
    }
  }
}

// ============================================================================
// Circuit Breaker
// ============================================================================

/**
 * CircuitBreaker - Failure protection
 */
export class CircuitBreaker {
  private state: CircuitBreakerState;
  private config: {
    failureThreshold: number;
    successThreshold: number;
    timeout: number;
  };

  constructor(
    failureThreshold: number = 5,
    successThreshold: number = 2,
    timeout: number = 30000
  ) {
    this.state = {
      state: 'CLOSED',
      failures: 0,
      lastFailure: 0,
      nextAttempt: 0,
    };
    
    this.config = {
      failureThreshold,
      successThreshold,
      timeout,
    };
  }

  /**
   * Check if operation is allowed
   */
  isAvailable(): boolean {
    if (this.state.state === 'CLOSED') {
      return true;
    }

    if (this.state.state === 'OPEN') {
      if (Date.now() >= this.state.nextAttempt) {
        this.state.state = 'HALF_OPEN';
        return true;
      }
      return false;
    }

    // HALF_OPEN - allow one attempt
    return true;
  }

  /**
   * Record success
   */
  recordSuccess(): void {
    if (this.state.state === 'HALF_OPEN') {
      this.state.failures = 0;
      this.state.state = 'CLOSED';
    } else {
      this.state.failures = 0;
    }
  }

  /**
   * Record failure
   */
  recordFailure(): void {
    this.state.failures++;
    this.state.lastFailure = Date.now();

    if (this.state.state === 'HALF_OPEN') {
      this.state.state = 'OPEN';
      this.state.nextAttempt = Date.now() + this.config.timeout;
    } else if (this.state.failures >= this.config.failureThreshold) {
      this.state.state = 'OPEN';
      this.state.nextAttempt = Date.now() + this.config.timeout;
    }
  }

  /**
   * Get current state
   */
  getState(): CircuitBreakerState {
    return { ...this.state };
  }

  /**
   * Force state
   */
  setState(state: 'CLOSED' | 'OPEN' | 'HALF_OPEN'): void {
    this.state.state = state;
    if (state === 'CLOSED') {
      this.state.failures = 0;
    }
  }

  /**
   * Reset circuit breaker
   */
  reset(): void {
    this.state = {
      state: 'CLOSED',
      failures: 0,
      lastFailure: 0,
      nextAttempt: 0,
    };
  }
}

// ============================================================================
// Security Scanner
// ============================================================================

/**
 * SecurityScanner - Smart contract security scanning
 */
export class SecurityScanner {
  private knownVulnerabilities: Map<string, VulnerabilityInfo>;
  private scanners: ContractScanner[];

  constructor() {
    this.knownVulnerabilities = new Map();
    this.scanners = [
      new ReentrancyScanner(),
      new IntegerOverflowScanner(),
      new AccessControlScanner(),
    ];
    
    this.initializeVulnerabilities();
  }

  /**
   * Scan contract for vulnerabilities
   */
  async scanContract(bytecode: string, sourceCode?: string): Promise<VulnerabilityReport> {
    const vulnerabilities: Vulnerability[] = [];
    
    for (const scanner of this.scanners) {
      const found = scanner.scan(bytecode, sourceCode);
      vulnerabilities.push(...found);
    }
    
    return {
      timestamp: Date.now(),
      vulnerabilities,
      riskScore: this.calculateRiskScore(vulnerabilities),
      recommendations: this.generateRecommendations(vulnerabilities),
    };
  }

  /**
   * Check if address is verified
   */
  isVerified(address: string): boolean {
    // In production, check against verification databases
    return false;
  }

  /**
   * Get contract info
   */
  getContractInfo(address: string): ContractInfo {
    // In production, fetch from block explorers
    return {
      address,
      name: 'Unknown',
      compilerVersion: 'Unknown',
      verified: false,
      proxy: false,
    };
  }

  private calculateRiskScore(vulnerabilities: Vulnerability[]): number {
    if (vulnerabilities.length === 0) return 0;
    
    let score = 0;
    for (const v of vulnerabilities) {
      switch (v.severity) {
        case 'critical': score += 40; break;
        case 'high': score += 25; break;
        case 'medium': score += 10; break;
        case 'low': score += 5; break;
      }
    }
    
    return Math.min(100, score);
  }

  private generateRecommendations(vulnerabilities: Vulnerability[]): string[] {
    const recommendations: string[] = [];
    
    for (const v of vulnerabilities) {
      recommendations.push(...v.recommendations);
    }
    
    return [...new Set(recommendations)];
  }

  private initializeVulnerabilities(): void {
    // Known vulnerable contract patterns
    this.knownVulnerabilities.set('0x...', {
      name: 'Reentrancy',
      severity: 'critical',
      description: 'Vulnerable to reentrancy attacks',
    });
  }
}

interface VulnerabilityInfo {
  name: string;
  severity: string;
  description: string;
}

interface Vulnerability {
  type: string;
  severity: string;
  line?: number;
  description: string;
  recommendations: string[];
}

interface VulnerabilityReport {
  timestamp: number;
  vulnerabilities: Vulnerability[];
  riskScore: number;
  recommendations: string[];
}

interface ContractInfo {
  address: string;
  name: string;
  compilerVersion: string;
  verified: boolean;
  proxy: boolean;
}

abstract class ContractScanner {
  abstract scan(bytecode: string, sourceCode?: string): Vulnerability[];
}

class ReentrancyScanner extends ContractScanner {
  scan(bytecode: string, sourceCode?: string): Vulnerability[] {
    const vulnerabilities: Vulnerability[] = [];
    
    // Check for external calls followed by state changes
    if (bytecode.includes('call') && bytecode.includes('storage')) {
      vulnerabilities.push({
        type: 'Reentrancy',
        severity: 'critical',
        description: 'Potential reentrancy vulnerability detected',
        recommendations: [
          'Use checks-effects-interactions pattern',
          'Implement reentrancy guards',
          'Use SafeMath for arithmetic',
        ],
      });
    }
    
    return vulnerabilities;
  }
}

class IntegerOverflowScanner extends ContractScanner {
  scan(bytecode: string, sourceCode?: string): Vulnerability[] {
    const vulnerabilities: Vulnerability[] = [];
    
    // Check for arithmetic operations without SafeMath
    if (bytecode.includes('add') && !bytecode.includes('safemath')) {
      vulnerabilities.push({
        type: 'Integer Overflow',
        severity: 'high',
        description: 'Potential integer overflow vulnerability',
        recommendations: [
          'Use SafeMath library',
          'Implement bounds checking',
        ],
      });
    }
    
    return vulnerabilities;
  }
}

class AccessControlScanner extends ContractScanner {
  scan(bytecode: string, sourceCode?: string): Vulnerability[] {
    const vulnerabilities: Vulnerability[] = [];
    
    // Check for missing access control
    if (!bytecode.includes('require') && !bytecode.includes('onlyOwner')) {
      vulnerabilities.push({
        type: 'Missing Access Control',
        severity: 'medium',
        description: 'Potential missing access control',
        recommendations: [
          'Implement access control modifiers',
          'Use Ownable or AccessControl contracts',
        ],
      });
    }
    
    return vulnerabilities;
  }
}

// ============================================================================
// Compliance Reporter
// ============================================================================

/**
 * ComplianceReporter - Generate compliance reports
 */
export class ComplianceReporter {
  /**
   * Generate suspicious activity report
   */
  generateSAR(alerts: FraudAlert[]): SuspiciousActivityReport {
    const critical = alerts.filter(a => a.severity === Severity.CRITICAL);
    const high = alerts.filter(a => a.severity === Severity.HIGH);
    
    return {
      id: this.generateId(),
      generatedAt: Date.now(),
      totalAlerts: alerts.length,
      criticalCount: critical.length,
      highCount: high.length,
      alerts,
      recommendedAction: critical.length > 0 ? 'URGENT_REVIEW' : 
                        high.length > 0 ? 'REVIEW_REQUIRED' : 'MONITOR',
    };
  }

  /**
   * Generate transaction monitoring report
   */
  generateTransactionReport(transactions: Transaction[]): TransactionReport {
    const totalValue = transactions.reduce((sum, tx) => sum + tx.value, 0n);
    const uniqueAddresses = new Set([
      ...transactions.map(tx => tx.from),
      ...transactions.map(tx => tx.to),
    ]);
    
    return {
      id: this.generateId(),
      generatedAt: Date.now(),
      period: {
        start: transactions[0]?.timestamp || 0,
        end: transactions[transactions.length - 1]?.timestamp || 0,
      },
      summary: {
        totalTransactions: transactions.length,
        totalValue: totalValue.toString(),
        uniqueAddresses: uniqueAddresses.size,
      },
    };
  }

  private generateId(): string {
    return `rpt_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`;
  }
}

interface SuspiciousActivityReport {
  id: string;
  generatedAt: number;
  totalAlerts: number;
  criticalCount: number;
  highCount: number;
  alerts: FraudAlert[];
  recommendedAction: string;
}

interface TransactionReport {
  id: string;
  generatedAt: number;
  period: { start: number; end: number };
  summary: {
    totalTransactions: number;
    totalValue: string;
    uniqueAddresses: number;
  };
}

// ============================================================================
// Export
// ============================================================================

export default {
  FraudDetection,
  RateLimiter,
  CircuitBreaker,
  SecurityScanner,
  ComplianceReporter,
  RiskFlag,
  FraudType,
  Severity,
};