package dotenvx

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
)

// Options configures how dotenv source is resolved. The zero value is ready
// to use and matches the defaults of dotenvx.
type Options struct {
	// Env is the existing environment that Parse and Read resolve against,
	// in the "KEY=value" form of os.Environ. A key that already exists in
	// Env keeps its existing value unless Overload is set. If Env is nil,
	// they use the current process environment; use an empty slice for no
	// existing environment at all. Load ignores Env: it always resolves
	// against the process environment it sets values in.
	Env []string

	// Overload lets values from the source replace values that already
	// exist in Env.
	Overload bool

	// DisableCommandSubstitution turns off $(command) substitution. By
	// default, as in dotenvx, each $(command) in a value that isn't
	// single-quoted is replaced with the output of running the command with
	// /bin/sh. That includes the existing value of a key the source
	// assigns, unless Overload is set. The command sees Env plus the values
	// parsed before it. When a command fails, writes more than 1 MiB to
	// stdout and stderr together, or can't run because there is no /bin/sh
	// (as on Windows or in minimal container images), the value is kept as
	// it is. What a command writes to stderr is written to os.Stderr after
	// it finishes, as dotenvx does.
	DisableCommandSubstitution bool

	// KeyFiles are the files, usually named .env.keys, to read the private
	// keys of encrypted values from: the values of their DOTENV_PRIVATE_KEY*
	// keys, each of which may list several keys separated by commas. Files
	// that can't be read are skipped. If KeyFiles is nil, Parse reads
	// .env.keys in the current directory, and Read and Load the .env.keys
	// next to each env file, as dotenvx does; but when that file's own
	// private key is set in the environment (DOTENV_PRIVATE_KEY_PRODUCTION
	// for .env.production), dotenvx reads .env.keys in the current
	// directory instead, and so do they. Use an empty slice to read none.
	//
	// Like dotenvx's config, a source that declares its public keys, in
	// DOTENV_PUBLIC_KEY* values, only takes the private keys of those (and
	// of the environment's DOTENV_PUBLIC_KEY* variables) from the key
	// files, so a value encrypted for another key in .env.keys stays
	// encrypted. A source that declares none takes every key.
	//
	// Private keys also come from the DOTENV_PRIVATE_KEY* variables of the
	// existing environment, each holding a single key, and always count.
	KeyFiles []string

	// KeyProvider, if set, is asked for the private key of each public key
	// that has none from KeyFiles or the environment: those the source
	// declares in its DOTENV_PUBLIC_KEY* values, and those of the
	// environment's DOTENV_PUBLIC_KEY* variables. It returns the private key
	// in hex, or "" if it has none; an invalid key is ignored. It is where
	// key stores plug in. If it returns an error, the source is parsed
	// without it, with every key in the key files, as dotenvx does, and
	// the error is returned with the values.
	KeyProvider func(publicKey string) (privateKey string, err error)
}

// Parse reads dotenv source from r and returns its values. It doesn't modify
// the process environment. When a key is assigned more than once, the last
// assignment wins.
//
// Values that start with "encrypted:" are decrypted first, with the keys
// that KeyFiles, the environment and KeyProvider supply. A value that can't
// be decrypted stays encrypted, and isn't evaluated or expanded; Parse
// returns the values together with an error that matches
// ErrDecryptionFailed and names the keys it couldn't decrypt.
//
// Values that aren't single-quoted then have each $(command) replaced
// with the command's output, unless DisableCommandSubstitution is set.
// Each output is inserted as it is, where the command was. dotenvx instead
// treats it as a String.prototype.replace pattern ($$, $&, $` and $') and
// replaces the first copy of the command's text, which may be in an
// earlier command's output; it also turns invalid UTF-8 into U+FFFD, and
// runs commands with cmd.exe on Windows rather than /bin/sh.
//
// Values that aren't single-quoted, and had no command substituted, then
// have their $NAME and ${NAME} references expanded, as do
// ${NAME:-default}, ${NAME-default}, ${NAME:+alternative} and
// ${NAME+alternative}, the way dotenvx expands them. Names resolve to
// existing values first, then to values parsed earlier, or the other way
// round with Overload. A backslash keeps a "$" from starting a reference.
// Existing values that aren't empty aren't expanded, though they do go
// through command substitution.
//
// Expansion differs from dotenvx where dotenvx relies on accidents of
// JavaScript: a value is inserted as it is, even if it contains "$$" or
// "$&"; the reference that matched is the one replaced, even if an escaped
// copy of it comes first; and names containing "null", or naming properties
// of Object.prototype, resolve like any other. Where dotenvx would loop
// forever, because a variable refers back to itself, Parse leaves the
// rest of that value unexpanded and returns its values with an error that
// wraps ErrReferenceCycle. It does that as soon as a reference that came
// entirely from a variable's value would insert that value again, which
// also stops a few contrived inputs that dotenvx finishes. A value that
// would take more than 10,000 substitutions of values containing "$", or
// grow past 16 MiB, stops the same way, with ErrExpansionLimit.
//
// Source that starts with a UTF-16LE byte order mark is decoded from
// UTF-16LE; anything else is read as UTF-8. Bytes that aren't valid UTF-8
// are kept as they are, where dotenvx, which only sees decoded text, would
// have replaced them with U+FFFD. Keys such as constructor or __proto__,
// which make dotenvx throw, are parsed like any other key.
func (o Options) Parse(r io.Reader) (map[string]string, error) {
	src, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("dotenvx: %w", err)
	}
	keyFiles := o.KeyFiles
	if keyFiles == nil {
		keyFiles = []string{".env.keys"}
	}
	values, _, err := o.parse(decode(src), o.environment(), keyFiles, "dotenvx: ")
	return values, err
}

// environment returns Env, or the process environment if Env is nil.
func (o Options) environment() environment {
	if o.Env == nil {
		return newEnvironment(os.Environ(), runtime.GOOS == "windows")
	}
	return newEnvironment(o.Env, false)
}

// parse resolves the values in src against the existing environment,
// decrypting with the keys of keyFiles, Env and KeyProvider. It also
// returns the keys in the order they were first assigned, which is the
// order dotenvx sets them in. Its errors start with prefix.
func (o Options) parse(src string, existing environment, keyFiles []string, prefix string) (map[string]string, []string, error) {
	assignments := scan(src)
	declared := publicKeys(assignments)
	var errs []error
	ring, err := newKeyring(declared, existing, keyFiles, len(declared) > 0, o.KeyProvider)
	if err != nil {
		// Like dotenvx, report the provider's error and make do without
		// it, with every key in the key files. Without a provider, building
		// the keyring can't fail.
		errs = append(errs, fmt.Errorf("%s%w", prefix, err))
		ring, _ = newKeyring(declared, existing, keyFiles, false, nil)
	}
	parsed := map[string]string{}
	// literals holds the values of single-quoted assignments.
	literals := map[string]string{}
	// lookup resolves a reference: existing values win, unless overloading.
	lookup := func(name string) (string, bool) {
		first, second := existing.vars, parsed
		if o.Overload {
			first, second = parsed, existing.vars
		}
		if v, ok := first[name]; ok {
			return v, true
		}
		v, ok := second[name]
		return v, ok
	}
	var keys, undecrypted []string
	failed := map[string]bool{}
	for _, a := range assignments {
		value := a.value
		current, exists := existing.get(a.key)
		if exists && !o.Overload {
			value = current
		}
		if isEncrypted(value) {
			value = ring.decrypt(value, declared)
			if isEncrypted(value) && !failed[a.key] {
				failed[a.key] = true
				undecrypted = append(undecrypted, a.key)
			}
		}
		encrypted := isEncrypted(value)
		// Like dotenvx, evaluate a value from the environment only if the
		// source didn't replace it.
		evaluated := false
		if !encrypted && a.quote != '\'' && !o.DisableCommandSubstitution &&
			strings.Contains(value, "$(") && (!exists || current == value) {
			next := evaluate(value, commandEnv(existing.vars, parsed))
			evaluated = next != value
			value = next
		}
		// Existing values are taken as they are, unless they are empty.
		if !encrypted && a.quote != '\'' && !evaluated && (current == "" || o.Overload) {
			expanded, err := expand(value, lookup, literals)
			if err != nil {
				errs = append(errs, fmt.Errorf("%sexpanding %s: %w", prefix, a.key, err))
			}
			value = strings.ReplaceAll(expanded, `\$`, "$")
		}
		if a.quote == '\'' {
			literals[a.key] = value
		}
		if _, ok := parsed[a.key]; !ok {
			keys = append(keys, a.key)
		}
		parsed[a.key] = value
	}
	if len(undecrypted) > 0 {
		errs = append(errs, fmt.Errorf("%s%w", prefix, &Error{Code: ErrDecryptionFailed.Code, Message: "could not decrypt " + strings.Join(undecrypted, ", ")}))
	}
	return parsed, keys, errors.Join(errs...)
}

// Parse reads dotenv source from r with the default Options.
func Parse(r io.Reader) (map[string]string, error) {
	return Options{}.Parse(r)
}
