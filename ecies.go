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
)

// Decrypt decrypts a value encrypted for dotenvx with privateKey, a
// secp256k1 private key in hex. Like dotenvx, it returns values that don't
// start with "encrypted:" as they are.
func Decrypt(privateKey, value string) (string, error) {
	data, ok := strings.CutPrefix(value, encryptedPrefix)
	if !ok {
		return value, nil
	}
	if privateKey == "" {
		return "", ErrMissingPrivateKey
	}
	key, ok := parsePrivateKey(privateKey)
	if !ok {
		return "", ErrInvalidPrivateKey
	}
	payload, err := base64.StdEncoding.DecodeString(data)
	if err != nil || len(payload) < ephemeralKeySize+nonceSize+tagSize {
		return "", ErrMalformedEncryptedData
	}
	ephemeral, err := secp256k1.ParsePubKey(payload[:ephemeralKeySize])
	if err != nil {
		return "", ErrMalformedEncryptedData
	}
	nonce := payload[ephemeralKeySize : ephemeralKeySize+nonceSize]
	tag := payload[ephemeralKeySize+nonceSize : ephemeralKeySize+nonceSize+tagSize]
	ciphertext := payload[ephemeralKeySize+nonceSize+tagSize:]
	gcm := newGCM(payload[:ephemeralKeySize], sharedPoint(key, ephemeral))
	plaintext, err := gcm.Open(nil, nonce, append(ciphertext[:len(ciphertext):len(ciphertext)], tag...), nil)
	if err != nil {
		return "", ErrWrongPrivateKey
	}
	return string(plaintext), nil
}

// Encrypt encrypts value for publicKey, a secp256k1 public key in hex,
// compressed or not, the way dotenvx does. Encryption is randomized, so
// encrypting the same value twice gives different results.
func Encrypt(publicKey, value string) (string, error) {
	b, err := hex.DecodeString(publicKey)
	if err != nil {
		return "", ErrInvalidPublicKey
	}
	recipient, err := secp256k1.ParsePubKey(b)
	if err != nil {
		return "", ErrInvalidPublicKey
	}
	ephemeral := newPrivateKey()
	ephemeralPub := ephemeral.PubKey().SerializeUncompressed()
	gcm := newGCM(ephemeralPub, sharedPoint(ephemeral, recipient))
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
		return "", &Error{ErrInvalidPrivateKey.Code, "could not derive public key"}
	}
	return hex.EncodeToString(key.PubKey().SerializeCompressed()), nil
}

// GenerateKeypair returns a new secp256k1 private key and its compressed
// public key, both in hex.
func GenerateKeypair() (privateKey, publicKey string) {
	key := newPrivateKey()
	return hex.EncodeToString(key.Serialize()), hex.EncodeToString(key.PubKey().SerializeCompressed())
}

// parsePrivateKey parses a private key of 32 bytes in hex, which must be a
// valid scalar: not zero, and less than the order of the curve.
func parsePrivateKey(s string) (*secp256k1.PrivateKey, bool) {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 {
		return nil, false
	}
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
		if key, ok := parsePrivateKey(hex.EncodeToString(b)); ok {
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
// shared. Its key is HKDF-SHA256 of the two, with no salt or info.
func newGCM(ephemeralPub, shared []byte) cipher.AEAD {
	// None of these can fail: they only reject keys longer than 255
	// SHA-256 blocks, AES keys that aren't 16, 24 or 32 bytes, and nonces
	// that are empty, and these are 32, 32 and 16 bytes.
	key, _ := hkdf.Key(sha256.New, append(ephemeralPub[:len(ephemeralPub):len(ephemeralPub)], shared...), nil, "", 32)
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCMWithNonceSize(block, nonceSize)
	return gcm
}
