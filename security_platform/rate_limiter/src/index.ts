/**
 * TigerSwap Security Platform - Rate Limiter
 * 
 * Native rate limiting for API protection.
 * Zero external dependencies - fully native implementation.
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

export interface RateLimitConfig {
  windowMs: number;
  maxRequests: number;
  keyGenerator?: (request: Request) => string;
}

export interface Request {
  ip?: string;
  headers?: Record<string, string>;
  user?: string;
}

export interface RateLimitEntry {
  count: number;
  resetTime: number;
  blocked: boolean;
}

// Token bucket algorithm
export class TokenBucketRateLimiter {
  private tokens: Map<string, { tokens: number; lastRefill: number }>;
  private config: RateLimitConfig;

  constructor(config: RateLimitConfig) {
    this.tokens = new Map();
    this.config = config;
  }

  /**
   * Check if request is allowed
   */
  allowRequest(key: string): { allowed: boolean; remaining: number; resetTime: number } {
    const now = Date.now();
    let entry = this.tokens.get(key);

    if (!entry || now >= entry.lastRefill + this.config.windowMs) {
      // Refill tokens
      entry = {
        tokens: this.config.maxRequests - 1,
        lastRefill: now,
      };
      this.tokens.set(key, entry);

      return {
        allowed: true,
        remaining: this.config.maxRequests - 1,
        resetTime: now + this.config.windowMs,
      };
    }

    if (entry.tokens > 0) {
      entry.tokens--;
      return {
        allowed: true,
        remaining: entry.tokens,
        resetTime: entry.lastRefill + this.config.windowMs,
      };
    }

    return {
      allowed: false,
      remaining: 0,
      resetTime: entry.lastRefill + this.config.windowMs,
    };
  }

  /**
   * Reset limit for key
   */
  reset(key: string): void {
    this.tokens.delete(key);
  }

  /**
   * Get current tokens
   */
  getTokens(key: string): number {
    return this.tokens.get(key)?.tokens || this.config.maxRequests;
  }
}

// Sliding window algorithm
export class SlidingWindowRateLimiter {
  private requests: Map<string, number[]>;
  private config: RateLimitConfig;

  constructor(config: RateLimitConfig) {
    this.requests = new Map();
    this.config = config;
  }

  /**
   * Check if request is allowed
   */
  allowRequest(key: string): { allowed: boolean; remaining: number; resetTime: number } {
    const now = Date.now();
    const windowStart = now - this.config.windowMs;
    
    let timestamps = this.requests.get(key) || [];
    
    // Remove old timestamps
    timestamps = timestamps.filter(t => t > windowStart);
    
    if (timestamps.length < this.config.maxRequests) {
      timestamps.push(now);
      this.requests.set(key, timestamps);

      return {
        allowed: true,
        remaining: this.config.maxRequests - timestamps.length,
        resetTime: now + this.config.windowMs,
      };
    }

    return {
      allowed: false,
      remaining: 0,
      resetTime: timestamps[0] + this.config.windowMs,
    };
  }

  /**
   * Reset limit for key
   */
  reset(key: string): void {
    this.requests.delete(key);
  }
}

// Fixed window algorithm
export class FixedWindowRateLimiter {
  private counters: Map<string, { count: number; windowEnd: number }>;
  private config: RateLimitConfig;

  constructor(config: RateLimitConfig) {
    this.counters = new Map();
    this.config = config;
  }

  /**
   * Check if request is allowed
   */
  allowRequest(key: string): { allowed: boolean; remaining: number; resetTime: number } {
    const now = Date.now();
    const windowEnd = Math.ceil(now / this.config.windowMs) * this.config.windowMs;
    
    let counter = this.counters.get(key);
    
    if (!counter || now > counter.windowEnd) {
      counter = { count: 1, windowEnd };
      this.counters.set(key, counter);

      return {
        allowed: true,
        remaining: this.config.maxRequests - 1,
        resetTime: windowEnd + this.config.windowMs,
      };
    }

    if (counter.count < this.config.maxRequests) {
      counter.count++;

      return {
        allowed: true,
        remaining: this.config.maxRequests - counter.count,
        resetTime: counter.windowEnd + this.config.windowMs,
      };
    }

    return {
      allowed: false,
      remaining: 0,
      resetTime: counter.windowEnd + this.config.windowMs,
    };
  }

  /**
   * Reset limit for key
   */
  reset(key: string): void {
    this.counters.delete(key);
  }
}

// Leaky bucket algorithm
export class LeakyBucketRateLimiter {
  private level: Map<string, number>;
  private lastLeak: Map<string, number>;
  private config: RateLimitConfig;
  private leakRate: number;

  constructor(config: RateLimitConfig) {
    this.level = new Map();
    this.lastLeak = new Map();
    this.config = config;
    this.leakRate = config.maxRequests / (config.windowMs / 1000);
  }

  /**
   * Check if request is allowed
   */
  allowRequest(key: string): { allowed: boolean; level: number; resetTime: number } {
    const now = Date.now();
    
    // Calculate leak
    let lastLeak = this.lastLeak.get(key) || now;
    const timePassed = now - lastLeak;
    const leaked = timePassed * this.leakRate;
    
    let level = this.level.get(key) || 0;
    level = Math.max(0, level - leaked);
    
    this.lastLeak.set(key, now);

    if (level < this.config.maxRequests) {
      level++;
      this.level.set(key, level);

      return {
        allowed: true,
        level,
        resetTime: now + (this.config.maxRequests - level) / this.leakRate * 1000,
      };
    }

    return {
      allowed: false,
      level: this.config.maxRequests,
      resetTime: now + this.config.windowMs,
    };
  }

  /**
   * Reset limit for key
   */
  reset(key: string): void {
    this.level.delete(key);
    this.lastLeak.delete(key);
  }
}

// Rate limiter factory
export class RateLimiterFactory {
  static create(type: 'token_bucket' | 'sliding_window' | 'fixed_window' | 'leaky_bucket', config: RateLimitConfig) {
    switch (type) {
      case 'token_bucket':
        return new TokenBucketRateLimiter(config);
      case 'sliding_window':
        return new SlidingWindowRateLimiter(config);
      case 'fixed_window':
        return new FixedWindowRateLimiter(config);
      case 'leaky_bucket':
        return new LeakyBucketRateLimiter(config);
      default:
        throw new Error('Unknown rate limiter type');
    }
  }
}

export default RateLimiterFactory;