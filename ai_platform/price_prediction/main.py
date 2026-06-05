#!/usr/bin/env python3
"""
TigerSwap Analytics Platform
Price prediction and risk scoring using machine learning
"""

import numpy as np
from typing import Dict, List, Tuple
from dataclasses import dataclass
from datetime import datetime, timedelta
import json

@dataclass
class PricePrediction:
    pair: str
    current_price: float
    predicted_price: float
    confidence: float
    direction: str  # 'up', 'down', 'stable'
    timeframe: str
    factors: Dict[str, float]

class PricePredictor:
    def __init__(self):
        self.models = {}
        self.price_history = {}
        self.volatility_cache = {}
    
    def predict_price(self, pair: str, timeframe: str = '1h') -> PricePrediction:
        """Predict price for a trading pair"""
        current = self._get_current_price(pair)
        volatility = self._calculate_volatility(pair)
        
        # Simplified prediction model
        trend = self._calculate_trend(pair)
        predicted = current * (1 + trend * 0.01)
        
        direction = 'stable'
        if predicted > current * 1.001:
            direction = 'up'
        elif predicted < current * 0.999:
            direction = 'down'
        
        factors = {
            'volatility': volatility,
            'trend': trend,
            'volume_score': self._calculate_volume_score(pair),
            'liquidity_score': self._calculate_liquidity_score(pair),
            'social_sentiment': 0.5,  # Would integrate social data in production
        }
        
        return PricePrediction(
            pair=pair,
            current_price=current,
            predicted_price=predicted,
            confidence=self._calculate_confidence(volatility),
            direction=direction,
            timeframe=timeframe,
            factors=factors
        )
    
    def _get_current_price(self, pair: str) -> float:
        # Mock - would fetch from exchange APIs
        mock_prices = {
            'ETH/USDT': 2450.50,
            'BTC/USDT': 62500.00,
            'BNB/USDT': 310.25,
            'MATIC/USDT': 0.85,
            'ARB/USDT': 1.20,
        }
        return mock_prices.get(pair, 100.0)
    
    def _calculate_volatility(self, pair: str) -> float:
        """Calculate price volatility (standard deviation of returns)"""
        if pair in self.volatility_cache:
            return self.volatility_cache[pair]
        
        # Mock volatility calculation
        base_volatility = {
            'ETH/USDT': 0.03,
            'BTC/USDT': 0.025,
            'BNB/USDT': 0.04,
            'MATIC/USDT': 0.05,
            'ARB/USDT': 0.06,
        }
        
        volatility = base_volatility.get(pair, 0.04)
        self.volatility_cache[pair] = volatility
        return volatility
    
    def _calculate_trend(self, pair: str) -> float:
        """Calculate price trend direction"""
        # Mock - would analyze historical price data
        return np.random.uniform(-2, 2)
    
    def _calculate_volume_score(self, pair: str) -> float:
        """Calculate volume strength score (0-1)"""
        # Mock - would analyze 24h volume
        return np.random.uniform(0.4, 0.9)
    
    def _calculate_liquidity_score(self, pair: str) -> float:
        """Calculate liquidity score (0-1)"""
        # Mock - would analyze order book depth
        return np.random.uniform(0.5, 0.95)
    
    def _calculate_confidence(self, volatility: float) -> float:
        """Calculate prediction confidence based on volatility"""
        return max(0.5, 1.0 - volatility * 10)


class RiskScorer:
    def __init__(self):
        self.risk_factors = {}
    
    def calculate_risk_score(self, address: str, chain: int) -> Dict:
        """Calculate comprehensive risk score for an address"""
        
        factors = {
            'wallet_age': self._analyze_wallet_age(address),
            'transaction_pattern': self._analyze_tx_pattern(address),
            'token_interactions': self._analyze_token_interactions(address),
            'smart_contract_interactions': self._analyze_contract_interactions(address),
            'volume_concentration': self._analyze_volume_concentration(address),
            'counterparty_risk': self._analyze_counterparties(address),
        }
        
        overall_score = sum(factors.values()) / len(factors)
        
        return {
            'address': address,
            'chain': chain,
            'overall_risk_score': overall_score,
            'risk_level': self._get_risk_level(overall_score),
            'factors': factors,
            'recommendations': self._generate_recommendations(factors),
            'timestamp': datetime.now().isoformat(),
        }
    
    def _analyze_wallet_age(self, address: str) -> float:
        """Analyze wallet age (older = more trustworthy)"""
        return np.random.uniform(0.3, 0.9)
    
    def _analyze_tx_pattern(self, address: str) -> float:
        """Analyze transaction patterns"""
        return np.random.uniform(0.4, 0.9)
    
    def _analyze_token_interactions(self, address: str) -> float:
        """Analyze token interaction patterns"""
        return np.random.uniform(0.3, 0.85)
    
    def _analyze_contract_interactions(self, address: str) -> float:
        """Analyze smart contract interactions"""
        return np.random.uniform(0.4, 0.8)
    
    def _analyze_volume_concentration(self, address: str) -> float:
        """Analyze volume concentration"""
        return np.random.uniform(0.3, 0.9)
    
    def _analyze_counterparties(self, address: str) -> float:
        """Analyze counterparty risk"""
        return np.random.uniform(0.4, 0.85)
    
    def _get_risk_level(self, score: float) -> str:
        if score < 0.3:
            return 'high'
        elif score < 0.6:
            return 'medium'
        else:
            return 'low'
    
    def _generate_recommendations(self, factors: Dict) -> List[str]:
        recommendations = []
        for factor, value in factors.items():
            if value < 0.5:
                recommendations.append(f"Review {factor.replace('_', ' ')} - score below threshold")
        return recommendations


class AnomalyDetector:
    def __init__(self):
        self.baseline_patterns = {}
        self.alerts = []
    
    def detect_anomalies(self, pair: str, current_metrics: Dict) -> List[Dict]:
        """Detect price and volume anomalies"""
        anomalies = []
        
        price_change = current_metrics.get('price_change_24h', 0)
        if abs(price_change) > 10:
            anomalies.append({
                'type': 'price_spike',
                'severity': 'high' if abs(price_change) > 20 else 'medium',
                'description': f"Price changed {price_change:.2f}% in 24h",
                'pair': pair,
                'timestamp': datetime.now().isoformat(),
            })
        
        volume_spike = current_metrics.get('volume_ratio', 1.0)
        if volume_spike > 3.0:
            anomalies.append({
                'type': 'volume_spike',
                'severity': 'medium',
                'description': f"Volume {volume_spike:.1f}x above average",
                'pair': pair,
                'timestamp': datetime.now().isoformat(),
            })
        
        return anomalies


class PortfolioAnalytics:
    def __init__(self):
        self.positions = {}
    
    def analyze_portfolio(self, addresses: List[str], chains: List[int]) -> Dict:
        """Comprehensive portfolio analysis"""
        
        total_value = 0
        positions = []
        
        for address in addresses:
            for chain in chains:
                position = self._get_position(address, chain)
                total_value += position['value']
                positions.append(position)
        
        allocation = self._calculate_allocation(positions, total_value)
        pnl = self._calculate_pnl(positions)
        risk = self._calculate_portfolio_risk(positions, allocation)
        
        return {
            'total_value': total_value,
            'positions': positions,
            'allocation': allocation,
            'total_pnl': pnl['total'],
            'daily_pnl': pnl['daily'],
            'risk_score': risk['score'],
            'risk_factors': risk['factors'],
            'recommendations': self._generate_rebalancing_recommendations(allocation),
        }
    
    def _get_position(self, address: str, chain: int) -> Dict:
        """Get position for an address on a chain (mock)"""
        return {
            'address': address,
            'chain': chain,
            'tokens': [
                {'symbol': 'ETH', 'amount': 2.5, 'value': 6125},
                {'symbol': 'USDT', 'amount': 10000, 'value': 10000},
            ],
            'value': 16125,
            'pnl_24h': 125.50,
        }
    
    def _calculate_allocation(self, positions: List[Dict], total: float) -> List[Dict]:
        """Calculate portfolio allocation"""
        allocation = {}
        for pos in positions:
            for token in pos['tokens']:
                symbol = token['symbol']
                if symbol not in allocation:
                    allocation[symbol] = 0
                allocation[symbol] += token['value']
        
        return [
            {'symbol': k, 'value': v, 'percentage': (v / total * 100) if total > 0 else 0}
            for k, v in allocation.items()
        ]
    
    def _calculate_pnl(self, positions: List[Dict]) -> Dict:
        """Calculate PnL metrics"""
        total_pnl = sum(p['pnl_24h'] for p in positions)
        return {'total': total_pnl * 30, 'daily': total_pnl}
    
    def _calculate_portfolio_risk(self, positions: List[Dict], allocation: List[Dict]) -> Dict:
        """Calculate portfolio risk score"""
        concentration = max(a['percentage'] for a in allocation) if allocation else 0
        return {
            'score': min(1.0, concentration / 50 + 0.3),
            'factors': {
                'concentration_risk': concentration / 100,
                'chain_diversity': len(set(p['chain'] for p in positions)) / 6,
            }
        }
    
    def _generate_rebalancing_recommendations(self, allocation: List[Dict]) -> List[str]:
        recommendations = []
        for a in allocation:
            if a['percentage'] > 40:
                recommendations.append(f"Consider reducing {a['symbol']} exposure (currently {a['percentage']:.1f}%)")
            elif a['percentage'] < 5:
                recommendations.append(f"Consider adding more {a['symbol']} exposure (currently {a['percentage']:.1f}%)")
        return recommendations


def main():
    print("=" * 60)
    print("TigerSwap Analytics Platform v1.0")
    print("=" * 60)
    
    # Price Prediction
    predictor = PricePredictor()
    pairs = ['ETH/USDT', 'BTC/USDT', 'BNB/USDT', 'MATIC/USDT', 'ARB/USDT']
    
    print("\n📈 Price Predictions:")
    print("-" * 40)
    for pair in pairs:
        pred = predictor.predict_price(pair)
        print(f"{pair}: ${pred.predicted_price:.2f} ({pred.direction})")
        print(f"  Confidence: {pred.confidence:.1%}, Volatility: {pred.factors['volatility']:.2%}")
    
    # Risk Scoring
    print("\n🛡️ Risk Analysis:")
    print("-" * 40)
    scorer = RiskScorer()
    test_address = "0x1234567890abcdef1234567890abcdef12345678"
    risk = scorer.calculate_risk_score(test_address, 1)
    print(f"Address: {risk['address'][:20]}...")
    print(f"Overall Risk: {risk['overall_risk_score']:.2f} ({risk['risk_level']})")
    for factor, score in risk['factors'].items():
        print(f"  {factor}: {score:.2f}")
    
    # Portfolio Analytics
    print("\n💼 Portfolio Analytics:")
    print("-" * 40)
    portfolio = PortfolioAnalytics()
    analysis = portfolio.analyze_portfolio([test_address], [1, 56, 137])
    print(f"Total Value: ${analysis['total_value']:,.2f}")
    print(f"Total PnL: ${analysis['total_pnl']:,.2f}")
    print(f"Risk Score: {analysis['risk_score']:.2f}")
    print("\nRecommendations:")
    for rec in analysis['recommendations']:
        print(f"  • {rec}")
    
    print("\n" + "=" * 60)


if __name__ == "__main__":
    main()