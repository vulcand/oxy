package multibuf

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiReader_Read(t *testing.T) {
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

			bb, err := New(r)
			require.NoError(t, err)

			t.Cleanup(func() { _ = bb.Close() })

			assert.Equal(t, hash, hashOfReader(bb))
		})
	}
}

func TestMultiReader_Seek(t *testing.T) {
	testCases := []struct {
		desc    string
		size    int64
		options []OptionSetter
	}{
		{
			desc: "memory",
			size: 1057576,
		},
		{
			desc: "with file",
			size: DefaultMemBytes,
			options: []OptionSetter{
				MemBytes(1),
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			r, hash := createReaderOfSize(t, test.size)

			bb, err := New(r, test.options...)
			require.NoError(t, err)

			t.Cleanup(func() { _ = bb.Close() })

			assert.Equal(t, hash, hashOfReader(bb))

			l, err := bb.Size()
			require.NoError(t, err)
			assert.Equal(t, test.size, l)

			_, _ = bb.Seek(0, 0)
			assert.Equal(t, hash, hashOfReader(bb))

			l, err = bb.Size()
			require.NoError(t, err)
			assert.Equal(t, test.size, l)
		})
	}
}

func TestMultiReader_Seek_first(t *testing.T) {
	tlen := int64(1057576)

	r, hash := createReaderOfSize(t, tlen)

	bb, err := New(r)
	require.NoError(t, err)

	t.Cleanup(func() { _ = bb.Close() })

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

func TestMultiReader_limitDoesNotExceed(t *testing.T) {
	requestSize := int64(1057576)

	r, hash := createReaderOfSize(t, requestSize)

	bb, err := New(r, MemBytes(1024), MaxBytes(requestSize+1))
	require.NoError(t, err)

	t.Cleanup(func() { _ = bb.Close() })

	assert.Equal(t, hash, hashOfReader(bb))

	size, err := bb.Size()
	require.NoError(t, err)
	assert.Equal(t, requestSize, size)
}

func TestMultiReader_limit_errors(t *testing.T) {
	const defaultSize = 1057576

	testCases := []struct {
		desc        string
		requestSize int64
		memBytes    int64
		maxBytes    int64
	}{
		{
			desc:        "exceeds",
			requestSize: defaultSize,
			memBytes:    1024,
			maxBytes:    defaultSize - 1,
		},
		{
			desc:        "exceeds MemBytes",
			requestSize: defaultSize,
			memBytes:    defaultSize + 1,
			maxBytes:    defaultSize - 1,
		},
	}

	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			r, _ := createReaderOfSize(t, test.requestSize)

			bb, err := New(r, MemBytes(test.memBytes), MaxBytes(test.maxBytes))

			var mErr *MaxSizeReachedError
			require.ErrorAs(t, err, &mErr)

			assert.Nil(t, bb)
		})
	}
}

func TestMultiReader_WriteTo(t *testing.T) {
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

			bb, err := New(r)
			require.NoError(t, err)

			t.Cleanup(func() { _ = bb.Close() })

			other := &bytes.Buffer{}

			wrote, err := bb.WriteTo(other)
			require.NoError(t, err)

			assert.Equal(t, test.size, wrote)
			assert.Equal(t, hash, hashOfReader(other))
		})
	}
}
