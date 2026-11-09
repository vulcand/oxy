package multibuf

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSmallBuffer(t *testing.T) {
	r, hash := createReaderOfSize(t, 1)

	bb, err := New(r)
	require.NoError(t, err)
	assert.Equal(t, hash, hashOfReader(bb))

	bb.Close()
}

func TestBigBuffer(t *testing.T) {
	r, hash := createReaderOfSize(t, 13631488)

	bb, err := New(r)
	require.NoError(t, err)
	assert.Equal(t, hash, hashOfReader(bb))
}

func TestSeek(t *testing.T) {
	tlen := int64(1057576)

	r, hash := createReaderOfSize(t, tlen)

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

	r, hash := createReaderOfSize(t, tlen)

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

	r, hash := createReaderOfSize(t, tlen)

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

	r, hash := createReaderOfSize(t, requestSize)

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

	r, _ := createReaderOfSize(t, requestSize)

	bb, err := New(r, MemBytes(1024), MaxBytes(requestSize-1))

	var mErr *MaxSizeReachedError
	require.ErrorAs(t, err, &mErr)

	assert.Nil(t, bb)
}

func TestLimitExceedsMemBytes(t *testing.T) {
	requestSize := int64(1057576)

	r, _ := createReaderOfSize(t, requestSize)

	bb, err := New(r, MemBytes(requestSize+1), MaxBytes(requestSize-1))

	var mErr *MaxSizeReachedError
	require.ErrorAs(t, err, &mErr)

	assert.Nil(t, bb)
}

func TestWriteToBigBuffer(t *testing.T) {
	l := int64(13631488)

	r, hash := createReaderOfSize(t, l)

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

	r, hash := createReaderOfSize(t, l)

	bb, err := New(r)
	require.NoError(t, err)

	other := &bytes.Buffer{}

	wrote, err := bb.WriteTo(other)
	require.NoError(t, err)

	assert.Equal(t, l, wrote)
	assert.Equal(t, hash, hashOfReader(other))
}
