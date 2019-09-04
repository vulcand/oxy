package roundrobin

import "encoding/hex"

// HexObfuscator is a roundrobin.Obfuscator that returns an hex-encoded version of the value
type HexObfuscator struct{}

// Obfuscate takes a raw string and returns the obfuscated value
func (o *HexObfuscator) Obfuscate(raw string) string {
	return hex.EncodeToString([]byte(raw))
}

// Normalize takes an obfuscated string and returns the raw value
func (o *HexObfuscator) Normalize(obfuscatedStr string) string {
	clear, _ := hex.DecodeString(obfuscatedStr)
	return string(clear)
}
