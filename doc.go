// Package dotenvx loads dotenvx-compatible .env files, including encrypted
// ones, into a Go process.
//
// It is a port of the loading half of dotenvx (https://dotenvx.com): parsing,
// variable expansion, command substitution and decryption. Encrypting,
// editing and running commands stay with the dotenvx CLI.
package dotenvx
