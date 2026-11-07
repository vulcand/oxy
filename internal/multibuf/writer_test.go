package multibuf

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriterOnceMemBytesDefault(t *testing.T) {
	w, err := NewWriterOnce(MemBytes(0))
	require.NoError(t, err)

	wo, ok := w.(*writerOnce)
	assert.True(t, ok)

	assert.Equal(t, DefaultMemBytes, int(wo.o.memBytes))
}

func TestWriterOnceSmallBuffer(t *testing.T) {
	r, hash := createReaderOfSize(t, 1)

	w, err := NewWriterOnce()
	require.NoError(t, err)

	total, err := io.Copy(w, r)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)

	bb, err := w.Reader()
	require.NoError(t, err)

	assert.Equal(t, hash, hashOfReader(bb))
	bb.Close()
}

func TestWriterOnceBigBuffer(t *testing.T) {
	size := int64(13631488)
	r, hash := createReaderOfSize(t, size)

	w, err := NewWriterOnce()
	require.NoError(t, err)

	total, err := io.Copy(w, r)
	require.NoError(t, err)
	assert.Equal(t, size, total)

	bb, err := w.Reader()
	require.NoError(t, err)

	assert.Equal(t, hash, hashOfReader(bb))
	bb.Close()
}

func TestWriterOncePartialWrites(t *testing.T) {
	size := int64(13631488)
	r, hash := createReaderOfSize(t, size)

	w, err := NewWriterOnce()
	require.NoError(t, err)

	total, err := io.CopyN(w, r, DefaultMemBytes+1)
	require.NoError(t, err)
	assert.Equal(t, int64(DefaultMemBytes+1), total)

	remained := size - DefaultMemBytes - 1
	total, err = io.CopyN(w, r, remained)
	require.NoError(t, err)
	assert.Equal(t, remained, total)

	bb, err := w.Reader()
	require.NoError(t, err)
	assert.Nil(t, w.(*writerOnce).mem)
	assert.Nil(t, w.(*writerOnce).file)

	assert.Equal(t, hash, hashOfReader(bb))
	bb.Close()
}

func TestWriterOnceMaxSizeExceeded(t *testing.T) {
	size := int64(1000)
	r, _ := createReaderOfSize(t, size)

	w, err := NewWriterOnce(MemBytes(10), MaxBytes(100))
	require.NoError(t, err)

	_, err = io.Copy(w, r)
	require.Error(t, err)
	require.NoError(t, w.Close())
}

func TestWriterReaderCalled(t *testing.T) {
	size := int64(1000)
	r, hash := createReaderOfSize(t, size)

	w, err := NewWriterOnce()
	require.NoError(t, err)

	_, err = io.Copy(w, r)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	bb, err := w.Reader()
	require.NoError(t, err)

	assert.Equal(t, hash, hashOfReader(bb))

	// Subsequent calls to write and get reader will fail
	_, err = w.Reader()
	require.EqualError(t, err, ErrReaderHasBeenCalled.Error())

	_, err = w.Write([]byte{1})
	require.EqualError(t, err, ErrReaderHasBeenCalled.Error())
}

func TestWriterNoData(t *testing.T) {
	w, err := NewWriterOnce()
	require.NoError(t, err)

	_, err = w.Reader()
	require.EqualError(t, err, ErrNoDataReady.Error())
}
