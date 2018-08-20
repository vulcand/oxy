package roundrobin

// Obfuscator is an interface you can pass to NewStickySessionWithObfuscator,
// to encode/encrypt/jumble/whatever your StickySession values
type Obfuscator interface {
	// Obfuscate takes a raw string and returns the obfuscated value
	Obfuscate(string) string
	// Normalize takes an obfuscated string and returns the raw value
	Normalize(string) string
}

// DefaultObfuscator is a no-op that returns the raw/obfuscated strings as-is
type DefaultObfuscator struct{}

// Obfuscate takes a raw string and returns the obfuscated value
func (o *DefaultObfuscator) Obfuscate(raw string) string {
	return raw
}

// Normalize takes an obfuscated string and returns the raw value
func (o *DefaultObfuscator) Normalize(obfuscated string) string {
	return obfuscated
}
