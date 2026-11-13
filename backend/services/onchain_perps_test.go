package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/go/perps"
	"tigerswap/backend/go/perps/perpstest"
	"tigerswap/backend/models"
)

// newOnChainPerpsTestDB builds an in-memory database with the tables the
// on-chain perps service reads and writes.
func newOnChainPerpsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(
		&models.User{},
		&models.FuturesMarket{},
		&models.PerpsMarket{},
		&models.OnChainPerpPosition{},
		&models.OnChainPerpCursor{},
		&models.OnChainPerpLiquidation{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// TestOnChainPerpsOpenValidatesInput proves the order builder refuses a bad side
// and a non-positive amount before any chain call, and reports a missing
// deployment rather than inventing one.
func TestOnChainPerpsOpenValidatesInput(t *testing.T) {
	svc := NewOnChainPerpsService(newOnChainPerpsTestDB(t), nil)

	cases := []struct {
		name string
		body string
	}{
		{"bad side", `{"chain_id":31337,"symbol":"BTC/RUSD","size":"1","collateral":"1","side":"sideways"}`},
		{"zero size", `{"chain_id":31337,"symbol":"BTC/RUSD","size":"0","collateral":"1","side":"long"}`},
		{"negative collateral", `{"chain_id":31337,"symbol":"BTC/RUSD","size":"1","collateral":"-1","side":"long"}`},
		{"nan size", `{"chain_id":31337,"symbol":"BTC/RUSD","size":"NaN","collateral":"1","side":"long"}`},
		{"too many decimals", `{"chain_id":31337,"symbol":"BTC/RUSD","size":"1.123456789","collateral":"1","side":"long"}`},
	}
	for _, tc := range cases {
		status, _ := callStringUserHandler(t, svc.BuildOpenPositionHandler, uuid.NewString(), "POST", "/onchain/perps/open", tc.body, nil)
		if status != 400 {
			t.Errorf("%s: want 400, got %d", tc.name, status)
		}
	}

	// A well-formed request with no deployment configured is a service error, not
	// a fabricated success.
	status, _ := callStringUserHandler(t, svc.BuildOpenPositionHandler, uuid.NewString(), "POST", "/onchain/perps/open",
		`{"chain_id":31337,"symbol":"BTC/RUSD","size":"1","collateral":"1","side":"long"}`, nil)
	if status == 200 {
		t.Fatal("a request with no configured chain must not succeed")
	}
}

// TestOnChainPerpsPositionsRequireBoundWallet proves the position list is keyed
// to the caller's own bound wallet and refuses an account with none.
func TestOnChainPerpsPositionsRequireBoundWallet(t *testing.T) {
	db := newOnChainPerpsTestDB(t)
	user := models.User{ID: uuid.New(), Email: "a@b.c", Username: "a"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	svc := NewOnChainPerpsService(db, nil)
	// ethclient.Dial is lazy, so a syntactically valid deployment registers
	// without a reachable node. This exercises the wallet check, not the RPC.
	if err := svc.RegisterDeployment(perps.Deployment{
		ChainID:    31337,
		Perpetuals: common.HexToAddress("0x2222222222222222222222222222222222222222"),
		RPCURL:     "http://127.0.0.1:1",
	}); err != nil {
		t.Fatalf("register deployment: %v", err)
	}

	status, _ := callStringUserHandler(t, svc.PositionsHandler, user.ID.String(), "GET", "/onchain/perps/positions?chain_id=31337", "", nil)
	if status != 403 {
		t.Fatalf("an account with no bound wallet must be refused, got %d", status)
	}
}

// TestOnChainPerpsSymbolMapping proves an indexed market id resolves to the
// symbol an operator configured, and an unknown id falls back to the id itself
// rather than a wrong label.
func TestOnChainPerpsSymbolMapping(t *testing.T) {
	db := newOnChainPerpsTestDB(t)
	if err := db.Create(&models.FuturesMarket{Symbol: "BTC/RUSD", Status: models.ListingStatusActive}).Error; err != nil {
		t.Fatalf("create listing: %v", err)
	}

	svc := NewOnChainPerpsService(db, nil)
	if err := svc.ReloadSymbols(); err != nil {
		t.Fatalf("reload symbols: %v", err)
	}

	got := svc.symbolFor(perps.MarketID("BTC/RUSD"))
	if got != "BTC/RUSD" {
		t.Fatalf("want BTC/RUSD, got %q", got)
	}
	unknown := perps.MarketID("NOPE/RUSD")
	if got := svc.symbolFor(unknown); got != unknown.Hex() {
		t.Fatalf("unknown market should fall back to its id, got %q", got)
	}
}

// TestBelowMaintenanceMarginMatchesContract proves the keeper's health check
// agrees with TigerSwapPerpetuals.liquidatePosition on worked examples.
func TestBelowMaintenanceMarginMatchesContract(t *testing.T) {
	// 500 bps expressed at the contract's RATIO_PRECISION (1e18), matching what
	// createMarket stores.
	maintenance := new(big.Int).Div(new(big.Int).Mul(big.NewInt(500), perps.OneUnit), big.NewInt(10000))
	market := &perps.Market{MaintenanceRate: maintenance}
	one := perps.OneUnit

	long := &perps.Position{Size: one, Collateral: new(big.Int).Mul(big.NewInt(12000), one), EntryPrice: new(big.Int).Mul(big.NewInt(60000), one), IsLong: true}
	if belowMaintenanceMargin(long, market, new(big.Int).Mul(big.NewInt(60000), one), nil, 0) {
		t.Fatal("a 20% equity position must be healthy")
	}
	if !belowMaintenanceMargin(long, market, new(big.Int).Mul(big.NewInt(50000), one), nil, 0) {
		t.Fatal("a 3.3% equity position must be liquidatable")
	}
	if belowMaintenanceMargin(long, market, new(big.Int).Mul(big.NewInt(57000), one), nil, 0) {
		t.Fatal("a 15% equity position must be healthy")
	}

	short := &perps.Position{Size: one, Collateral: new(big.Int).Mul(big.NewInt(12000), one), EntryPrice: new(big.Int).Mul(big.NewInt(60000), one), IsLong: false}
	if belowMaintenanceMargin(short, market, new(big.Int).Mul(big.NewInt(50000), one), nil, 0) {
		t.Fatal("a short profiting from a price drop must be healthy")
	}
	if !belowMaintenanceMargin(short, market, new(big.Int).Mul(big.NewInt(70000), one), nil, 0) {
		t.Fatal("a short losing 3.3% of notional must be liquidatable")
	}
}

// TestRationalToBigInt proves decimal amounts convert to the exact integer the
// contract expects, without a float round-trip.
func TestRationalToBigInt(t *testing.T) {
	cases := map[string]string{
		"1":       "1000000000000000000",
		"1.5":     "1500000000000000000",
		"0.00001": "10000000000000",
		"60000":   "60000000000000000000000",
	}
	for in, want := range cases {
		rat, err := perpAmountValue(in)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		got := rationalToBigInt(rat).String()
		if got != want {
			t.Errorf("%s: want %s, got %s", in, want, got)
		}
	}
}

// TestIsBytes32 proves the position-id guard accepts a real id and rejects a
// truncated or un-prefixed string.
func TestIsBytes32(t *testing.T) {
	valid := perps.MarketID("BTC/RUSD").Hex()
	if !isBytes32(valid) {
		t.Fatalf("a 32-byte hex id must be accepted: %s", valid)
	}
	for _, bad := range []string{"", "0x1234", strings.Repeat("a", 64), "zz" + strings.Repeat("0", 64)} {
		if isBytes32(bad) {
			t.Errorf("must reject %q", bad)
		}
	}
}

// TestApplyPositionClosedTransitionsMirror proves a PositionClosed event closes
// the indexed row and records the realised PnL.
func TestApplyPositionClosedTransitionsMirror(t *testing.T) {
	db := newOnChainPerpsTestDB(t)
	svc := NewOnChainPerpsService(db, nil)

	positionID := [32]byte{0x07}
	row := models.OnChainPerpPosition{
		ChainID:    31337,
		PositionID: common.Hash(positionID).Hex(),
		MarketID:   perps.MarketID("BTC/RUSD").Hex(),
		Symbol:     "BTC/RUSD",
		Trader:     "0x1111111111111111111111111111111111111111",
		Size:       "1000000000000000000",
		Collateral: "12000000000000000000000",
		Status:     "open",
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create row: %v", err)
	}

	abiJSON := perps.ABI()
	data, err := abiJSON.Events["PositionClosed"].Inputs.NonIndexed().Pack(positionID, big.NewInt(-500))
	if err != nil {
		t.Fatalf("pack closed: %v", err)
	}
	closedLog := types.Log{
		Topics:      []common.Hash{abiJSON.Events["PositionClosed"].ID},
		Data:        data,
		BlockNumber: 120,
	}
	if err := svc.applyPositionClosed(context.Background(), nil, 31337, closedLog, "closed"); err != nil {
		t.Fatalf("apply close: %v", err)
	}

	var stored models.OnChainPerpPosition
	if err := db.Where("chain_id = ? AND position_id = ?", 31337, common.Hash(positionID).Hex()).First(&stored).Error; err != nil {
		t.Fatalf("read row: %v", err)
	}
	if stored.Status != "closed" {
		t.Fatalf("row should be closed, got %s", stored.Status)
	}
	if stored.RealizedPnL != "-500" {
		t.Fatalf("realised pnl should be -500, got %s", stored.RealizedPnL)
	}
}

// TestOnChainPerpsAnvilEndToEnd deploys the real contract, opens a position
// through the service's own calldata, indexes the confirmed event and reads the
// position back through the API shape. It proves the whole path: service →
// contract → receipt → indexer → API.
func TestOnChainPerpsAnvilEndToEnd(t *testing.T) {
	h := perpstest.Start(t)
	db := newOnChainPerpsTestDB(t)

	user := models.User{ID: uuid.New(), Email: "trader@tigerswap.test", Username: "trader", WalletAddress: h.Owner.Hex()}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&models.FuturesMarket{Symbol: "BTC/RUSD", Status: models.ListingStatusActive}).Error; err != nil {
		t.Fatalf("create listing: %v", err)
	}

	svc := NewOnChainPerpsService(db, nil)
	if err := svc.RegisterDeployment(h.Deployment()); err != nil {
		t.Fatalf("register deployment: %v", err)
	}
	if err := svc.ReloadSymbols(); err != nil {
		t.Fatalf("reload symbols: %v", err)
	}

	// Open through the service's calldata path.
	status, body := callStringUserHandler(t, svc.BuildOpenPositionHandler, user.ID.String(), "POST", "/onchain/perps/open",
		`{"chain_id":31337,"symbol":"BTC/RUSD","size":"1","collateral":"12000","side":"long"}`, nil)
	if status != 200 {
		t.Fatalf("build open: %d %s", status, body)
	}
	var built struct {
		To   string `json:"to"`
		Data string `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &built); err != nil {
		t.Fatalf("decode: %v", err)
	}
	receipt := h.SendCalldata(common.HexToAddress(built.To), built.Data)
	posID := h.PositionIDFromReceipt(receipt)
	if posID == (common.Hash{}) {
		t.Fatal("no position opened")
	}

	// Index the confirmed block range, then read the mirror back.
	ctx := context.Background()
	if err := svc.IndexRange(ctx, 31337, receipt.BlockNumber.Uint64(), receipt.BlockNumber.Uint64()); err != nil {
		t.Fatalf("index: %v", err)
	}
	var row models.OnChainPerpPosition
	if err := db.Where("chain_id = ? AND position_id = ?", 31337, posID.Hex()).First(&row).Error; err != nil {
		t.Fatalf("mirror row missing: %v", err)
	}
	if row.Status != "open" || row.Symbol != "BTC/RUSD" || row.Trader != strings.ToLower(h.Owner.Hex()) {
		t.Fatalf("unexpected mirror row: %+v", row)
	}

	// The authenticated owner reads the position through the API.
	status, body = callStringUserHandler(t, svc.PositionsHandler, user.ID.String(), "GET", "/onchain/perps/positions?chain_id=31337", "", nil)
	if status != 200 {
		t.Fatalf("list positions: %d %s", status, body)
	}
	if !strings.Contains(body, posID.Hex()) {
		t.Fatalf("positions response missing the open position: %s", body)
	}
}

// TestAccruedFundingMatchesContractFormula proves the keeper's funding mirror
// reproduces TigerSwapPerpetuals.calculateFunding: notional * rate * elapsed /
// (FUNDING_RATE_PRECISION * FUNDING_INTERVAL), sign-flipped for a short.
func TestAccruedFundingMatchesContractFormula(t *testing.T) {
	one := perps.OneUnit
	// notional 60000e18, rate 1e4 (1%), elapsed 1 hour = one interval.
	pos := &perps.Position{
		Size:       one,
		Collateral: new(big.Int).Mul(big.NewInt(100000), one),
		EntryPrice: new(big.Int).Mul(big.NewInt(60000), one),
		LastUpdate: big.NewInt(1_000_000),
		IsLong:     true,
	}
	funding := &perps.Funding{LastFundingRate: big.NewInt(10_000)}
	now := int64(1_000_000 + 3600)

	got := accruedFunding(pos, funding, now)
	// 60000 * 0.01 = 600.
	want := new(big.Int).Mul(big.NewInt(600), one)
	if got.Cmp(want) != 0 {
		t.Fatalf("long funding: want %s, got %s", want, got)
	}

	// A short receives the same amount, so the sign flips.
	pos.IsLong = false
	got = accruedFunding(pos, funding, now)
	if got.Cmp(new(big.Int).Neg(want)) != 0 {
		t.Fatalf("short funding: want %s, got %s", new(big.Int).Neg(want), got)
	}

	// Half the interval charges half the funding.
	got = accruedFunding(pos, funding, int64(1_000_000+1800))
	half := new(big.Int).Neg(new(big.Int).Div(want, big.NewInt(2)))
	if got.Cmp(half) != 0 {
		t.Fatalf("half-interval funding: want %s, got %s", half, got)
	}

	// No elapsed time accrues nothing.
	if accruedFunding(pos, funding, 1_000_000).Sign() != 0 {
		t.Fatal("zero elapsed time must accrue no funding")
	}
	// A nil funding source accrues nothing rather than panicking.
	if accruedFunding(pos, nil, now).Sign() != 0 {
		t.Fatal("nil funding must accrue nothing")
	}
}

// TestFundingCanFlipAHealthyPositionToLiquidatable proves the keeper applies
// funding before the health check, so a position that funding pushed under
// maintenance is liquidated rather than left open.
func TestFundingCanFlipAHealthyPositionToLiquidatable(t *testing.T) {
	one := perps.OneUnit
	maintenance := new(big.Int).Div(new(big.Int).Mul(big.NewInt(500), perps.OneUnit), big.NewInt(10000))
	market := &perps.Market{MaintenanceRate: maintenance}

	// 9x leverage: collateral 60000 for 540000 notional, equity ~11% at entry.
	pos := &perps.Position{
		Size:       new(big.Int).Mul(big.NewInt(9), one),
		Collateral: new(big.Int).Mul(big.NewInt(60000), one),
		EntryPrice: new(big.Int).Mul(big.NewInt(60000), one),
		LastUpdate: big.NewInt(1_000_000),
		IsLong:     true,
	}
	price := new(big.Int).Mul(big.NewInt(60000), one)

	// With no funding the position is healthy at the entry price.
	if belowMaintenanceMargin(pos, market, price, nil, 1_000_000) {
		t.Fatal("an unfunded position at entry must be healthy")
	}
	// A 2% premium held for four intervals drains it under maintenance.
	funding := &perps.Funding{LastFundingRate: big.NewInt(20_000)}
	if !belowMaintenanceMargin(pos, market, price, funding, 1_000_000+4*3600) {
		t.Fatal("funding must be able to push a healthy position below maintenance")
	}
}
