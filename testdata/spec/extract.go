//go:build ignore

// Extract rebuilds this directory from the report.json of the dotenvx.com
// site repository (github.com/dotenvx/dotenvx.github.io, _data/report.json):
//
//	go run extract.go path/to/report.json
//
// Each scenario becomes <scenario>.env holding the exact input bytes, and
// expected.json maps each scenario to the values dotenvx is expected to load.
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"unicode/utf16"
)

type report struct {
	Scenarios []struct {
		Scenario string `json:"scenario"`
		Env      string `json:"env"`
		Expected string `json:"expected"`
	} `json:"scenarios"`
}

func main() {
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var r report
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	expected := map[string]map[string]string{}
	for _, s := range r.Scenarios {
		var values map[string]string
		if err := json.Unmarshal([]byte(s.Expected), &values); err != nil {
			return fmt.Errorf("%s: %w", s.Scenario, err)
		}
		expected[s.Scenario] = values
		src := []byte(s.Env)
		if s.Scenario == "902_UTF16LE" {
			// The report stores this file as text, which turned its byte
			// order mark into two U+FFFD characters. Rebuild the real bytes.
			src = utf16le("HELLO=utf16le\n")
		}
		if err := os.WriteFile(s.Scenario+".env", src, 0o644); err != nil {
			return err
		}
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(expected); err != nil {
		return err
	}
	return os.WriteFile("expected.json", out.Bytes(), 0o644)
}

// utf16le encodes s as UTF-16LE with a byte order mark.
func utf16le(s string) []byte {
	out := []byte{0xFF, 0xFE}
	for _, u := range utf16.Encode([]rune(s)) {
		out = binary.LittleEndian.AppendUint16(out, u)
	}
	return out
}
