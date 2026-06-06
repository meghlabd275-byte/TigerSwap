"""
TigerSwap Security Platform - Fraud Detection & Audit Engine
Production-ready security monitoring and anomaly detection

Features:
- Pattern recognition for suspicious activity
- Real-time transaction monitoring
- Risk scoring system
- Audit trail management
"""

import time
from typing import Dict, List, Optional, Tuple
from collections import defaultdict

class TransactionAnalyzer:
    """Analyze transactions for fraud patterns"""
    
    def __init__(self):
        self.suspicious_patterns = []
        self.risk_scores = defaultdict(float)
        
    def analyze_transaction(self, tx: Dict) -> Dict:
        """Analyze a transaction for fraud indicators"""
        risk_factors = []
        total_risk = 0.0
        
        # Check transaction size
        amount = tx.get("amount", 0)
        if amount > 1000000:  # > $1M
            risk_factors.append("large_amount")
            total_risk += 30
            
        # Check frequency
        if tx.get("frequency", 0) > 10:
            risk_factors.append("high_frequency")
            total_risk += 20
            
        # Check new wallet
        if tx.get("is_new_wallet", False):
            risk_factors.append("new_wallet")
            total_risk += 15
            
        # Check unusual timing
        hour = tx.get("hour", 12)
        if hour < 3 or hour > 23:
            risk_factors.append("unusual_timing")
            total_risk += 10
            
        # Check mixing patterns
        if tx.get("uses_mixer", False):
            risk_factors.append("mixer_detected")
            total_risk += 50
            
        # Check rapid transfers
        if tx.get("rapid_transfer", False):
            risk_factors.append("rapid_transfer")
            total_risk += 25
            
        return {
            "tx_hash": tx.get("hash", ""),
            "risk_score": min(100, total_risk),
            "risk_factors": risk_factors,
            "is_suspicious": total_risk > 50,
            "timestamp": int(time.time())
        }


class PatternDetector:
    """Detect patterns in transaction data"""
    
    def __init__(self):
        self.patterns = []
        self.thresholds = {
            "velocity": 10,  # txs per minute
            "amount": 1000000,  # $1M
            "frequency": 50,  # txs per hour
        }
        
    def detect_sandwich_attack(self, txs: List[Dict]) -> Optional[Dict]:
        """Detect sandwich attack pattern"""
        if len(txs) < 3:
            return None
            
        for i in range(len(txs) - 2):
            tx1, tx2, tx3 = txs[i], txs[i+1], txs[i+2]
            
            # Check if same pool, increasing gas, same token
            if (tx1.get("pool") == tx3.get("pool") and
                tx2.get("gas_price", 0) > tx1.get("gas_price", 0) and
                tx1.get("token") == tx3.get("token")):
                return {
                    "type": "sandwich_attack",
                    "victim_tx": tx1.get("hash"),
                    "attacker_front": tx2.get("hash"),
                    "attacker_back": tx3.get("hash"),
                    "confidence": 0.85
                }
                
        return None
        
    def detect_wash_trading(self, txs: List[Dict]) -> List[Dict]:
        """Detect wash trading patterns"""
        suspicious = []
        
        # Group by wallet
        wallet_txs = defaultdict(list)
        for tx in txs:
            wallet = tx.get("wallet")
            if wallet:
                wallet_txs[wallet].append(tx)
                
        # Check for circular trades
        for wallet, wallet_tx_list in wallet_txs.items():
            if len(wallet_tx_list) > 20:
                # Check for same tokens being traded back and forth
                tokens = set()
                for tx in wallet_tx_list:
                    tokens.add(tx.get("token"))
                    
                if len(tokens) < 3:
                    suspicious.append({
                        "type": "wash_trading",
                        "wallet": wallet,
                        "tx_count": len(wallet_tx_list),
                        "confidence": 0.7
                    })
                    
        return suspicious


class AuditEngine:
    """Main audit engine for security monitoring"""
    
    def __init__(self):
        self.analyzer = TransactionAnalyzer()
        self.detector = PatternDetector()
        self.alerts = []
        self.audit_log = []
        
    def process_transaction(self, tx: Dict) -> Dict:
        """Process a transaction through the audit pipeline"""
        # Analyze transaction
        analysis = self.analyzer.analyze_transaction(tx)
        
        # Log to audit trail
        self.audit_log.append({
            "tx_hash": tx.get("hash"),
            "timestamp": int(time.time()),
            "analysis": analysis
        })
        
        # Create alert if suspicious
        if analysis["is_suspicious"]:
            alert = {
                "id": len(self.alerts),
                "tx_hash": tx.get("hash"),
                "risk_score": analysis["risk_score"],
                "risk_factors": analysis["risk_factors"],
                "timestamp": int(time.time()),
                "status": "open"
            }
            self.alerts.append(alert)
            
        return analysis
        
    def detect_attacks(self, txs: List[Dict]) -> List[Dict]:
        """Detect various attack patterns"""
        attacks = []
        
        # Sandwich attacks
        sandwich = self.detector.detect_sandwich_attack(txs)
        if sandwich:
            attacks.append(sandwich)
            
        # Wash trading
        wash_trades = self.detector.detect_wash_trading(txs)
        attacks.extend(wash_trades)
        
        return attacks
        
    def get_alerts(self, status: Optional[str] = None) -> List[Dict]:
        """Get security alerts"""
        if status:
            return [a for a in self.alerts if a.get("status") == status]
        return self.alerts
        
    def get_audit_trail(self, limit: int = 100) -> List[Dict]:
        """Get audit trail"""
        return self.audit_log[-limit:]


# API endpoints
def handle_audit_request(data: Dict) -> Dict:
    """Handle audit request"""
    engine = AuditEngine()
    
    if "transaction" in data:
        result = engine.process_transaction(data["transaction"])
        return {"success": True, "analysis": result}
        
    if "transactions" in data:
        attacks = engine.detect_attacks(data["transactions"])
        return {"success": True, "attacks": attacks}
        
    return {"error": "Invalid request"}


if __name__ == "__main__":
    print("TigerSwap Security Platform - Fraud Detection")
    print("=" * 50)
    
    engine = AuditEngine()
    
    # Test transaction analysis
    tx = {
        "hash": "0x123",
        "amount": 5000000,
        "wallet": "0xWallet",
        "pool": "0xPool",
        "gas_price": 100,
        "token": "USDC",
        "frequency": 5,
        "is_new_wallet": True
    }
    
    result = engine.process_transaction(tx)
    print(f"Transaction Analysis: {result}")
    
    # Test pattern detection
    print(f"\nAlerts: {len(engine.get_alerts())}")
    print(f"Audit Trail: {len(engine.get_audit_trail())} entries")
