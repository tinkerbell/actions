package image

import "sync"

// Chunk is an offset-tagged unit of decoded image data.
//
// Data always has cap equal to the owning pool's chunk size; len is short only
// for the final chunk of a stream. A Chunk belongs to exactly one goroutine at
// a time: the chunker fills it, hands it to a writer worker over a channel, and
// only that worker returns it to the pool.
type Chunk struct {
	// Off is the absolute byte offset of Data within the destination.
	Off int64
	// Data is the chunk payload.
	Data []byte
}

// chunkPool recycles Chunk buffers of a fixed size.
//
// The pool does not bound memory; the pipeline does, structurally. At most
// queueDepth+writers+1 chunks can be alive at once because every other one is
// either sitting in a bounded channel or owned by a single goroutine.
type chunkPool struct {
	pool sync.Pool
	size int
}

// newChunkPool returns a pool of Chunks whose Data has capacity size.
func newChunkPool(size int) *chunkPool {
	p := &chunkPool{size: size}
	p.pool = sync.Pool{
		New: func() any { return &Chunk{Data: make([]byte, 0, size)} },
	}
	return p
}

// get returns a zeroed Chunk with an empty, full-capacity Data.
func (p *chunkPool) get() *Chunk {
	c, ok := p.pool.Get().(*Chunk)
	if !ok || cap(c.Data) < p.size {
		c = &Chunk{Data: make([]byte, 0, p.size)}
	}
	c.Data = c.Data[:0]
	c.Off = 0
	return c
}

// put returns c to the pool. The caller must not touch c afterwards.
func (p *chunkPool) put(c *Chunk) {
	c.Data = c.Data[:0]
	c.Off = 0
	p.pool.Put(c)
}

// bufPool recycles plain byte buffers of a fixed size, used by the diff writer
// for device read-back. It stores pointers so that staticcheck's SA6002 (which
// objects to storing slice headers in a sync.Pool) is satisfied.
type bufPool struct {
	pool sync.Pool
	size int
}

// newBufPool returns a pool of byte buffers of the given size.
func newBufPool(size int) *bufPool {
	p := &bufPool{size: size}
	p.pool = sync.Pool{
		New: func() any {
			b := make([]byte, size)
			return &b
		},
	}
	return p
}

// get returns a buffer of at least the pool's size.
func (p *bufPool) get() *[]byte {
	b, ok := p.pool.Get().(*[]byte)
	if !ok || len(*b) < p.size {
		nb := make([]byte, p.size)
		return &nb
	}
	return b
}

// put returns b to the pool.
func (p *bufPool) put(b *[]byte) {
	p.pool.Put(b)
}
