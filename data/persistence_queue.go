package data

import (
	"context"
	"log"
	"sync/atomic"
)

const defaultPersistenceQueueCap = 4096
const persistenceFlushBatchMax = 256

// PersistJob is one closed bar destined for the SQLite archive.
type PersistJob struct {
	Symbol   string
	Interval string
	Candle   Candle
}

// PersistenceQueue isolates disk I/O from the live WS/DAG hot path (Shot 9C/9E).
// It is the sole production writer into historical_klines (via SaveKlines).
// Enqueue never blocks the caller; a full buffer drops the job and increments Dropped.
// AppendClosedBars blocks (for REST catch-up) until space is available or ctx cancels.
type PersistenceQueue struct {
	ch      chan PersistJob
	Dropped atomic.Uint64
}

// NewPersistenceQueue creates a buffered archive writer. buffer<=0 → defaultCapacity.
func NewPersistenceQueue(buffer int) *PersistenceQueue {
	if buffer <= 0 {
		buffer = defaultPersistenceQueueCap
	}
	return &PersistenceQueue{ch: make(chan PersistJob, buffer)}
}

// Start launches the single worker that drains the queue into SaveKlines (UPSERT).
func (q *PersistenceQueue) Start(ctx context.Context) {
	if q == nil {
		return
	}
	go q.worker(ctx)
}

// Enqueue offers a closed candle to the archive worker without blocking.
// Returns false if the buffer is full (job dropped) — live loop must continue.
func (q *PersistenceQueue) Enqueue(symbol, interval string, candle Candle) bool {
	if q == nil || q.ch == nil {
		return false
	}
	job := PersistJob{Symbol: symbol, Interval: interval, Candle: candle}
	select {
	case q.ch <- job:
		return true
	default:
		q.Dropped.Add(1)
		n := q.Dropped.Load()
		if n == 1 || n%100 == 0 {
			log.Printf("[PersistenceQueue] drop closed bar (buffer full) dropped=%d %s %s open=%d",
				n, symbol, interval, candle.OpenTime)
		}
		return false
	}
}

// AppendClosedBars enqueues real closed bars for archive write (blocking).
// Used by SQLite tip catch-up so REST never calls SaveKlines directly (Shot 9E).
func (q *PersistenceQueue) AppendClosedBars(ctx context.Context, symbol, interval string, candles []Candle) error {
	if q == nil || q.ch == nil {
		return nil
	}
	for _, c := range candles {
		job := PersistJob{Symbol: symbol, Interval: interval, Candle: c}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case q.ch <- job:
		}
	}
	return nil
}

func (q *PersistenceQueue) worker(ctx context.Context) {
	log.Printf("[PersistenceQueue] worker started (cap=%d)", cap(q.ch))
	for {
		select {
		case <-ctx.Done():
			q.drainRemaining()
			log.Printf("[PersistenceQueue] worker stopped dropped=%d", q.Dropped.Load())
			return
		case job := <-q.ch:
			q.flushBatch(collectPersistBatch(q.ch, job))
		}
	}
}

func collectPersistBatch(ch <-chan PersistJob, first PersistJob) []PersistJob {
	batch := []PersistJob{first}
	for len(batch) < persistenceFlushBatchMax {
		select {
		case job := <-ch:
			batch = append(batch, job)
		default:
			return batch
		}
	}
	return batch
}

func (q *PersistenceQueue) flushBatch(jobs []PersistJob) {
	if len(jobs) == 0 {
		return
	}
	// Group by symbol+interval for one UPSERT transaction each.
	type key struct{ sym, iv string }
	groups := make(map[key][]Candle, 4)
	order := make([]key, 0, 4)
	for _, job := range jobs {
		k := key{job.Symbol, job.Interval}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], job.Candle)
	}
	for _, k := range order {
		if err := SaveKlines(k.sym, k.iv, groups[k]); err != nil {
			log.Printf("[PersistenceQueue] SaveKlines %s %s n=%d: %v",
				k.sym, k.iv, len(groups[k]), err)
		}
	}
}

func (q *PersistenceQueue) drainRemaining() {
	var jobs []PersistJob
	for {
		select {
		case job := <-q.ch:
			jobs = append(jobs, job)
			if len(jobs) >= persistenceFlushBatchMax {
				q.flushBatch(jobs)
				jobs = jobs[:0]
			}
		default:
			q.flushBatch(jobs)
			return
		}
	}
}
