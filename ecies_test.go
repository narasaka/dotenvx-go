package dotenvx

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

const (
	// The key pair of testdata/encrypted.
	testPrivateKey = "ec9e80073d7ace817d35acb8b7293cbf8e5981b4d2f5708ee5be405122993cd1"
	testPublicKey  = "03eaf2142ab3d55bdf108962334e06696db798e7412cfc51d75e74b4f87f299bba"
	// testEncrypted is "encrypted", from testdata/encrypted/.env.
	testEncrypted = "encrypted:BG8M6U+GKJGwpGA42ml2erb9+T2NBX6Z2JkBLynDy21poz0UfF5aPxCgRbIyhnQFdWKd0C9GZ7lM5PeL86xghoMcWvvPpkyQ0yaD2pZ64RzoxFGB1lTZYlEgQOxTDJnWxODHfuQcFY10uA=="
)

func TestDecryptNodeVectors(t *testing.T) {
	var file struct {
		PrivateKey string `json:"privateKey"`
		PublicKey  string `json:"publicKey"`
		Vectors    []struct {
			Plaintext string `json:"plaintext"`
			Encrypted string `json:"encrypted"`
		} `json:"vectors"`
	}
	readJSON(t, "testdata/ecies/vectors.json", &file)
	if len(file.Vectors) == 0 {
		t.Fatal("no vectors")
	}
	for _, v := range file.Vectors {
		got, err := Decrypt(file.PrivateKey, v.Encrypted)
		if err != nil || got != v.Plaintext {
			t.Errorf("Decrypt(%.30q…) = %.30q, %v; want %.30q", v.Encrypted, got, err, v.Plaintext)
		}
	}
	if got, err := PublicKey(file.PrivateKey); err != nil || got != file.PublicKey {
		t.Errorf("PublicKey = %q, %v; want %q", got, err, file.PublicKey)
	}
}

func TestDecryptFixture(t *testing.T) {
	got, err := Decrypt(testPrivateKey, testEncrypted)
	if err != nil || got != "encrypted" {
		t.Errorf("Decrypt = %q, %v; want %q", got, err, "encrypted")
	}
	// Uppercase hex works too.
	if got, err := Decrypt(strings.ToUpper(testPrivateKey), testEncrypted); err != nil || got != "encrypted" {
		t.Errorf("Decrypt with uppercase key = %q, %v", got, err)
	}
}

// Like Node's Buffer, Decrypt reads base64 with or without padding, in
// either alphabet, skipping other characters and whatever follows "=".
// Node decrypts each of these.
func TestDecryptBase64(t *testing.T) {
	data := strings.TrimPrefix(testEncrypted, "encrypted:")
	for _, variant := range []string{
		strings.TrimRight(data, "="),
		strings.NewReplacer("+", "-", "/", "_").Replace(data),
		data[:40] + "\n" + data[40:] + "\r\n",
		data[:50] + " \t!" + data[50:],
		data + "ignored",
	} {
		if got, err := Decrypt(testPrivateKey, "encrypted:"+variant); err != nil || got != "encrypted" {
			t.Errorf("Decrypt(%q) = %q, %v", variant, got, err)
		}
	}
}

func TestDecryptPlaintext(t *testing.T) {
	// Like dotenvx, values without the prefix pass through, even without a key.
	if got, err := Decrypt("", "plain"); err != nil || got != "plain" {
		t.Errorf("Decrypt = %q, %v; want the value as it is", got, err)
	}
}

func TestEncryptRoundTrip(t *testing.T) {
	privateKey, publicKey := GenerateKeypair()
	if got, err := PublicKey(privateKey); err != nil || got != publicKey {
		t.Fatalf("PublicKey(%s) = %q, %v; want %q", privateKey, got, err, publicKey)
	}
	uncompressed := uncompressedHex(t, publicKey)
	for _, key := range []string{publicKey, uncompressed} {
		for _, plaintext := range []string{"", "hello", "héllo ✓", strings.Repeat("x", 10000)} {
			encrypted, err := Encrypt(key, plaintext)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(encrypted, "encrypted:") {
				t.Fatalf("Encrypt = %q, want the encrypted: prefix", encrypted)
			}
			if got, err := Decrypt(privateKey, encrypted); err != nil || got != plaintext {
				t.Errorf("round trip of %.20q = %.20q, %v", plaintext, got, err)
			}
		}
	}
	// Encryption is randomized.
	a, _ := Encrypt(publicKey, "x")
	b, _ := Encrypt(publicKey, "x")
	if a == b {
		t.Error("two encryptions of the same value are equal")
	}
}

func TestDecryptErrors(t *testing.T) {
	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(testEncrypted, "encrypted:"))
	if err != nil {
		t.Fatal(err)
	}
	encode := func(b []byte) string { return "encrypted:" + base64.StdEncoding.EncodeToString(b) }
	tampered := append([]byte{}, payload...)
	tampered[len(tampered)-1] ^= 1
	badPoint := append([]byte{}, payload...)
	badPoint[1] ^= 1
	hybrid := append([]byte{}, payload...)
	hybrid[0] = 7
	otherKey, _ := GenerateKeypair()
	tests := []struct {
		name       string
		privateKey string
		value      string
		want       error
	}{
		{"missing key", "", testEncrypted, ErrMissingPrivateKey},
		{"not hex", "zz" + testPrivateKey[2:], testEncrypted, ErrInvalidPrivateKey},
		{"short", testPrivateKey[:62], testEncrypted, ErrInvalidPrivateKey},
		{"zero", strings.Repeat("0", 64), testEncrypted, ErrInvalidPrivateKey},
		{"the curve order", "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141", testEncrypted, ErrInvalidPrivateKey},
		{"wrong key", otherKey, testEncrypted, ErrWrongPrivateKey},
		{"tampered", testPrivateKey, encode(tampered), ErrWrongPrivateKey},
		// Node gives DECRYPTION_FAILED for all of these.
		{"empty", testPrivateKey, "encrypted:", ErrDecryptionFailed},
		{"not base64", testPrivateKey, "encrypted:!!!", ErrDecryptionFailed},
		{"too short", testPrivateKey, encode(payload[:96]), ErrDecryptionFailed},
		{"not a point", testPrivateKey, encode(badPoint), ErrDecryptionFailed},
		{"a hybrid point", testPrivateKey, encode(hybrid), ErrDecryptionFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decrypt(tt.privateKey, tt.value)
			if !errors.Is(err, tt.want) || got != "" {
				t.Errorf("Decrypt = %q, %v; want %v", got, err, tt.want)
			}
			var e *Error
			if !errors.As(err, &e) || !strings.HasPrefix(err.Error(), "["+e.Code+"] ") {
				t.Errorf("error %q isn't an *Error in dotenvx's [CODE] message form", err)
			}
		})
	}
}

func TestPublicKeyErrors(t *testing.T) {
	for _, key := range []string{"", "zz", strings.Repeat("0", 64), testPrivateKey + "00"} {
		if _, err := PublicKey(key); !errors.Is(err, ErrInvalidPrivateKey) {
			t.Errorf("PublicKey(%q) error = %v, want ErrInvalidPrivateKey", key, err)
		}
	}
}

func TestEncryptErrors(t *testing.T) {
	hybrid := "07" + uncompressedHex(t, testPublicKey)[2:]
	for _, key := range []string{"", "zz", testPublicKey[:64], "04" + strings.Repeat("00", 64), "02" + strings.Repeat("ff", 32), hybrid} {
		if _, err := Encrypt(key, "x"); !errors.Is(err, ErrInvalidPublicKey) {
			t.Errorf("Encrypt(%q) error = %v, want ErrInvalidPublicKey", key, err)
		}
	}
}

func TestEncryptHexPrefix(t *testing.T) {
	for _, prefix := range []string{"0x", "0X"} {
		encrypted, err := Encrypt(prefix+testPublicKey, "hi")
		if err != nil {
			t.Fatal(err)
		}
		if got, err := Decrypt(testPrivateKey, encrypted); err != nil || got != "hi" {
			t.Errorf("round trip with %s = %q, %v", prefix, got, err)
		}
	}
}

func TestErrorIs(t *testing.T) {
	err := &Error{Code: "WRONG_PRIVATE_KEY", Message: "something else"}
	if !errors.Is(err, ErrWrongPrivateKey) || errors.Is(err, ErrInvalidPrivateKey) || errors.Is(err, errors.New("x")) || errors.Is(err, (*Error)(nil)) {
		t.Error("Error.Is doesn't match by code")
	}
	cause := errors.New("cause")
	if err := newError(ErrDecryptionFailed, cause); !errors.Is(err, cause) || err.Error() != "[DECRYPTION_FAILED] could not decrypt value: cause" {
		t.Errorf("error with a cause = %q, which should unwrap to it", err)
	}
	// Returned errors are copies, which callers may change.
	_, err1 := Decrypt("", testEncrypted)
	err1.(*Error).Message = "changed"
	if _, err2 := Decrypt("", testEncrypted); err2.Error() != ErrMissingPrivateKey.Error() {
		t.Errorf("after changing a returned error, Decrypt returns %q", err2)
	}
	if got := ErrWrongPrivateKey.Error(); got != "[WRONG_PRIVATE_KEY] could not decrypt using private key" {
		t.Errorf("Error() = %q", got)
	}
}

// uncompressedHex returns the uncompressed form of a compressed public key.
func uncompressedHex(t *testing.T, compressed string) string {
	t.Helper()
	b, err := hex.DecodeString(compressed)
	if err != nil {
		t.Fatal(err)
	}
	key, err := secp256k1.ParsePubKey(b)
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(key.SerializeUncompressed())
}

// Each result here is what Node's Buffer.from(s, "base64") gives.
func TestDecodeBase64(t *testing.T) {
	tests := map[string]string{
		"QQ": "41", "QQ==": "41", "Q": "", "QUJD": "414243", "QUJDR": "414243",
		"_w": "ff", "-w": "fb", "/w": "ff", "+w": "fb",
		"Q Q=x": "41", "QQ=\nQQ": "41", "!Q@Q#": "41",
	}
	for s, want := range tests {
		if got := hex.EncodeToString(decodeBase64(s)); got != want {
			t.Errorf("decodeBase64(%q) = %s, want %s", s, got, want)
		}
	}
}
