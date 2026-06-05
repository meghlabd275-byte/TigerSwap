{
  "name": "tigerswap-security",
  "version": "1.0.0"
}

// Security Platform - Fraud Detection & Rate Limiting
// Implements security measures for the TigerSwap ecosystem

import time
import hashlib

class RateLimiter:
    def __init__(self, max_requests, window_seconds):
        self.max_requests = max_requests
        self.window_seconds = window_seconds
        self.requests = {}
        
    def is_allowed(self, identifier):
        now = time.time()
        if identifier not in self.requests:
            self.requests[identifier] = []
            
        # Clean old requests
        self.requests[identifier] = [
            ts for ts in self.requests[identifier] 
            if now - ts < self.window_seconds
        ]
        
        if len(self.requests[identifier]) >= self.max_requests:
            return False
            
        self.requests[identifier].append(now)
        return True
    
    def get_remaining(self, identifier):
        now = time.time()
        if identifier not in self.requests:
            return self.max_requests
            
        recent = [ts for ts in self.requests.get(identifier, []) 
                 if now - ts < self.window_seconds]
        return max(0, self.max_requests - len(recent))


class FraudDetector:
    def __init__(self):
        self.suspicious_addresses = set()
        self.blacklisted_addresses = set()
        self.transaction_patterns = {}
        
    def analyze_transaction(self, address, amount, frequency):
        risk_score = 0.0
        
        # Check amount anomaly
        if amount > 100000:
            risk_score += 0.3
            
        # Check frequency anomaly
        if frequency > 100:
            risk_score += 0.3
            
        # Check for wash trading patterns
        if self.is_wash_trading(address):
            risk_score += 0.4
            
        return {
            'address': address,
            'risk_score': risk_score,
            'risk_level': 'high' if risk_score > 0.6 else 'medium' if risk_score > 0.3 else 'low',
            'timestamp': int(time.time())
        }
    
    def is_wash_trading(self, address):
        patterns = self.transaction_patterns.get(address, [])
        if len(patterns) < 5:
            return False
        # Simple pattern detection - would be more complex in production
        return patterns[-1] == patterns[-2]
    
    def blacklist_address(self, address):
        self.blacklisted_addresses.add(address)
        
    def is_blacklisted(self, address):
        return address in self.blacklisted_addresses


class CircuitBreaker:
    def __init__(self, threshold, reset_timeout):
        self.threshold = threshold
        self.reset_timeout = reset_timeout
        self.failures = {}
        self.last_failure = {}
        self.state = {}  # CLOSED, OPEN, HALF_OPEN
        
    def call(self, service, func):
        if self.state.get(service) == 'OPEN':
            if time.time() - self.last_failure.get(service, 0) > self.reset_timeout:
                self.state[service] = 'HALF_OPEN'
            else:
                raise Exception(f"Circuit open for {service}")
                
        try:
            result = func()
            if self.state.get(service) == 'HALF_OPEN':
                self.state[service] = 'CLOSED'
                self.failures[service] = 0
            return result
        except Exception as e:
            self.failures[service] = self.failures.get(service, 0) + 1
            self.last_failure[service] = time.time()
            
            if self.failures[service] >= self.threshold:
                self.state[service] = 'OPEN'
                
            raise e


class AuditLogger:
    def __init__(self):
        self.logs = []
        
    def log_event(self, event_type, address, details):
        log = {
            'timestamp': int(time.time()),
            'event_type': event_type,
            'address': address,
            'details': details,
            'hash': hashlib.sha256(f"{event_type}{address}{time.time()}".encode()).hexdigest()
        }
        self.logs.append(log)
        
    def get_logs(self, event_type=None, address=None, limit=100):
        filtered = self.logs
        if event_type:
            filtered = [l for l in filtered if l['event_type'] == event_type]
        if address:
            filtered = [l for l in filtered if l['address'] == address]
        return filtered[-limit:]


def main():
    print("=" * 60)
    print("TigerSwap Security Platform v1.0")
    print("=" * 60)
    
    # Rate Limiter
    print("\n🛡️ Rate Limiter:")
    limiter = RateLimiter(max_requests=100, window_seconds=60)
    for i in range(5):
        print(f"  Request {i+1}: {'allowed' if limiter.is_allowed('user_123') else 'blocked'}")
    
    # Fraud Detector
    print("\n🔍 Fraud Detection:")
    detector = FraudDetector()
    result = detector.analyze_transaction("0x1234...abcd", 150000, 150)
    print(f"  Address: {result['address'][:20]}...")
    print(f"  Risk Score: {result['risk_score']:.2f}")
    print(f"  Risk Level: {result['risk_level']}")
    
    # Circuit Breaker
    print("\n⚡ Circuit Breaker:")
    breaker = CircuitBreaker(threshold=3, reset_timeout=30)
    services = ['api', 'database', 'cache']
    for s in services:
        print(f"  {s}: {breaker.state.get(s, 'CLOSED')}")
    
    # Audit Logger
    print("\n📋 Audit Logs:")
    logger = AuditLogger()
    logger.log_event('SWAP', '0x1234', {'token_in': 'ETH', 'token_out': 'USDT', 'amount': 1000})
    logger.log_event('BRIDGE', '0x5678', {'from_chain': 1, 'to_chain': 56, 'amount': 5000})
    logs = logger.get_logs()
    print(f"  Total logs: {len(logs)}")
    for log in logs:
        print(f"  - [{log['event_type']}] {log['address'][:20]}... ({log['hash'][:16]}...)")
    
    print("\n" + "=" * 60)


if __name__ == "__main__":
    main()