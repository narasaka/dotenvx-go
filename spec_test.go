package dotenvx

import (
	"bytes"
	"maps"
	"testing"
)

// specEnv is the environment the conformance suite was run with.
var specEnv = []string{"MACHINE=machine"}

// specPending lists the conformance scenarios that need behaviour that
// hasn't been ported yet, with the reason. TestSpec fails if one of them
// starts passing, so the list can only shrink.
var specPending = map[string]string{
	"310_DOUBLE_QUOTES_WITH_NO_SPACE_BRACKET": "expansion",
	"501_EXPAND":                                              "expansion",
	"502_EXPAND_MACHINE":                                      "expansion",
	"503_EXPAND_FILE":                                         "expansion",
	"504_EXPAND_PARENTHESES":                                  "command substitution",
	"505_EXPAND_RETAIN_INNER_QUOTES":                          "command substitution",
	"506_EXPAND_SINGLE_QUOTES_RETAIN_INNER_QUOTES":            "command substitution",
	"507_EXPAND_ESCAPED":                                      "expansion",
	"508_EXPAND_NO_QUOTES_COMBOS":                             "expansion",
	"509_EXPAND_SELF":                                         "expansion",
	"510_EXPAND_URI":                                          "expansion",
	"511_EXPAND_URI_RECURSIVE":                                "expansion",
	"512_EXPAND_URI_NO_CURLY_BRACES":                          "expansion",
	"513_EXPAND_URI_RECURSIVE_NO_CURLY_BRACES":                "expansion",
	"514_EXPAND_DOTS":                                         "expansion",
	"516_EXPAND_PROGRESSIVE":                                  "expansion",
	"517_EXPAND_DEFAULT":                                      "expansion",
	"518_EXPAND_DEFAULT2":                                     "expansion",
	"519_EXPAND_DEFAULT_NESTED":                               "expansion",
	"520_EXPAND_DEFAULT_NESTED2":                              "expansion",
	"521_EXPAND_DEFAULT_NESTED_TWICE":                         "expansion",
	"522_EXPAND_DEFAULT_NESTED_TWICE2":                        "expansion",
	"523_EXPAND_DEFAULT_SPECIAL_CHARACTERS":                   "expansion",
	"524_EXPAND_DEFAULT_SPECIAL_CHARACTERS2":                  "expansion",
	"525_EXPAND_UNDEFINED":                                    "expansion",
	"526_EXPAND_UNDEFINED_NESTED":                             "expansion",
	"527_EXPAND_UNDEFINED_DEFAULT":                            "expansion",
	"528_EXPAND_UNDEFINED_DEFAULT2":                           "expansion",
	"529_EXPAND_UNDEFINED_DEFAULT_NESTED":                     "expansion",
	"530_EXPAND_UNDEFINED_DEFAULT2_NESTED":                    "expansion",
	"531_EXPAND_UNDEFINED_DEFAULT_NESTED_TWICE":               "expansion",
	"532_EXPAND_UNDEFINED_DEFAULT2_NESTED_TWICE":              "expansion",
	"533_EXPAND_UNDEFINED_DEFAULT_SPECIAL_CHARACTERS":         "expansion",
	"534_EXPAND_UNDEFINED_DEFAULT2_SPECIAL_CHARACTERS":        "expansion",
	"535_EXPAND_UNDEFINED_DEFAULT_SPECIAL_CHARACTERS_NESTED":  "expansion",
	"536_EXPAND_UNDEFINED_DEFAULT2_SPECIAL_CHARACTERS_NESTED": "expansion",
	"537_EXPAND_DEEP8":                                        "expansion",
	"538_EXPAND_DEEP_SELF":                                    "expansion",
	"539_EXPAND_DEEP_SELF_PRIOR":                              "expansion",
	"601_EVAL":                                                "command substitution",
	"902_UTF16LE":                                             "UTF-16 decoding",
}

// nodePending is specPending for the goldens in testdata/node.
var nodePending = map[string]string{
	"basic":           "expansion",
	"basic/overload":  "expansion",
	"eval":            "command substitution",
	"eval/overload":   "command substitution",
	"expand":          "expansion",
	"expand/overload": "expansion",
}

func TestSpec(t *testing.T) {
	for _, sc := range loadSpec(t) {
		t.Run(sc.name, func(t *testing.T) {
			got, err := Options{Env: specEnv}.Parse(bytes.NewReader(sc.src))
			checkGolden(t, specPending[sc.name], got, err, sc.expected)
		})
	}
}

func TestNodeGoldens(t *testing.T) {
	for _, g := range loadNodeGoldens(t) {
		t.Run(g.name, func(t *testing.T) {
			got, err := Options{Env: specEnv, Overload: g.overload}.Parse(bytes.NewReader(g.src))
			checkGolden(t, nodePending[g.name], got, err, g.expected)
		})
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
