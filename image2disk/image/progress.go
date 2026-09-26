package image

import (
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
)

// counters accumulate per-stage byte counts for progress reporting.
type counters struct {
	// read is compressed bytes taken off the wire.
	read atomic.Int64
	// decoded is bytes produced by the decompressor.
	decoded atomic.Int64
	// written is bytes actually pwritten to the device.
	written atomic.Int64
	// skipped is bytes the diff writer found identical.
	skipped atomic.Int64
	// calls is the number of pwrite syscalls issued.
	calls atomic.Int64
}

// snapshot is a point-in-time reading of counters, used to compute rates.
type snapshot struct {
	at                              time.Time
	read, decoded, written, skipped int64
}

// take reads the counters.
func (c *counters) take() snapshot {
	return snapshot{
		at:      time.Now(),
		read:    c.read.Load(),
		decoded: c.decoded.Load(),
		written: c.written.Load(),
		skipped: c.skipped.Load(),
	}
}

// reporter periodically logs transfer progress and per-stage throughput.
type reporter struct {
	log      *slog.Logger
	ctr      *counters
	total    int64 // compressed content length, or -1 when unknown
	diff     bool
	stopped  chan struct{}
	finished chan struct{}
}

// startReporter begins periodic progress logging and returns a function that
// stops it and emits one final line. A non-positive interval disables reporting
// entirely; time.NewTicker panics on one.
func startReporter(log *slog.Logger, ctr *counters, total int64, diff bool, interval time.Duration) func() {
	if interval <= 0 {
		return func() {}
	}

	r := &reporter{
		log:      log,
		ctr:      ctr,
		total:    total,
		diff:     diff,
		stopped:  make(chan struct{}),
		finished: make(chan struct{}),
	}
	go r.loop(interval)

	return func() {
		close(r.stopped)
		<-r.finished
	}
}

// loop emits a progress line every interval until stopped.
func (r *reporter) loop(interval time.Duration) {
	defer close(r.finished)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	start := r.ctr.take()
	prev := start

	for {
		select {
		case <-r.stopped:
			r.emit("transfer complete", start, time.Now())
			return
		case now := <-ticker.C:
			r.emit("progress", prev, now)
			prev = r.ctr.take()
		}
	}
}

// emit logs one progress line, with rates measured since prev.
func (r *reporter) emit(msg string, prev snapshot, now time.Time) {
	cur := r.ctr.take()
	elapsed := now.Sub(prev.at).Seconds()

	attrs := []any{
		"read", humanize.IBytes(uint64(cur.read)),
		"readRate", rate(cur.read-prev.read, elapsed),
		"decoded", humanize.IBytes(uint64(cur.decoded)),
		"decodeRate", rate(cur.decoded-prev.decoded, elapsed),
		"written", humanize.IBytes(uint64(cur.written)),
		"writeRate", rate(cur.written-prev.written, elapsed),
	}

	// A chunked response has no content length, so percentage and ETA are not
	// knowable. Omit them rather than reporting nonsense.
	if r.total > 0 {
		pct := float64(cur.read) / float64(r.total) * 100
		attrs = append(attrs, "pct", fmt.Sprintf("%.1f%%", pct), "compressedSize", humanize.IBytes(uint64(r.total)))
		if d := cur.read - prev.read; d > 0 && elapsed > 0 {
			remaining := time.Duration(float64(r.total-cur.read)/(float64(d)/elapsed)) * time.Second
			attrs = append(attrs, "eta", remaining.Round(time.Second).String())
		}
	}

	if r.diff {
		attrs = append(attrs, "skipped", humanize.IBytes(uint64(cur.skipped)))
		if touched := cur.written + cur.skipped; touched > 0 {
			attrs = append(attrs, "skippedPct", fmt.Sprintf("%.1f%%", float64(cur.skipped)/float64(touched)*100))
		}
	}

	r.log.Info(msg, attrs...)
}

// rate formats a byte delta over elapsed seconds as a human-readable throughput.
func rate(delta int64, elapsed float64) string {
	if elapsed <= 0 || delta < 0 {
		return "0 B/s"
	}
	return humanize.IBytes(uint64(float64(delta)/elapsed)) + "/s"
}
