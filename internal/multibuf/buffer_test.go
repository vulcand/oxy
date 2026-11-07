package multibuf

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
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
	_, _ = io.Copy(ioutil.Discard, tr)
	return hex.EncodeToString(h.Sum(nil))
}

func TestSmallBuffer(t *testing.T) {
	r, hash := createReaderOfSize(1)
	bb, err := New(r)
	assert.NoError(t, err)
	assert.Equal(t, hash, hashOfReader(bb))
	bb.Close()
}

func TestBigBuffer(t *testing.T) {
	r, hash := createReaderOfSize(13631488)
	bb, err := New(r)
	assert.NoError(t, err)
	assert.Equal(t, hash, hashOfReader(bb))
}

func TestSeek(t *testing.T) {
	tlen := int64(1057576)
	r, hash := createReaderOfSize(tlen)
	bb, err := New(r)

	assert.NoError(t, err)
	assert.Equal(t, hash, hashOfReader(bb))
	l, err := bb.Size()
	assert.NoError(t, err)
	assert.Equal(t, tlen, l)

	bb.Seek(0, 0)
	assert.Equal(t, hash, hashOfReader(bb))
	l, err = bb.Size()
	assert.NoError(t, err)
	assert.Equal(t, tlen, l)
}

func TestSeekWithFile(t *testing.T) {
	tlen := int64(DefaultMemBytes)
	r, hash := createReaderOfSize(tlen)
	bb, err := New(r, MemBytes(1))

	assert.NoError(t, err)
	assert.Equal(t, hash, hashOfReader(bb))
	l, err := bb.Size()
	assert.NoError(t, err)
	assert.Equal(t, tlen, l)

	bb.Seek(0, 0)
	assert.Equal(t, hash, hashOfReader(bb))
	l, err = bb.Size()
	assert.NoError(t, err)
	assert.Equal(t, tlen, l)
}

func TestSeekFirst(t *testing.T) {
	tlen := int64(1057576)
	r, hash := createReaderOfSize(tlen)
	bb, err := New(r)
	assert.NoError(t, err)

	l, err := bb.Size()
	assert.NoError(t, err)
	assert.Equal(t, tlen, l)

	assert.NoError(t, err)
	assert.Equal(t, hash, hashOfReader(bb))

	bb.Seek(0, 0)

	assert.Equal(t, hash, hashOfReader(bb))
	l, err = bb.Size()
	assert.NoError(t, err)
	assert.Equal(t, tlen, l)
}

func TestLimitDoesNotExceed(t *testing.T) {
	requestSize := int64(1057576)
	r, hash := createReaderOfSize(requestSize)
	bb, err := New(r, MemBytes(1024), MaxBytes(requestSize+1))
	assert.NoError(t, err)
	assert.Equal(t, hash, hashOfReader(bb))
	size, err := bb.Size()
	assert.NoError(t, err)
	assert.Equal(t, requestSize, size)
	bb.Close()
}

func TestLimitExceeds(t *testing.T) {
	requestSize := int64(1057576)
	r, _ := createReaderOfSize(requestSize)
	bb, err := New(r, MemBytes(1024), MaxBytes(requestSize-1))
	assert.IsType(t, &MaxSizeReachedError{}, err)
	assert.Nil(t, bb)
}

func TestLimitExceedsMemBytes(t *testing.T) {
	requestSize := int64(1057576)
	r, _ := createReaderOfSize(requestSize)
	bb, err := New(r, MemBytes(requestSize+1), MaxBytes(requestSize-1))
	assert.IsType(t, &MaxSizeReachedError{}, err)
	assert.Nil(t, bb)
}

func TestWriteToBigBuffer(t *testing.T) {
	l := int64(13631488)
	r, hash := createReaderOfSize(l)
	bb, err := New(r)
	assert.NoError(t, err)

	other := &bytes.Buffer{}
	wrote, err := bb.WriteTo(other)
	assert.NoError(t, err)
	assert.Equal(t, l, wrote)
	assert.Equal(t, hash, hashOfReader(other))
}

func TestWriteToSmallBuffer(t *testing.T) {
	l := int64(1)
	r, hash := createReaderOfSize(l)
	bb, err := New(r)
	assert.NoError(t, err)

	other := &bytes.Buffer{}
	wrote, err := bb.WriteTo(other)
	assert.NoError(t, err)
	assert.Equal(t, l, wrote)
	assert.Equal(t, hash, hashOfReader(other))
}

func TestWriterOnceMemBytesDefault(t *testing.T) {
	w, err := NewWriterOnce(MemBytes(0))
	assert.NoError(t, err)

	wo, ok := w.(*writerOnce)
	assert.True(t, ok)

	assert.Equal(t, DefaultMemBytes, int(wo.o.memBytes))
}

func TestWriterOnceSmallBuffer(t *testing.T) {
	r, hash := createReaderOfSize(1)

	w, err := NewWriterOnce()
	assert.NoError(t, err)

	total, err := io.Copy(w, r)
	assert.Equal(t, nil, err)
	assert.Equal(t, int64(1), total)

	bb, err := w.Reader()
	assert.NoError(t, err)

	assert.Equal(t, hash, hashOfReader(bb))
	bb.Close()
}

func TestWriterOnceBigBuffer(t *testing.T) {
	size := int64(13631488)
	r, hash := createReaderOfSize(size)

	w, err := NewWriterOnce()
	assert.NoError(t, err)

	total, err := io.Copy(w, r)
	assert.Equal(t, nil, err)
	assert.Equal(t, size, total)

	bb, err := w.Reader()
	assert.NoError(t, err)

	assert.Equal(t, hash, hashOfReader(bb))
	bb.Close()
}

func TestWriterOncePartialWrites(t *testing.T) {
	size := int64(13631488)
	r, hash := createReaderOfSize(size)

	w, err := NewWriterOnce()
	assert.NoError(t, err)

	total, err := io.CopyN(w, r, DefaultMemBytes+1)
	assert.Equal(t, nil, err)
	assert.Equal(t, int64(DefaultMemBytes+1), total)

	remained := size - DefaultMemBytes - 1
	total, err = io.CopyN(w, r, remained)
	assert.Equal(t, nil, err)
	assert.Equal(t, remained, total)

	bb, err := w.Reader()
	assert.NoError(t, err)
	assert.Nil(t, w.(*writerOnce).mem)
	assert.Nil(t, w.(*writerOnce).file)

	assert.Equal(t, hash, hashOfReader(bb))
	bb.Close()
}

func TestWriterOnceMaxSizeExceeded(t *testing.T) {
	size := int64(1000)
	r, _ := createReaderOfSize(size)

	w, err := NewWriterOnce(MemBytes(10), MaxBytes(100))
	assert.NoError(t, err)

	_, err = io.Copy(w, r)
	assert.Error(t, err)
	assert.NoError(t, w.Close())
}

func TestWriterReaderCalled(t *testing.T) {
	size := int64(1000)
	r, hash := createReaderOfSize(size)

	w, err := NewWriterOnce()
	assert.NoError(t, err)

	_, err = io.Copy(w, r)
	assert.NoError(t, err)
	assert.NoError(t, w.Close())

	bb, err := w.Reader()
	assert.NoError(t, err)

	assert.Equal(t, hash, hashOfReader(bb))

	// Subsequent calls to write and get reader will fail
	_, err = w.Reader()
	assert.EqualError(t, err, ErrReaderHasBeenCalled.Error())

	_, err = w.Write([]byte{1})
	assert.EqualError(t, err, ErrReaderHasBeenCalled.Error())
}

func TestWriterNoData(t *testing.T) {
	w, err := NewWriterOnce()
	assert.NoError(t, err)

	_, err = w.Reader()
	assert.EqualError(t, err, ErrNoDataReady.Error())
}
