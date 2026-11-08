// Package multibuf implements buffer optimized for streaming large chunks of data,
// multiple reads and  optional partial buffering to disk.
package multibuf

import (
	"errors"
	"io"
)

const tempFilePrefix = "temp-multibuf-"

// Default sizes.
const (
	DefaultMemBytes = 1048576
	DefaultMaxBytes = -1
	// DefaultBufferBytes equivalent of bytes.MinRead used in io.ReadAll.
	DefaultBufferBytes = 512
)

type cleanupFunc func() error

func newBuf(length int64, cleanup cleanupFunc, readers ...io.ReadSeeker) *multiReaderSeek {
	converted := make([]io.Reader, len(readers))
	for i, r := range readers {
		// This conversion is safe as ReadSeeker includes Reader
		converted[i] = r.(io.Reader)
	}

	return &multiReaderSeek{
		length:  length,
		readers: readers,
		mr:      io.MultiReader(converted...),
		cleanup: cleanup,
	}
}

// Constraints:
// - Implements io.Reader
// - Implements Seek(0, 0)
// - Designed for Write once, Read many times.
type multiReaderSeek struct {
	length  int64
	readers []io.ReadSeeker
	mr      io.Reader
	cleanup cleanupFunc
}

func (mr *multiReaderSeek) Close() (err error) {
	if mr.cleanup != nil {
		return mr.cleanup()
	}

	return nil
}

func (mr *multiReaderSeek) WriteTo(w io.Writer) (int64, error) {
	b := make([]byte, DefaultBufferBytes)

	var total int64

	for {
		n, err := mr.mr.Read(b)

		// Recommended way is to always handle non 0 reads despite the errors
		if n > 0 {
			nw, errw := w.Write(b[:n])
			total += int64(nw)
			// Write must return a non-nil error if it returns nw < n
			if nw != n || errw != nil {
				return total, errw
			}
		}

		if err != nil {
			if err == io.EOF {
				return total, nil
			}

			return total, err
		}
	}
}

func (mr *multiReaderSeek) Read(p []byte) (n int, err error) {
	return mr.mr.Read(p)
}

func (mr *multiReaderSeek) Size() (int64, error) {
	return mr.length, nil
}

func (mr *multiReaderSeek) Seek(offset int64, whence int) (int64, error) {
	// TODO: implement other whence
	// TODO: implement real offsets
	if whence != 0 {
		return 0, errors.New("multiReaderSeek: unsupported whence")
	}

	if offset != 0 {
		return 0, errors.New("multiReaderSeek: unsupported offset")
	}

	for _, seeker := range mr.readers {
		_, _ = seeker.Seek(0, 0)
	}

	ior := make([]io.Reader, len(mr.readers))
	for i, arg := range mr.readers {
		ior[i] = arg.(io.Reader)
	}

	mr.mr = io.MultiReader(ior...)

	return 0, nil
}
