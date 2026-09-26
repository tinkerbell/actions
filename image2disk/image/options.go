package image

import (
	"log/slog"
	"net/http"
	"runtime"
	"time"
)

// WriteMode selects how decoded chunks reach the device.
type WriteMode string

const (
	// ModePlain writes every chunk unconditionally.
	ModePlain WriteMode = "plain"
	// ModeDiff reads each target range first and rewrites only differing blocks.
	ModeDiff WriteMode = "diff"
)

// Defaults for Options. Buffers are sized for a host with about a gigabyte to
// spare, which is what image2disk runs on in practice.
const (
	defaultMemoryBudget     = 768 << 20
	defaultWriters          = 4
	defaultRotationalWriter = 1
	defaultQueueDepth       = 8
	defaultReadAheadBufs    = 8
	defaultReadAheadBufSize = 8 << 20
	defaultChunkSize        = 16 << 20
	defaultStallTimeout     = 120 * time.Second
	defaultHeaderTimeout    = 30 * time.Second
	maxZstdConcurrency      = 8
)

// Options configures Write. The zero value is usable: applyDefaults fills in
// every field that matters.
type Options struct {
	// Logger receives progress and diagnostics. Required.
	Logger *slog.Logger
	// ProgressInterval is how often to log progress. Zero or negative disables
	// progress logging entirely.
	ProgressInterval time.Duration

	// Compressed enables decompression, chosen from the image URL's extension.
	Compressed bool

	// Mode selects the chunk writer. Empty means ModePlain.
	Mode WriteMode
	// DiffBlockSize is the comparison granularity in ModeDiff. Zero means the
	// device's physical block size.
	DiffBlockSize int

	// MemoryBudget is a hard cap in bytes on the pipeline's buffer pools.
	MemoryBudget int64
	// ChunkSize is the pipeline chunk size in bytes. Zero derives it from
	// MemoryBudget. It is always rounded to a multiple of the device alignment.
	ChunkSize int
	// Writers is the number of concurrent pwrite workers. Zero picks a default
	// from the device: one for rotational media, four otherwise.
	Writers int
	// QueueDepth is the capacity of the chunk channel. Zero uses a default.
	QueueDepth int
	// ReadAheadBufs and ReadAheadBufSize size the HTTP read-ahead ring.
	ReadAheadBufs    int
	ReadAheadBufSize int

	// StallTimeout cancels the transfer when no bytes arrive for this long.
	// Zero uses a default; negative disables the watchdog.
	StallTimeout time.Duration
	// HTTPClient overrides the tuned default client.
	HTTPClient *http.Client

	// SkipSync skips the closing fsync and partition-table reread. Tests only.
	SkipSync bool
}

// Result reports what a completed Write did.
type Result struct {
	// Format names the decompressor used, or "none".
	Format string
	// Geometry is the destination's probed block geometry.
	Geometry Geometry
	// Mode is the write mode actually used, which may differ from the requested
	// mode if a read-back probe forced a downgrade to ModePlain.
	Mode WriteMode
	// ChunkSize and Writers are the values the pipeline settled on.
	ChunkSize int
	Writers   int

	// BytesRead is the compressed byte count taken off the wire.
	BytesRead int64
	// BytesDecoded is the byte count the decompressor produced.
	BytesDecoded int64
	// BytesWritten is the byte count actually pwritten to the device.
	BytesWritten int64
	// BytesSkipped is the byte count ModeDiff found identical and did not rewrite.
	BytesSkipped int64
	// WriteCalls is the number of pwrite syscalls issued.
	WriteCalls int64
	// DiffFallbacks counts chunks written unconditionally because reading the
	// device back failed.
	DiffFallbacks int64

	// Duration is the wall clock time of the transfer.
	Duration time.Duration
}

// applyDefaults fills in unset fields, sizing the buffer pools so that the
// pipeline's peak footprint stays within MemoryBudget.
func (o *Options) applyDefaults(g Geometry) {
	if o.Mode == "" {
		o.Mode = ModePlain
	}
	if o.MemoryBudget <= 0 {
		o.MemoryBudget = defaultMemoryBudget
	}
	if o.Writers <= 0 {
		o.Writers = defaultWriters
		if g.Rotational {
			// Concurrent pwrites defeat seek ordering on spinning media.
			o.Writers = defaultRotationalWriter
		}
	}
	if o.QueueDepth <= 0 {
		o.QueueDepth = defaultQueueDepth
	}
	if o.ReadAheadBufs <= 0 {
		o.ReadAheadBufs = defaultReadAheadBufs
	}
	if o.ReadAheadBufSize <= 0 {
		o.ReadAheadBufSize = defaultReadAheadBufSize
	}
	if o.DiffBlockSize <= 0 {
		o.DiffBlockSize = g.DiffBlockSize()
	}
	if o.StallTimeout == 0 {
		o.StallTimeout = defaultStallTimeout
	}

	o.fitToBudget(g)
}

// fitToBudget sets ChunkSize, shrinking the read-ahead ring and then the chunk
// queue if the requested sizes do not fit MemoryBudget.
func (o *Options) fitToBudget(g Geometry) {
	align := g.Align()

	// Shrink the read-ahead ring until it is at most a quarter of the budget,
	// so there is always room left for the chunk pool.
	for o.ReadAheadBufs > 1 && int64(o.ReadAheadBufs)*int64(o.ReadAheadBufSize) > o.MemoryBudget/4 {
		o.ReadAheadBufs--
	}
	for o.ReadAheadBufSize > align && int64(o.ReadAheadBufs)*int64(o.ReadAheadBufSize) > o.MemoryBudget/4 {
		o.ReadAheadBufSize /= 2
	}

	remaining := o.MemoryBudget - int64(o.ReadAheadBufs)*int64(o.ReadAheadBufSize)
	if remaining < int64(align) {
		remaining = int64(align)
	}

	requested := o.ChunkSize
	for {
		// One chunk per queue slot, one in flight per writer, one being filled
		// by the chunker; in diff mode each writer also holds a compare buffer.
		live := int64(o.QueueDepth + o.Writers + 1)
		if o.Mode == ModeDiff {
			live += int64(o.Writers)
		}

		budgeted := int(remaining / live)
		want := requested
		if want <= 0 {
			want = min(budgeted, defaultChunkSize)
		}
		want = min(want, budgeted)

		if want >= align || o.QueueDepth <= 1 {
			o.ChunkSize = g.AlignChunkSize(want)
			return
		}
		o.QueueDepth--
	}
}

// zstdConcurrency is the decoder concurrency for zstd streams. klauspost's
// stream decoder saturates around three cores whatever this is set to, so the
// cap exists to bound decoder window memory rather than to limit throughput.
func zstdConcurrency() int {
	return min(runtime.GOMAXPROCS(0), maxZstdConcurrency)
}
