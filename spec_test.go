package dotenvx

import (
	"bytes"
	"maps"
	"slices"
	"testing"
)

// specEnv is the environment the conformance suite was run with.
var specEnv = []string{"MACHINE=machine"}

// specPending lists the conformance scenarios that need behaviour that
// hasn't been ported yet, with the reason. TestSpec fails if one of them
// starts passing, so the list can only shrink.
var specPending = map[string]string{
	"504_EXPAND_PARENTHESES":                       "command substitution",
	"505_EXPAND_RETAIN_INNER_QUOTES":               "command substitution",
	"506_EXPAND_SINGLE_QUOTES_RETAIN_INNER_QUOTES": "command substitution",
	"601_EVAL":    "command substitution",
	"902_UTF16LE": "UTF-16 decoding",
}

// nodePending is specPending for the goldens in testdata/node.
var nodePending = map[string]string{
	"eval":          "command substitution",
	"eval/overload": "command substitution",
}

func TestSpec(t *testing.T) {
	scenarios := loadSpec(t)
	var names []string
	for _, sc := range scenarios {
		names = append(names, sc.name)
	}
	checkPending(t, specPending, names)
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			got, err := Options{Env: specEnv}.Parse(bytes.NewReader(sc.src))
			checkGolden(t, specPending[sc.name], got, err, sc.expected)
		})
	}
}

func TestNodeGoldens(t *testing.T) {
	goldens := loadNodeGoldens(t)
	var names []string
	for _, g := range goldens {
		names = append(names, g.name)
	}
	checkPending(t, nodePending, names)
	for _, g := range goldens {
		t.Run(g.name, func(t *testing.T) {
			got, err := Options{Env: specEnv, Overload: g.overload}.Parse(bytes.NewReader(g.src))
			checkGolden(t, nodePending[g.name], got, err, g.expected)
		})
	}
}

// checkPending fails if a pending entry names no case.
func checkPending(t *testing.T, pending map[string]string, names []string) {
	t.Helper()
	for name := range pending {
		if !slices.Contains(names, name) {
			t.Errorf("pending entry %s names no case", name)
		}
	}
}

// checkGolden compares parsed values with the expected ones. A pending case
// is skipped while it fails and fails once it passes.
func checkGolden(t *testing.T, pending string, got map[string]string, err error, want map[string]string) {
	t.Helper()
	pass := err == nil && maps.Equal(got, want)
	switch {
	case pending != "" && pass:
		t.Fatalf("passes now; remove it from the pending list (%s)", pending)
	case pending != "":
		t.Skipf("pending: %s", pending)
	case err != nil:
		t.Fatal(err)
	case !pass:
		t.Errorf("got %q, want %q", got, want)
	}
}
