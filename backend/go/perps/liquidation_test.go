package perps

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// perpsHarness deploys the contract and configures one market, returning the
// client under test plus the pieces a scenario needs.
type perpsHarness struct {
	client   *Client
	raw      *ethclient.Client
	perps    common.Address
	rusd     common.Address
	marketID common.Hash
	url      string
	chainID  int64
}

// newPerpsHarness stands up anvil, deploys the token and the perps engine,
// configures BTC/RUSD at a $60,000 mark, seeds LP liquidity and approves the
// engine. It returns a ready client.
func newPerpsHarness(t *testing.T) *perpsHarness {
	t.Helper()
	url := startAnvil(t)
	chainID := int64(31337)

	priv, err := crypto.HexToECDSA(anvilKey)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	owner := crypto.PubkeyToAddress(priv.PublicKey)

	erc20ABI := loadABI(t, "TestERC20")
	perpsABIForDeploy := loadABI(t, "TigerSwapPerpetuals")

	raw, err := ethclient.Dial(url)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { raw.Close() })

	ctor, err := abiStrings(t, "RUSD", "RUSD")
	if err != nil {
		t.Fatalf("pack erc20 ctor: %v", err)
	}
	rusd := deploy(t, raw, chainID, anvilKey, "TestERC20", ctor)
	perpsAddr := deploy(t, raw, chainID, anvilKey, "TigerSwapPerpetuals", nil)

	marketID := MarketID("BTC/RUSD")
	createMarket, err := perpsABIForDeploy.Pack("createMarket", marketID, rusd, wad(10), bps(500), bps(500), bps(1000))
	if err != nil {
		t.Fatalf("pack createMarket: %v", err)
	}
	sendRaw(t, raw, chainID, anvilKey, perpsAddr, createMarket)

	setOracle, _ := perpsABIForDeploy.Pack("setOracle", owner)
	sendRaw(t, raw, chainID, anvilKey, perpsAddr, setOracle)

	updatePrice, _ := perpsABIForDeploy.Pack("updatePrice", marketID, wad(60000))
	sendRaw(t, raw, chainID, anvilKey, perpsAddr, updatePrice)

	mint, _ := erc20ABI.Pack("mint", owner, wad(10_000_000))
	sendRaw(t, raw, chainID, anvilKey, rusd, mint)

	maxUint := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	approve, _ := erc20ABI.Pack("approve", perpsAddr, maxUint)
	sendRaw(t, raw, chainID, anvilKey, rusd, approve)

	addLiq, _ := perpsABIForDeploy.Pack("addLiquidity", rusd, wad(5_000_000))
	sendRaw(t, raw, chainID, anvilKey, perpsAddr, addLiq)

	client, err := NewClient(Deployment{ChainID: chainID, Perpetuals: perpsAddr, RPCURL: url}, priv)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(client.Close)

	return &perpsHarness{
		client:   client,
		raw:      raw,
		perps:    perpsAddr,
		rusd:     rusd,
		marketID: marketID,
		url:      url,
		chainID:  chainID,
	}
}

// TestOnChainLiquidation proves the permissionless liquidation path: a long
// opened at $60,000 becomes under-maintenance after the oracle marks the market
// down, and a signed liquidatePosition call closes it.
func TestOnChainLiquidation(t *testing.T) {
	h := newPerpsHarness(t)
	ctx := context.Background()

	size := OneUnit          // $60,000 notional at the $60,000 mark
	collateral := wad(12000) // 5x
	openCall, err := h.client.BuildOpenPosition(h.marketID, size, collateral, true)
	if err != nil {
		t.Fatalf("build open: %v", err)
	}
	openReceipt, err := h.client.Send(ctx, openCall)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	posID := positionIDFromReceipt(t, h.raw, openReceipt, h.perps)
	if posID == (common.Hash{}) {
		t.Fatal("no position opened")
	}

	// A healthy position must not be liquidatable.
	liqCall, err := h.client.BuildLiquidatePosition(posID)
	if err != nil {
		t.Fatalf("build liquidate: %v", err)
	}
	if _, err := h.client.Send(ctx, liqCall); err == nil {
		t.Fatal("liquidating a healthy position must revert")
	}

	// Mark the market down so equity falls below the 5% maintenance margin.
	// equity = 12000 + (p - 60000); at p = 50000 equity is 2000, ratio 3.3%.
	dropPrice(t, h, 50000)

	if _, err := h.client.Send(ctx, liqCall); err != nil {
		t.Fatalf("liquidate: %v", err)
	}

	closed, err := h.client.Position(ctx, posID)
	if err != nil {
		t.Fatalf("read position: %v", err)
	}
	if closed.Size.Sign() != 0 {
		t.Fatalf("liquidated position should be gone, size=%s", closed.Size)
	}
}

// TestOnChainFundingRate proves the oracle-side funding update round-trips: the
// client packs updateFundingRate and reads the stored rate back.
func TestOnChainFundingRate(t *testing.T) {
	h := newPerpsHarness(t)
	ctx := context.Background()

	mark := wad(60500)
	index := wad(60000)
	call, err := h.client.BuildUpdateFundingRate(h.marketID, mark, index)
	if err != nil {
		t.Fatalf("build funding: %v", err)
	}
	if _, err := h.client.Send(ctx, call); err != nil {
		t.Fatalf("send funding: %v", err)
	}

	funding, err := h.client.FundingInfo(ctx, h.marketID)
	if err != nil {
		t.Fatalf("read funding: %v", err)
	}
	if funding.MarkPrice.Cmp(mark) != 0 {
		t.Fatalf("mark mismatch: %s", funding.MarkPrice)
	}
	if funding.IndexPrice.Cmp(index) != 0 {
		t.Fatalf("index mismatch: %s", funding.IndexPrice)
	}
	// fundingRate = (mark - index) / index * 1e6 = 500/60000 * 1e6 = 8333.
	want := big.NewInt(8333)
	if funding.LastFundingRate.Cmp(want) != 0 {
		t.Fatalf("funding rate: want %s, got %s", want, funding.LastFundingRate)
	}
}

// TestOnChainLiquidityAccounting proves LP shares are tracked per account and
// that free liquidity is reserved for open positions.
func TestOnChainLiquidityAccounting(t *testing.T) {
	h := newPerpsHarness(t)
	ctx := context.Background()

	liquidity, err := h.client.PoolLiquidity(ctx, h.rusd)
	if err != nil {
		t.Fatalf("pool liquidity: %v", err)
	}
	if liquidity.Cmp(wad(5_000_000)) != 0 {
		t.Fatalf("expected 5,000,000 liquidity, got %s", liquidity)
	}

	owner := h.client.SignerAddress()
	shares, err := h.client.PoolShares(ctx, h.rusd, owner)
	if err != nil {
		t.Fatalf("pool shares: %v", err)
	}
	if shares.Sign() <= 0 {
		t.Fatal("provider should hold shares")
	}

	free, err := h.client.FreeLiquidity(ctx, h.rusd)
	if err != nil {
		t.Fatalf("free liquidity: %v", err)
	}
	if free.Cmp(liquidity) != 0 {
		t.Fatalf("with no positions free liquidity should equal pool liquidity: free=%s pool=%s", free, liquidity)
	}

	// Open a position; its reserved payout must reduce free liquidity.
	openCall, err := h.client.BuildOpenPosition(h.marketID, OneUnit, wad(12000), true)
	if err != nil {
		t.Fatalf("build open: %v", err)
	}
	if _, err := h.client.Send(ctx, openCall); err != nil {
		t.Fatalf("open: %v", err)
	}

	freeAfter, err := h.client.FreeLiquidity(ctx, h.rusd)
	if err != nil {
		t.Fatalf("free liquidity after: %v", err)
	}
	if freeAfter.Cmp(free) >= 0 {
		t.Fatalf("free liquidity should drop when a position is open: before=%s after=%s", free, freeAfter)
	}
	reserved, err := h.client.ReservedLiquidity(ctx, h.rusd)
	if err != nil {
		t.Fatalf("reserved: %v", err)
	}
	if reserved.Sign() <= 0 {
		t.Fatal("reserved liquidity should be positive with an open position")
	}
}

// dropPrice updates the mark price through the oracle path.
func dropPrice(t *testing.T, h *perpsHarness, price int64) {
	t.Helper()
	perpsABIForDeploy := loadABI(t, "TigerSwapPerpetuals")
	updatePrice, err := perpsABIForDeploy.Pack("updatePrice", h.marketID, wad(price))
	if err != nil {
		t.Fatalf("pack updatePrice: %v", err)
	}
	sendRaw(t, h.raw, h.chainID, anvilKey, h.perps, updatePrice)
}

// abiStrings packs two string constructor arguments.
func abiStrings(t *testing.T, a, b string) ([]byte, error) {
	t.Helper()
	strType := mustType(t, "string")
	args := abi.Arguments{{Type: strType}, {Type: strType}}
	return args.Pack(a, b)
}
