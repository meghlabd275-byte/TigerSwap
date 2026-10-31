"""Tests for the TigerSwap Python SDK.

These cover the parts that are pure logic and therefore testable without a live
node: parameter validation, deployment guards, token lookups, chain management,
position maths and request construction.

No test asserts a fabricated on-chain value; where a node is required the
contract call is stubbed so the code path under test is the SDK's own.
"""

import asyncio

import pytest

from tigerswap_sdk.chain import Chain, ChainManager
from tigerswap_sdk.contracts import (
    DeploymentError,
    get_contract_address,
    get_deployment,
    is_deployed,
)
from tigerswap_sdk.perpetuals import PRICE_PRECISION, Perpetuals, Position
from tigerswap_sdk.tokens import Token, TokenList


# --------------------------------------------------------------------------
# deployments
# --------------------------------------------------------------------------


ROUTER = "0x1111111111111111111111111111111111111111"
FACTORY = "0x2222222222222222222222222222222222222222"


def test_get_deployment_is_empty_when_nothing_is_configured(monkeypatch):
    monkeypatch.delenv("TIGERSWAP_ROUTER_1", raising=False)
    monkeypatch.delenv("TIGERSWAP_FACTORY_1", raising=False)
    monkeypatch.delenv("TIGERSWAP_WETH_1", raising=False)

    # No TigerSwap contract is configured, so the chain counts as undeployed.
    deployment = get_deployment(1)
    assert "router" not in deployment
    assert "factory" not in deployment
    assert is_deployed(1) is False


def test_get_deployment_reads_the_configured_addresses(monkeypatch):
    monkeypatch.setenv("TIGERSWAP_ROUTER_1", ROUTER)
    monkeypatch.setenv("TIGERSWAP_FACTORY_1", FACTORY)

    deployment = get_deployment(1)
    assert deployment["router"] == ROUTER
    assert deployment["factory"] == FACTORY
    # The wrapped-native default is available without extra configuration.
    assert deployment["weth"].startswith("0x")


def test_chain_scoped_configuration_does_not_leak_across_chains(monkeypatch):
    monkeypatch.setenv("TIGERSWAP_ROUTER_1", ROUTER)
    monkeypatch.setenv("TIGERSWAP_ROUTER_56", FACTORY)

    assert get_deployment(1)["router"] == ROUTER
    assert get_deployment(56)["router"] == FACTORY
    assert "router" not in get_deployment(137)


def test_get_contract_address_raises_when_unconfigured(monkeypatch):
    monkeypatch.delenv("TIGERSWAP_ROUTER_1", raising=False)
    with pytest.raises(DeploymentError):
        get_contract_address(1, "router")


def test_get_contract_address_rejects_the_zero_address(monkeypatch):
    monkeypatch.setenv("TIGERSWAP_ROUTER_1", "0x0000000000000000000000000000000000000000")
    with pytest.raises(DeploymentError):
        get_contract_address(1, "router")


def test_get_contract_address_rejects_an_unknown_name(monkeypatch):
    with pytest.raises(DeploymentError):
        get_contract_address(1, "not_a_contract")


def test_malformed_configured_address_raises(monkeypatch):
    # A typo must fail loudly rather than be read as "not configured".
    monkeypatch.setenv("TIGERSWAP_ROUTER_1", "0xnothex")
    with pytest.raises(DeploymentError):
        get_deployment(1)


def test_is_deployed_tracks_router_and_factory(monkeypatch):
    monkeypatch.setenv("TIGERSWAP_ROUTER_1", ROUTER)
    monkeypatch.delenv("TIGERSWAP_FACTORY_1", raising=False)
    assert is_deployed(1) is False

    monkeypatch.setenv("TIGERSWAP_FACTORY_1", FACTORY)
    assert is_deployed(1) is True
    assert is_deployed(999_999) is False


# --------------------------------------------------------------------------
# chains
# --------------------------------------------------------------------------


def test_chain_manager_returns_a_chain():
    manager = ChainManager.__new__(ChainManager)
    manager.CHAINS = ChainManager.CHAINS
    chain = manager.get_chain(1)
    assert isinstance(chain, Chain)
    assert chain.name == "Ethereum"


def test_chain_manager_raises_for_an_unknown_chain():
    manager = ChainManager.__new__(ChainManager)
    manager.CHAINS = ChainManager.CHAINS
    with pytest.raises(KeyError):
        manager.get_chain(999_999)


def test_chain_manager_filters_by_type():
    manager = ChainManager.__new__(ChainManager)
    manager.CHAINS = ChainManager.CHAINS
    evm = manager.get_chain_by_type("evm")
    assert evm
    assert all(chain.chain_type == "evm" for chain in evm)


def test_switch_chain_validates_before_switching():
    class FakeClient:
        chain_id = 1

    manager = ChainManager.__new__(ChainManager)
    manager.CHAINS = ChainManager.CHAINS
    manager.client = FakeClient()

    manager.switch_chain(137)
    assert manager.client.chain_id == 137

    with pytest.raises(KeyError):
        manager.switch_chain(999_999)
    assert manager.client.chain_id == 137


# --------------------------------------------------------------------------
# tokens
# --------------------------------------------------------------------------


def test_token_list_finds_seeded_tokens():
    tokens = TokenList(client=None)
    usdc = tokens.get_token_by_symbol("USDC", 1)
    assert usdc is not None
    assert usdc.decimals == 6

    by_address = tokens.get_token("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", 1)
    assert by_address is not None
    assert by_address.symbol == "USDC"


def test_token_list_returns_none_for_unknown_tokens():
    tokens = TokenList(client=None)
    assert tokens.get_token_by_symbol("NOPE", 1) is None
    assert tokens.get_token("0x0000000000000000000000000000000000000001", 1) is None


def test_token_list_search_is_case_insensitive():
    tokens = TokenList(client=None)
    results = tokens.search_tokens("usd", 1)
    symbols = {token.symbol for token in results}
    assert "USDC" in symbols
    assert "USDT" in symbols


def test_add_token_registers_on_the_right_chain():
    tokens = TokenList(client=None)
    new_token = Token("0xabc", "TEST", "Test Token", 18, 137)
    tokens.add_token(new_token)

    assert tokens.get_token_by_symbol("TEST", 137) is not None
    assert tokens.get_token_by_symbol("TEST", 1) is None


def test_token_formatted_address_truncates():
    token = Token("0x1234567890abcdef1234567890abcdef12345678", "X", "X", 18, 1)
    assert token.formatted_address.startswith("0x1234")
    assert token.formatted_address.endswith("5678")


# --------------------------------------------------------------------------
# perpetuals: position maths and validation
# --------------------------------------------------------------------------


def test_perpetuals_market_id_is_deterministic_keccak():
    first = Perpetuals.market_id("BTC-USD")
    second = Perpetuals.market_id("BTC-USD")
    assert first == second
    assert len(first) == 32
    assert first != Perpetuals.market_id("ETH-USD")


def test_position_leverage_and_notional():
    position = Position(
        position_id="0x01",
        market_id="0x02",
        trader="0x03",
        size=1 * PRICE_PRECISION,
        collateral=10_000 * PRICE_PRECISION,
        entry_price=50_000 * PRICE_PRECISION,
        last_updated=0,
        is_long=True,
    )

    # One whole unit at 50,000 with 10,000 collateral is 5x leverage.
    assert position.notional == 50_000
    assert position.collateral_whole == 10_000
    assert position.leverage == pytest.approx(5.0)
    assert position.side == "long"


def test_position_unrealized_pnl_for_a_long():
    position = Position(
        position_id="0x01",
        market_id="0x02",
        trader="0x03",
        size=1 * PRICE_PRECISION,
        collateral=10_000 * PRICE_PRECISION,
        entry_price=50_000 * PRICE_PRECISION,
        last_updated=0,
        is_long=True,
        mark_price=55_000 * PRICE_PRECISION,
    )
    assert position.unrealized_pnl == pytest.approx(5_000)


def test_position_unrealized_pnl_for_a_short():
    position = Position(
        position_id="0x01",
        market_id="0x02",
        trader="0x03",
        size=1 * PRICE_PRECISION,
        collateral=10_000 * PRICE_PRECISION,
        entry_price=50_000 * PRICE_PRECISION,
        last_updated=0,
        is_long=False,
        mark_price=45_000 * PRICE_PRECISION,
    )
    # A short gains when the price falls.
    assert position.unrealized_pnl == pytest.approx(5_000)


def test_position_pnl_is_zero_without_a_mark_price():
    position = Position(
        "0x01", "0x02", "0x03", PRICE_PRECISION, PRICE_PRECISION, PRICE_PRECISION, 0, True
    )
    assert position.unrealized_pnl == 0


def test_to_bytes32_accepts_hex_and_text():
    engine = Perpetuals(client=None)

    hex_value = "0x" + "ab" * 32
    assert engine._to_bytes32(hex_value) == bytes.fromhex("ab" * 32)

    text_value = engine._to_bytes32("BTC-USD")
    assert len(text_value) == 32


def test_to_bytes32_rejects_a_short_hex_string():
    engine = Perpetuals(client=None)
    with pytest.raises(ValueError):
        engine._to_bytes32("0xabcd")


def test_perpetuals_requires_a_wallet_to_trade():
    class FakeClient:
        wallet = None

    engine = Perpetuals(client=FakeClient())
    with pytest.raises(ValueError):
        asyncio.run(engine.open_position("BTC-USD", 1, 1, True))


# --------------------------------------------------------------------------
# package wiring
# --------------------------------------------------------------------------


def test_sdk_exports_every_documented_class():
    import tigerswap_sdk

    for name in tigerswap_sdk.__all__:
        assert hasattr(tigerswap_sdk, name), f"{name} is exported but missing"


def test_client_requires_an_rpc_url(monkeypatch):
    import tigerswap_sdk

    monkeypatch.delenv("RPC_URL", raising=False)
    with pytest.raises(ValueError):
        tigerswap_sdk.TigerSwapClient(rpc_url=None)


def test_wallet_derives_an_address():
    from tigerswap_sdk.wallet import Wallet

    class FakeWeb3:
        @staticmethod
        def to_checksum_address(address):
            return address

    private_key = "0x" + "11" * 32
    wallet = Wallet(private_key, FakeWeb3())
    assert wallet.address.startswith("0x")
    assert len(wallet.address) == 42


def test_wallet_rejects_an_invalid_private_key():
    from tigerswap_sdk.wallet import Wallet

    class FakeWeb3:
        @staticmethod
        def to_checksum_address(address):
            return address

    with pytest.raises(Exception):
        Wallet("0xnotakey", FakeWeb3())
