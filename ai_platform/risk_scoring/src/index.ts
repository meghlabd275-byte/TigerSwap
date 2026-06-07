/**
 * TigerSwap AI Platform - Risk Scoring Engine
 * 
 * Native risk scoring for DeFi operations.
 * Zero external dependencies - fully native implementation.
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

export interface RiskScore {
  address: string;
  score: number; // 0-100
  level: 'low' | 'medium' | 'high' | 'critical';
  factors: RiskFactor[];
  flags: string[];
  timestamp: number;
}

export interface RiskFactor {
  name: string;
  weight: number;
  value: number;
  contribution: number;
}

export interface TransactionRisk {
  from: string;
  to: string;
  token: string;
  amount: bigint;
  riskScore: number;
  isMalicious: boolean;
  reason?: string;
}

// Risk factors weights
const RISK_FACTORS = {
  // Address risk factors
  newAddress: { weight: 30, threshold: 30 },
  highVolume: { weight: 20, threshold: 100000 },
  flashLoan: { weight: 40, threshold: 1 },
  contractInteraction: { weight: 15, threshold: 1 },
  
  // Behavioral factors
  unusualPattern: { weight: 25, threshold: 1 },
  rapidTransactions: { weight: 20, threshold: 10 },
  mixedFunds: { weight: 20, threshold: 1 },
  tornadoInteraction: { weight: 50, threshold: 1 },
  
  // Token factors
  unauditedToken: { weight: 20, threshold: 1 },
  honeypot: { weight: 50, threshold: 1 },
  fakeToken: { weight: 40, threshold: 1 },
};

// Blacklist addresses (simplified)
const BLACKLIST = new Set([
  '0x000000000000000000000000000000000000000000',
  '0xdead00000000000000000000000000000000000000',
]);

export class RiskScoringEngine {
  private transactionHistory: Map<string, { timestamp: number; type: string }[]>;
  private riskScores: Map<string, RiskScore>;
  private honeypotTokens: Set<string>;
  private unauditedTokens: Set<string>;

  constructor() {
    this.transactionHistory = new Map();
    this.riskScores = new Map();
    this.honeypotTokens = new Set();
    this.unauditedTokens = new Set();
  }

  /**
   * Score address risk
   */
  async scoreAddress(address: string): Promise<RiskScore> {
    const factors: RiskFactor[] = [];
    let totalScore = 0;
    let maxPossible = 0;
    const flags: string[] = [];

    // Check if address is blacklisted
    if (BLACKLIST.has(address.toLowerCase())) {
      return {
        address,
        score: 100,
        level: 'critical',
        factors: [{
          name: 'Blacklisted Address',
          weight: 100,
          value: 100,
          contribution: 100,
        }],
        flags: ['Address is blacklisted'],
        timestamp: Date.now(),
      };
    }

    // Check address age (simplified - would check on-chain)
    const isNew = true; // Simplified
    if (isNew) {
      const contribution = RISK_FACTORS.newAddress.weight;
      factors.push({
        name: 'New Address',
        weight: RISK_FACTORS.newAddress.weight,
        value: 1,
        contribution,
      });
      totalScore += contribution;
      maxPossible += contribution;
      flags.push('New address without history');
    }

    // Check transaction pattern
    const history = this.transactionHistory.get(address) || [];
    const rapidTxCount = this.countRapidTransactions(history);
    if (rapidTxCount > RISK_FACTORS.rapidTransactions.threshold) {
      const contribution = RISK_FACTORS.rapidTransactions.weight;
      factors.push({
        name: 'Rapid Transactions',
        weight: RISK_FACTORS.rapidTransactions.weight,
        value: rapidTxCount,
        contribution,
      });
      totalScore += contribution;
      maxPossible += contribution;
      flags.push('Unusually high transaction frequency');
    }

    // Check for contract interactions
    const hasContractInteraction = true; // Simplified
    if (hasContractInteraction) {
      const contribution = RISK_FACTORS.contractInteraction.weight;
      factors.push({
        name: 'Contract Interaction',
        weight: RISK_FACTORS.contractInteraction.weight,
        value: 1,
        contribution,
      });
      totalScore += contribution;
      maxPossible += contribution;
    }

    // Calculate normalized score
    const normalizedScore = maxPossible > 0 
      ? Math.round((totalScore / maxPossible) * 100) 
      : 50;

    // Determine level
    let level: RiskScore['level'];
    if (normalizedScore >= 80) level = 'critical';
    else if (normalizedScore >= 60) level = 'high';
    else if (normalizedScore >= 40) level = 'medium';
    else level = 'low';

    const riskScore: RiskScore = {
      address,
      score: normalizedScore,
      level,
      factors,
      flags,
      timestamp: Date.now(),
    };

    this.riskScores.set(address, riskScore);
    return riskScore;
  }

  /**
   * Score transaction
   */
  async scoreTransaction(tx: TransactionRisk): Promise<TransactionRisk> {
    let riskScore = 0;
    let isMalicious = false;
    let reason = '';

    // Check sender risk
    const senderRisk = await this.scoreAddress(tx.from);
    riskScore += senderRisk.score * 0.5;

    // Check receiver risk
    const receiverRisk = await this.scoreAddress(tx.to);
    riskScore += receiverRisk.score * 0.3;

    // Check token
    if (this.honeypotTokens.has(tx.token)) {
      isMalicious = true;
      reason = 'Honeypot token detected';
      riskScore += 50;
    }

    if (this.unauditedTokens.has(tx.token)) {
      riskScore += 20;
      if (!reason) reason = 'Unaudited token';
    }

    // Check amount
    const amountUSD = Number(tx.amount);
    if (amountUSD > RISK_FACTORS.highVolume.threshold) {
      riskScore += RISK_FACTORS.highVolume.weight;
      if (!reason) reason = 'Unusually large transaction amount';
    }

    // Check for flash loan pattern
    const history = this.transactionHistory.get(tx.from) || [];
    if (history.length === 0 && amountUSD > 10000) {
      riskScore += RISK_FACTORS.flashLoan.weight;
      if (!reason) reason = 'Potential flash loan attack';
    }

    return {
      ...tx,
      riskScore: Math.min(100, riskScore),
      isMalicious,
      reason,
    };
  }

  /**
   * Analyze wallet
   */
  async analyzeWallet(address: string): Promise<{
    riskScore: number;
    recommendation: 'approve' | 'review' | 'reject';
    reasons: string[];
  }> {
    const score = await this.scoreAddress(address);
    
    let recommendation: 'approve' | 'review' | 'reject';
    if (score.score < 30) recommendation = 'approve';
    else if (score.score < 70) recommendation = 'review';
    else recommendation = 'reject';

    return {
      riskScore: score.score,
      recommendation,
      reasons: score.flags,
    };
  }

  /**
   * Add to blacklist
   */
  addToBlacklist(address: string): void {
    BLACKLIST.add(address.toLowerCase());
  }

  /**
   * Remove from blacklist
   */
  removeFromBlacklist(address: string): void {
    BLACKLIST.delete(address.toLowerCase());
  }

  /**
   * Mark honeypot token
   */
  markHoneypot(token: string): void {
    this.honeypotTokens.add(token.toLowerCase());
  }

  /**
   * Mark unaudited token
   */
  markUnaudited(token: string): void {
    this.unauditedTokens.add(token.toLowerCase());
  }

  /**
   * Record transaction
   */
  recordTransaction(address: string, type: string): void {
    const history = this.transactionHistory.get(address) || [];
    history.push({ timestamp: Date.now(), type });
    this.transactionHistory.set(address, history);
  }

  /**
   * Get risk score
   */
  getRiskScore(address: string): RiskScore | undefined {
    return this.riskScores.get(address);
  }

  private countRapidTransactions(history: { timestamp: number; type: string }[]): number {
    const now = Date.now();
    const oneHourAgo = now - 3600000;
    return history.filter(h => h.timestamp > oneHourAgo).length;
  }
}

export default RiskScoringEngine;