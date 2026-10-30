package dotenvx_test

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"

	"github.com/narasaka/dotenvx-go"
)

func ExampleOptions_Parse() {
	src := `# comments and blank lines are skipped
HELLO=World
GREETING="Hello, ${HELLO}
on two lines"
EXISTING=from the file
`
	opts := dotenvx.Options{Env: []string{"EXISTING=from the environment"}}
	values, err := opts.Parse(strings.NewReader(src))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(values["HELLO"])
	fmt.Println(values["GREETING"])
	fmt.Println(values["EXISTING"])

	opts.Overload = true
	values, _ = opts.Parse(strings.NewReader(src))
	fmt.Println(values["EXISTING"])
	// Output:
	// World
	// Hello, World
	// on two lines
	// from the environment
	// from the file
}

func ExampleOptions_Read() {
	// Files are read in order, and the first to set a key wins, so the
	// local override comes first. The later file sees its values.
	values, err := dotenvx.Options{Env: []string{}}.Read("testdata/load/.env.local", "testdata/load/.env")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(values["HELLO"])
	fmt.Println(values["GREETING"])
	// Output:
	// local
	// Hello, local
}

func ExampleLoad() {
	// Load .env from the current directory, if there is one, keeping
	// variables that are already set.
	if err := dotenvx.Load(".env"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Fatal(err)
	}
	fmt.Println(os.Getenv("HELLO"))
}

func ExampleOverload() {
	// Overload replaces variables that are already set, and .env.local,
	// read last, wins over .env.
	if err := dotenvx.Overload(".env", ".env.local"); err != nil {
		log.Fatal(err)
	}
}

func ExampleDecrypt() {
	// The private key and an encrypted value from testdata/encrypted.
	const privateKey = "ec9e80073d7ace817d35acb8b7293cbf8e5981b4d2f5708ee5be405122993cd1"
	value, err := dotenvx.Decrypt(privateKey, "encrypted:BG8M6U+GKJGwpGA42ml2erb9+T2NBX6Z2JkBLynDy21poz0UfF5aPxCgRbIyhnQFdWKd0C9GZ7lM5PeL86xghoMcWvvPpkyQ0yaD2pZ64RzoxFGB1lTZYlEgQOxTDJnWxODHfuQcFY10uA==")
	fmt.Println(value, err)

	other, _ := dotenvx.GenerateKeypair()
	_, err = dotenvx.Decrypt(other, "encrypted:BG8M6U+GKJGwpGA42ml2erb9+T2NBX6Z2JkBLynDy21poz0UfF5aPxCgRbIyhnQFdWKd0C9GZ7lM5PeL86xghoMcWvvPpkyQ0yaD2pZ64RzoxFGB1lTZYlEgQOxTDJnWxODHfuQcFY10uA==")
	fmt.Println(errors.Is(err, dotenvx.ErrWrongPrivateKey), err)
	// Output:
	// encrypted <nil>
	// true [WRONG_PRIVATE_KEY] could not decrypt using private key
}

func ExampleEncrypt() {
	privateKey, publicKey := dotenvx.GenerateKeypair()
	encrypted, err := dotenvx.Encrypt(publicKey, "World")
	if err != nil {
		fmt.Println(err)
		return
	}
	value, _ := dotenvx.Decrypt(privateKey, encrypted)
	fmt.Println(value)
	// Output: World
}

func ExamplePublicKey() {
	publicKey, err := dotenvx.PublicKey("ec9e80073d7ace817d35acb8b7293cbf8e5981b4d2f5708ee5be405122993cd1")
	fmt.Println(publicKey, err)
	// Output: 03eaf2142ab3d55bdf108962334e06696db798e7412cfc51d75e74b4f87f299bba <nil>
}

func ExampleGenerateKeypair() {
	privateKey, publicKey := dotenvx.GenerateKeypair()
	// 32 bytes, and a compressed point of 33, in hex.
	fmt.Println(len(privateKey), len(publicKey))
	// Output: 64 66
}

func ExampleOptions_keyProvider() {
	// A provider looks private keys up by public key, for example in a key
	// store. This one knows the key of testdata/encrypted.
	keys := map[string]string{
		"03eaf2142ab3d55bdf108962334e06696db798e7412cfc51d75e74b4f87f299bba": "ec9e80073d7ace817d35acb8b7293cbf8e5981b4d2f5708ee5be405122993cd1",
	}
	opts := dotenvx.Options{
		Env:      []string{},
		KeyFiles: []string{}, // Don't read .env.keys.
		KeyProvider: func(publicKey string) (string, error) {
			return keys[publicKey], nil
		},
	}
	values, err := opts.Read("testdata/encrypted/.env")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(values["HELLO"])
	// Output: encrypted
}

//go:embed testdata/encrypted/.env
var encryptedEnv []byte

// An encrypted .env can be compiled into the binary, which then only needs
// the private key at run time.
func ExampleOptions_Parse_embedded() {
	// In production, the private key would be the only dotenvx variable in
	// the environment: here, it is the only entry in Env, and there are no
	// key files.
	opts := dotenvx.Options{
		Env:      []string{"DOTENV_PRIVATE_KEY=ec9e80073d7ace817d35acb8b7293cbf8e5981b4d2f5708ee5be405122993cd1"},
		KeyFiles: []string{},
	}
	values, err := opts.Parse(bytes.NewReader(encryptedEnv))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(values["HELLO"])
	// Output: encrypted
}
