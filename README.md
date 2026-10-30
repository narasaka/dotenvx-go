# dotenvx-go

A pure-Go port of the [dotenvx](https://dotenvx.com) loading library. It reads
`.env` files, including encrypted ones, straight into your Go process, with no
Node and no `dotenvx` binary.

```go
import "github.com/narasaka/dotenvx-go"

func main() {
	if err := dotenvx.Load(); err != nil {
		log.Fatal(err)
	}
	// os.Getenv("HELLO") now works, decrypted if it was encrypted.
}
```

It's the Go equivalent of `require('@dotenvx/dotenvx').config()`, not of
`dotenvx run`. It is a community port, as [godotenv](https://github.com/joho/godotenv)
is of dotenv. Use the `dotenvx` CLI to encrypt, set and edit values; this
library only loads them.

## Install

```sh
go get github.com/narasaka/dotenvx-go
```

It needs Go 1.24 or later. Its one dependency is
[decred's secp256k1](https://pkg.go.dev/github.com/decred/dcrd/dcrec/secp256k1/v4),
because the standard library doesn't have that curve.

## Loading

```go
dotenvx.Load()                        // .env, keeping variables already set
dotenvx.Load(".env.local", ".env")    // the first file to set a key wins
dotenvx.Overload(".env", ".env.local") // replace variables; the last file wins
values, err := dotenvx.Read(".env")   // the values, without touching the environment
```

With no files, `Load` reads the files named in `DOTENV_FILE`, or the ones the
`DOTENV_PRIVATE_KEY*` variables imply, or else `.env`, as dotenvx does. A
missing file is an error that wraps `fs.ErrNotExist`, so `.env` can be
optional:

```go
if err := dotenvx.Load(".env"); err != nil && !errors.Is(err, fs.ErrNotExist) {
	log.Fatal(err)
}
```

Values are parsed exactly as dotenvx parses them. That covers comments,
`export`, single, double and backtick quotes, multiline values, and UTF-16
files. It also expands `$VAR`, `${VAR}`, `${VAR:-default}` and the other forms,
and substitutes `$(command)` output. The port passes all 88 scenarios of
[dotenvx's conformance suite](https://dotenvx.com/spec).

`dotenvx.Options` holds the settings: the environment to resolve against, overload,
key files, a key provider, and a switch to turn off command substitution.

## Encrypted files

Encrypt a `.env` with the dotenvx CLI:

```sh
dotenvx encrypt
```

That replaces each value with `encrypted:…` and writes the private key to
`.env.keys`, which you keep out of version control. `Load` finds `.env.keys`
next to each file. In production, set only the private key:

```sh
DOTENV_PRIVATE_KEY=… ./your-app
```

Private keys come from `.env.keys` files, from `DOTENV_PRIVATE_KEY*`
variables (`DOTENV_PRIVATE_KEY_PRODUCTION` for `.env.production`), and from
`Options.KeyProvider`, a hook for key stores. As in dotenvx, a file that
declares its public key only takes the matching key from `.env.keys`, so a
value encrypted for another environment's key stays encrypted. A value that
can't be decrypted stays encrypted, and `Load` reports it in an error that
matches `dotenvx.ErrDecryptionFailed`, while still loading everything else.

### Embedding

The encrypted file is safe to compile into the binary, so a single binary can
carry its configuration and only needs the key at run time:

```go
//go:embed .env
var envFile []byte

func main() {
	values, err := dotenvx.Parse(bytes.NewReader(envFile))
	if err != nil {
		log.Fatal(err)
	}
	for k, v := range values {
		os.Setenv(k, v)
	}
}
```

With `DOTENV_PRIVATE_KEY` set, `Parse` decrypts the values; existing
variables keep their values, as with `Load`.

`Encrypt`, `Decrypt`, `PublicKey` and `GenerateKeypair` are there too, for
tools and tests.

## What it doesn't do

`dotenvx run` starts your program as a child process, which lets it do things
an in-process library can't, such as redacting secrets from the program's
output, and more coming. This library loads values into your own process and
does none of that. It also leaves out the CLI's other features: encrypting
and editing files, key stores like 1Password, Bitwarden, the system keychain
and dotenvx Armor, `Dotenvxfile` policies, and the inject summary.

## Command substitution

As in dotenvx, `KEY=$(command)` runs the command with `/bin/sh` while loading.
That's surprising in a library, and there is no `/bin/sh` on Windows or in
minimal container images (where the value is kept as it is), so it can be
turned off:

```go
dotenvx.Options{DisableCommandSubstitution: true}.Load()
```

## Compatibility

The port follows Node's `@dotenvx/primitives` 4.3.0, which is what dotenvx
runs, including where the Rust implementation differs. It deviates only where
dotenvx relies on accidents of JavaScript, for example how `String.prototype.replace`
treats `$$` in a value, or it would hang or crash. Each deviation is
documented on the function concerned and pinned by a test.

## License

BSD 3-Clause, like dotenvx, whose logic this ports. See [LICENSE](LICENSE).
