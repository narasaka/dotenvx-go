package dotenvx

import (
	"bytes"
	"maps"
	"testing"
)

// specEnv is the environment the conformance suite was run with.
var specEnv = []string{"MACHINE=machine"}

func TestSpec(t *testing.T) {
	for _, sc := range loadSpec(t) {
		t.Run(sc.name, func(t *testing.T) {
			got, err := Options{Env: specEnv}.Parse(bytes.NewReader(sc.src))
			checkGolden(t, got, err, sc.expected)
		})
	}
}

func TestNodeGoldens(t *testing.T) {
	for _, g := range loadNodeGoldens(t) {
		t.Run(g.name, func(t *testing.T) {
			got, err := Options{Env: specEnv, Overload: g.overload}.Parse(bytes.NewReader(g.src))
			checkGolden(t, got, err, g.expected)
		})
	}
}

func checkGolden(t *testing.T, got map[string]string, err error, want map[string]string) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
