package multibuf

import (
	"bytes"
	"io"
	"os"
)

// MultiReader provides Read, Close, Seek and Size methods. In addition to that it supports WriterTo interface
// to provide efficient writing schemes, as functions like io.Copy use WriterTo when it's available.
type MultiReader interface {
	io.Reader
	io.Seeker
	io.Closer
	io.WriterTo

	// Size calculates and returns the total size of the reader and not the length remaining.
	Size() (int64, error)
}

// New returns MultiReader that can limit the size of the buffer and persist large buffers to disk.
// By default New returns unbound buffer that will read up to 1MB in RAM and will start buffering to disk
// It supports multiple functional optional arguments:
//
//	// Buffer up to 1MB in RAM and limit max buffer size to 20MB
//	multibuf.New(r, multibuf.MemBytes(1024 * 1024), multibuf.MaxBytes(1024 * 1024 * 20))
func New(input io.Reader, setters ...OptionSetter) (MultiReader, error) {
	o := options{
		memBytes: DefaultMemBytes,
		maxBytes: DefaultMaxBytes,
	}

	for _, s := range setters {
		if err := s(&o); err != nil {
			return nil, err
		}
	}

	if o.memBytes == 0 {
		o.memBytes = DefaultMemBytes
	}

	if o.maxBytes > 0 && o.maxBytes < o.memBytes {
		o.memBytes = o.maxBytes
	}

	memReader := &io.LimitedReader{
		R: input,      // Read from this reader
		N: o.memBytes, // Maximum amount of data to read
	}

	readers := make([]io.ReadSeeker, 0, 2)

	buffer, err := io.ReadAll(memReader)
	if err != nil {
		return nil, err
	}

	readers = append(readers, bytes.NewReader(buffer))

	var file *os.File

	// This means that we have exceeded all the memory capacity and we will start buffering the body to disk.
	totalBytes := int64(len(buffer))

	if memReader.N <= 0 {
		file, err = os.CreateTemp(o.tempDir, tempFilePrefix)
		if err != nil {
			return nil, err
		}

		_ = os.Remove(file.Name())

		readSrc := input
		if o.maxBytes > 0 {
			readSrc = &maxReader{R: input, Max: o.maxBytes - o.memBytes}
		}

		writtenBytes, err := io.Copy(file, readSrc)
		if err != nil {
			_ = file.Close()

			return nil, err
		}

		totalBytes += writtenBytes

		_, _ = file.Seek(0, 0)

		readers = append(readers, file)
	}

	var cleanupFn cleanupFunc
	if file != nil {
		cleanupFn = func() error {
			_ = file.Close()
			return nil
		}
	}

	return newBuf(totalBytes, cleanupFn, readers...), nil
}

// MaxReader does not allow to read more than Max bytes and returns error if this limit has been exceeded.
type maxReader struct {
	R   io.Reader // underlying reader
	N   int64     // bytes read
	Max int64     // max bytes to read
}

func (r *maxReader) Read(p []byte) (int, error) {
	readBytes, err := r.R.Read(p)
	if err != nil && err != io.EOF {
		return readBytes, err
	}

	r.N += int64(readBytes)
	if r.N > r.Max {
		return readBytes, &MaxSizeReachedError{MaxSize: r.Max}
	}

	return readBytes, err
}
