"""
TigerSwap Analytics Platform

Enterprise-grade analytics with Python.
Provides real-time dashboards, portfolio analytics, protocol analytics, and liquidity analytics.

Features:
- Real-time TVL tracking
- Volume analytics by chain/DEX
- Revenue waterfall
- User behavior analytics
- Bot performance tracking
- Fee analytics

Author: TigerSwap
Version: 1.0.0
"""

from dataclasses import dataclass
from typing import Dict, List, Optional, Any
from datetime import datetime, timedelta
from enum import Enum
import json
import hashlib


# ============================================================================
# Types & Enums
# ============================================================================

class MetricType(Enum):
    TVL = "tvl"
    VOLUME = "volume"
    REVENUE = "revenue"
    USERS = "users"
    FEES = "fees"
    LIQUIDITY = "liquidity"


class TimeRange(Enum):
    HOUR = "1h"
    DAY = "24h"
    WEEK = "7d"
    MONTH = "30d"
    YEAR = "1y"
    ALL = "all"


@dataclass
class Metric:
    value: float
    change_24h: float
    change_percent: float
    timestamp: datetime


@dataclass
class TVLData:
    total_tvl: float
    change_24h: float
    by_chain: Dict[str, float]
    by_protocol: Dict[str, float]


@dataclass
class VolumeData:
    total_volume: float
    buy_volume: float
    sell_volume: float
    by_chain: Dict[str, float]
    by_dex: Dict[str, float]
    trades: int


@dataclass
class RevenueData:
    total_revenue: float
    protocol_fees: float
    liquidity_fees: float
    partner_fees: float
    by_source: Dict[str, float]


@dataclass
class UserData:
    total_users: int
    new_users_24h: int
    active_users: int
    dau: int
    mau: int
    retention_rate: float


@dataclass
class FeeData:
    total_fees: float
    by_type: Dict[str, float]
    by_chain: Dict[str, float]
    by_token: Dict[str, float]


@dataclass
class LiquidityData:
    total_liquidity: float
    by_pool: Dict[str, float]
    concentration: float
    utilization: float


@dataclass
class PortfolioPosition:
    token: str
    amount: float
    value: float
    pnl: float
    pnl_percent: float


@dataclass
class PortfolioSummary:
    total_value: float
    total_pnl: float
    total_pnl_percent: float
    positions: List[PortfolioPosition]
    allocation: Dict[str, float]


@dataclass
class BotPerformance:
    bot_id: str
    bot_name: str
    total_pnl: float
    roi: float
    win_rate: float
    trades: int
    avg_trade: float
    sharpe_ratio: float
    max_drawdown: float


# ============================================================================
# Analytics Engine
# ============================================================================

class AnalyticsEngine:
    """Core analytics engine"""
    
    def __init__(self):
        self.metrics: Dict[str, List[Metric]] = {}
        self.transactions: List[Dict] = []
        self.users: Dict[str, Dict] = {}
        self.pools: Dict[str, Dict] = {}
        
    def record_transaction(self, tx: Dict[str, Any]) -> None:
        """Record a transaction for analytics"""
        self.transactions.append({
            **tx,
            'timestamp': datetime.now()
        })
        
    def record_metric(self, metric_type: MetricType, value: float) -> None:
        """Record a metric value"""
        key = metric_type.value
        if key not in self.metrics:
            self.metrics[key] = []
            
        change_24h = self._calculate_24h_change(key, value)
        change_percent = (change_24h / (value - change_24h) * 100) if value > change_24h else 0
        
        self.metrics[key].append(Metric(
            value=value,
            change_24h=change_24h,
            change_percent=change_percent,
            timestamp=datetime.now()
        ))
        
    def get_tvl(self) -> TVLData:
        """Get TVL data"""
        tvl_by_chain = {}
        tvl_by_protocol = {}
        total_tvl = 0.0
        
        for tx in self.transactions:
            chain = tx.get('chain', 'unknown')
            protocol = tx.get('protocol', 'unknown')
            value = tx.get('value', 0)
            
            tvl_by_chain[chain] = tvl_by_chain.get(chain, 0) + value
            tvl_by_protocol[protocol] = tvl_by_protocol.get(protocol, 0) + value
            total_tvl += value
            
        return TVLData(
            total_tvl=total_tvl,
            change_24h=0,
            by_chain=tvl_by_chain,
            by_protocol=tvl_by_protocol
        )
        
    def get_volume(self) -> VolumeData:
        """Get volume data"""
        volume_by_chain = {}
        volume_by_dex = {}
        total_volume = 0.0
        buy_volume = 0.0
        sell_volume = 0.0
        trades = 0
        
        for tx in self.transactions:
            if tx.get('type') != 'swap':
                continue
                
            chain = tx.get('chain', 'unknown')
            dex = tx.get('dex', 'unknown')
            value = tx.get('value', 0)
            side = tx.get('side', 'buy')
            
            volume_by_chain[chain] = volume_by_chain.get(chain, 0) + value
            volume_by_dex[dex] = volume_by_dex.get(dex, 0) + value
            
            if side == 'buy':
                buy_volume += value
            else:
                sell_volume += value
                
            total_volume += value
            trades += 1
            
        return VolumeData(
            total_volume=total_volume,
            buy_volume=buy_volume,
            sell_volume=sell_volume,
            by_chain=volume_by_chain,
            by_dex=volume_by_dex,
            trades=trades
        )
        
    def get_revenue(self) -> RevenueData:
        """Get revenue data"""
        revenue_by_source = {}
        total_revenue = 0.0
        protocol_fees = 0.0
        liquidity_fees = 0.0
        partner_fees = 0.0
        
        for tx in self.transactions:
            fee = tx.get('fee', 0)
            fee_type = tx.get('fee_type', 'protocol')
            
            revenue_by_source[fee_type] = revenue_by_source.get(fee_type, 0) + fee
            total_revenue += fee
            
            if fee_type == 'protocol':
                protocol_fees += fee
            elif fee_type == 'liquidity':
                liquidity_fees += fee
            elif fee_type == 'partner':
                partner_fees += fee
                
        return RevenueData(
            total_revenue=total_revenue,
            protocol_fees=protocol_fees,
            liquidity_fees=liquidity_fees,
            partner_fees=partner_fees,
            by_source=revenue_by_source
        )
        
    def get_users(self) -> UserData:
        """Get user data"""
        total_users = len(self.users)
        new_users_24h = 0
        active_users = 0
        
        # Calculate new users in last 24h
        cutoff = datetime.now() - timedelta(hours=24)
        for user_id, user_data in self.users.items():
            created = user_data.get('created_at', datetime.now())
            if created > cutoff:
                new_users_24h += 1
                
        # Active users (last 30 days)
        cutoff = datetime.now() - timedelta(days=30)
        for user_id, user_data in self.users.items():
            last_active = user_data.get('last_active', datetime.now())
            if last_active > cutoff:
                active_users += 1
                
        # Calculate DAU/MAU (simplified)
        dau = active_users // 30
        mau = total_users
        
        # Retention rate
        retention = ((total_users - new_users_24h) / total_users * 100) if total_users > 0 else 0
        
        return UserData(
            total_users=total_users,
            new_users_24h=new_users_24h,
            active_users=active_users,
            dau=dau,
            mau=mau,
            retention_rate=retention
        )
        
    def _calculate_24h_change(self, metric_type: str, current_value: float) -> float:
        """Calculate 24h change for a metric"""
        cutoff = datetime.now() - timedelta(hours=24)
        history = self.metrics.get(metric_type, [])
        
        # Find last value before cutoff
        for metric in reversed(history):
            if metric.timestamp < cutoff:
                return metric.value
                
        return 0


# ============================================================================
# Portfolio Analytics
# ============================================================================

class PortfolioAnalytics:
    """Portfolio tracking and analytics"""
    
    def __init__(self):
        self.positions: Dict[str, PortfolioPosition] = {}
        self.history: List[Dict] = []
        
    def add_position(self, token: str, amount: float, value: float, entry_price: float) -> None:
        """Add or update a position"""
        current_price = entry_price  # In production, fetch real price
        current_value = amount * current_price
        pnl = current_value - (amount * entry_price)
        pnl_percent = (pnl / (amount * entry_price) * 100) if amount > 0 else 0
        
        self.positions[token] = PortfolioPosition(
            token=token,
            amount=amount,
            value=current_value,
            pnl=pnl,
            pnl_percent=pnl_percent
        )
        
    def get_summary(self) -> PortfolioSummary:
        """Get portfolio summary"""
        total_value = 0.0
        total_pnl = 0.0
        allocation = {}
        
        positions = list(self.positions.values())
        
        for position in positions:
            total_value += position.value
            total_pnl += position.pnl
            
        for position in positions:
            if total_value > 0:
                allocation[position.token] = position.value / total_value * 100
                
        total_pnl_percent = (total_pnl / (total_value - total_pnl) * 100) if total_value > total_pnl else 0
        
        return PortfolioSummary(
            total_value=total_value,
            total_pnl=total_pnl,
            total_pnl_percent=total_pnl_percent,
            positions=positions,
            allocation=allocation
        )
        
    def calculate_tax_loss_harvest(self, token: str, current_price: float) -> Optional[Dict]:
        """Calculate tax loss harvesting opportunity"""
        position = self.positions.get(token)
        if not position:
            return None
            
        cost_basis = position.amount * (position.value - position.pnl)
        current_value = position.amount * current_price
        unrealized_loss = current_value - cost_basis
        
        if unrealized_loss > 0:
            return {
                'token': token,
                'unrealized_loss': unrealized_loss,
                'potential_tax_savings': unrealized_loss * 0.3,  # Assume 30% tax rate
                'recommendation': 'harvest'
            }
            
        return None


# ============================================================================
# Protocol Analytics
# ============================================================================

class ProtocolAnalytics:
    """Protocol-level analytics"""
    
    def __init__(self):
        self.pairs: Dict[str, Dict] = {}
        self.tokens: Dict[str, Dict] = {}
        self.fees_by_type: Dict[str, float] = {}
        
    def track_pair(self, pair: str, data: Dict) -> None:
        """Track trading pair"""
        if pair not in self.pairs:
            self.pairs[pair] = {
                'volume': 0,
                'trades': 0,
                'fees': 0,
                'liquidity': 0
            }
            
        self.pairs[pair]['volume'] += data.get('volume', 0)
        self.pairs[pair]['trades'] += 1
        self.pairs[pair]['fees'] += data.get('fee', 0)
        
    def get_top_pairs(self, limit: int = 10) -> List[Dict]:
        """Get top trading pairs"""
        pairs = [
            {**data, 'pair': pair}
            for pair, data in self.pairs.items()
        ]
        pairs.sort(key=lambda x: x['volume'], reverse=True)
        return pairs[:limit]
        
    def get_token_analytics(self, token: str) -> Dict:
        """Get token-specific analytics"""
        return self.tokens.get(token, {
            'volume_24h': 0,
            'traders': 0,
            'price_change': 0,
            'liquidity': 0
        })


# ============================================================================
# Liquidity Analytics
# ============================================================================

class LiquidityAnalytics:
    """Liquidity pool analytics"""
    
    def __init__(self):
        self.pools: Dict[str, LiquidityData] = {}
        
    def add_pool(self, pool_id: str, liquidity: float, token_a: str, token_b: str) -> None:
        """Add liquidity pool"""
        self.pools[pool_id] = LiquidityData(
            total_liquidity=liquidity,
            by_pool={pool_id: liquidity},
            concentration=0,
            utilization=0
        )
        
    def get_liquidity_heatmap(self) -> Dict[str, float]:
        """Get liquidity concentration heatmap"""
        heatmap = {}
        
        for pool_id, data in self.pools.items():
            heatmap[pool_id] = data.total_liquidity
            
        return heatmap
        
    def calculate_concentration(self) -> float:
        """Calculate liquidity concentration (HHI)"""
        if not self.pools:
            return 0
            
        total = sum(p.total_liquidity for p in self.pools.values())
        if total == 0:
            return 0
            
        hhi = 0
        for pool in self.pools.values():
            share = (pool.total_liquidity / total) * 100
            hhi += share ** 2
            
        return hhi


# ============================================================================
# Bot Analytics
# ============================================================================

class BotAnalytics:
    """Trading bot analytics"""
    
    def __init__(self):
        self.bots: Dict[str, BotPerformance] = {}
        self.trades: Dict[str, List[Dict]] = {}
        
    def register_bot(self, bot_id: str, bot_name: str) -> None:
        """Register a new bot"""
        self.bots[bot_id] = BotPerformance(
            bot_id=bot_id,
            bot_name=bot_name,
            total_pnl=0,
            roi=0,
            win_rate=0,
            trades=0,
            avg_trade=0,
            sharpe_ratio=0,
            max_drawdown=0
        )
        self.trades[bot_id] = []
        
    def record_trade(self, bot_id: str, trade: Dict) -> None:
        """Record a trade for a bot"""
        if bot_id not in self.trades:
            self.trades[bot_id] = []
            
        self.trades[bot_id].append(trade)
        
    def calculate_performance(self, bot_id: str) -> BotPerformance:
        """Calculate bot performance metrics"""
        trades = self.trades.get(bot_id, [])
        
        if not trades:
            return self.bots.get(bot_id, BotPerformance(
                bot_id=bot_id,
                bot_name='',
                total_pnl=0,
                roi=0,
                win_rate=0,
                trades=0,
                avg_trade=0,
                sharpe_ratio=0,
                max_drawdown=0
            ))
            
        wins = 0
        total_pnl = 0.0
        trade_pnls = []
        
        for trade in trades:
            pnl = trade.get('pnl', 0)
            total_pnl += pnl
            
            if pnl > 0:
                wins += 1
                
            trade_pnls.append(pnl)
            
        trades_count = len(trades)
        win_rate = (wins / trades_count * 100) if trades_count > 0 else 0
        avg_trade = total_pnl / trades_count if trades_count > 0 else 0
        
        # Calculate ROI (simplified)
        initial_capital = 10000  # Assume $10k initial
        roi = (total_pnl / initial_capital * 100) if initial_capital > 0 else 0
        
        # Calculate Sharpe ratio (simplified)
        if len(trade_pnls) > 1:
            avg = sum(trade_pnls) / len(trade_pnls)
            variance = sum((x - avg) ** 2 for x in trade_pnls) / len(trade_pnls)
            std_dev = variance ** 0.5
            sharpe_ratio = (avg / std_dev * (252 ** 0.5)) if std_dev > 0 else 0
        else:
            sharpe_ratio = 0
            
        # Calculate max drawdown
        peak = 0
        max_dd = 0
        running = 0
        
        for pnl in trade_pnls:
            running += pnl
            if running > peak:
                peak = running
            dd = peak - running
            if dd > max_dd:
                max_dd = dd
                
        return BotPerformance(
            bot_id=bot_id,
            bot_name=self.bots[bot_id].bot_name if bot_id in self.bots else '',
            total_pnl=total_pnl,
            roi=roi,
            win_rate=win_rate,
            trades=trades_count,
            avg_trade=avg_trade,
            sharpe_ratio=sharpe_ratio,
            max_drawdown=max_dd
        )
        
    def get_leaderboard(self, limit: int = 10) -> List[BotPerformance]:
        """Get bot performance leaderboard"""
        leaderboard = []
        
        for bot_id in self.bots.keys():
            perf = self.calculate_performance(bot_id)
            leaderboard.append(perf)
            
        leaderboard.sort(key=lambda x: x.total_pnl, reverse=True)
        return leaderboard[:limit]


# ============================================================================
# Fee Analytics
# ============================================================================

class FeeAnalytics:
    """Fee analytics and reporting"""
    
    def __init__(self):
        self.fees: Dict[str, FeeData] = {}
        
    def record_fee(self, fee_type: str, amount: float, chain: str, token: str) -> None:
        """Record a fee"""
        key = f"{chain}:{token}"
        
        if key not in self.fees:
            self.fees[key] = FeeData(
                total_fees=0,
                by_type={},
                by_chain={},
                by_token={}
            )
            
        self.fees[key].total_fees += amount
        self.fees[key].by_type[fee_type] = self.fees[key].by_type.get(fee_type, 0) + amount
        self.fees[key].by_chain[chain] = self.fees[key].by_chain.get(chain, 0) + amount
        self.fees[key].by_token[token] = self.fees[key].by_token.get(token, 0) + amount
        
    def get_fee_summary(self) -> FeeData:
        """Get total fee summary"""
        total_fees = 0
        by_type = {}
        by_chain = {}
        by_token = {}
        
        for fee_data in self.fees.values():
            total_fees += fee_data.total_fees
            
            for ft, amount in fee_data.by_type.items():
                by_type[ft] = by_type.get(ft, 0) + amount
                
            for chain, amount in fee_data.by_chain.items():
                by_chain[chain] = by_chain.get(chain, 0) + amount
                
            for token, amount in fee_data.by_token.items():
                by_token[token] = by_token.get(token, 0) + amount
                
        return FeeData(
            total_fees=total_fees,
            by_type=by_type,
            by_chain=by_chain,
            by_token=by_token
        )


# ============================================================================
# Chain Analytics
# ============================================================================

class ChainAnalytics:
    """Per-chain analytics"""
    
    def __init__(self):
        self.chain_data: Dict[str, Dict] = {}
        
    def record_chain_activity(self, chain: str, data: Dict) -> None:
        """Record activity for a chain"""
        if chain not in self.chain_data:
            self.chain_data[chain] = {
                'volume': 0,
                'transactions': 0,
                'gas_spent': 0,
                'unique_users': set()
            }
            
        self.chain_data[chain]['volume'] += data.get('volume', 0)
        self.chain_data[chain]['transactions'] += 1
        self.chain_data[chain]['gas_spent'] += data.get('gas', 0)
        self.chain_data[chain]['unique_users'].add(data.get('user', ''))
        
    def get_chain_report(self) -> Dict[str, Dict]:
        """Get report for all chains"""
        report = {}
        
        for chain, data in self.chain_data.items():
            report[chain] = {
                'volume': data['volume'],
                'transactions': data['transactions'],
                'gas_spent': data['gas_spent'],
                'unique_users': len(data['unique_users'])
            }
            
        return report


# ============================================================================
# User Analytics
# ============================================================================

class UserAnalytics:
    """User behavior analytics"""
    
    def __init__(self):
        self.user_events: Dict[str, List[Dict]] = {}
        
    def track_event(self, user_id: str, event: str, data: Dict) -> None:
        """Track user event"""
        if user_id not in self.user_events:
            self.user_events[user_id] = []
            
        self.user_events[user_id].append({
            'event': event,
            'data': data,
            'timestamp': datetime.now()
        })
        
    def get_user_funnel(self) -> Dict[str, int]:
        """Calculate user acquisition funnel"""
        funnels = {
            'visits': 0,
            'signups': 0,
            'deposits': 0,
            'trades': 0,
            'retained': 0
        }
        
        for user_id, events in self.user_events.items():
            event_types = [e['event'] for e in events]
            
            if 'visit' in event_types:
                funnels['visits'] += 1
            if 'signup' in event_types:
                funnels['signups'] += 1
            if 'deposit' in event_types:
                funnels['deposits'] += 1
            if 'trade' in event_types:
                funnels['trades'] += 1
            if len(events) > 5:
                funnels['retained'] += 1
                
        return funnels
        
    def get_cohort_retention(self, cohort_date: datetime) -> Dict[str, float]:
        """Calculate cohort retention"""
        # Simplified cohort analysis
        return {
            'day_1': 0,
            'day_7': 0,
            'day_30': 0
        }


# ============================================================================
# Export
# ============================================================================

__all__ = [
    'MetricType',
    'TimeRange',
    'Metric',
    'TVLData',
    'VolumeData',
    'RevenueData',
    'UserData',
    'FeeData',
    'LiquidityData',
    'PortfolioPosition',
    'PortfolioSummary',
    'BotPerformance',
    'AnalyticsEngine',
    'PortfolioAnalytics',
    'ProtocolAnalytics',
    'LiquidityAnalytics',
    'BotAnalytics',
    'FeeAnalytics',
    'ChainAnalytics',
    'UserAnalytics',
]