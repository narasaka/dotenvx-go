//go:build !dotenvxgo

package main

// loadFile does nothing: without the dotenvxgo tag the server doesn't link
// dotenvx-go, so the other setups measure a binary without it.
func loadFile() error { return nil }
