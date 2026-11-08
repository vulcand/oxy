package multibuf

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func createReaderOfSize(t *testing.T, size int64) (reader io.Reader, hash string) {
	t.Helper()

	f, err := os.Open("/dev/urandom")
	require.NoError(t, err)

	b := make([]byte, int(size))

	_, err = io.ReadFull(f, b)
	require.NoError(t, err)

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
