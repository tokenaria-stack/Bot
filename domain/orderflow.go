package domain

import "sync"

const (
	DefaultTickBufferCap        = 100_000
	DefaultLiquidationBufferCap = 1_000
)

// AggTrade is a normalized Binance futures aggregated trade tick.
type AggTrade struct {
	Price        float64
	Quantity     float64
	Time         int64 // Unix milliseconds (trade time)
	IsBuyerMaker bool
}

// Liquidation is a normalized Binance forceOrder (liquidation) event.
type Liquidation struct {
	Price    float64
	Quantity float64
	Side     string // exchange side: SELL = long liq, BUY = short liq
	Time     int64  // Unix milliseconds
}

// OrderFlowStore holds live microstructure buffers shared by WS and dashboard.
type OrderFlowStore struct {
	Ticks        *TickBuffer
	Liquidations *LiquidationBuffer
}

// NewOrderFlowStore creates ring buffers with default capacities.
func NewOrderFlowStore() *OrderFlowStore {
	return &OrderFlowStore{
		Ticks:        NewTickBuffer(DefaultTickBufferCap),
		Liquidations: NewLiquidationBuffer(DefaultLiquidationBufferCap),
	}
}

// TickBuffer is a fixed-size ring buffer for raw aggTrade ticks.
type TickBuffer struct {
	mu   sync.RWMutex
	cap  int
	data []AggTrade
	head int
	size int
}

// NewTickBuffer creates a tick ring buffer with the given capacity.
func NewTickBuffer(capacity int) *TickBuffer {
	if capacity <= 0 {
		capacity = DefaultTickBufferCap
	}
	return &TickBuffer{
		cap:  capacity,
		data: make([]AggTrade, capacity),
	}
}

// Add appends a trade, overwriting the oldest slot when full.
func (b *TickBuffer) Add(trade AggTrade) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.data[b.head] = trade
	b.head = (b.head + 1) % b.cap
	if b.size < b.cap {
		b.size++
	}
}

// Len returns the number of trades currently stored.
func (b *TickBuffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.size
}

// All returns all trades in chronological order.
func (b *TickBuffer) All() []AggTrade {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.snapshotLocked(0)
}

// Since returns trades with Time >= timestampMs in chronological order.
func (b *TickBuffer) Since(timestampMs int64) []AggTrade {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.snapshotLocked(timestampMs)
}

// Before returns trades with Time < beforeMs in chronological order.
func (b *TickBuffer) Before(beforeMs int64) []AggTrade {
	b.mu.RLock()
	defer b.mu.RUnlock()

	out := make([]AggTrade, 0, b.size)
	b.iterateLocked(func(t AggTrade) {
		if t.Time < beforeMs {
			out = append(out, t)
		}
	})
	return out
}

func (b *TickBuffer) snapshotLocked(sinceMs int64) []AggTrade {
	out := make([]AggTrade, 0, b.size)
	b.iterateLocked(func(t AggTrade) {
		if sinceMs == 0 || t.Time >= sinceMs {
			out = append(out, t)
		}
	})
	return out
}

func (b *TickBuffer) iterateLocked(fn func(AggTrade)) {
	if b.size == 0 {
		return
	}
	start := 0
	if b.size == b.cap {
		start = b.head
	}
	for i := 0; i < b.size; i++ {
		fn(b.data[(start+i)%b.cap])
	}
}

// LiquidationBuffer is a fixed-size ring buffer for liquidation events.
type LiquidationBuffer struct {
	mu   sync.RWMutex
	cap  int
	data []Liquidation
	head int
	size int
}

// NewLiquidationBuffer creates a liquidation ring buffer.
func NewLiquidationBuffer(capacity int) *LiquidationBuffer {
	if capacity <= 0 {
		capacity = DefaultLiquidationBufferCap
	}
	return &LiquidationBuffer{
		cap:  capacity,
		data: make([]Liquidation, capacity),
	}
}

// Add appends a liquidation, overwriting the oldest slot when full.
func (b *LiquidationBuffer) Add(liq Liquidation) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.data[b.head] = liq
	b.head = (b.head + 1) % b.cap
	if b.size < b.cap {
		b.size++
	}
}

// Len returns the number of liquidations stored.
func (b *LiquidationBuffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.size
}

// All returns all liquidations in chronological order.
func (b *LiquidationBuffer) All() []Liquidation {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.size == 0 {
		return nil
	}
	out := make([]Liquidation, 0, b.size)
	start := 0
	if b.size == b.cap {
		start = b.head
	}
	for i := 0; i < b.size; i++ {
		out = append(out, b.data[(start+i)%b.cap])
	}
	return out
}

// PushAggTrade implements exchange.OrderFlowSink for WebSocket ingestion.
func (s *OrderFlowStore) PushAggTrade(price, qty float64, timeMs int64, isBuyerMaker bool) {
	if s == nil || s.Ticks == nil || price <= 0 || timeMs <= 0 {
		return
	}
	s.Ticks.Add(AggTrade{
		Price:        price,
		Quantity:     qty,
		Time:         timeMs,
		IsBuyerMaker: isBuyerMaker,
	})
}

// PushLiquidation implements exchange.OrderFlowSink for WebSocket ingestion.
func (s *OrderFlowStore) PushLiquidation(price, qty float64, side string, timeMs int64) {
	if s == nil || s.Liquidations == nil || price <= 0 || qty <= 0 || timeMs <= 0 {
		return
	}
	s.Liquidations.Add(Liquidation{
		Price:    price,
		Quantity: qty,
		Side:     side,
		Time:     timeMs,
	})
}
