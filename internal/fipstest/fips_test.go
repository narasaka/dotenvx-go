//go:debug fips140=only

// Package fipstest checks dotenvx in FIPS 140-only mode, which applies to a
// whole test binary, so it has a package of its own.
package fipstest

import (
	"errors"
	"testing"

	"github.com/narasaka/dotenvx-go"
)

// The scheme's 16-byte GCM nonces aren't allowed in this mode, so neither
// decrypting nor encrypting works, and both say so.
func TestFIPSOnly(t *testing.T) {
	const (
		privateKey = "ec9e80073d7ace817d35acb8b7293cbf8e5981b4d2f5708ee5be405122993cd1"
		publicKey  = "03eaf2142ab3d55bdf108962334e06696db798e7412cfc51d75e74b4f87f299bba"
		value      = "encrypted:BG8M6U+GKJGwpGA42ml2erb9+T2NBX6Z2JkBLynDy21poz0UfF5aPxCgRbIyhnQFdWKd0C9GZ7lM5PeL86xghoMcWvvPpkyQ0yaD2pZ64RzoxFGB1lTZYlEgQOxTDJnWxODHfuQcFY10uA=="
	)
	if _, err := dotenvx.Decrypt(privateKey, value); !errors.Is(err, dotenvx.ErrDecryptionFailed) || errors.Unwrap(err) == nil {
		t.Errorf("Decrypt error = %v, want ErrDecryptionFailed with its cause", err)
	}
	if _, err := dotenvx.Encrypt(publicKey, "x"); !errors.Is(err, dotenvx.ErrEncryptionFailed) || errors.Unwrap(err) == nil {
		t.Errorf("Encrypt error = %v, want ErrEncryptionFailed with its cause", err)
	}
}
