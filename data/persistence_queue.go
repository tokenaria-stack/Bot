package data

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const defaultPersistenceQueueCap = 4096
const persistenceFlushBatchMax = 256

const (
	persistSaveMaxAttempts = 8
	persistRetryBase       = 25 * time.Millisecond
	persistRetryMaxSleep   = 2 * time.Second
)

// PersistJob is one closed bar destined for the SQLite archive.
type PersistJob struct {
	Symbol   string
	Interval string
	Candle   Candle
}

// PersistenceQueue isolates disk I/O from the live WS/DAG hot path (Shot 9C/9E).
// It is the sole production writer into historical_klines (via SaveKlines).
//
// Closed bars are never silently discarded:
//   - Enqueue blocks until the job is accepted (no drop-on-full).
//   - SaveKlines transient failures (SQLITE_BUSY) are retried with backoff.
//   - After exhausting retries, jobs stay on an in-memory spill and the failure
//     is surfaced via Failures/LastError — never reported as success.
type PersistenceQueue struct {
	ch   chan PersistJob
	save func(symbol, interval string, klines []Candle) error

	spillMu sync.Mutex
	spill   []PersistJob

	Failures atomic.Uint64
	lastErr  atomic.Value // string

	// Dropped is retained for metrics compatibility; the live path no longer increments it.
	Dropped atomic.Uint64
}

// NewPersistenceQueue creates a buffered archive writer. buffer<=0 → defaultCapacity.
func NewPersistenceQueue(buffer int) *PersistenceQueue {
	if buffer <= 0 {
		buffer = defaultPersistenceQueueCap
	}
	q := &PersistenceQueue{ch: make(chan PersistJob, buffer)}
	q.save = SaveKlines
	q.lastErr.Store("")
	return q
}

// SetSaveFunc overrides the UPSERT sink (tests only).
func (q *PersistenceQueue) SetSaveFunc(fn func(symbol, interval string, klines []Candle) error) {
	if q == nil {
		return
	}
	if fn == nil {
		q.save = SaveKlines
		return
	}
	q.save = fn
}

// Start launches the single worker that drains the queue into SaveKlines (UPSERT).
func (q *PersistenceQueue) Start(ctx context.Context) {
	if q == nil {
		return
	}
	go q.worker(ctx)
}

// Enqueue offers a closed candle to the archive worker.
// Blocks until the job is queued — closed market data is never silently dropped.
// Returns false only when q is nil/uninitialized.
func (q *PersistenceQueue) Enqueue(symbol, interval string, candle Candle) bool {
	if q == nil || q.ch == nil {
		return false
	}
	job := PersistJob{Symbol: symbol, Interval: interval, Candle: candle}
	q.ch <- job
	return true
}

// EnqueueContext is Enqueue with cancellation (tests / controlled shutdown).
func (q *PersistenceQueue) EnqueueContext(ctx context.Context, symbol, interval string, candle Candle) error {
	if q == nil || q.ch == nil {
		return nil
	}
	job := PersistJob{Symbol: symbol, Interval: interval, Candle: candle}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case q.ch <- job:
		return nil
	}
}

// AppendClosedBars enqueues real closed bars for archive write (blocking).
// Used by SQLite tip/gap catch-up so REST never calls SaveKlines directly (Shot 9E).
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

// FailuresCount returns how many save groups exhausted retries (still held on spill).
func (q *PersistenceQueue) FailuresCount() uint64 {
	if q == nil {
		return 0
	}
	return q.Failures.Load()
}

// LastError returns the most recent exhausted-retry error string (empty if none).
func (q *PersistenceQueue) LastError() string {
	if q == nil {
		return ""
	}
	v, _ := q.lastErr.Load().(string)
	return v
}

// SpillLen reports pending spill jobs (tests / health).
func (q *PersistenceQueue) SpillLen() int {
	if q == nil {
		return 0
	}
	q.spillMu.Lock()
	defer q.spillMu.Unlock()
	return len(q.spill)
}

// walCheckpointInterval paces forced WAL truncation from the sole writer:
// between flushBatch calls no write transaction is open, so TRUNCATE can succeed.
const walCheckpointInterval = 5 * time.Minute

func (q *PersistenceQueue) worker(ctx context.Context) {
	log.Printf("[PersistenceQueue] worker started (cap=%d)", cap(q.ch))
	walTicker := time.NewTicker(walCheckpointInterval)
	defer walTicker.Stop()
	spillBackoff := time.Duration(0)
	for {
		if q.SpillLen() > 0 {
			if spillBackoff > 0 {
				timer := time.NewTimer(spillBackoff)
				select {
				case <-ctx.Done():
					timer.Stop()
					q.drainRemaining()
					log.Printf("[PersistenceQueue] worker stopped failures=%d spill=%d",
						q.Failures.Load(), q.SpillLen())
					return
				case <-timer.C:
				}
			}
			before := q.SpillLen()
			q.flushSpill()
			if q.SpillLen() > 0 && q.SpillLen() >= before {
				if spillBackoff == 0 {
					spillBackoff = persistRetryBase
				} else {
					spillBackoff *= 2
					if spillBackoff > persistRetryMaxSleep {
						spillBackoff = persistRetryMaxSleep
					}
				}
			} else {
				spillBackoff = 0
			}
			continue
		}
		spillBackoff = 0
		select {
		case <-ctx.Done():
			q.drainRemaining()
			log.Printf("[PersistenceQueue] worker stopped failures=%d spill=%d",
				q.Failures.Load(), q.SpillLen())
			return
		case <-walTicker.C:
			if err := CheckpointWAL(); err != nil {
				log.Printf("[PersistenceQueue] WAL checkpoint: %v", err)
			}
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
	type key struct{ sym, iv string }
	groups := make(map[key][]Candle, 4)
	order := make([]key, 0, 4)
	jobByKey := make(map[key][]PersistJob, 4)
	for _, job := range jobs {
		k := key{job.Symbol, job.Interval}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], job.Candle)
		jobByKey[k] = append(jobByKey[k], job)
	}
	save := q.save
	if save == nil {
		save = SaveKlines
	}
	for _, k := range order {
		candles := groups[k]
		if err := q.saveWithRetry(save, k.sym, k.iv, candles); err != nil {
			q.Failures.Add(1)
			q.lastErr.Store(err.Error())
			log.Printf("[PersistenceQueue] HARD SaveKlines exhausted retries %s %s n=%d: %v — re-spilling (closed bars not discarded)",
				k.sym, k.iv, len(candles), err)
			q.pushSpill(jobByKey[k])
			continue
		}
		NotePersistEdges(k.sym, k.iv, candles)
	}
}

func (q *PersistenceQueue) saveWithRetry(save func(string, string, []Candle) error, sym, iv string, candles []Candle) error {
	var err error
	sleep := persistRetryBase
	for attempt := 1; attempt <= persistSaveMaxAttempts; attempt++ {
		err = save(sym, iv, candles)
		if err == nil {
			return nil
		}
		if !IsTransientSQLiteError(err) {
			return err
		}
		if attempt == persistSaveMaxAttempts {
			break
		}
		log.Printf("[PersistenceQueue] SaveKlines busy retry %d/%d %s %s n=%d: %v",
			attempt, persistSaveMaxAttempts, sym, iv, len(candles), err)
		time.Sleep(sleep)
		sleep *= 2
		if sleep > persistRetryMaxSleep {
			sleep = persistRetryMaxSleep
		}
	}
	return err
}

func (q *PersistenceQueue) pushSpill(jobs []PersistJob) {
	if len(jobs) == 0 {
		return
	}
	q.spillMu.Lock()
	q.spill = append(q.spill, jobs...)
	q.spillMu.Unlock()
}

// flushSpill retries one spilled batch. Returns true if work was attempted.
func (q *PersistenceQueue) flushSpill() bool {
	q.spillMu.Lock()
	if len(q.spill) == 0 {
		q.spillMu.Unlock()
		return false
	}
	n := len(q.spill)
	if n > persistenceFlushBatchMax {
		n = persistenceFlushBatchMax
	}
	batch := append([]PersistJob(nil), q.spill[:n]...)
	q.spill = q.spill[n:]
	q.spillMu.Unlock()
	q.flushBatch(batch)
	return true
}

func (q *PersistenceQueue) drainRemaining() {
	for q.flushSpill() {
	}
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
			for q.flushSpill() {
			}
			return
		}
	}
}
