// Package dotenvx loads dotenvx-compatible .env files, including encrypted
// ones, into a Go process.
//
// It is a port of the loading half of dotenvx (https://dotenvx.com), the Go
// equivalent of require('@dotenvx/dotenvx').config(). It reads files in
// process, with no Node and no dotenvx binary. Encrypting and editing files,
// and what dotenvx run does around a child process, stay with the dotenvx CLI.
//
// Load reads .env into the process environment, keeping variables that are
// already set; Overload replaces them; Read and Parse return the values
// without touching the environment. Options configures all four.
//
// Values are parsed as dotenvx parses them, including quotes, multiline
// values, $VAR and ${VAR:-default} expansion and $(command) substitution.
// Values that start with "encrypted:" are decrypted with private keys from
// .env.keys files, DOTENV_PRIVATE_KEY* variables or Options.KeyProvider.
//
// The port follows Node's @dotenvx/primitives, which is what dotenvx runs.
// Where it deliberately differs, because dotenvx relies on an accident of
// JavaScript, the function concerned documents it.
package dotenvx
