package roundrobin

import (
	. "github.com/smartystreets/goconvey/convey"

	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"testing"
	"time"
)

var ErrorOut = log.New(ioutil.Discard, "[ERROR] ", 0)

// AesObfuscator is a roundrobin.Obfuscator that returns an nonceless encrypted version
type AesObfuscator struct {
	block cipher.AEAD
	ttl   time.Duration
}

// NewAesObfuscator takes a fixed-size key and returns an Obfuscator or an error.
// Key size must be exactly one of 16, 24, or 32 bytes to select AES-128, AES-192, or AES-256.
func NewAesObfuscator(key []byte) (Obfuscator, error) {
	var a AesObfuscator

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	a.block = aesgcm

	return &a, nil
}

// NewAesObfuscatorWithExpiration takes a fixed-size key and a TTL, and returns an Obfuscator or an error.
// Key size must be exactly one of 16, 24, or 32 bytes to select AES-128, AES-192, or AES-256.
func NewAesObfuscatorWithExpiration(key []byte, ttl time.Duration) (Obfuscator, error) {
	var a AesObfuscator

	a.ttl = ttl
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	a.block = aesgcm

	return &a, nil
}

// Obfuscate takes a raw string and returns the obfuscated value
func (o *AesObfuscator) Obfuscate(raw string) string {
	if o.ttl > 0 {
		raw = fmt.Sprintf("%s|%d", raw, time.Now().UTC().Add(o.ttl).Unix())
	}

	/*
		Nonce is the 64bit nanosecond-resolution time, plus 32bits of crypto/rand, for 96bits (12Bytes).
		Theoretically, if 2^32 calls were made in 1 nanosecon, there might be a repeat.
		Adds ~765ns, and 4B heap in 1 alloc (Benchmark_NonceTimeRandom4 below)

		Benchmark_NonceRandom12-8      	 2000000	       723 ns/op	      16 B/op	       1 allocs/op
		Benchmark_NonceRandom4-8       	 2000000	       698 ns/op	       4 B/op	       1 allocs/op
		Benchmark_NonceTimeRandom4-8   	 2000000	       765 ns/op	       4 B/op	       1 allocs/op
	*/
	nonce := make([]byte, 12)
	binary.PutVarint(nonce, time.Now().UnixNano())
	rpend := make([]byte, 4)
	if _, err := io.ReadFull(rand.Reader, rpend); err != nil {
		// This is a near-impossible error condition on Linux systems.
		// An error here means rand.Reader (and thus getrandom(2), and thus /dev/urandom) returned
		// less than 4 bytes of data. /dev/urandom is guaranteed to always return the number of
		// bytes requested up to 512 bytes on modern kernels. Behaviour on non-Linux systems
		// varies, of course.
		panic(err)
	}
	for i := 0; i < 4; i++ {
		nonce[i+8] = rpend[i]
	}

	obfuscated := o.block.Seal(nil, nonce, []byte(raw), nil)
	// We append the 12byte nonce onto the end of the message
	obfuscated = append(obfuscated, nonce...)
	obfuscatedStr := base64.RawURLEncoding.EncodeToString(obfuscated)
	return obfuscatedStr
}

// Normalize takes an obfuscated string and returns the raw value
func (o *AesObfuscator) Normalize(obfuscatedStr string) string {
	obfuscated, err := base64.RawURLEncoding.DecodeString(obfuscatedStr)
	if err != nil {
		ErrorOut.Printf("AesObfuscator.Normalize Decoding base64 failed with '%s'\n", err)
		return ""
	}

	// The first len-12 bytes is the ciphertext, the last 12 bytes is the nonce
	n := len(obfuscated) - 12
	if n <= 0 {
		// Protect against range errors causing panics
		ErrorOut.Printf("AesObfuscator.Normalize post-base64-decoded string is too short\n")
		return ""
	}

	nonce := obfuscated[n:]
	obfuscated = obfuscated[:n]

	raw, err := o.block.Open(nil, nonce, []byte(obfuscated), nil)
	if err != nil {
		// um....
		ErrorOut.Printf("AesObfuscator.Normalize Open failed with '%s'\n", err)
		return "" // (badpokerface)
	}
	if o.ttl > 0 {
		rawparts := strings.Split(string(raw), "|")
		if len(rawparts) < 2 {
			ErrorOut.Printf("AesObfuscator.Normalize TTL set but cookie doesn't contain an expiration: '%s'\n", raw)
			return "" // (sadpanda)
		}
		// validate the ttl
		i, err := strconv.ParseInt(rawparts[1], 10, 64)
		if err != nil {
			ErrorOut.Printf("AesObfuscator.Normalize TTL can't be parsed: '%s'\n", raw)
			return "" // (sadpanda)
		}
		if time.Now().UTC().After(time.Unix(i, 0).UTC()) {
			strTime := time.Unix(i, 0).UTC().String()
			ErrorOut.Printf("AesObfuscator.Normalize TTL expired: '%s' (%s)\n", raw, strTime)
			return "" // (curiousgeorge)
		}
		raw = []byte(rawparts[0])
	}
	return string(raw)
}

func TestAesObfuscator128(t *testing.T) {

	Convey("When an AesObfuscator is created with a known 16byte key", t, func() {

		message := "This is a test"
		o, err := NewAesObfuscator([]byte("95Bx9JkKX3xbd7z3"))
		So(err, ShouldBeNil)
		So(o, ShouldNotBeNil)

		Convey("and a message is obfuscated with it", func() {
			obfuscated := o.Obfuscate(message)
			So(obfuscated, ShouldNotBeEmpty)
			So(obfuscated, ShouldNotEqual, message)

			Convey("the message is recoverable", func() {

				clear := o.Normalize(obfuscated)
				So(clear, ShouldEqual, message)
			})

		})

	})
}

func TestAesObfuscatorBadKey(t *testing.T) {

	Convey("When an AesObfuscator is created with a bad 15byte key, if fails as expected", t, func() {

		o, err := NewAesObfuscator([]byte("95Bx9JkKX3xbd7z"))
		So(err.Error(), ShouldEqual, "crypto/aes: invalid key size 15")
		So(o, ShouldBeNil)

	})
}

func TestAesObfuscator128Ttl(t *testing.T) {

	Convey("When an AesObfuscator is created with a known 16byte key, and a future TTL", t, func() {

		message := "This is a test"
		o, err := NewAesObfuscatorWithExpiration([]byte("95Bx9JkKX3xbd7z3"), 5*time.Second)
		So(err, ShouldBeNil)
		So(o, ShouldNotBeNil)

		Convey("and a message is obfuscated with it", func() {
			obfuscated := o.Obfuscate(message)
			So(obfuscated, ShouldNotBeEmpty)
			So(obfuscated, ShouldNotEqual, message)

			Convey("the message is recoverable", func() {

				clear := o.Normalize(obfuscated)
				So(clear, ShouldEqual, message)
			})

		})

	})
}

func TestAesObfuscator128TtlFail(t *testing.T) {

	Convey("When an AesObfuscator is created with a known 16byte key, and a future TTL", t, func() {

		message := "This is a test"
		o, err := NewAesObfuscatorWithExpiration([]byte("95Bx9JkKX3xbd7z3"), 1*time.Second)
		So(err, ShouldBeNil)
		So(o, ShouldNotBeNil)

		Convey("and a message is obfuscated with it", func() {
			obfuscated := o.Obfuscate(message)
			So(obfuscated, ShouldNotBeEmpty)
			So(obfuscated, ShouldNotEqual, message)

			Convey("after sleeping past the TTL, the message is NOT recoverable", func() {
				time.Sleep(1100 * time.Millisecond)
				clear := o.Normalize(obfuscated)
				So(clear, ShouldBeEmpty)
			})

		})

	})
}

func TestAesObfuscator128TtlBadExpiration(t *testing.T) {

	Convey("When an AesObfuscator is created with a known 16byte key, and a future TTL", t, func() {

		message := "This is a test"
		o, err := NewAesObfuscatorWithExpiration([]byte("95Bx9JkKX3xbd7z3"), 5*time.Second)
		So(err, ShouldBeNil)
		So(o, ShouldNotBeNil)

		no, err := NewAesObfuscator([]byte("95Bx9JkKX3xbd7z3"))
		So(err, ShouldBeNil)
		So(no, ShouldNotBeNil)

		Convey("and a message is obfuscated with the same key, but no TTL (contrived)", func() {
			obfuscated := no.Obfuscate(message)
			So(obfuscated, ShouldNotBeEmpty)
			So(obfuscated, ShouldNotEqual, message)

			Convey("the message is not recoverable", func() {

				clear := o.Normalize(obfuscated)
				So(clear, ShouldEqual, "")
			})

		})

	})
}

func TestAesObfuscator192(t *testing.T) {

	Convey("When an AesObfuscator is created with a known 24byte key", t, func() {

		message := "This is a test"
		o, err := NewAesObfuscator([]byte("cf2nO99ZuWtc4lXsRNONCbp7"))
		So(err, ShouldBeNil)
		So(o, ShouldNotBeNil)

		Convey("and a message is obfuscated with it", func() {
			obfuscated := o.Obfuscate(message)
			So(obfuscated, ShouldNotBeEmpty)
			So(obfuscated, ShouldNotEqual, message)

			Convey("the message is recoverable", func() {

				clear := o.Normalize(obfuscated)
				So(clear, ShouldEqual, message)
			})

		})

	})
}
func TestAesObfuscator256(t *testing.T) {

	Convey("When an AesObfuscator is created with a known 32byte key", t, func() {

		message := "This is a test"
		o, err := NewAesObfuscator([]byte("fOFWV7E4fFuj6cvNPHYbCCD0C90dUnQx"))
		So(err, ShouldBeNil)
		So(o, ShouldNotBeNil)

		Convey("and a message is obfuscated with it", func() {
			obfuscated := o.Obfuscate(message)
			So(obfuscated, ShouldNotBeEmpty)
			So(obfuscated, ShouldNotEqual, message)

			Convey("the message is recoverable", func() {

				clear := o.Normalize(obfuscated)
				So(clear, ShouldEqual, message)
			})

		})

	})
}

func TestAesObfuscatorGarbageNormalized(t *testing.T) {

	Convey("When an AesObfuscator is created with a known 16byte key", t, func() {

		message := "sdflsdkjf4wSDfsdfksjd4RSDFFFv"
		o, err := NewAesObfuscator([]byte("95Bx9JkKX3xbd7z3"))
		So(err, ShouldBeNil)
		So(o, ShouldNotBeNil)

		Convey("and a garbage message is Normalized with it, an empty string is returned", func() {
			obfuscated := o.Normalize(message)
			So(obfuscated, ShouldBeEmpty)
		})

	})
}

func TestAesObfuscatorBase64dGarbageNormalized(t *testing.T) {

	Convey("When an AesObfuscator is created with a known 16byte key", t, func() {

		message := "aGVsbG8K"
		o, err := NewAesObfuscator([]byte("95Bx9JkKX3xbd7z3"))
		So(err, ShouldBeNil)
		So(o, ShouldNotBeNil)

		Convey("and a garbage message is Normalized with it, an empty string is returned", func() {
			obfuscated := o.Normalize(message)
			So(obfuscated, ShouldBeEmpty)
		})

	})
}

func TestAesObfuscatorFixedNonceNormalized(t *testing.T) {

	Convey("When an AesObfuscator is created with a known 16byte key", t, func() {

		message := "ylFZ5v1JgjWWAJMDXPLLkRiwI3ielRPBSef55he-CVaHV3NYJVgexA"
		o, err := NewAesObfuscator([]byte("95Bx9JkKX3xbd7z3"))
		So(err, ShouldBeNil)
		So(o, ShouldNotBeNil)

		Convey("and a previously fixed-nonce message is Normalized with it, an empty string is returned", func() {
			obfuscated := o.Normalize(message)
			So(obfuscated, ShouldBeEmpty)
		})

	})
}

func TestHexObfuscator(t *testing.T) {

	Convey("When a HexObfuscator is created", t, func() {

		message := "This is a test"
		o := HexObfuscator{}
		So(o, ShouldNotBeNil)

		Convey("and a message is obfuscated with it", func() {
			obfuscated := o.Obfuscate(message)
			So(obfuscated, ShouldNotBeEmpty)
			So(obfuscated, ShouldNotEqual, message)

			Convey("the message is recoverable", func() {

				clear := o.Normalize(obfuscated)
				So(clear, ShouldEqual, message)
			})

		})

	})
}

func Test_NonceTimeRandom12Uniqish(t *testing.T) {

	t.Skip("Silly long test that barely proves anything")

	list := make(map[string]bool)
	for i := 0; i < 10000000; i++ {
		nonce := make([]byte, 12)
		binary.PutVarint(nonce, time.Now().UnixNano())
		rpend := make([]byte, 4)
		if _, err := io.ReadFull(rand.Reader, rpend); err != nil {
			panic(err.Error())
		}
		for i := 0; i < 4; i++ {
			nonce[i+8] = rpend[i]
		}
		if _, ok := list[string(nonce)]; ok {
			t.Fail()
		}
		list[string(nonce)] = true
	}
}

func Benchmark_AesObfuscator128(b *testing.B) {

	o, err := NewAesObfuscator([]byte("95Bx9JkKX3xbd7z3"))
	if err != nil {
		b.Fatalf("Creating new AesObfuscator failed: %s\n", err)
	}

	s := "This is a test"
	b.ResetTimer()
	var os string
	for i := 0; i < b.N; i++ {
		os = o.Obfuscate(s)
		if os == "" {
			b.Fail()
		}
	}
}

func Benchmark_AesNormalizer128(b *testing.B) {

	o, err := NewAesObfuscator([]byte("95Bx9JkKX3xbd7z3"))
	if err != nil {
		b.Fatalf("Creating new AesObfuscator failed: %s\n", err)
	}

	s := "C7Gr2cONX6h7o8sZzMkHnVHPnLLBxa_gR5GxcV47zpru2rb0qv5B0REz"
	var ns string
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ns = o.Normalize(s)
		if ns == "" {
			b.Fail()
		}
	}
}

func Benchmark_AesObfuscator128Ttl(b *testing.B) {

	o, err := NewAesObfuscatorWithExpiration([]byte("95Bx9JkKX3xbd7z3"), 5*time.Second)
	if err != nil {
		b.Fatalf("Creating new AesObfuscator failed: %s\n", err)
	}

	s := "This is a test"
	var os string
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		os = o.Obfuscate(s)
		if os == "" {
			b.Fail()
		}
	}
}

func Benchmark_AesNormalizer128Ttl(b *testing.B) {

	o, err := NewAesObfuscatorWithExpiration([]byte("95Bx9JkKX3xbd7z3"), 5*time.Second)
	if err != nil {
		b.Fatalf("Creating new AesObfuscator failed: %s\n", err)
	}

	s := "This is a test"
	os := o.Obfuscate(s)
	var ns string
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ns = o.Normalize(os)
		if ns == "" {
			b.Fail()
		}
	}
}

func Benchmark_HexObfuscator(b *testing.B) {

	o := HexObfuscator{}

	s := "This is a test"
	var os string
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		os = o.Obfuscate(s)
		if os == "" {
			b.Fail()
		}
	}
}

func Benchmark_HexNormalizer(b *testing.B) {

	o := HexObfuscator{}

	s := "5468697320697320612074657374"
	var ns string
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ns = o.Normalize(s)
		if ns == "" {
			b.Fail()
		}
	}
}

func Benchmark_NonceRandom12(b *testing.B) {

	for i := 0; i < b.N; i++ {
		nonce := make([]byte, 12)
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			panic(err.Error())
		}
	}
}

func Benchmark_NonceRandom4(b *testing.B) {

	for i := 0; i < b.N; i++ {
		nonce := make([]byte, 4)
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			panic(err.Error())
		}
	}
}

func Benchmark_NonceTimeRandom4(b *testing.B) {

	for i := 0; i < b.N; i++ {
		nonce := make([]byte, 12)
		binary.PutVarint(nonce, time.Now().UnixNano())
		rpend := make([]byte, 4)
		if _, err := io.ReadFull(rand.Reader, rpend); err != nil {
			panic(err.Error())
		}
		for i := 0; i < 4; i++ {
			nonce[i+8] = rpend[i]
		}
	}
}
