package multibuf

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createReaderOfSize(size int64) (reader io.Reader, hash string) {
	f, err := os.Open("/dev/urandom")
	if err != nil {
		panic(err)
	}

	b := make([]byte, int(size))

	_, err = io.ReadFull(f, b)
	if err != nil {
		panic(err)
	}

	h := md5.New()
	h.Write(b)

	return bytes.NewReader(b), hex.EncodeToString(h.Sum(nil))
}

func hashOfReader(r io.Reader) string {
	h := md5.New()

	tr := io.TeeReader(r, h)

	_, _ = io.Copy(io.Discard, tr)

	return hex.EncodeToString(h.Sum(nil))
}

func TestSmallBuffer(t *testing.T) {
	r, hash := createReaderOfSize(1)

	bb, err := New(r)
	require.NoError(t, err)
	assert.Equal(t, hash, hashOfReader(bb))

	bb.Close()
}

func TestBigBuffer(t *testing.T) {
	r, hash := createReaderOfSize(13631488)

	bb, err := New(r)
	require.NoError(t, err)
	assert.Equal(t, hash, hashOfReader(bb))
}

func TestSeek(t *testing.T) {
	tlen := int64(1057576)

	r, hash := createReaderOfSize(tlen)

	bb, err := New(r)
	require.NoError(t, err)
	assert.Equal(t, hash, hashOfReader(bb))

	l, err := bb.Size()
	require.NoError(t, err)
	assert.Equal(t, tlen, l)

	_, _ = bb.Seek(0, 0)
	assert.Equal(t, hash, hashOfReader(bb))

	l, err = bb.Size()
	require.NoError(t, err)
	assert.Equal(t, tlen, l)
}

func TestSeekWithFile(t *testing.T) {
	tlen := int64(DefaultMemBytes)

	r, hash := createReaderOfSize(tlen)

	bb, err := New(r, MemBytes(1))

	require.NoError(t, err)
	assert.Equal(t, hash, hashOfReader(bb))

	l, err := bb.Size()
	require.NoError(t, err)
	assert.Equal(t, tlen, l)

	_, _ = bb.Seek(0, 0)
	assert.Equal(t, hash, hashOfReader(bb))

	l, err = bb.Size()
	require.NoError(t, err)
	assert.Equal(t, tlen, l)
}

func TestSeekFirst(t *testing.T) {
	tlen := int64(1057576)
	r, hash := createReaderOfSize(tlen)
	bb, err := New(r)
	require.NoError(t, err)

	l, err := bb.Size()
	require.NoError(t, err)
	assert.Equal(t, tlen, l)

	assert.Equal(t, hash, hashOfReader(bb))

	_, _ = bb.Seek(0, 0)

	assert.Equal(t, hash, hashOfReader(bb))
	l, err = bb.Size()
	require.NoError(t, err)
	assert.Equal(t, tlen, l)
}

func TestLimitDoesNotExceed(t *testing.T) {
	requestSize := int64(1057576)

	r, hash := createReaderOfSize(requestSize)

	bb, err := New(r, MemBytes(1024), MaxBytes(requestSize+1))
	require.NoError(t, err)
	assert.Equal(t, hash, hashOfReader(bb))

	size, err := bb.Size()
	require.NoError(t, err)

	assert.Equal(t, requestSize, size)
	bb.Close()
}

func TestLimitExceeds(t *testing.T) {
	requestSize := int64(1057576)

	r, _ := createReaderOfSize(requestSize)

	bb, err := New(r, MemBytes(1024), MaxBytes(requestSize-1))

	var mErr *MaxSizeReachedError
	require.ErrorAs(t, err, &mErr)

	assert.Nil(t, bb)
}

func TestLimitExceedsMemBytes(t *testing.T) {
	requestSize := int64(1057576)

	r, _ := createReaderOfSize(requestSize)

	bb, err := New(r, MemBytes(requestSize+1), MaxBytes(requestSize-1))

	var mErr *MaxSizeReachedError
	require.ErrorAs(t, err, &mErr)

	assert.Nil(t, bb)
}

func TestWriteToBigBuffer(t *testing.T) {
	l := int64(13631488)

	r, hash := createReaderOfSize(l)

	bb, err := New(r)
	require.NoError(t, err)

	other := &bytes.Buffer{}

	wrote, err := bb.WriteTo(other)
	require.NoError(t, err)

	assert.Equal(t, l, wrote)
	assert.Equal(t, hash, hashOfReader(other))
}

func TestWriteToSmallBuffer(t *testing.T) {
	l := int64(1)

	r, hash := createReaderOfSize(l)

	bb, err := New(r)
	require.NoError(t, err)

	other := &bytes.Buffer{}

	wrote, err := bb.WriteTo(other)
	require.NoError(t, err)

	assert.Equal(t, l, wrote)
	assert.Equal(t, hash, hashOfReader(other))
}

func TestWriterOnceMemBytesDefault(t *testing.T) {
	w, err := NewWriterOnce(MemBytes(0))
	require.NoError(t, err)

	wo, ok := w.(*writerOnce)
	assert.True(t, ok)

	assert.Equal(t, DefaultMemBytes, int(wo.o.memBytes))
}

func TestWriterOnceSmallBuffer(t *testing.T) {
	r, hash := createReaderOfSize(1)

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
	r, hash := createReaderOfSize(size)

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
	r, hash := createReaderOfSize(size)

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
	r, _ := createReaderOfSize(size)

	w, err := NewWriterOnce(MemBytes(10), MaxBytes(100))
	require.NoError(t, err)

	_, err = io.Copy(w, r)
	require.Error(t, err)
	require.NoError(t, w.Close())
}

func TestWriterReaderCalled(t *testing.T) {
	size := int64(1000)
	r, hash := createReaderOfSize(size)

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
