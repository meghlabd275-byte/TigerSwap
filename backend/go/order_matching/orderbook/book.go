// Package orderbook implements TigerSwap's own central limit order book.
//
// TigerSwap routes to no external venue, so this engine is the source of truth
// for price-time priority on every listed pair. Prices and quantities are held
// as integers (1/priceScale quote units and 1/qtyScale base units) so matching
// is exact: a fill never depends on a floating point comparison.
//
// The book is strict price-time priority. Orders at a better price match first;
// orders at the same price match in arrival order. A limit order that cannot
// cross rests on the book. A market order never rests.
package orderbook

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// Order statuses as reported over the API.
const (
	StatusOpen      = "open"
	StatusPartial   = "partially_filled"
	StatusFilled    = "filled"
	StatusCancelled = "cancelled"
	StatusRejected  = "rejected"
)

// Order sides.
const (
	SideBuy  = "buy"
	SideSell = "sell"
)

// Order types.
const (
	TypeLimit  = "limit"
	TypeMarket = "market"
)

// Time in force.
const (
	TIFGTC = "GTC"
	TIFIOC = "IOC"
	TIFFOK = "FOK"
)

// Default scales: eight decimal places, matching the ERC-20 convention used by
// the tokens TigerSwap lists.
const (
	DefaultPriceScale = int64(100_000_000)
	DefaultQtyScale   = int64(100_000_000)

	// defaultTradeHistory is how many trades a book retains for the tape.
	defaultTradeHistory = 5000
)

var (
	ErrUnknownPair  = errors.New("pair is not listed")
	ErrBadSide      = errors.New("side must be buy or sell")
	ErrBadType      = errors.New("type must be limit or market")
	ErrBadTIF       = errors.New("time in force must be GTC, IOC or FOK")
	ErrBadPrice     = errors.New("limit orders require a positive price")
	ErrBadQuantity  = errors.New("quantity must be positive")
	ErrQtyRange     = errors.New("quantity is outside the listed range")
	ErrUnknownOrder = errors.New("order not found")
	ErrNotOwner     = errors.New("order belongs to another trader")
	ErrNoLiquidity  = errors.New("no liquidity to fill the order")
)

// Order is a single order, resting or incoming. The exported fields are the
// wire representation; the unexported fields are the exact integer state the
// matching loop works in.
type Order struct {
	OrderID   uint64  `json:"order_id"`
	Trader    string  `json:"trader"`
	Pair      string  `json:"pair"`
	Side      string  `json:"side"`
	Type      string  `json:"type"`
	TIF       string  `json:"tif"`
	Price     float64 `json:"price"`
	Quantity  float64 `json:"quantity"`
	Filled    float64 `json:"filled"`
	Leaves    float64 `json:"leaves"`
	Status    string  `json:"status"`
	CreatedAt int64   `json:"created_at"`
	UpdatedAt int64   `json:"updated_at"`

	priceTicks  int64
	qtyUnits    int64
	filledUnits int64
}

// Trade is a completed fill between a resting (maker) and incoming (taker)
// order. Trades are the audit record of the venue, so the fields are exact:
// quantity and price come from the integer state, not from re-parsing floats.
type Trade struct {
	TradeID    uint64  `json:"trade_id"`
	Pair       string  `json:"pair"`
	MakerID    uint64  `json:"maker_order_id"`
	TakerID    uint64  `json:"taker_order_id"`
	TakerSide  string  `json:"taker_side"`
	Maker      string  `json:"maker"`
	Taker      string  `json:"taker"`
	Price      float64 `json:"price"`
	Quantity   float64 `json:"quantity"`
	Notional   float64 `json:"notional"`
	MakerFee   float64 `json:"maker_fee"`
	TakerFee   float64 `json:"taker_fee"`
	Timestamp  int64   `json:"timestamp"`
	priceTicks int64
	qtyUnits   int64
}

// Level is one aggregated price level of the book.
type Level struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
	Orders   int     `json:"orders"`
}

// PairConfig describes how one listed pair trades. It is supplied by the
// operator (the admin listing tables) rather than hardcoded, so a pair that is
// not listed cannot be traded.
type PairConfig struct {
	Pair        string
	PriceScale  int64
	QtyScale    int64
	MakerFeeBps int64
	TakerFeeBps int64
	MinQuantity float64
	MaxQuantity float64
}

func (c PairConfig) withDefaults() PairConfig {
	if c.PriceScale <= 0 {
		c.PriceScale = DefaultPriceScale
	}
	if c.QtyScale <= 0 {
		c.QtyScale = DefaultQtyScale
	}
	if c.MakerFeeBps < 0 {
		c.MakerFeeBps = 0
	}
	if c.TakerFeeBps < 0 {
		c.TakerFeeBps = 0
	}
	return c
}

// level holds every resting order at one price, oldest first.
type level struct {
	priceTicks int64
	orders     []*Order
	totalUnits int64
}

func (l *level) add(o *Order) {
	l.orders = append(l.orders, o)
	l.totalUnits += o.leavesUnits()
}

func (l *level) remove(orderID uint64) bool {
	for i, o := range l.orders {
		if o.OrderID == orderID {
			l.totalUnits -= o.leavesUnits()
			l.orders = append(l.orders[:i], l.orders[i+1:]...)
			return true
		}
	}
	return false
}

// sideBook is one side of a pair. Levels are kept sorted best-first: bids by
// descending price, asks by ascending price, so the best level is always [0].
type sideBook struct {
	side   string
	levels []*level
	index  map[int64]*level
}

func newSideBook(side string) *sideBook {
	return &sideBook{side: side, index: make(map[int64]*level)}
}

// better reports whether price a is a better price than b for this side.
func (s *sideBook) better(a, b int64) bool {
	if s.side == SideBuy {
		return a > b
	}
	return a < b
}

// insertLevel finds or creates the level for priceTicks, keeping the slice
// sorted best-first.
func (s *sideBook) insertLevel(priceTicks int64) *level {
	if l, ok := s.index[priceTicks]; ok {
		return l
	}
	l := &level{priceTicks: priceTicks}

	// Binary search for the first level that is not better than the new price.
	lo, hi := 0, len(s.levels)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if s.better(s.levels[mid].priceTicks, priceTicks) {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	s.levels = append(s.levels, nil)
	copy(s.levels[lo+1:], s.levels[lo:])
	s.levels[lo] = l
	s.index[priceTicks] = l
	return l
}

// dropLevel removes an empty level.
func (s *sideBook) dropLevel(priceTicks int64) {
	l, ok := s.index[priceTicks]
	if !ok {
		return
	}
	delete(s.index, priceTicks)
	for i, cand := range s.levels {
		if cand == l {
			s.levels = append(s.levels[:i], s.levels[i+1:]...)
			return
		}
	}
}

func (s *sideBook) best() *level {
	if len(s.levels) == 0 {
		return nil
	}
	return s.levels[0]
}

// Book is the order book for a single pair.
type Book struct {
	mu   sync.RWMutex
	cfg  PairConfig
	bids *sideBook
	asks *sideBook

	lastPriceTk int64
	openOrders  int
	trades      []Trade
	tradeStart  int
	tradeCount  int
}

// NewBook builds an empty book for one pair.
func NewBook(cfg PairConfig) *Book {
	cfg = cfg.withDefaults()
	return &Book{
		cfg:    cfg,
		bids:   newSideBook(SideBuy),
		asks:   newSideBook(SideSell),
		trades: make([]Trade, defaultTradeHistory),
	}
}

func (b *Book) Pair() string { return b.cfg.Pair }

func (o *Order) leavesUnits() int64 { return o.qtyUnits - o.filledUnits }

// toTicks converts a decimal value to integer ticks, rejecting values that are
// not finite or that would overflow.
func toTicks(value float64, scale int64) (int64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("value %v is not finite", value)
	}
	scaled := value * float64(scale)
	if scaled > math.MaxInt64 || scaled < math.MinInt64 {
		return 0, fmt.Errorf("value %v is out of range", value)
	}
	return int64(math.Round(scaled)), nil
}

// validate normalises an incoming order and fills in its integer state.
func (b *Book) validate(o *Order) error {
	switch o.Side {
	case SideBuy, SideSell:
	default:
		return ErrBadSide
	}
	switch o.Type {
	case TypeLimit, TypeMarket:
	default:
		return ErrBadType
	}
	if o.TIF == "" {
		o.TIF = TIFGTC
	}
	switch o.TIF {
	case TIFGTC, TIFIOC, TIFFOK:
	default:
		return ErrBadTIF
	}
	if o.Type == TypeMarket && o.TIF == TIFGTC {
		// A market order cannot rest, so GTC is meaningless for it.
		o.TIF = TIFIOC
	}
	if math.IsNaN(o.Quantity) || math.IsInf(o.Quantity, 0) || o.Quantity <= 0 {
		return ErrBadQuantity
	}
	if b.cfg.MinQuantity > 0 && o.Quantity < b.cfg.MinQuantity {
		return ErrQtyRange
	}
	if b.cfg.MaxQuantity > 0 && o.Quantity > b.cfg.MaxQuantity {
		return ErrQtyRange
	}

	units, err := toTicks(o.Quantity, b.cfg.QtyScale)
	if err != nil || units <= 0 {
		return ErrBadQuantity
	}
	o.qtyUnits = units
	o.filledUnits = 0

	if o.Type == TypeLimit {
		if math.IsNaN(o.Price) || math.IsInf(o.Price, 0) || o.Price <= 0 {
			return ErrBadPrice
		}
		ticks, err := toTicks(o.Price, b.cfg.PriceScale)
		if err != nil || ticks <= 0 {
			return ErrBadPrice
		}
		o.priceTicks = ticks
	} else {
		o.priceTicks = 0
	}
	return nil
}

// Submit matches an order against the book and, when it is a resting limit
// order with quantity left, adds it to the book. It returns the trades the
// order produced. A rejection is reported as an error and leaves the book
// untouched.
func (b *Book) Submit(o *Order) (Order, []Trade, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// The book owns the orders it rests, so it works on a copy. The caller's
	// struct is never retained: it may be reused or shared between goroutines.
	owned := *o

	if err := b.validate(&owned); err != nil {
		owned.Status = StatusRejected
		b.syncPublic(&owned)
		return owned, nil, err
	}

	now := time.Now().UnixMilli()
	owned.CreatedAt = now
	owned.UpdatedAt = now

	// A fill-all-or-kill order that cannot be filled entirely within its price
	// limit is rejected without trading anything.
	if owned.TIF == TIFFOK && b.availableUnits(&owned) < owned.qtyUnits {
		owned.Status = StatusRejected
		b.syncPublic(&owned)
		return owned, nil, ErrNoLiquidity
	}

	trades := b.match(&owned)

	if owned.leavesUnits() > 0 && owned.Type == TypeLimit && owned.TIF == TIFGTC {
		b.sideFor(owned.Side).insertLevel(owned.priceTicks).add(&owned)
		b.openOrders++
	} else if owned.leavesUnits() > 0 {
		// IOC, FOK remainder and market remainder never rest.
		owned.Status = StatusCancelled
	}

	b.syncPublic(&owned)
	return owned, trades, nil
}

// sideFor returns the book side an order of the given side rests on.
func (b *Book) sideFor(side string) *sideBook {
	if side == SideBuy {
		return b.bids
	}
	return b.asks
}

// availableUnits totals the units an incoming order could fill, honouring its
// price limit. Market orders accept any price.
func (b *Book) availableUnits(o *Order) int64 {
	opposite := b.sideFor(oppositeSide(o.Side))
	var total int64
	for _, l := range opposite.levels {
		if !b.crosses(o, l.priceTicks) {
			break
		}
		total += l.totalUnits
	}
	return total
}

// crosses reports whether an incoming order's price limit allows it to trade at
// the given resting price. Market orders cross any price.
func (b *Book) crosses(o *Order, restingTicks int64) bool {
	if o.Type == TypeMarket {
		return true
	}
	if o.Side == SideBuy {
		return restingTicks <= o.priceTicks
	}
	return restingTicks >= o.priceTicks
}

func oppositeSide(side string) string {
	if side == SideBuy {
		return SideSell
	}
	return SideBuy
}

// match walks the opposite side best-first, filling the incoming order.
func (b *Book) match(o *Order) []Trade {
	opposite := b.sideFor(oppositeSide(o.Side))
	var trades []Trade

	for o.leavesUnits() > 0 && len(opposite.levels) > 0 {
		l := opposite.levels[0]

		if !b.crosses(o, l.priceTicks) {
			break
		}

		for len(l.orders) > 0 && o.leavesUnits() > 0 {
			maker := l.orders[0]

			if maker.Trader == o.Trader {
				// Self-trade prevention: the resting order is cancelled so the
				// incoming order continues against the next participant. The
				// trader's own liquidity is never used to fill itself, which
				// would create a wash trade.
				b.cancelResting(opposite, l, maker)
				continue
			}

			qty := minInt64(o.leavesUnits(), maker.leavesUnits())
			if qty <= 0 {
				b.cancelResting(opposite, l, maker)
				continue
			}

			o.filledUnits += qty
			maker.filledUnits += qty
			l.totalUnits -= qty

			// The resting order sets the price, so the taker never pays worse
			// than the level it hit.
			trade := b.recordTrade(maker, o, l.priceTicks, qty)
			trades = append(trades, trade)

			maker.UpdatedAt = trade.Timestamp
			if maker.leavesUnits() == 0 {
				maker.Status = StatusFilled
				b.syncPublic(maker)
				b.openOrders--
				l.orders = l.orders[1:]
			} else {
				maker.Status = StatusPartial
				b.syncPublic(maker)
			}
		}

		if len(l.orders) == 0 {
			opposite.dropLevel(l.priceTicks)
		}
	}

	if len(trades) > 0 {
		b.lastPriceTk = trades[len(trades)-1].priceTicks
	}
	return trades
}

// cancelResting removes a resting order from the book as part of self-trade
// prevention, dropping its level when it empties.
func (b *Book) cancelResting(side *sideBook, l *level, o *Order) {
	l.remove(o.OrderID)
	o.Status = StatusCancelled
	o.UpdatedAt = time.Now().UnixMilli()
	b.openOrders--
	if len(l.orders) == 0 {
		side.dropLevel(l.priceTicks)
	}
}

// recordTrade builds the trade and appends it to the tape.
func (b *Book) recordTrade(maker, taker *Order, priceTicks, qtyUnits int64) Trade {
	price := float64(priceTicks) / float64(b.cfg.PriceScale)
	qty := float64(qtyUnits) / float64(b.cfg.QtyScale)
	notional := price * qty

	trade := Trade{
		Pair:       b.cfg.Pair,
		MakerID:    maker.OrderID,
		TakerID:    taker.OrderID,
		TakerSide:  taker.Side,
		Maker:      maker.Trader,
		Taker:      taker.Trader,
		Price:      price,
		Quantity:   qty,
		Notional:   notional,
		MakerFee:   notional * float64(b.cfg.MakerFeeBps) / 10000,
		TakerFee:   notional * float64(b.cfg.TakerFeeBps) / 10000,
		Timestamp:  time.Now().UnixMilli(),
		priceTicks: priceTicks,
		qtyUnits:   qtyUnits,
	}
	b.pushTrade(trade)
	return trade
}

func (b *Book) pushTrade(t Trade) {
	b.trades[b.tradeStart] = t
	b.tradeStart = (b.tradeStart + 1) % len(b.trades)
	if b.tradeCount < len(b.trades) {
		b.tradeCount++
	}
}

// syncPublic writes the order's public fields from its integer state. It does
// not overwrite a rejected or cancelled status.
func (b *Book) syncPublic(o *Order) {
	qtyScale := float64(b.cfg.QtyScale)
	o.Filled = float64(o.filledUnits) / qtyScale
	o.Leaves = float64(o.leavesUnits()) / qtyScale

	switch o.Status {
	case StatusRejected:
		o.Leaves = float64(o.leavesUnits()) / qtyScale
		return
	case StatusCancelled:
		// A cancelled remainder is no longer live, so it has no leaves.
		o.Leaves = 0
		return
	}

	switch {
	case o.leavesUnits() == 0:
		o.Status = StatusFilled
	case o.filledUnits > 0:
		o.Status = StatusPartial
	default:
		o.Status = StatusOpen
	}
}

// Cancel removes a resting order. Only the trader who placed it may cancel it.
func (b *Book) Cancel(orderID uint64, trader string) (Order, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, side := range []*sideBook{b.bids, b.asks} {
		for _, l := range side.levels {
			for _, o := range l.orders {
				if o.OrderID != orderID {
					continue
				}
				if o.Trader != trader {
					return Order{}, ErrNotOwner
				}
				l.remove(orderID)
				if len(l.orders) == 0 {
					side.dropLevel(l.priceTicks)
				}
				b.openOrders--
				o.Status = StatusCancelled
				o.UpdatedAt = time.Now().UnixMilli()
				b.syncPublic(o)
				return *o, nil
			}
		}
	}
	return Order{}, ErrUnknownOrder
}

// GetOrder returns a copy of a resting order.
func (b *Book) GetOrder(orderID uint64) (Order, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, side := range []*sideBook{b.bids, b.asks} {
		for _, l := range side.levels {
			for _, o := range l.orders {
				if o.OrderID == orderID {
					return *o, true
				}
			}
		}
	}
	return Order{}, false
}

// OrdersByTrader returns every resting order belonging to a trader, newest
// first. It walks the book because the venue does not keep a per-trader index.
func (b *Book) OrdersByTrader(trader string) []Order {
	b.mu.RLock()
	defer b.mu.RUnlock()

	out := make([]Order, 0)
	for _, side := range []*sideBook{b.bids, b.asks} {
		for _, l := range side.levels {
			for _, o := range l.orders {
				if o.Trader == trader {
					out = append(out, *o)
				}
			}
		}
	}
	return out
}

// Depth aggregates the book into up to limit levels per side.
func (b *Book) Depth(limit int) ([]Level, []Level) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}
	qtyScale := float64(b.cfg.QtyScale)
	priceScale := float64(b.cfg.PriceScale)

	build := func(s *sideBook) []Level {
		n := len(s.levels)
		if n > limit {
			n = limit
		}
		out := make([]Level, 0, n)
		for i := 0; i < n; i++ {
			l := s.levels[i]
			out = append(out, Level{
				Price:    float64(l.priceTicks) / priceScale,
				Quantity: float64(l.totalUnits) / qtyScale,
				Orders:   len(l.orders),
			})
		}
		return out
	}

	return build(b.bids), build(b.asks)
}

// BestBid and BestAsk return the top of book, or 0 when that side is empty.
func (b *Book) BestBid() float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if l := b.bids.best(); l != nil {
		return float64(l.priceTicks) / float64(b.cfg.PriceScale)
	}
	return 0
}

func (b *Book) BestAsk() float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if l := b.asks.best(); l != nil {
		return float64(l.priceTicks) / float64(b.cfg.PriceScale)
	}
	return 0
}

// LastPrice is the price of the most recent trade.
func (b *Book) LastPrice() float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return float64(b.lastPriceTk) / float64(b.cfg.PriceScale)
}

// OpenOrders counts resting orders.
func (b *Book) OpenOrders() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.openOrders
}

// Trades returns up to limit of the most recent trades, oldest first.
func (b *Book) Trades(limit int) []Trade {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if limit <= 0 || limit > b.tradeCount {
		limit = b.tradeCount
	}
	out := make([]Trade, 0, limit)
	start := (b.tradeStart - limit + len(b.trades)) % len(b.trades)
	for i := 0; i < limit; i++ {
		out = append(out, b.trades[(start+i)%len(b.trades)])
	}
	return out
}

// Engine owns every book and the global order index.
type Engine struct {
	mu        sync.RWMutex
	books     map[string]*Book
	orderPair map[uint64]string

	nextOrderID atomic.Uint64
	nextTradeID atomic.Uint64

	// onTrade is stored atomically so it can be installed at start-up and read
	// on the matching path without taking a lock per trade.
	onTrade atomic.Pointer[func(Trade)]

	submitted atomic.Uint64
	traded    atomic.Uint64
	rejected  atomic.Uint64
	started   time.Time
}

// NewEngine builds an empty engine with no listed pairs.
func NewEngine() *Engine {
	e := &Engine{
		books:     make(map[string]*Book),
		orderPair: make(map[uint64]string),
		started:   time.Now(),
	}
	// Seed the counters from the wall clock so ids are unique across restarts
	// even before any order is placed.
	seed := uint64(time.Now().UnixMilli()) << 20
	e.nextOrderID.Store(seed)
	e.nextTradeID.Store(seed)
	return e
}

// OnTrade registers a callback invoked after a trade settles. It runs outside
// the book lock, so the callback may call back into the engine.
func (e *Engine) OnTrade(fn func(Trade)) { e.onTrade.Store(&fn) }

// RegisterPair lists a pair for trading. Re-registering updates its config
// without dropping the resting book.
func (e *Engine) RegisterPair(cfg PairConfig) error {
	if cfg.Pair == "" {
		return errors.New("pair is required")
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	if existing, ok := e.books[cfg.Pair]; ok {
		existing.mu.Lock()
		existing.cfg = cfg.withDefaults()
		existing.mu.Unlock()
		return nil
	}
	e.books[cfg.Pair] = NewBook(cfg)
	return nil
}

// UnregisterPair stops trading a pair. It reports whether the pair was listed.
func (e *Engine) UnregisterPair(pair string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.books[pair]; !ok {
		return false
	}
	delete(e.books, pair)
	return true
}

// ListedPairs returns the currently tradable pairs.
func (e *Engine) ListedPairs() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]string, 0, len(e.books))
	for pair := range e.books {
		out = append(out, pair)
	}
	return out
}

func (e *Engine) book(pair string) (*Book, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	b, ok := e.books[pair]
	if !ok {
		return nil, ErrUnknownPair
	}
	return b, nil
}

// Submit validates and matches an order against its pair's book. The returned
// order carries its final state.
func (e *Engine) Submit(o Order) (Order, []Trade, error) {
	book, err := e.book(o.Pair)
	if err != nil {
		e.rejected.Add(1)
		return o, nil, err
	}

	o.OrderID = e.nextOrderID.Add(1)

	snapshot, trades, err := book.Submit(&o)
	if err != nil {
		e.rejected.Add(1)
		return snapshot, nil, err
	}

	for i := range trades {
		trades[i].TradeID = e.nextTradeID.Add(1)
	}

	e.mu.Lock()
	e.orderPair[snapshot.OrderID] = snapshot.Pair
	e.mu.Unlock()

	e.submitted.Add(1)
	e.traded.Add(uint64(len(trades)))

	if fn := e.onTrade.Load(); fn != nil {
		for _, t := range trades {
			(*fn)(t)
		}
	}
	return snapshot, trades, nil
}

// Cancel cancels a resting order, enforcing that the caller owns it.
func (e *Engine) Cancel(orderID uint64, trader string) (Order, error) {
	e.mu.RLock()
	pair, ok := e.orderPair[orderID]
	e.mu.RUnlock()
	if !ok {
		return Order{}, ErrUnknownOrder
	}

	book, err := e.book(pair)
	if err != nil {
		return Order{}, err
	}

	return book.Cancel(orderID, trader)
}

// GetOrder finds an order in whichever book holds it.
func (e *Engine) GetOrder(orderID uint64) (Order, error) {
	e.mu.RLock()
	pair, ok := e.orderPair[orderID]
	e.mu.RUnlock()
	if !ok {
		return Order{}, ErrUnknownOrder
	}
	book, err := e.book(pair)
	if err != nil {
		return Order{}, err
	}
	o, found := book.GetOrder(orderID)
	if !found {
		return Order{}, ErrUnknownOrder
	}
	return o, nil
}

// Book returns the book for a pair, or an error when it is not listed.
func (e *Engine) Book(pair string) (*Book, error) { return e.book(pair) }

// Stats reports engine-wide counters.
type Stats struct {
	Submitted       uint64    `json:"total_orders"`
	Trades          uint64    `json:"total_trades"`
	Rejected        uint64    `json:"rejected_orders"`
	ListedPairs     int       `json:"listed_pairs"`
	OpenOrders      int       `json:"open_orders"`
	OrdersPerSecond float64   `json:"orders_per_second"`
	LastUpdate      time.Time `json:"last_update"`
}

// Stats returns a consistent snapshot of the engine counters.
func (e *Engine) Stats() Stats {
	e.mu.RLock()
	books := make([]*Book, 0, len(e.books))
	for _, b := range e.books {
		books = append(books, b)
	}
	listed := len(e.books)
	e.mu.RUnlock()

	open := 0
	for _, b := range books {
		open += b.OpenOrders()
	}

	submitted := e.submitted.Load()
	elapsed := time.Since(e.started).Seconds()
	var rate float64
	if elapsed > 0 {
		rate = float64(submitted) / elapsed
	}

	return Stats{
		Submitted:       submitted,
		Trades:          e.traded.Load(),
		Rejected:        e.rejected.Load(),
		ListedPairs:     listed,
		OpenOrders:      open,
		OrdersPerSecond: rate,
		LastUpdate:      time.Now(),
	}
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
