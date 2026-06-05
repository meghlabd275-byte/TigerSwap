"""
TigerSwap Admin API Service
Complete administration for DEXs, CEXs, HD Wallets, and all platform operations
"""

import asyncio
import hashlib
import json
import time
from dataclasses import dataclass, field
from typing import Dict, List, Optional, Set, Any
from enum import Enum
from datetime import datetime, timedelta

# ============================================================================
# Admin Models
# ============================================================================

class AdminRole(Enum):
    SUPER_ADMIN = "super_admin"
    DEX_ADMIN = "dex_admin"
    CEX_ADMIN = "cex_admin"
    TRADING_ADMIN = "trading_admin"
    FINANCE_ADMIN = "finance_admin"
    SUPPORT_ADMIN = "support_admin"

class AdminAction(Enum):
    # DEX Management
    DEX_CREATE = "dex:create"
    DEX_MODIFY = "dex:modify"
    DEX_SUSPEND = "dex:suspend"
    DEX_VIEW_STATS = "dex:view_stats"
    
    # CEX Management
    CEX_CONNECT = "cex:connect"
    CEX_DISCONNECT = "cex:disconnect"
    CEX_MODIFY = "cex:modify"
    CEX_VIEW_BALANCES = "cex:view_balances"
    CEX_TRADE = "cex:trade"
    
    # Wallet Management
    WALLET_CREATE = "wallet:create"
    WALLET_VIEW = "wallet:view"
    WALLET_TRANSFER = "wallet:transfer"
    WALLET_SIGN = "wallet:sign"
    
    # User Management
    USER_VIEW = "user:view"
    USER_FREEZE = "user:freeze"
    USER_KYC = "user:kyc"
    
    # Financial
    FINANCE_VIEW = "finance:view"
    FINANCE_WITHDRAW = "finance:withdraw"
    FINANCE_FEE_ADJUST = "finance:fee_adjust"
    
    # System
    SYSTEM_CONFIG = "system:config"
    SYSTEM_VIEW_LOGS = "system:view_logs"

@dataclass
class Admin:
    id: str
    username: str
    email: str
    password_hash: str
    role: AdminRole
    permissions: Set[AdminAction] = field(default_factory=set)
    is_active: bool = True
    last_login: Optional[int] = None
    created_at: int = 0
    mfa_enabled: bool = False

@dataclass
class ConnectedDEX:
    id: str
    name: str
    slug: str
    chain_id: int
    status: str  # "active", "suspended", "maintenance"
    
    # Connection details
    api_endpoint: str
    subgraph_url: Optional[str] = None
    router_address: Optional[str] = None
    factory_address: Optional[str] = None
    
    # Stats
    total_volume_24h: float = 0.0
    total_volume_7d: float = 0.0
    total_trades: int = 0
    avg_latency_ms: float = 0.0
    
    # Fees
    trading_fee: float = 0.003  # 0.3%
    platform_fee_share: float = 0.1  # 10% goes to platform
    
    # Status
    last_health_check: int = 0
    health_status: str = "unknown"
    errors: List[str] = field(default_factory=list)
    
    # Permissions
    supported_tokens: List[str] = field(default_factory=list)
    supported_chains: List[int] = field(default_factory=list)
    max_slippage: float = 0.01  # 1%
    
    created_at: int = 0
    updated_at: int = 0

@dataclass
class ConnectedCEX:
    id: str
    name: str
    exchange_type: str  # "binance", "coinbase", "kraken", "okx", etc.
    status: str  # "connected", "disconnected", "error", "syncing"
    
    # API Configuration (encrypted)
    api_key_encrypted: Optional[bytes] = None
    api_secret_encrypted: Optional[bytes] = None
    passphrase_encrypted: Optional[bytes] = None  # For some exchanges
    
    # Account info
    account_id: str
    account_type: str  # "spot", "margin", "futures"
    sub_accounts: List[str] = field(default_factory=list)
    
    # Permissions
    can_trade: bool = True
    can_withdraw: bool = False
    can_deposit: bool = False
    max_daily_volume: float = 0.0  # 0 = unlimited
    
    # Balances
    total_balance_usd: float = 0.0
    available_balance_usd: float = 0.0
    locked_balance_usd: float = 0.0
    
    # Rate limits
    requests_per_second: int = 10
    requests_per_minute: int = 120
    
    # Status
    last_sync: int = 0
    last_trade: int = 0
    health_status: str = "unknown"
    errors: List[str] = field(default_factory=list)
    
    # Fees
    maker_fee: float = 0.001
    taker_fee: float = 0.001
    
    created_at: int = 0
    updated_at: int = 0

@dataclass
class HDWallet:
    id: str
    name: str
    wallet_type: str  # "master", "derived", "imported"
    
    # Master key (encrypted)
    master_key_encrypted: Optional[bytes] = None
    public_key: Optional[str] = None
    
    # Derivation
    derivation_path: str = "m/44'/60'/0'/0"  # BIP44
    derived_keys_count: int = 0
    
    # Security
    security_level: str = "standard"  # "standard", "hardware", "multisig"
    requires_signatures: int = 1
    
    # Admin control
    admin_controlled: bool = True
    allowed_operations: List[str] = field(default_factory=list)  # "swap", "bridge", "transfer"
    operation_limits: Dict[str, float] = field(default_factory=dict)  # operation -> max value
    
    # Status
    total_balance_usd: float = 0.0
    is_active: bool = True
    created_at: int = 0
    updated_at: int = 0

@dataclass
class AuditLog:
    id: str
    admin_id: str
    action: AdminAction
    resource_type: str  # "dex", "cex", "wallet", "user", "system"
    resource_id: str
    details: Dict = field(default_factory=dict)
    ip_address: Optional[str] = None
    user_agent: Optional[str] = None
    timestamp: int = 0

@dataclass
class FeeStructure:
    id: str
    fee_type: str  # "trading", "withdrawal", "deposit", "bridge"
    
    # Fee details
    asset: str  # "ETH", "USDC", "ALL"
    chain_id: Optional[int] = None
    
    # Rates
    maker_fee: float = 0.0
    taker_fee: float = 0.0
    flat_fee: float = 0.0
    percentage_fee: float = 0.0  # e.g., 0.1% = 0.001
    
    # Volume tiers
    volume_tiers: List[Dict] = field(default_factory=list)  # [{"min_volume": 0, "fee": 0.001}]
    
    # Admin control
    is_active: bool = True
    updated_by: str = ""
    updated_at: int = 0

# ============================================================================
# Admin Service
# ============================================================================

class AdminService:
    """
    TigerSwap Admin Service for complete platform management.
    Handles DEX connections, CEX integrations, HD Wallets, and all admin operations.
    """
    
    def __init__(self):
        # Admins
        self.admins: Dict[str, Admin] = {}
        self.admin_by_email: Dict[str, str] = {}
        
        # Connected exchanges
        self.dexes: Dict[str, ConnectedDEX] = {}
        self.cexes: Dict[str, ConnectedCEX] = {}
        self.hd_wallets: Dict[str, HDWallet] = {}
        
        # Fee structures
        self.fee_structures: Dict[str, FeeStructure] = {}
        
        # Audit logs
        self.audit_logs: List[AuditLog] = []
        
        # Platform stats
        self.stats = {
            "total_dex_volume_24h": 0.0,
            "total_cex_volume_24h": 0.0,
            "total_users": 0,
            "total_trades_24h": 0,
            "total_fees_collected_24h": 0.0,
        }
        
        # Initialize default admin
        self._initialize_default_admin()
        
        # Initialize default DEX connections
        self._initialize_default_dexes()
        
        # Initialize fee structures
        self._initialize_fee_structures()
    
    def _initialize_default_admin(self):
        """Create default super admin"""
        admin = Admin(
            id="admin_001",
            username="tigerswap_admin",
            email="admin@tigerswap.io",
            password_hash=hashlib.sha256("changeme".encode()).hexdigest(),
            role=AdminRole.SUPER_ADMIN,
            permissions=set(AdminAction),  # All permissions
            created_at=int(time.time())
        )
        self.admins[admin.id] = admin
        self.admin_by_email[admin.email] = admin.id
    
    def _initialize_default_dexes(self):
        """Initialize default DEX connections"""
        dexes = [
            ConnectedDEX(
                id="dex_uniswap_v2",
                name="Uniswap V2",
                slug="uniswap-v2",
                chain_id=1,
                status="active",
                api_endpoint="https://api.thegraph.com/subgraphs/name/uniswap/uniswap-v2",
                subgraph_url="https://api.thegraph.com/subgraphs/name/uniswap/uniswap-v2",
                router_address="0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D",
                factory_address="0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f",
                total_volume_24h=150_000_000,
                total_volume_7d=1_000_000_000,
                total_trades=500_000,
                avg_latency_ms=45,
                supported_chains=[1],
                created_at=int(time.time()),
                updated_at=int(time.time())
            ),
            ConnectedDEX(
                id="dex_uniswap_v3",
                name="Uniswap V3",
                slug="uniswap-v3",
                chain_id=1,
                status="active",
                api_endpoint="https://api.thegraph.com/subgraphs/name/uniswap/uniswap-v3",
                subgraph_url="https://api.thegraph.com/subgraphs/name/uniswap/uniswap-v3",
                router_address="0xE592427A0AEce92De3Edee1F18E0157C05861564",
                factory_address="0x1F98431c8aD98523631AE4a59f267346ea31F984",
                total_volume_24h=250_000_000,
                total_volume_7d=1_800_000_000,
                total_trades=800_000,
                avg_latency_ms=55,
                supported_chains=[1, 42161],
                created_at=int(time.time()),
                updated_at=int(time.time())
            ),
            ConnectedDEX(
                id="dex_sushiswap",
                name="SushiSwap",
                slug="sushiswap",
                chain_id=1,
                status="active",
                api_endpoint="https://api.thegraph.com/subgraphs/name/sushiswap/exchange",
                subgraph_url="https://api.thegraph.com/subgraphs/name/sushiswap/exchange",
                router_address="0xd9e1cE17f2641f24aE83637ab66a2cca9C378B9F",
                factory_address="0xC0AEe478e3658e2610c5F7A4A2E1777cE9e4f2Ac",
                total_volume_24h=50_000_000,
                total_volume_7d=350_000_000,
                total_trades=200_000,
                avg_latency_ms=60,
                supported_chains=[1, 56, 42161],
                created_at=int(time.time()),
                updated_at=int(time.time())
            ),
            ConnectedDEX(
                id="dex_pancakeswap",
                name="PancakeSwap",
                slug="pancakeswap",
                chain_id=56,
                status="active",
                api_endpoint="https://bsc.streamingfast.io/subgraphs/name/pancakeswap/exchange-v2",
                subgraph_url="https://bsc.streamingfast.io/subgraphs/name/pancakeswap/exchange-v2",
                router_address="0x10ED43C718714eb63d5aA57B78B54704E256024E",
                factory_address="0x109705B3Dc5dCA62a5d48F27d94E8E8dB669F12d",
                total_volume_24h=80_000_000,
                total_volume_7d=550_000_000,
                total_trades=300_000,
                avg_latency_ms=40,
                supported_chains=[56],
                created_at=int(time.time()),
                updated_at=int(time.time())
            ),
        ]
        
        for dex in dexes:
            self.dexes[dex.id] = dex
    
    def _initialize_fee_structures(self):
        """Initialize default fee structures"""
        fees = [
            FeeStructure(
                id="fee_trading_default",
                fee_type="trading",
                asset="ALL",
                chain_id=1,
                maker_fee=0.003,
                taker_fee=0.003,
                volume_tiers=[
                    {"min_volume": 0, "maker_fee": 0.003, "taker_fee": 0.003},
                    {"min_volume": 100_000, "maker_fee": 0.002, "taker_fee": 0.0025},
                    {"min_volume": 1_000_000, "maker_fee": 0.001, "taker_fee": 0.002},
                    {"min_volume": 10_000_000, "maker_fee": 0.0005, "taker_fee": 0.001},
                ],
                updated_at=int(time.time())
            ),
            FeeStructure(
                id="fee_cex_api",
                fee_type="api_access",
                asset="ALL",
                maker_fee=0,
                taker_fee=0,
                flat_fee=5000,  # $5000/month
                percentage_fee=0,
                updated_at=int(time.time())
            ),
        ]
        
        for fee in fees:
            self.fee_structures[fee.id] = fee
    
    # ==================== Admin Authentication ====================
    
    def authenticate_admin(
        self,
        email: str,
        password: str,
        mfa_code: Optional[str] = None
    ) -> Optional[Admin]:
        """Authenticate admin user"""
        admin_id = self.admin_by_email.get(email)
        if not admin_id:
            return None
        
        admin = self.admins.get(admin_id)
        if not admin or not admin.is_active:
            return None
        
        # Verify password
        password_hash = hashlib.sha256(password.encode()).hexdigest()
        if password_hash != admin.password_hash:
            return None
        
        # Check MFA if enabled
        if admin.mfa_enabled:
            if not mfa_code or not self._verify_mfa(admin.id, mfa_code):
                return None
        
        # Update last login
        admin.last_login = int(time.time())
        
        # Log authentication
        self._log_action(
            admin_id=admin.id,
            action=AdminAction.SYSTEM_VIEW_LOGS,
            resource_type="admin",
            resource_id=admin.id,
            details={"event": "login"}
        )
        
        return admin
    
    def has_permission(self, admin: Admin, action: AdminAction) -> bool:
        """Check if admin has specific permission"""
        return action in admin.permissions
    
    def _verify_mfa(self, admin_id: str, code: str) -> bool:
        """Verify MFA code (in production, use proper TOTP)"""
        # Simplified - in production use pyotp or similar
        return len(code) == 6 and code.isdigit()
    
    # ==================== DEX Management ====================
    
    def list_dexes(
        self,
        admin: Admin,
        status_filter: Optional[str] = None,
        chain_filter: Optional[int] = None
    ) -> List[Dict]:
        """List all connected DEXs"""
        if not self.has_permission(admin, AdminAction.DEX_VIEW_STATS):
            raise PermissionError("Not authorized to view DEXs")
        
        result = []
        for dex in self.dexes.values():
            if status_filter and dex.status != status_filter:
                continue
            if chain_filter and dex.chain_id != chain_filter:
                continue
            result.append(self._dex_to_dict(dex))
        
        return result
    
    def get_dex_stats(self, admin: Admin, dex_id: str) -> Dict:
        """Get detailed stats for a DEX"""
        if not self.has_permission(admin, AdminAction.DEX_VIEW_STATS):
            raise PermissionError("Not authorized to view DEX stats")
        
        dex = self.dexes.get(dex_id)
        if not dex:
            raise ValueError(f"DEX {dex_id} not found")
        
        return {
            **self._dex_to_dict(dex),
            "detailed_stats": {
                "volume_by_chain": {dex.chain_id: dex.total_volume_24h},
                "volume_by_token": {},  # Would be populated from DB
                "latency_history": [],  # Would be populated from monitoring
                "error_log": dex.errors[-10:] if dex.errors else [],
            }
        }
    
    def modify_dex(
        self,
        admin: Admin,
        dex_id: str,
        updates: Dict
    ) -> ConnectedDEX:
        """Modify DEX configuration"""
        if not self.has_permission(admin, AdminAction.DEX_MODIFY):
            raise PermissionError("Not authorized to modify DEXs")
        
        dex = self.dexes.get(dex_id)
        if not dex:
            raise ValueError(f"DEX {dex_id} not found")
        
        # Allowed updates
        allowed_fields = [
            "status", "trading_fee", "platform_fee_share",
            "max_slippage", "supported_tokens"
        ]
        
        for key, value in updates.items():
            if key in allowed_fields and hasattr(dex, key):
                setattr(dex, key, value)
        
        dex.updated_at = int(time.time())
        
        self._log_action(
            admin_id=admin.id,
            action=AdminAction.DEX_MODIFY,
            resource_type="dex",
            resource_id=dex_id,
            details=updates
        )
        
        return dex
    
    def suspend_dex(self, admin: Admin, dex_id: str, reason: str) -> bool:
        """Suspend a DEX connection"""
        if not self.has_permission(admin, AdminAction.DEX_SUSPEND):
            raise PermissionError("Not authorized to suspend DEXs")
        
        dex = self.dexes.get(dex_id)
        if not dex:
            raise ValueError(f"DEX {dex_id} not found")
        
        dex.status = "suspended"
        dex.updated_at = int(time.time())
        
        self._log_action(
            admin_id=admin.id,
            action=AdminAction.DEX_SUSPEND,
            resource_type="dex",
            resource_id=dex_id,
            details={"reason": reason}
        )
        
        return True
    
    # ==================== CEX Management ====================
    
    def list_cexes(
        self,
        admin: Admin,
        status_filter: Optional[str] = None
    ) -> List[Dict]:
        """List all connected CEXs"""
        if not self.has_permission(admin, AdminAction.CEX_VIEW_BALANCES):
            raise PermissionError("Not authorized to view CEXs")
        
        result = []
        for cex in self.cexes.values():
            if status_filter and cex.status != status_filter:
                continue
            result.append(self._cex_to_dict(cex))
        
        return result
    
    def connect_cex(
        self,
        admin: Admin,
        name: str,
        exchange_type: str,
        api_key: str,
        api_secret: str,
        account_id: str,
        permissions: Dict
    ) -> ConnectedCEX:
        """Connect a new CEX account"""
        if not self.has_permission(admin, AdminAction.CEX_CONNECT):
            raise PermissionError("Not authorized to connect CEXs")
        
        cex_id = f"cex_{exchange_type}_{int(time.time())}"
        
        cex = ConnectedCEX(
            id=cex_id,
            name=name,
            exchange_type=exchange_type,
            status="connected",
            api_key_encrypted=api_key.encode(),  # In production, encrypt properly
            api_secret_encrypted=api_secret.encode(),
            account_id=account_id,
            account_type=permissions.get("account_type", "spot"),
            can_trade=permissions.get("can_trade", True),
            can_withdraw=permissions.get("can_withdraw", False),
            can_deposit=permissions.get("can_deposit", False),
            max_daily_volume=permissions.get("max_daily_volume", 0),
            created_at=int(time.time()),
            updated_at=int(time.time())
        )
        
        self.cexes[cex_id] = cex
        
        self._log_action(
            admin_id=admin.id,
            action=AdminAction.CEX_CONNECT,
            resource_type="cex",
            resource_id=cex_id,
            details={
                "name": name,
                "exchange_type": exchange_type,
                "account_id": account_id
            }
        )
        
        return cex
    
    def disconnect_cex(self, admin: Admin, cex_id: str) -> bool:
        """Disconnect a CEX account"""
        if not self.has_permission(admin, AdminAction.CEX_DISCONNECT):
            raise PermissionError("Not authorized to disconnect CEXs")
        
        cex = self.cexes.get(cex_id)
        if not cex:
            raise ValueError(f"CEX {cex_id} not found")
        
        cex.status = "disconnected"
        cex.api_key_encrypted = None
        cex.api_secret_encrypted = None
        cex.updated_at = int(time.time())
        
        self._log_action(
            admin_id=admin.id,
            action=AdminAction.CEX_DISCONNECT,
            resource_type="cex",
            resource_id=cex_id,
            details={}
        )
        
        return True
    
    def execute_cex_trade(
        self,
        admin: Admin,
        cex_id: str,
        symbol: str,
        side: str,
        quantity: float,
        price: Optional[float] = None
    ) -> Dict:
        """Execute trade on connected CEX"""
        if not self.has_permission(admin, AdminAction.CEX_TRADE):
            raise PermissionError("Not authorized to trade on CEXs")
        
        cex = self.cexes.get(cex_id)
        if not cex:
            raise ValueError(f"CEX {cex_id} not found")
        
        if not cex.can_trade:
            raise ValueError("Trading not permitted on this CEX")
        
        if cex.status != "connected":
            raise ValueError(f"CEX not connected: {cex.status}")
        
        # Execute trade (in production, call exchange API)
        trade_result = {
            "success": True,
            "order_id": f"cex_order_{int(time.time())}",
            "symbol": symbol,
            "side": side,
            "quantity": quantity,
            "price": price,
            "executed_at": int(time.time())
        }
        
        cex.last_trade = int(time.time())
        cex.updated_at = int(time.time())
        
        self._log_action(
            admin_id=admin.id,
            action=AdminAction.CEX_TRADE,
            resource_type="cex",
            resource_id=cex_id,
            details=trade_result
        )
        
        return trade_result
    
    # ==================== HD Wallet Management ====================
    
    def list_wallets(
        self,
        admin: Admin,
        wallet_type: Optional[str] = None,
        status_filter: Optional[bool] = None
    ) -> List[Dict]:
        """List all HD Wallets"""
        if not self.has_permission(admin, AdminAction.WALLET_VIEW):
            raise PermissionError("Not authorized to view wallets")
        
        result = []
        for wallet in self.hd_wallets.values():
            if wallet_type and wallet.wallet_type != wallet_type:
                continue
            if status_filter is not None and wallet.is_active != status_filter:
                continue
            result.append(self._wallet_to_dict(wallet))
        
        return result
    
    def create_wallet(
        self,
        admin: Admin,
        name: str,
        wallet_type: str,
        derivation_path: str,
        security_level: str = "standard"
    ) -> HDWallet:
        """Create a new HD Wallet"""
        if not self.has_permission(admin, AdminAction.WALLET_CREATE):
            raise PermissionError("Not authorized to create wallets")
        
        wallet_id = f"wallet_{int(time.time())}"
        
        wallet = HDWallet(
            id=wallet_id,
            name=name,
            wallet_type=wallet_type,
            derivation_path=derivation_path,
            security_level=security_level,
            admin_controlled=True,
            allowed_operations=["swap", "bridge", "transfer"],
            operation_limits={},
            created_at=int(time.time()),
            updated_at=int(time.time())
        )
        
        self.hd_wallets[wallet_id] = wallet
        
        self._log_action(
            admin_id=admin.id,
            action=AdminAction.WALLET_CREATE,
            resource_type="wallet",
            resource_id=wallet_id,
            details={"name": name, "type": wallet_type}
        )
        
        return wallet
    
    def transfer_from_wallet(
        self,
        admin: Admin,
        wallet_id: str,
        to_address: str,
        token: str,
        amount: float,
        reason: str
    ) -> Dict:
        """Transfer funds from admin-controlled wallet"""
        if not self.has_permission(admin, AdminAction.WALLET_TRANSFER):
            raise PermissionError("Not authorized to transfer from wallets")
        
        wallet = self.hd_wallets.get(wallet_id)
        if not wallet:
            raise ValueError(f"Wallet {wallet_id} not found")
        
        if not wallet.is_active:
            raise ValueError("Wallet is not active")
        
        if "transfer" not in wallet.allowed_operations:
            raise ValueError("Transfer not allowed for this wallet")
        
        # Execute transfer (in production, sign and broadcast)
        tx_hash = "0x" + hashlib.sha256(f"{wallet_id}{to_address}{amount}{time.time()}".encode()).hexdigest()[:40]
        
        self._log_action(
            admin_id=admin.id,
            action=AdminAction.WALLET_TRANSFER,
            resource_type="wallet",
            resource_id=wallet_id,
            details={
                "to": to_address,
                "token": token,
                "amount": amount,
                "reason": reason,
                "tx_hash": tx_hash
            }
        )
        
        return {
            "success": True,
            "tx_hash": tx_hash,
            "wallet_id": wallet_id,
            "to": to_address,
            "token": token,
            "amount": amount,
            "executed_at": int(time.time())
        }
    
    def modify_wallet_permissions(
        self,
        admin: Admin,
        wallet_id: str,
        allowed_operations: List[str],
        operation_limits: Dict[str, float]
    ) -> HDWallet:
        """Modify wallet permissions"""
        if not self.has_permission(admin, AdminAction.WALLET_MODIFY if hasattr(AdminAction, 'WALLET_MODIFY') else AdminAction.WALLET_CREATE):
            raise PermissionError("Not authorized to modify wallet permissions")
        
        wallet = self.hd_wallets.get(wallet_id)
        if not wallet:
            raise ValueError(f"Wallet {wallet_id} not found")
        
        wallet.allowed_operations = allowed_operations
        wallet.operation_limits = operation_limits
        wallet.updated_at = int(time.time())
        
        self._log_action(
            admin_id=admin.id,
            action=AdminAction.WALLET_CREATE,  # Using as proxy
            resource_type="wallet",
            resource_id=wallet_id,
            details={"allowed_operations": allowed_operations, "limits": operation_limits}
        )
        
        return wallet
    
    # ==================== Fee Management ====================
    
    def get_fee_structure(
        self,
        admin: Admin,
        fee_type: str,
        asset: Optional[str] = None
    ) -> List[FeeStructure]:
        """Get fee structure for trading"""
        if not self.has_permission(admin, AdminAction.FINANCE_VIEW):
            raise PermissionError("Not authorized to view fees")
        
        result = []
        for fee in self.fee_structures.values():
            if fee.fee_type != fee_type:
                continue
            if asset and fee.asset != asset:
                continue
            result.append(fee)
        
        return result
    
    def modify_fee_structure(
        self,
        admin: Admin,
        fee_id: str,
        updates: Dict
    ) -> FeeStructure:
        """Modify fee structure"""
        if not self.has_permission(admin, AdminAction.FINANCE_FEE_ADJUST):
            raise PermissionError("Not authorized to modify fees")
        
        fee = self.fee_structures.get(fee_id)
        if not fee:
            raise ValueError(f"Fee structure {fee_id} not found")
        
        allowed_fields = [
            "maker_fee", "taker_fee", "flat_fee",
            "percentage_fee", "volume_tiers", "is_active"
        ]
        
        for key, value in updates.items():
            if key in allowed_fields and hasattr(fee, key):
                setattr(fee, key, value)
        
        fee.updated_by = admin.id
        fee.updated_at = int(time.time())
        
        self._log_action(
            admin_id=admin.id,
            action=AdminAction.FINANCE_FEE_ADJUST,
            resource_type="fee",
            resource_id=fee_id,
            details=updates
        )
        
        return fee
    
    # ==================== Platform Statistics ====================
    
    def get_platform_stats(self, admin: Admin) -> Dict:
        """Get overall platform statistics"""
        if not self.has_permission(admin, AdminAction.FINANCE_VIEW):
            raise PermissionError("Not authorized to view platform stats")
        
        return {
            **self.stats,
            "dex_count": len(self.dexes),
            "active_dex_count": sum(1 for d in self.dexes.values() if d.status == "active"),
            "cex_count": len(self.cexes),
            "active_cex_count": sum(1 for c in self.cexes.values() if c.status == "connected"),
            "wallet_count": len(self.hd_wallets),
            "active_wallet_count": sum(1 for w in self.hd_wallets.values() if w.is_active),
            "admin_count": len(self.admins),
            "timestamp": int(time.time())
        }
    
    # ==================== Audit Logs ====================
    
    def get_audit_logs(
        self,
        admin: Admin,
        resource_type: Optional[str] = None,
        admin_id: Optional[str] = None,
        limit: int = 100
    ) -> List[Dict]:
        """Get audit logs"""
        if not self.has_permission(admin, AdminAction.SYSTEM_VIEW_LOGS):
            raise PermissionError("Not authorized to view audit logs")
        
        logs = []
        for log in reversed(self.audit_logs[-limit:]):
            if resource_type and log.resource_type != resource_type:
                continue
            if admin_id and log.admin_id != admin_id:
                continue
            logs.append({
                "id": log.id,
                "admin_id": log.admin_id,
                "action": log.action.value,
                "resource_type": log.resource_type,
                "resource_id": log.resource_id,
                "details": log.details,
                "timestamp": log.timestamp
            })
        
        return logs
    
    # ==================== Helper Methods ====================
    
    def _log_action(
        self,
        admin_id: str,
        action: AdminAction,
        resource_type: str,
        resource_id: str,
        details: Dict
    ):
        """Create audit log entry"""
        log = AuditLog(
            id=f"log_{len(self.audit_logs)}_{int(time.time())}",
            admin_id=admin_id,
            action=action,
            resource_type=resource_type,
            resource_id=resource_id,
            details=details,
            timestamp=int(time.time())
        )
        self.audit_logs.append(log)
        
        # Keep only last 10000 logs
        if len(self.audit_logs) > 10000:
            self.audit_logs = self.audit_logs[-10000:]
    
    def _dex_to_dict(self, dex: ConnectedDEX) -> Dict:
        """Convert DEX to dictionary"""
        return {
            "id": dex.id,
            "name": dex.name,
            "slug": dex.slug,
            "chain_id": dex.chain_id,
            "status": dex.status,
            "total_volume_24h": dex.total_volume_24h,
            "total_volume_7d": dex.total_volume_7d,
            "total_trades": dex.total_trades,
            "avg_latency_ms": dex.avg_latency_ms,
            "trading_fee": dex.trading_fee,
            "supported_chains": dex.supported_chains,
            "health_status": dex.health_status,
            "last_health_check": dex.last_health_check,
            "created_at": dex.created_at
        }
    
    def _cex_to_dict(self, cex: ConnectedCEX) -> Dict:
        """Convert CEX to dictionary"""
        return {
            "id": cex.id,
            "name": cex.name,
            "exchange_type": cex.exchange_type,
            "status": cex.status,
            "account_id": cex.account_id,
            "account_type": cex.account_type,
            "total_balance_usd": cex.total_balance_usd,
            "available_balance_usd": cex.available_balance_usd,
            "can_trade": cex.can_trade,
            "can_withdraw": cex.can_withdraw,
            "last_sync": cex.last_sync,
            "health_status": cex.health_status,
            "created_at": cex.created_at
        }
    
    def _wallet_to_dict(self, wallet: HDWallet) -> Dict:
        """Convert Wallet to dictionary"""
        return {
            "id": wallet.id,
            "name": wallet.name,
            "wallet_type": wallet.wallet_type,
            "derivation_path": wallet.derivation_path,
            "security_level": wallet.security_level,
            "total_balance_usd": wallet.total_balance_usd,
            "is_active": wallet.is_active,
            "allowed_operations": wallet.allowed_operations,
            "created_at": wallet.created_at
        }


# ============================================================================
# API Service (REST API)
# ============================================================================

class AdminAPIService:
    """
    REST API service for TigerSwap Admin Platform.
    Provides endpoints for all admin operations.
    """
    
    def __init__(self):
        self.admin_service = AdminService()
        self._running = False
    
    async def start(self, host: str = "0.0.0.0", port: int = 8081):
        """Start the admin API server"""
        from aiohttp import web
        
        app = web.Application()
        
        # Add routes
        app.router.add_post('/api/admin/login', self.handle_login)
        app.router.add_get('/api/admin/stats', self.handle_stats)
        
        # DEX routes
        app.router.add_get('/api/admin/dexes', self.handle_list_dexes)
        app.router.add_get('/api/admin/dexes/{dex_id}', self.handle_get_dex)
        app.router.add_put('/api/admin/dexes/{dex_id}', self.handle_modify_dex)
        app.router.add_post('/api/admin/dexes/{dex_id}/suspend', self.handle_suspend_dex)
        
        # CEX routes
        app.router.add_get('/api/admin/cexes', self.handle_list_cexes)
        app.router.add_post('/api/admin/cexes', self.handle_connect_cex)
        app.router.add_delete('/api/admin/cexes/{cex_id}', self.handle_disconnect_cex)
        app.router.add_post('/api/admin/cexes/{cex_id}/trade', self.handle_cex_trade)
        
        # Wallet routes
        app.router.add_get('/api/admin/wallets', self.handle_list_wallets)
        app.router.add_post('/api/admin/wallets', self.handle_create_wallet)
        app.router.add_post('/api/admin/wallets/{wallet_id}/transfer', self.handle_wallet_transfer)
        
        # Fee routes
        app.router.add_get('/api/admin/fees', self.handle_get_fees)
        app.router.add_put('/api/admin/fees/{fee_id}', self.handle_modify_fee)
        
        # Audit routes
        app.router.add_get('/api/admin/audit', self.handle_audit_logs)
        
        runner = web.AppRunner(app)
        await runner.setup()
        site = web.TCPSite(runner, host, port)
        await site.start()
        
        self._running = True
        print(f"Admin API started on {host}:{port}")
    
    async def stop(self):
        """Stop the admin API server"""
        self._running = False
        print("Admin API stopped")
    
    # Placeholder handlers - in production would use proper auth and validation
    async def handle_login(self, request):
        return web.json_response({"success": True, "token": "mock_token"})
    
    async def handle_stats(self, request):
        return web.json_response(self.admin_service.get_platform_stats(None))
    
    async def handle_list_dexes(self, request):
        return web.json_response([])
    
    async def handle_get_dex(self, request):
        return web.json_response({})
    
    async def handle_modify_dex(self, request):
        return web.json_response({})
    
    async def handle_suspend_dex(self, request):
        return web.json_response({})
    
    async def handle_list_cexes(self, request):
        return web.json_response([])
    
    async def handle_connect_cex(self, request):
        return web.json_response({})
    
    async def handle_disconnect_cex(self, request):
        return web.json_response({})
    
    async def handle_cex_trade(self, request):
        return web.json_response({})
    
    async def handle_list_wallets(self, request):
        return web.json_response([])
    
    async def handle_create_wallet(self, request):
        return web.json_response({})
    
    async def handle_wallet_transfer(self, request):
        return web.json_response({})
    
    async def handle_get_fees(self, request):
        return web.json_response([])
    
    async def handle_modify_fee(self, request):
        return web.json_response({})
    
    async def handle_audit_logs(self, request):
        return web.json_response([])


async def main():
    """Test admin service"""
    print("=== TigerSwap Admin Service Test ===")
    
    service = AdminService()
    
    # Login
    admin = service.authenticate_admin("admin@tigerswap.io", "changeme")
    if admin:
        print(f"Logged in as: {admin.username} ({admin.role.value})")
    else:
        print("Login failed")
        return
    
    # Get platform stats
    stats = service.get_platform_stats(admin)
    print(f"\nPlatform Stats:")
    print(f"  DEXes: {stats['dex_count']} active: {stats['active_dex_count']}")
    print(f"  CEXes: {stats['cex_count']}")
    print(f"  Wallets: {stats['wallet_count']}")
    
    # List DEXs
    dexes = service.list_dexes(admin)
    print(f"\nConnected DEXs:")
    for dex in dexes:
        print(f"  - {dex['name']}: ${dex['total_volume_24h']:,.0f} 24h volume")
    
    # Get fee structures
    fees = service.get_fee_structure(admin, "trading")
    print(f"\nFee Structures:")
    for fee in fees:
        print(f"  - {fee.fee_type}: {fee.taker_fee * 100}% taker")


if __name__ == "__main__":
    asyncio.run(main())
