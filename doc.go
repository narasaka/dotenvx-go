// Package dotenvx loads dotenvx-compatible .env files, including encrypted
// ones, into a Go process.
//
// It ports the parsing and decryption of dotenvx's config()
// (https://dotenvx.com), reading files in process, with no Node and no
// dotenvx binary. It doesn't redact output, use key stores or apply
// Dotenvxfile policies, as config() can; for redaction, run the program
// with dotenvx run. Encrypting and editing files stay with the dotenvx CLI.
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
// Where it deliberately differs, mostly because dotenvx relies on an accident
// of JavaScript, the function concerned documents it.
package dotenvx
