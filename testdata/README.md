# testdata

## spec

The dotenvx conformance suite from https://dotenvx.com/spec: 88 parsing
scenarios, each `<scenario>.env` with its expected values in `expected.json`.
They come from `_data/report.json` in the dotenvx.com site repository
(github.com/dotenvx/dotenvx.github.io) and were extracted with
`go run extract.go path/to/report.json`.

The suite was run with `MACHINE=machine` already in the environment, and
existing variables win unless overload is on. That is why `103_MACHINE`
expects `machine` even though the file says `file`.

`902_UTF16LE.env` is the one file that isn't copied verbatim. The report stores
it as text, which turned the UTF-16LE byte order mark into two U+FFFD
characters, so `extract.go` rebuilds the real bytes: `FF FE` followed by
`HELLO=utf16le\n` in UTF-16LE.

The site repository has no license. Clear that with its author before this
repository goes public.

## node

Extra parsing fixtures from the dotenvx CLI test suite
(github.com/dotenvx/dotenvx, commit 5e80bfe, `tests/.env*`, BSD-3-Clause,
Copyright (c) 2024, Scott Motte):

| File | Source |
| --- | --- |
| `basic.env` | `tests/.env` |
| `eval.env` | `tests/.env.eval` |
| `expand.env` | `tests/.env.expand` |
| `export.env` | `tests/.env.export` |
| `multiline.env` | `tests/.env.multiline` |

The `*.json` goldens are generated output, not hand-written expectations. They
are what `@dotenvx/primitives` 4.3.0, the Node implementation of dotenvx,
returns for each file with `MACHINE=machine` as the only existing environment
variable: `NAME.json` without overload, and `NAME.overload.json` with it.
Regenerate them with `node generate.js path/to/node_modules/@dotenvx/primitives`.

## encrypted, multiple

Encrypted fixtures, with their private keys committed on purpose, from the
dotenvx CLI test suite (same commit and license as above):

- `encrypted/` is `tests/monorepo/apps/encrypted`: `HELLO` decrypts to
  `encrypted`.
- `multiple/` is `tests/monorepo/apps/multiple`: `.env.production` decrypts
  to `HELLO=one`, `HELLO2=two` and `HELLO3=three`, the same values as the
  plaintext `.env`.

Both were checked by decrypting them with `@dotenvx/primitives` 4.3.0.
