package multipipe

import (
	"io"
	"sync"
)

// TODO: reduce lock contention if it becomes a bottleneck

// MultiPipe is an io.Writer that can create multiple readers from its contents
type MultiPipe struct {
	buf    []byte
	rdErr  error
	closed bool
	cond   *sync.Cond
}

// Reader is an io.Reader that reads from the beginning of its parent MultiPipe
// without affecting the other readers
type Reader struct {
	parent *MultiPipe
	offset int
}

// Read reads all the available contents from the MultiPipe parent, then if
// there is a read error or the stream is closed, an error will be returned
func (m *Reader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}

	m.parent.cond.L.Lock()
	defer m.parent.cond.L.Unlock()

	// Wait for IO if at the end of the buffer and the input is still open
	if m.offset >= len(m.parent.buf) && !m.parent.closed {
		m.parent.cond.Wait()
	}

	n = copy(p, m.parent.buf[m.offset:])
	m.offset += n

	if m.offset >= len(m.parent.buf) {
		if m.parent.rdErr != nil {
			err = m.parent.rdErr
		} else if m.parent.closed {
			err = io.EOF
		}
	}

	return
}

// NewMultiPipe creates and initializes a new MultiPipe
func NewMultiPipe() *MultiPipe {
	return &MultiPipe{cond: sync.NewCond(&sync.Mutex{})}
}

// NewReader creates a new Reader that will get its contents from the parent
// MultiPipe
func (m *MultiPipe) NewReader() *Reader {
	return &Reader{parent: m}
}

// Write writes the given byte slice to the MultiPipe, writing to a closed
// MultiPipe will result in an error
func (m *MultiPipe) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	m.cond.L.Lock()
	defer m.cond.L.Unlock()

	if m.closed {
		return 0, io.ErrClosedPipe
	}

	m.buf = append(m.buf, p...)
	m.cond.Broadcast()

	return len(p), nil
}

// Close closes the MultiPipe without errors
func (m *MultiPipe) Close() error {
	return m.CloseWithError(nil)
}

// CloseWithError closes the MultiPipe and sets the given error, that will be
// return on subsequent reads by the child readers after their contents are
// drained
func (m *MultiPipe) CloseWithError(err error) error {
	m.cond.L.Lock()
	defer m.cond.L.Unlock()

	if m.closed {
		return io.ErrClosedPipe
	} else if err != nil {
		m.rdErr = err
	}

	m.closed = true
	m.cond.Broadcast()

	return nil
}
