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

It ports the parsing and decryption of `require('@dotenvx/dotenvx').config()`,
not `dotenvx run`. It is a community port, as [godotenv](https://github.com/joho/godotenv)
is of dotenv. Use the `dotenvx` CLI to encrypt, set and edit values; this
library only loads them. See [what it doesn't do](#what-it-doesnt-do).

## Install

```sh
go get github.com/narasaka/dotenvx-go
```

It needs Go 1.24 or later. Its one dependency is
[decred's secp256k1](https://pkg.go.dev/github.com/decred/dcrd/dcrec/secp256k1/v4),
because the standard library doesn't have that curve. Unlike the library
dotenvx uses, it isn't written to resist timing attacks, so a process sharing the
machine could learn a private key by watching decryption; see `Decrypt`.

## Loading

```go
dotenvx.Load()                        // .env, keeping variables already set
dotenvx.Load(".env.local", ".env")    // the first file to set a key wins
dotenvx.Overload(".env", ".env.local") // replace variables; the last file wins
values, err := dotenvx.Read(".env")   // the values, without touching the environment
```

With no files, `Load` reads the files named in `DOTENV_FILE`, or else
`DOTENV_PATH`, or else `DOTENV_F`, or the ones the `DOTENV_PRIVATE_KEY*`
variables imply, or else `.env`, as dotenvx does. A
missing file is an error that wraps `fs.ErrNotExist`, so `.env` can be
optional:

```go
if err := dotenvx.Load(".env"); err != nil && !errors.Is(err, fs.ErrNotExist) {
	log.Fatal(err)
}
```

Values are parsed exactly as dotenvx parses them. That covers comments,
`export`, single, double and backtick quotes, multiline values, and UTF-16LE
files with a byte order mark. It also expands `$VAR`, `${VAR}`, `${VAR:-default}` and the other forms,
and substitutes `$(command)` output. The port passes all 88 scenarios of
[dotenvx's conformance suite](https://dotenvx.com/spec).

`dotenvx.Options` holds the settings: the environment to resolve against, overload,
key files, a key provider, and a switch to turn off command substitution.

## Encrypted files

Encrypt a `.env` with the dotenvx CLI:

```sh
dotenvx encrypt
```

That replaces each value with `encrypted:…`, except for keys a
`Dotenvxfile` stores as plaintext and, unless it says otherwise, keys ending
in `_PLAIN`. The first time, it asks where to keep the private key (in CI or
without a terminal it uses `.env.keys` without asking). Choose the
`.env.keys` file and keep it out of version control: this library doesn't
read key stores, so a key kept in one has to be exported and set as
`DOTENV_PRIVATE_KEY`, or fetched through `Options.KeyProvider`. Enclaved
dotenvx Armor keys never leave their remote enclave, so this library can't use
them.

`Load` finds `.env.keys` next to each file, except that, as in dotenvx, it
reads `.env.keys` in the current directory when the file's own key variable
is already set (`DOTENV_PRIVATE_KEY_PRODUCTION` for `.env.production`).
`Options.KeyFiles` names the key files explicitly. In production, set only
the private key:

```sh
DOTENV_PRIVATE_KEY=… ./your-app
```

Private keys come from:

- `.env.keys` files. As in dotenvx, a file that declares its public key
  (`DOTENV_PUBLIC_KEY`) only takes the keys of that public key, and of any
  `DOTENV_PUBLIC_KEY*` variables in the environment, from `.env.keys`. A
  file that declares none takes every key in it.
- `DOTENV_PRIVATE_KEY*` variables, each holding one key. Their names
  follow the files by convention, but each one is tried for every file.
- `Options.KeyProvider`, a hook for key stores. If it fails, the file is
  read with every key in `.env.keys`, and its error is returned too.

A value that can't be decrypted keeps its `encrypted:…` text, and `Load`
reports it in an error that matches `dotenvx.ErrDecryptionFailed`, while
still loading everything else.

### Embedding

Once every secret in it is encrypted, the file is safe to compile into the
binary, so a single binary can carry its configuration and only needs the key
at run time:

```go
package main

import (
	"bytes"
	_ "embed"
	"log"
	"os"

	"github.com/narasaka/dotenvx-go"
)

//go:embed .env
var envFile []byte

func main() {
	values, err := dotenvx.Parse(bytes.NewReader(envFile))
	if err != nil {
		log.Fatal(err)
	}
	for k, v := range values {
		if err := os.Setenv(k, v); err != nil {
			log.Fatalf("setting %s: %v", k, err)
		}
	}
}
```

With `DOTENV_PRIVATE_KEY` set, `Parse` decrypts the values; existing
variables keep their values, as with `Load`.

`Encrypt`, `Decrypt`, `PublicKey` and `GenerateKeypair` are there too, for
tools and tests.

## What it doesn't do

dotenvx's own `config()` does more than load values. It checks the loaded
values against `Dotenvxfile` policies (required keys, types, allowed values
and ranges, failing hard under `strict`), redacts secrets from the program's
output as those policies say, by patching Node's output streams, and it can
get keys from key stores (1Password, Bitwarden, the system keychain, dotenvx
Armor). This port does none of that, and doesn't print dotenvx's inject
summary either.

A Go library can't redact output reliably anyway: writes straight to file
descriptors 1 and 2, cgo and child processes would get past it. That's what
`dotenvx run` is for. It starts your program as a child process, so it sees
all of its output, and it is where dotenvx's upcoming credential brokering
will live too. Encrypting and editing files also stay with the CLI.
`Options.KeyProvider` is the place to plug in a key store.

## Command substitution

As in dotenvx, `KEY=$(command)` runs the command with `/bin/sh` while
loading, and so does an existing value of a key the file sets (single-quoted
values such as `KEY='$(command)'` are never run). That's surprising in a
library, and on Windows or in minimal container images, where there is no
`/bin/sh`, the value is kept as it is. It can be turned off:

```go
dotenvx.Options{DisableCommandSubstitution: true}.Load()
```

## Compatibility

The port follows Node's `@dotenvx/primitives` 4.3.0, which is what dotenvx
runs, including where the Rust implementation differs. It deviates where
dotenvx relies on accidents of JavaScript (how `String.prototype.replace`
treats `$$` in a value, for example), where dotenvx would hang or crash, and
in a few places where Go calls for it: commands always run with `/bin/sh`,
even on Windows, where dotenvx uses `cmd.exe`, paths are used as given,
without `~` expansion, and keys must be exactly 64 hex digits. Each
deviation is documented in the godoc where it applies.

## License

BSD 3-Clause, like dotenvx, whose logic this ports. See [LICENSE](LICENSE).
