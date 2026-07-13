package data

import (
	"context"
	"log"
	"sync/atomic"
)

const defaultPersistenceQueueCap = 4096

// PersistJob is one closed bar destined for the SQLite archive.
type PersistJob struct {
	Symbol   string
	Interval string
	Candle   Candle
}

// PersistenceQueue isolates disk I/O from the live WS/DAG hot path (Shot 9C).
// Enqueue never blocks the caller; a full buffer drops the job and increments Dropped.
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
// Returns when ctx is cancelled after the channel is drained for in-flight jobs up to buffer.
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

func (q *PersistenceQueue) worker(ctx context.Context) {
	log.Printf("[PersistenceQueue] worker started (cap=%d)", cap(q.ch))
	for {
		select {
		case <-ctx.Done():
			q.drainRemaining()
			log.Printf("[PersistenceQueue] worker stopped dropped=%d", q.Dropped.Load())
			return
		case job := <-q.ch:
			q.flushJob(job)
		}
	}
}

func (q *PersistenceQueue) flushJob(job PersistJob) {
	if err := SaveKlines(job.Symbol, job.Interval, []Candle{job.Candle}); err != nil {
		log.Printf("[PersistenceQueue] SaveKlines %s %s open=%d: %v",
			job.Symbol, job.Interval, job.Candle.OpenTime, err)
	}
}

func (q *PersistenceQueue) drainRemaining() {
	for {
		select {
		case job := <-q.ch:
			q.flushJob(job)
		default:
			return
		}
	}
}
