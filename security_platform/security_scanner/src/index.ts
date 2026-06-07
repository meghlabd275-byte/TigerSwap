/**
 * TigerSwap Security Platform - Security Scanner
 * 
 * Native smart contract vulnerability scanner.
 * Zero external dependencies - fully native implementation.
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

export interface Vulnerability {
  id: string;
  severity: 'critical' | 'high' | 'medium' | 'low' | 'info';
  title: string;
  description: string;
  location: string;
  recommendation: string;
}

export interface ScanResult {
  contract: string;
  vulnerabilities: Vulnerability[];
  score: number;
  passed: boolean;
  timestamp: number;
}

// Vulnerability patterns
const VULNERABILITY_PATTERNS = {
  reentrancy: {
    severity: 'critical',
    title: 'Reentrancy Vulnerability',
    description: 'Potential reentrancy attack due to external call before state change',
    patterns: ['call{value:', 'transfer(', 'send(', '.call.value'],
  },
  integerOverflow: {
    severity: 'high',
    title: 'Integer Overflow',
    description: 'Potential integer overflow/underflow',
    patterns: ['+', '-', '*', '/', 'uint256', 'uint8'],
  },
  accessControl: {
    severity: 'high',
    title: 'Missing Access Control',
    description: 'Function lacks access control modifiers',
    patterns: ['function', 'public', 'external'],
  },
  frontRunning: {
    severity: 'medium',
    title: 'Front-Running Vulnerability',
    description: 'Transaction can be front-run',
    patterns: ['setPrice', 'updatePrice', 'swap'],
  },
  flashLoanAttack: {
    severity: 'critical',
    title: 'Flash Loan Attack',
    description: 'Vulnerable to flash loan attacks',
    patterns: ['flashLoan', 'flashSwap'],
  },
  approval: {
    severity: 'high',
    title: 'Unlimited Approval',
    description: 'Token approval allows unlimited transfer',
    patterns: ['approve(uint256(-1)', 'approve(2**256-1)'],
  },
  txOrigin: {
    severity: 'medium',
    title: 'tx.origin Usage',
    description: 'Using tx.origin for authorization is vulnerable to phishing',
    patterns: ['tx.origin', 'require(tx.origin'],
  },
  randomness: {
    severity: 'medium',
    title: 'Weak Randomness',
    description: 'Using block variables for randomness is predictable',
    patterns: ['block.timestamp', 'blockhash', 'block.difficulty'],
  },
};

// Security Scanner
export class SecurityScanner {
  private contracts: Map<string, string>;
  private scanHistory: Map<string, ScanResult>;

  constructor() {
    this.contracts = new Map();
    this.scanHistory = new Map();
  }

  /**
   * Add contract to scan
   */
  addContract(address: string, sourceCode: string): void {
    this.contracts.set(address, sourceCode);
  }

  /**
   * Scan contract
   */
  scan(contractAddress: string): ScanResult {
    const sourceCode = this.contracts.get(contractAddress);
    if (!sourceCode) {
      return {
        contract: contractAddress,
        vulnerabilities: [],
        score: 100,
        passed: true,
        timestamp: Date.now(),
      };
    }

    const vulnerabilities: Vulnerability[] = [];

    // Check for reentrancy
    if (this.checkPattern(sourceCode, VULNERABILITY_PATTERNS.reentrancy.patterns)) {
      vulnerabilities.push(this.createVulnerability('reentrancy', 'function'));
    }

    // Check for integer overflow
    if (this.checkPattern(sourceCode, VULNERABILITY_PATTERNS.integerOverflow.patterns)) {
      vulnerabilities.push(this.createVulnerability('integerOverflow', 'line'));
    }

    // Check for tx.origin usage
    if (this.checkPattern(sourceCode, VULNERABILITY_PATTERNS.txOrigin.patterns)) {
      vulnerabilities.push(this.createVulnerability('txOrigin', 'function'));
    }

    // Check for weak randomness
    if (this.checkPattern(sourceCode, VULNERABILITY_PATTERNS.randomness.patterns)) {
      vulnerabilities.push(this.createVulnerability('randomness', 'line'));
    }

    // Check for unlimited approval
    if (this.checkPattern(sourceCode, VULNERABILITY_PATTERNS.approval.patterns)) {
      vulnerabilities.push(this.createVulnerability('approval', 'line'));
    }

    // Calculate score
    const score = this.calculateScore(vulnerabilities);

    const result: ScanResult = {
      contract: contractAddress,
      vulnerabilities,
      score,
      passed: score >= 80,
      timestamp: Date.now(),
    };

    this.scanHistory.set(contractAddress, result);
    return result;
  }

  /**
   * Create vulnerability
   */
  private createVulnerability(id: string, location: string): Vulnerability {
    const pattern = VULNERABILITY_PATTERNS[id as keyof typeof VULNERABILITY_PATTERNS];
    
    return {
      id,
      severity: pattern.severity,
      title: pattern.title,
      description: pattern.description,
      location,
      recommendation: `Fix ${pattern.title} vulnerability`,
    };
  }

  /**
   * Check pattern
   */
  private checkPattern(sourceCode: string, patterns: string[]): boolean {
    for (const pattern of patterns) {
      if (sourceCode.includes(pattern)) {
        return true;
      }
    }
    return false;
  }

  /**
   * Calculate security score
   */
  private calculateScore(vulnerabilities: Vulnerability[]): number {
    let score = 100;
    
    for (const vuln of vulnerabilities) {
      switch (vuln.severity) {
        case 'critical':
          score -= 40;
          break;
        case 'high':
          score -= 20;
          break;
        case 'medium':
          score -= 10;
          break;
        case 'low':
          score -= 5;
          break;
        case 'info':
          score -= 1;
          break;
      }
    }

    return Math.max(0, score);
  }

  /**
   * Get scan history
   */
  getHistory(contractAddress: string): ScanResult | undefined {
    return this.scanHistory.get(contractAddress);
  }

  /**
   * Batch scan
   */
  scanAll(): ScanResult[] {
    const results: ScanResult[] = [];
    
    for (const address of this.contracts.keys()) {
      results.push(this.scan(address));
    }

    return results;
  }

  /**
   * Get vulnerability report
   */
  getReport(contractAddress: string): {
    critical: number;
    high: number;
    medium: number;
    low: number;
    info: number;
  } {
    const result = this.scanHistory.get(contractAddress);
    if (!result) {
      return { critical: 0, high: 0, medium: 0, low: 0, info: 0 };
    }

    return {
      critical: result.vulnerabilities.filter(v => v.severity === 'critical').length,
      high: result.vulnerabilities.filter(v => v.severity === 'high').length,
      medium: result.vulnerabilities.filter(v => v.severity === 'medium').length,
      low: result.vulnerabilities.filter(v => v.severity === 'low').length,
      info: result.vulnerabilities.filter(v => v.severity === 'info').length,
    };
  }
}

export default SecurityScanner;