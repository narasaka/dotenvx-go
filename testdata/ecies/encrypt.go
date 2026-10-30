//go:build ignore

// Encrypt writes go_vectors.json: values this port encrypted, for
// generate.js to check that Node decrypts them.
//
//	go run encrypt.go
package main

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/narasaka/dotenvx-go"
)

func main() {
	const privateKey = "ec9e80073d7ace817d35acb8b7293cbf8e5981b4d2f5708ee5be405122993cd1"
	compressed, err := dotenvx.PublicKey(privateKey)
	check(err)
	b, err := hex.DecodeString(compressed)
	check(err)
	pub, err := secp256k1.ParsePubKey(b)
	check(err)
	type vector struct {
		Plaintext string `json:"plaintext"`
		Encrypted string `json:"encrypted"`
	}
	var vectors []vector
	for _, plaintext := range []string{"", "a", "hello world", "héllo wörld ✓ 𝄞", strings.Repeat("x", 1000), "line1\nline2"} {
		for _, key := range []string{compressed, hex.EncodeToString(pub.SerializeUncompressed())} {
			encrypted, err := dotenvx.Encrypt(key, plaintext)
			check(err)
			vectors = append(vectors, vector{plaintext, encrypted})
		}
	}
	out, err := json.MarshalIndent(map[string]any{"privateKey": privateKey, "publicKey": compressed, "vectors": vectors}, "", "  ")
	check(err)
	check(os.WriteFile("go_vectors.json", append(out, '\n'), 0o644))
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
