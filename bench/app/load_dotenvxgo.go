//go:build dotenvxgo

package main

import (
	"os"

	"github.com/narasaka/dotenvx-go"
)

func loadFile() error {
	if file := os.Getenv("BENCH_ENV_FILE"); file != "" {
		return dotenvx.Load(file)
	}
	return nil
}
