package dotenvx

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

// encryptedPrefix starts every encrypted value.
const encryptedPrefix = "encrypted:"

// Encrypted values use the ECIES scheme of eciesjs and ecies-rs, as
// dotenvx configures them. The payload, base64-encoded after the prefix, is
// an ephemeral public key (uncompressed), a nonce, a GCM tag and the
// ciphertext.
const (
	ephemeralKeySize = 65
	nonceSize        = 16
	tagSize          = 16
	// uncompressedPoint starts an uncompressed public key. eciesjs rejects
	// the hybrid forms, 6 and 7, that decred would accept.
	uncompressedPoint = 4
)

// Decrypt decrypts a value encrypted for dotenvx with privateKey, a
// secp256k1 private key in hex. Like dotenvx, it returns values that don't
// start with "encrypted:" as they are.
//
// Its errors match ErrMissingPrivateKey, ErrInvalidPrivateKey,
// ErrWrongPrivateKey (when the value was encrypted for another key) and
// ErrDecryptionFailed (when it isn't a valid encrypted value), as dotenvx's
// do. Unlike dotenvx, it parses the private key strictly as 64 hex digits:
// Node reads hex up to the first character that isn't. It skips characters
// outside ASCII in the base64, which Node reads by the low byte of their
// UTF-16 code units. And it returns plaintext that isn't valid UTF-8 as it
// is, where dotenvx gives U+FFFD.
func Decrypt(privateKey, value string) (string, error) {
	data, ok := strings.CutPrefix(value, encryptedPrefix)
	if !ok {
		return value, nil
	}
	if privateKey == "" {
		return "", newError(ErrMissingPrivateKey, nil)
	}
	key, ok := parsePrivateKey(privateKey)
	if !ok {
		return "", newError(ErrInvalidPrivateKey, nil)
	}
	payload := decodeBase64(data)
	if len(payload) < ephemeralKeySize+nonceSize+tagSize || payload[0] != uncompressedPoint {
		return "", newError(ErrDecryptionFailed, nil)
	}
	ephemeral, err := secp256k1.ParsePubKey(payload[:ephemeralKeySize])
	if err != nil {
		return "", newError(ErrDecryptionFailed, err)
	}
	gcm, err := newGCM(payload[:ephemeralKeySize], sharedPoint(key, ephemeral))
	if err != nil {
		return "", newError(ErrDecryptionFailed, err)
	}
	nonce := payload[ephemeralKeySize : ephemeralKeySize+nonceSize]
	tag := payload[ephemeralKeySize+nonceSize : ephemeralKeySize+nonceSize+tagSize]
	ciphertext := payload[ephemeralKeySize+nonceSize+tagSize:]
	plaintext, err := gcm.Open(nil, nonce, append(ciphertext[:len(ciphertext):len(ciphertext)], tag...), nil)
	if err != nil {
		return "", newError(ErrWrongPrivateKey, nil)
	}
	return string(plaintext), nil
}

// decodeBase64 decodes base64 as Node's Buffer does: it reads both the
// standard and the URL-safe alphabet, skips any other characters, stops at
// the first "=", and drops a final character that makes no whole byte.
// Characters outside ASCII are skipped too. Node instead reads each UTF-16
// code unit by its low byte, so it takes Ł (U+0141) for A and stops at the
// first half of an emoji.
func decodeBase64(s string) []byte {
	var b []byte
	for i := 0; i < len(s) && s[i] != '='; i++ {
		switch c := s[i]; {
		case c == '-':
			b = append(b, '+')
		case c == '_':
			b = append(b, '/')
		case 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z', '0' <= c && c <= '9', c == '+', c == '/':
			b = append(b, c)
		}
	}
	if len(b)%4 == 1 {
		b = b[:len(b)-1]
	}
	// What is left is valid unpadded base64, which always decodes.
	out, _ := base64.RawStdEncoding.DecodeString(string(b))
	return out
}

// Encrypt encrypts value for publicKey, a secp256k1 public key in hex,
// compressed or not and optionally starting with "0x", the way dotenvx
// does. Encryption is randomized, so encrypting the same value twice gives
// different results. Its errors match ErrInvalidPublicKey, or
// ErrEncryptionFailed when the scheme isn't allowed, as with
// GODEBUG=fips140=only.
func Encrypt(publicKey, value string) (string, error) {
	if len(publicKey) > 2 && publicKey[0] == '0' && (publicKey[1] == 'x' || publicKey[1] == 'X') {
		publicKey = publicKey[2:]
	}
	b, err := hex.DecodeString(publicKey)
	if err != nil || len(b) == 0 || b[0] != uncompressedPoint && b[0] != 2 && b[0] != 3 {
		return "", newError(ErrInvalidPublicKey, err)
	}
	recipient, err := secp256k1.ParsePubKey(b)
	if err != nil {
		return "", newError(ErrInvalidPublicKey, err)
	}
	ephemeral := newPrivateKey()
	ephemeralPub := ephemeral.PubKey().SerializeUncompressed()
	gcm, err := newGCM(ephemeralPub, sharedPoint(ephemeral, recipient))
	if err != nil {
		return "", newError(ErrEncryptionFailed, err)
	}
	nonce := make([]byte, nonceSize)
	rand.Read(nonce)
	sealed := gcm.Seal(nil, nonce, []byte(value), nil)
	ciphertext, tag := sealed[:len(value)], sealed[len(value):]
	payload := append(append(append(ephemeralPub, nonce...), tag...), ciphertext...)
	return encryptedPrefix + base64.StdEncoding.EncodeToString(payload), nil
}

// PublicKey returns the compressed public key, in hex, of privateKey, a
// secp256k1 private key in hex. dotenvx files name the keys they are
// encrypted for by this public key.
func PublicKey(privateKey string) (string, error) {
	key, ok := parsePrivateKey(privateKey)
	if !ok {
		return "", &Error{Code: ErrInvalidPrivateKey.Code, Message: "could not derive public key"}
	}
	return hex.EncodeToString(key.PubKey().SerializeCompressed()), nil
}

// GenerateKeypair returns a new secp256k1 private key and its compressed
// public key, both in hex.
func GenerateKeypair() (privateKey, publicKey string) {
	key := newPrivateKey()
	return hex.EncodeToString(key.Serialize()), hex.EncodeToString(key.PubKey().SerializeCompressed())
}

// parsePrivateKey parses a private key of 32 bytes in hex.
func parsePrivateKey(s string) (*secp256k1.PrivateKey, bool) {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 {
		return nil, false
	}
	return privateKeyFromBytes(b)
}

// privateKeyFromBytes makes a private key of 32 bytes, which must be a
// valid scalar: not zero, and less than the order of the curve.
func privateKeyFromBytes(b []byte) (*secp256k1.PrivateKey, bool) {
	var k secp256k1.ModNScalar
	if overflow := k.SetByteSlice(b); overflow || k.IsZero() {
		return nil, false
	}
	return secp256k1.NewPrivateKey(&k), true
}

// newPrivateKey returns a random private key.
func newPrivateKey() *secp256k1.PrivateKey {
	b := make([]byte, 32)
	for {
		// crypto/rand.Read never fails: since Go 1.24 it crashes instead.
		rand.Read(b)
		if key, ok := privateKeyFromBytes(b); ok {
			return key
		}
	}
}

// sharedPoint returns key times pub, uncompressed: the ECDH shared point
// that eciesjs derives its keys from.
func sharedPoint(key *secp256k1.PrivateKey, pub *secp256k1.PublicKey) []byte {
	var point, result secp256k1.JacobianPoint
	pub.AsJacobian(&point)
	secp256k1.ScalarMultNonConst(&key.Key, &point, &result)
	result.ToAffine()
	return secp256k1.NewPublicKey(&result.X, &result.Y).SerializeUncompressed()
}

// newGCM returns the AES-256-GCM cipher, with eciesjs's 16-byte nonces, for
// a message whose ephemeral public key is ephemeralPub and shared point is
// shared. Its key is HKDF-SHA256 of the two, with no salt or info. It fails
// only when the nonce size isn't allowed, as in FIPS 140-only mode.
func newGCM(ephemeralPub, shared []byte) (cipher.AEAD, error) {
	// These can't fail: they only reject keys longer than 255 SHA-256
	// blocks and AES keys that aren't 16, 24 or 32 bytes, and this is 32.
	key, _ := hkdf.Key(sha256.New, append(ephemeralPub[:len(ephemeralPub):len(ephemeralPub)], shared...), nil, "", 32)
	block, _ := aes.NewCipher(key)
	return cipher.NewGCMWithNonceSize(block, nonceSize)
}
