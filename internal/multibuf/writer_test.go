package multibuf

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriterOnce_memBytesDefault(t *testing.T) {
	w, err := NewWriterOnce(MemBytes(0))
	require.NoError(t, err)

	wo, ok := w.(*writerOnce)
	assert.True(t, ok)

	assert.Equal(t, DefaultMemBytes, int(wo.o.memBytes))
}

func TestWriterOnce_size(t *testing.T) {
	testCases := []struct {
		desc string
		size int64
	}{
		{
			desc: "small buffer",
			size: 1,
		},
		{
			desc: "big buffer",
			size: 13631488,
		},
	}

	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			r, hash := createReaderOfSize(t, test.size)

			w, err := NewWriterOnce()
			require.NoError(t, err)

			t.Cleanup(func() { _ = w.Close() })

			total, err := io.Copy(w, r)
			require.NoError(t, err)
			assert.Equal(t, test.size, total)

			bb, err := w.Reader()
			require.NoError(t, err)

			t.Cleanup(func() { _ = bb.Close() })

			assert.Equal(t, hash, hashOfReader(bb))
		})
	}
}

func TestWriterOnce_partialWrites(t *testing.T) {
	size := int64(13631488)

	r, hash := createReaderOfSize(t, size)

	w, err := NewWriterOnce()
	require.NoError(t, err)

	t.Cleanup(func() { _ = w.Close() })

	total, err := io.CopyN(w, r, DefaultMemBytes+1)
	require.NoError(t, err)
	assert.Equal(t, int64(DefaultMemBytes+1), total)

	remained := size - DefaultMemBytes - 1
	total, err = io.CopyN(w, r, remained)
	require.NoError(t, err)
	assert.Equal(t, remained, total)

	bb, err := w.Reader()
	require.NoError(t, err)

	t.Cleanup(func() { _ = bb.Close() })

	assert.Nil(t, w.(*writerOnce).mem)
	assert.Nil(t, w.(*writerOnce).file)

	assert.Equal(t, hash, hashOfReader(bb))
}

func TestWriterOnce_maxSizeExceeded(t *testing.T) {
	size := int64(1000)

	r, _ := createReaderOfSize(t, size)

	w, err := NewWriterOnce(MemBytes(10), MaxBytes(100))
	require.NoError(t, err)

	_, err = io.Copy(w, r)
	require.EqualError(t, err, "total size of 1000 exceeded allowed 100")
	require.NoError(t, w.Close())
}

func TestWriterOnce_writerReaderCalled(t *testing.T) {
	size := int64(1000)

	r, hash := createReaderOfSize(t, size)

	w, err := NewWriterOnce()
	require.NoError(t, err)

	_, err = io.Copy(w, r)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	bb, err := w.Reader()
	require.NoError(t, err)

	t.Cleanup(func() { _ = bb.Close() })

	assert.Equal(t, hash, hashOfReader(bb))

	// Subsequent calls to write and get reader will fail
	_, err = w.Reader()
	require.ErrorIs(t, err, ErrReaderHasBeenCalled)

	_, err = w.Write([]byte{1})
	require.ErrorIs(t, err, ErrReaderHasBeenCalled)
}

func TestWriterOnce_writerNoData(t *testing.T) {
	w, err := NewWriterOnce()
	require.NoError(t, err)

	t.Cleanup(func() { _ = w.Close() })

	_, err = w.Reader()
	require.ErrorIs(t, err, ErrNoDataReady)
}

func TestWriterOnce_Close_cleanup(t *testing.T) {
	testCases := []struct {
		desc      string
		operation func(t *testing.T, w WriterOnce)
	}{
		{
			desc: "close",
			operation: func(t *testing.T, w WriterOnce) {
				t.Helper()

				assert.NoError(t, w.Close())
			},
		},
		{
			desc: "limit",
			operation: func(t *testing.T, w WriterOnce) {
				t.Helper()

				_, err := w.Write(bytes.Repeat([]byte("x"), 4096))
				require.Error(t, err)

				require.NoError(t, w.Close())
			},
		},
		{
			desc: "panic",
			operation: func(t *testing.T, w WriterOnce) {
				t.Helper()

				assert.Panics(t, func() {
					defer w.Close()

					panic("oops")
				})
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			w, err := NewWriterOnce(MemBytes(1024), MaxBytes(4096), TempDir(t.TempDir()))
			require.NoError(t, err)

			_, err = w.Write(bytes.Repeat([]byte("x"), 2048))
			require.NoError(t, err)

			file := w.(*writerOnce).file
			require.NotNil(t, file)
			require.FileExists(t, file.Name())

			test.operation(t, w)

			assert.NoFileExists(t, file.Name())
		})
	}
}
