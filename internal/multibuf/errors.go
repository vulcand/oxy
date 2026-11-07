package multibuf

import (
	"errors"
	"fmt"
)

var (
	// ErrNoDataReady is an error when getting the reader while the data is not
	// ready.
	ErrNoDataReady = errors.New("mailgun/multibuf: no data ready")
	// ErrReaderHasBeenCalled is an error when getting the reader or writing
	// while the reader has been called.
	ErrReaderHasBeenCalled = errors.New("mailgun/multibuf: reader has been called")
)

// MaxSizeReachedError is returned when the maximum allowed buffer size is reached when reading.
type MaxSizeReachedError struct {
	MaxSize int64
}

func (e *MaxSizeReachedError) Error() string {
	return fmt.Sprintf("Maximum size %d was reached", e.MaxSize)
}
