package nodes

import (
	"bytes"
	"fmt"
	"sync"
)

type boundedBuffer struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func newBoundedBuffer(limit int) *boundedBuffer {
	if limit <= 0 {
		limit = 1 << 20
	}
	return &boundedBuffer{limit: limit}
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 {
		b.exceeded = true
		return len(p), nil
	}
	toWrite := p
	if len(toWrite) > remaining {
		toWrite = toWrite[:remaining]
		b.exceeded = true
	}
	_, _ = b.buffer.Write(toWrite)
	// Report the complete input as consumed so child-process pipes keep draining;
	// excess bytes are intentionally discarded after the configured limit.
	return len(p), nil
}

func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func (b *boundedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buffer.Bytes()...)
}

func (b *boundedBuffer) Exceeded() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.exceeded
}

func (b *boundedBuffer) Error(label string) error {
	if !b.Exceeded() {
		return nil
	}
	return fmt.Errorf("%s output exceeded %d byte limit", label, b.limit)
}
