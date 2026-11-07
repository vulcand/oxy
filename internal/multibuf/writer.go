package multibuf

import (
	"bytes"
	"fmt"
	"os"
)

const (
	writerInit = iota
	writerMem
	writerFile
	writerCalledRead
	writerErr
)

// WriterOnce implements write once, read many times writer. Create a WriterOnce and write to it, once Reader() function has been
// called, the internal data is transferred to MultiReader and this instance of WriterOnce should be no longer used.
type WriterOnce interface {
	// Write implements io.Writer
	Write(p []byte) (int, error)
	// Reader transfers all data written to this writer to MultiReader. If there was no data written it returns an error
	Reader() (MultiReader, error)
	// WriterOnce owns the data before Reader has been called, so Close will close all the underlying files if Reader has not been called.
	Close() error
}

// NewWriterOnce returns io.ReadWrite compatible object that can limit the size of the buffer and persist large buffers to disk.
// WriterOnce implements write once, read many times writer. Create a WriterOnce and write to it, once Reader() function has been
// called, the internal data is transferred to MultiReader and this instance of WriterOnce should be no longer used.
// By default NewWriterOnce returns unbound buffer that will allow to write up to 1MB in RAM and will start buffering to disk
// It supports multiple functional optional arguments:
//
//	// Buffer up to 1MB in RAM and limit max buffer size to 20MB
//	multibuf.NewWriterOnce(r, multibuf.MemBytes(1024 * 1024), multibuf.MaxBytes(1024 * 1024 * 20))
func NewWriterOnce(setters ...OptionSetter) (WriterOnce, error) {
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

	return &writerOnce{o: o}, nil
}

type writerOnce struct {
	o         options
	state     int
	mem       *bytes.Buffer
	file      *os.File
	total     int64
	cleanupFn cleanupFunc
}

func (w *writerOnce) Reader() (MultiReader, error) {
	switch w.state {
	case writerInit:
		return nil, ErrNoDataReady

	case writerCalledRead:
		return nil, ErrReaderHasBeenCalled

	case writerMem:
		w.state = writerCalledRead
		return newBuf(w.total, nil, bytes.NewReader(w.mem.Bytes())), nil

	case writerFile:
		_, err := w.file.Seek(0, 0)
		if err != nil {
			return nil, err
		}

		// we are not responsible for file and buffer any more
		w.state = writerCalledRead

		br, fr := bytes.NewReader(w.mem.Bytes()), w.file

		w.file = nil
		w.mem = nil

		return newBuf(w.total, w.cleanupFn, br, fr), nil
	}

	return nil, fmt.Errorf("unsupported state: %d", w.state)
}

func (w *writerOnce) Write(p []byte) (int, error) {
	return w.write(p)
}

func (w *writerOnce) Close() error {
	if w.file != nil {
		return w.file.Close()
	}

	return nil
}

func (w *writerOnce) initFile() error {
	file, err := os.CreateTemp("", tempFilePrefix)
	if err != nil {
		return err
	}

	w.file = file

	w.cleanupFn = func() error {
		_ = file.Close()
		_ = os.Remove(file.Name())

		return nil
	}

	return nil
}

// how many bytes we can still write to memory.
func (w *writerOnce) writeToMem(p []byte) int {
	left := w.o.memBytes - w.total
	if left <= 0 {
		return 0
	}

	bufLen := len(p)
	if int64(bufLen) < left {
		return bufLen
	}

	return int(left)
}

func (w *writerOnce) write(p []byte) (int, error) {
	if w.o.maxBytes > 0 && int64(len(p))+w.total > w.o.maxBytes {
		return 0, fmt.Errorf("total size of %d exceeded allowed %d", int64(len(p))+w.total, w.o.maxBytes)
	}

	switch w.state {
	case writerCalledRead:
		return 0, ErrReaderHasBeenCalled

	case writerInit:
		w.mem = &bytes.Buffer{}
		w.state = writerMem

		fallthrough

	case writerMem:
		writeToMem := w.writeToMem(p)
		if writeToMem > 0 {
			wrote, err := w.mem.Write(p[:writeToMem])

			w.total += int64(wrote)

			if err != nil {
				return wrote, err
			}
		}

		left := len(p) - writeToMem
		if left <= 0 {
			return len(p), nil
		}

		// we can't write to memory anymore, switch to file
		if err := w.initFile(); err != nil {
			return writeToMem, err
		}

		w.state = writerFile

		wrote, err := w.file.Write(p[writeToMem:])

		w.total += int64(wrote)

		return len(p), err

	case writerFile:
		wrote, err := w.file.Write(p)
		w.total += int64(wrote)

		return wrote, err
	}

	return 0, fmt.Errorf("unsupported state: %d", w.state)
}
