package multibuf

import "errors"

type options struct {
	// MemBufferBytes sets up the size of the memory buffer for this request.
	// If the data size exceeds the limit, the remaining request part will be saved on the file system.
	memBytes int64

	maxBytes int64

	tempDir string
}

type OptionSetter func(o *options) error

// MaxBytes ignored if set to value >=, if request exceeds the specified limit, the reader will return error,
// by default buffer is not limited, negative values mean no limit.
func MaxBytes(m int64) OptionSetter {
	return func(o *options) error {
		o.maxBytes = m
		return nil
	}
}

// MemBytes specifies the largest buffer to hold in RAM before writing to disk, default is 1MB.
func MemBytes(m int64) OptionSetter {
	return func(o *options) error {
		if m < 0 {
			return errors.New("memBytes should be >= 0")
		}

		o.memBytes = m

		return nil
	}
}

// TempDir sets a custom temp directory to store files.
func TempDir(dir string) OptionSetter {
	return func(o *options) error {
		o.tempDir = dir

		return nil
	}
}
