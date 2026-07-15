package cbe

import (
	"crypto/rand"
	"time"
)

type Timestamp int64 // Seconds since the epoch (or sometimes just seconds)

// Return linux epoch timestamp for current time
func NewTimestamp() Timestamp {
	return Timestamp(time.Now().Unix())
}

// Create a generically lengthed random string of uppercase letters and digits with length
func NewGenericLengthID(length int) string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	for i := range b {
		b[i] = letters[b[i]%byte(len(letters))]
	}
	return string(b)
}

// Note: words 'secure' and 'unsecure' are relative, they don't mean safety, simply increased brute-force difficulty.

// Not secure identifier, used for internal IDs and public display
// 8-character alphanumeric uppercase string
type GenericUnsecureID string

// Generate new ID
func NewGenericUnsecureID() GenericUnsecureID {
	return GenericUnsecureID(NewGenericLengthID(8))
}

// Secure id for payments etc, not used for public display
// 64-character alphanumeric uppercase string
type GenericSecureID string

// Generate new secure ID of length 64
func NewGenericSecureID() GenericSecureID {
	return GenericSecureID(NewGenericLengthID(64))
}

// Secure key for shares etc, 256 characters
type GenericSecureKey string

func NewGenericSecureKey() GenericSecureKey {
	return GenericSecureKey(NewGenericLengthID(256))
}
