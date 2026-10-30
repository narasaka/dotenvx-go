package dotenvx

import (
	"os"
	"path/filepath"
	"strings"
)

// keyring holds private keys by their public keys, both in hex, in the
// order dotenvx would try them. A public key may have no private key yet.
type keyring struct {
	keys  map[string]string
	order []string
}

func (k *keyring) add(publicKey, privateKey string) {
	if _, ok := k.keys[publicKey]; !ok {
		k.order = append(k.order, publicKey)
	}
	if privateKey != "" || k.keys[publicKey] == "" {
		k.keys[publicKey] = privateKey
	}
}

// addPrivateKey adds privateKey under its public key, if it is valid.
func (k *keyring) addPrivateKey(privateKey string) {
	if publicKey, err := PublicKey(privateKey); err == nil {
		k.add(publicKey, privateKey)
	}
}

// newKeyring builds the keyring dotenvx's config would for a source that
// declares publicKeys. It holds those public keys and the DOTENV_PUBLIC_KEY*
// values of env, and the private keys of env's DOTENV_PRIVATE_KEY*
// variables, each whole. The private keys in keyFiles, where each
// DOTENV_PRIVATE_KEY* value may list several separated by commas, are
// handled two ways:
//
//   - With restricted set, a key file only supplies the private key of a
//     public key still without one, the first that matches, so a file's
//     other keys can't decrypt the source. dotenvx does this for sources
//     that declare public keys.
//   - Otherwise, all of them are added.
//
// Last, the provider is asked about each public key still without a
// private key. Its error is returned.
func newKeyring(publicKeys []string, env environment, keyFiles []string, restricted bool, provider func(string) (string, error)) (*keyring, error) {
	k := &keyring{keys: map[string]string{}}
	for _, pub := range publicKeys {
		k.add(pub, "")
	}
	// fromFiles holds each key-file key by its public key, the first one
	// for each.
	fromFiles := map[string]string{}
	var privateKeys []string
	for _, file := range keyFiles {
		src, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for _, a := range scan(decode(src)) {
			if !strings.HasPrefix(a.key, "DOTENV_PRIVATE_KEY") {
				continue
			}
			for _, privateKey := range strings.Split(a.value, ",") {
				pub, err := PublicKey(privateKey)
				if err != nil {
					continue
				}
				if _, ok := fromFiles[pub]; !ok {
					fromFiles[pub] = privateKey
				}
				if !restricted {
					privateKeys = append(privateKeys, privateKey)
				}
			}
		}
	}
	for _, name := range env.names {
		switch {
		case strings.HasPrefix(name, "DOTENV_PUBLIC_KEY"):
			k.add(env.vars[name], "")
		case strings.HasPrefix(name, "DOTENV_PRIVATE_KEY"):
			privateKeys = append(privateKeys, env.vars[name])
		}
	}
	for _, privateKey := range privateKeys {
		k.addPrivateKey(privateKey)
	}
	for _, pub := range k.order {
		if k.keys[pub] != "" {
			continue
		}
		if privateKey, ok := fromFiles[pub]; ok {
			k.add(pub, privateKey)
			continue
		}
		if provider == nil {
			continue
		}
		privateKey, err := provider(pub)
		if err != nil {
			return nil, err
		}
		k.addPrivateKey(privateKey)
	}
	return k, nil
}

// decrypt decrypts value with the private keys of publicKeys first, then
// with every other key, like dotenvx. It returns value as it is if none of
// them can.
func (k *keyring) decrypt(value string, publicKeys []string) string {
	tried := map[string]bool{}
	for _, keys := range [][]string{publicKeys, k.order} {
		for _, pub := range keys {
			privateKey := k.keys[pub]
			if privateKey == "" || tried[privateKey] {
				continue
			}
			tried[privateKey] = true
			if plaintext, err := Decrypt(privateKey, value); err == nil {
				return plaintext
			}
		}
	}
	return value
}

// publicKeys returns the DOTENV_PUBLIC_KEY* values that assignments
// declare: for each such key, in the order of its first assignment, its
// last value.
func publicKeys(assignments []assignment) []string {
	var names []string
	last := map[string]string{}
	for _, a := range assignments {
		if !strings.HasPrefix(a.key, "DOTENV_PUBLIC_KEY") {
			continue
		}
		if _, ok := last[a.key]; !ok {
			names = append(names, a.key)
		}
		last[a.key] = a.value
	}
	var keys []string
	for _, name := range names {
		keys = append(keys, last[name])
	}
	return keys
}

// keyFiles returns the key files to read for the env file at path, given
// the existing environment, as dotenvx chooses them: the .env.keys next to
// it, or .env.keys in the current directory when the file's own private
// key is set.
func (o Options) keyFilesFor(path string, env environment) []string {
	if o.KeyFiles != nil {
		return o.KeyFiles
	}
	if v, _ := env.get(privateKeyName(path)); v != "" {
		return []string{".env.keys"}
	}
	return []string{filepath.Join(filepath.Dir(path), ".env.keys")}
}

// privateKeyName returns the name dotenvx gives the private key of the env
// file at path: DOTENV_PRIVATE_KEY for .env, DOTENV_PRIVATE_KEY_PRODUCTION
// for .env.production, and so on.
func privateKeyName(path string) string {
	name := strings.ToLower(filepath.Base(path))
	if strings.HasPrefix(name, ".env") && strings.HasSuffix(name, ".txt") {
		name = strings.TrimSuffix(name, ".txt")
	}
	if name == ".env" {
		return "DOTENV_PRIVATE_KEY"
	}
	parts := strings.Split(name, ".")
	var environment string
	switch extra := parts[min(2, len(parts)):]; len(extra) {
	case 0:
		environment = strings.Replace(name, ".env", "development", 1)
	case 1:
		environment = extra[0]
	default:
		environment = extra[0] + "_" + extra[1]
	}
	return "DOTENV_PRIVATE_KEY_" + strings.ToUpper(environment)
}

// guessFile returns the env file that a private key's name implies, the
// reverse of privateKeyName: .env.ci.test for DOTENV_PRIVATE_KEY_CI_TEST.
func guessFile(name string) string {
	if name == "DOTENV_PRIVATE_KEY" {
		return ".env"
	}
	suffix := name[min(len("DOTENV_PRIVATE_KEY_"), len(name)):]
	return ".env." + strings.ToLower(strings.ReplaceAll(suffix, "_", "."))
}
