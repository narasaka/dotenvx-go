package dotenvx

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The plaintext and encrypted sources of testdata/encrypted.
var (
	encryptedSrc  = string(readFileOnce("testdata/encrypted/.env"))
	encryptedWant = map[string]string{"DOTENV_PUBLIC_KEY": testPublicKey, "HELLO": "encrypted"}
)

func readFileOnce(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return b
}

// Every case here was checked against @dotenvx/primitives 4.3.0.
func TestParseDecrypt(t *testing.T) {
	keys := filepath.Join("testdata", "encrypted", ".env.keys")
	tests := []struct {
		name    string
		src     string
		opts    Options
		want    map[string]string
		wantErr string
	}{
		{name: "key in the environment", src: encryptedSrc, opts: Options{Env: []string{"DOTENV_PRIVATE_KEY=" + testPrivateKey}, KeyFiles: []string{}}, want: encryptedWant},
		{name: "key under another name", src: encryptedSrc, opts: Options{Env: []string{"DOTENV_PRIVATE_KEY_PRODUCTION=" + testPrivateKey}, KeyFiles: []string{}}, want: encryptedWant},
		{name: "key file", src: encryptedSrc, opts: Options{Env: []string{}, KeyFiles: []string{keys}}, want: encryptedWant},
		{name: "missing key file", src: encryptedSrc, opts: Options{Env: []string{}, KeyFiles: []string{"missing", keys}}, want: encryptedWant},
		// Only key files list several keys with commas.
		{name: "keys in a key file", src: encryptedSrc, opts: Options{Env: []string{}, KeyFiles: []string{writeKeys(t, "DOTENV_PRIVATE_KEY=bad,"+testPrivateKey)}}, want: encryptedWant},
		{
			name: "keys in the environment", src: encryptedSrc, opts: Options{Env: []string{"DOTENV_PRIVATE_KEY=bad," + testPrivateKey}, KeyFiles: []string{}},
			want:    map[string]string{"DOTENV_PUBLIC_KEY": testPublicKey, "HELLO": testEncrypted},
			wantErr: "[DECRYPTION_FAILED] could not decrypt HELLO",
		},
		{
			name: "no key", src: encryptedSrc + "\nB=encrypted:abc\nC=$HELLO", opts: Options{Env: []string{}, KeyFiles: []string{}},
			want:    map[string]string{"DOTENV_PUBLIC_KEY": testPublicKey, "HELLO": testEncrypted, "B": "encrypted:abc", "C": testEncrypted},
			wantErr: "[DECRYPTION_FAILED] could not decrypt HELLO, B",
		},
		{name: "an existing encrypted value", src: "HELLO=x", opts: Options{Env: []string{"HELLO=" + testEncrypted, "DOTENV_PRIVATE_KEY=" + testPrivateKey}, KeyFiles: []string{}}, want: map[string]string{"HELLO": "encrypted"}},
		{
			name: "decrypted values expand", src: encryptedSrc + "\nB=${HELLO}-x\nC='$HELLO'", opts: Options{Env: []string{"DOTENV_PRIVATE_KEY=" + testPrivateKey}, KeyFiles: []string{}},
			want: map[string]string{"DOTENV_PUBLIC_KEY": testPublicKey, "HELLO": "encrypted", "B": "encrypted-x", "C": "$HELLO"},
		},
		{name: "a provider", src: encryptedSrc, opts: Options{Env: []string{}, KeyFiles: []string{}, KeyProvider: func(pub string) (string, error) {
			if pub != testPublicKey {
				return "", nil
			}
			return testPrivateKey, nil
		}}, want: encryptedWant},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.opts.Parse(strings.NewReader(tt.src))
			if !maps.Equal(got, tt.want) {
				t.Errorf("Parse = %q, want %q", got, tt.want)
			}
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("Parse error = %v", err)
			case tt.wantErr != "" && (!errors.Is(err, ErrDecryptionFailed) || !strings.HasSuffix(err.Error(), tt.wantErr)):
				t.Errorf("Parse error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func writeKeys(t *testing.T, src string) string {
	t.Helper()
	return writeFile(t, t.TempDir(), ".env.keys", src)
}

func TestParseKeyProvider(t *testing.T) {
	// It is asked only about public keys without private keys, from the
	// source or the environment, once each.
	otherPub := "02" + strings.Repeat("1", 64)
	var asked []string
	opts := Options{
		Env:      []string{"DOTENV_PUBLIC_KEY_ENV=" + otherPub, "DOTENV_PUBLIC_KEY_DUP=" + testPublicKey},
		KeyFiles: []string{},
		KeyProvider: func(pub string) (string, error) {
			asked = append(asked, pub)
			return "", nil
		},
	}
	if _, err := opts.Parse(strings.NewReader(encryptedSrc)); !errors.Is(err, ErrDecryptionFailed) {
		t.Errorf("Parse error = %v, want ErrDecryptionFailed", err)
	}
	if want := []string{testPublicKey, otherPub}; !slicesEqual(asked, want) {
		t.Errorf("provider asked about %q, want %q", asked, want)
	}

	// It isn't asked about public keys whose private keys are known.
	asked = nil
	opts.Env = append(opts.Env, "DOTENV_PRIVATE_KEY="+testPrivateKey)
	if got, err := opts.Parse(strings.NewReader(encryptedSrc)); err != nil || got["HELLO"] != "encrypted" {
		t.Errorf("Parse = %q, %v; want HELLO decrypted", got, err)
	}
	if want := []string{otherPub}; !slicesEqual(asked, want) {
		t.Errorf("provider asked about %q, want %q", asked, want)
	}
	opts.Env = opts.Env[:len(opts.Env)-1]

	// Its errors stop parsing, and its invalid keys are ignored.
	providerErr := errors.New("vault is sealed")
	opts.KeyProvider = func(string) (string, error) { return "", providerErr }
	if got, err := opts.Parse(strings.NewReader(encryptedSrc)); !errors.Is(err, providerErr) || got != nil {
		t.Errorf("Parse = %q, %v; want no values and the provider's error", got, err)
	}
	opts.KeyProvider = func(string) (string, error) { return "not a key", nil }
	if _, err := opts.Parse(strings.NewReader(encryptedSrc)); !errors.Is(err, ErrDecryptionFailed) {
		t.Errorf("Parse error = %v, want ErrDecryptionFailed", err)
	}
}

func slicesEqual(a, b []string) bool {
	return strings.Join(a, "\x00") == strings.Join(b, "\x00") && len(a) == len(b)
}

func TestParseDefaultKeyFile(t *testing.T) {
	// With no KeyFiles, Parse reads .env.keys in the current directory.
	t.Chdir(filepath.Join("testdata", "encrypted"))
	got, err := Options{Env: []string{}}.Parse(strings.NewReader(encryptedSrc))
	if err != nil || !maps.Equal(got, encryptedWant) {
		t.Errorf("Parse = %q, %v; want %q", got, err, encryptedWant)
	}
}

func TestReadEncrypted(t *testing.T) {
	// Read finds .env.keys next to each file.
	got, err := Options{Env: []string{}}.Read("testdata/multiple/.env.production", "testdata/encrypted/.env")
	want := map[string]string{
		"DOTENV_PUBLIC_KEY_PRODUCTION": "027907f1e455c0e881f12e0e385461df4166dc6324d1a584ce47858485357e16bb",
		"DOTENV_PUBLIC_KEY":            testPublicKey,
		"HELLO":                        "one", "HELLO2": "two", "HELLO3": "three",
	}
	if err != nil || !maps.Equal(got, want) {
		t.Errorf("Read = %q, %v; want %q", got, err, want)
	}
}

// When the file's own private key is set, dotenvx reads .env.keys in the
// current directory instead of the one next to the file.
func TestReadKeyFileWithKeyInEnv(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env.keys", "DOTENV_PRIVATE_KEY="+testPrivateKey)
	t.Chdir(dir)
	env := filepath.Join(dir, "sub")
	if err := os.Mkdir(env, 0o700); err != nil {
		t.Fatal(err)
	}
	file := writeFile(t, env, ".env.production", encryptedSrc)
	got, err := Options{Env: []string{"DOTENV_PRIVATE_KEY_PRODUCTION=wrong"}}.Read(file)
	if err != nil || got["HELLO"] != "encrypted" {
		t.Errorf("Read = %q, %v; want HELLO decrypted with ./.env.keys", got, err)
	}
}

func TestPrivateKeyName(t *testing.T) {
	tests := map[string]string{
		".env":                  "DOTENV_PRIVATE_KEY",
		"dir/.env":              "DOTENV_PRIVATE_KEY",
		".ENV":                  "DOTENV_PRIVATE_KEY",
		".env.txt":              "DOTENV_PRIVATE_KEY",
		".env.production":       "DOTENV_PRIVATE_KEY_PRODUCTION",
		".env.production.txt":   "DOTENV_PRIVATE_KEY_PRODUCTION",
		".env.production.local": "DOTENV_PRIVATE_KEY_PRODUCTION_LOCAL",
		".env.a.b.c":            "DOTENV_PRIVATE_KEY_A_B",
		".env1":                 "DOTENV_PRIVATE_KEY_DEVELOPMENT1",
		"secrets":               "DOTENV_PRIVATE_KEY_SECRETS",
	}
	for file, want := range tests {
		if got := privateKeyName(file); got != want {
			t.Errorf("privateKeyName(%q) = %q, want %q", file, got, want)
		}
	}
}

func TestReadDefaultFilesFromKeys(t *testing.T) {
	// With no files and no DOTENV_FILE, set private keys name the files,
	// in the environment's order.
	dir := t.TempDir()
	writeFile(t, dir, ".env.ci.test", "A=ci")
	writeFile(t, dir, ".env", "B=dev")
	t.Chdir(dir)
	env := newEnvironment([]string{"DOTENV_PRIVATE_KEY_CI_TEST=x", "OTHER=1", "DOTENV_PRIVATE_KEY=y"}, false)
	got, _, err := Options{KeyFiles: []string{}}.read(nil, env)
	if want := map[string]string{"A": "ci", "B": "dev"}; err != nil || !maps.Equal(got, want) {
		t.Errorf("read = %q, %v; want %q", got, err, want)
	}
}

// The embedding use case: an encrypted source, and only its private key in
// the process environment.
func TestParseWithProcessKey(t *testing.T) {
	t.Setenv("DOTENV_PRIVATE_KEY", testPrivateKey)
	got, err := Options{KeyFiles: []string{}}.Parse(strings.NewReader(encryptedSrc))
	if err != nil || !maps.Equal(got, encryptedWant) {
		t.Errorf("Parse = %q, %v; want %q", got, err, encryptedWant)
	}
}

// dotenvx sets each file's values before reading the next, so a private key
// that one file sets decrypts the files after it.
func TestReadKeyFromEarlierFile(t *testing.T) {
	dir := t.TempDir()
	keys := writeFile(t, dir, "keys.env", "DOTENV_PRIVATE_KEY="+testPrivateKey)
	encrypted := writeFile(t, dir, ".env", encryptedSrc)
	got, err := Options{Env: []string{}, KeyFiles: []string{}}.Read(keys, encrypted)
	if err != nil || got["HELLO"] != "encrypted" {
		t.Errorf("Read = %q, %v; want HELLO decrypted with the key from keys.env", got, err)
	}
}
