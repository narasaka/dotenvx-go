package dotenvx

import (
	"strings"
	"testing"
)

// Every case here was checked against @dotenvx/primitives 4.3.0.
func TestParseCommandSubstitution(t *testing.T) {
	tests := []parseCase{
		{name: "command", src: "A=$(echo hi)", env: nil, overload: false, want: map[string]string{"A": "hi"}},
		{name: "double-quoted command with quotes inside", src: "B=x y\nA=\"$(echo \"$B\")\"", env: nil, overload: false, want: map[string]string{"B": "x y", "A": "x y"}},
		{name: "backticks", src: "A=`$(echo hi)`", env: nil, overload: false, want: map[string]string{"A": "hi"}},
		{name: "single quotes are not evaluated", src: "A='$(echo hi)'", env: nil, overload: false, want: map[string]string{"A": "$(echo hi)"}},
		{name: "text around commands", src: "A=<$(echo a)-$(echo b)>", env: nil, overload: false, want: map[string]string{"A": "<a-b>"}},
		{name: "nested command", src: "A=$(echo $(echo hi))", env: nil, overload: false, want: map[string]string{"A": "hi"}},
		{name: "parentheses inside quotes", src: "A=$(echo \")\" '(')", env: nil, overload: false, want: map[string]string{"A": ") ("}},
		{name: "escaped quote inside double quotes", src: "A=$(echo \"a\\\")b\")", env: nil, overload: false, want: map[string]string{"A": "a\")b"}},
		{name: "escaped parenthesis", src: "A=$(echo \\))", env: nil, overload: false, want: map[string]string{"A": ")"}},
		{name: "nested command inside double quotes", src: "A=$(echo \"$(echo ')')\")", env: nil, overload: false, want: map[string]string{"A": ")"}},
		{name: "unterminated command", src: "B=b\nA=$(echo hi $B", env: nil, overload: false, want: map[string]string{"B": "b", "A": "$(echo hi b"}},
		{name: "trailing newlines are trimmed", src: "A=$(printf 'a\\n\\r\\n')|$(printf 'b\\nc')", env: nil, overload: false, want: map[string]string{"A": "a|b\nc"}},
		{name: "failed command keeps the value, which still expands", src: "B=b\nA=$(exit 1)$B", env: nil, overload: false, want: map[string]string{"B": "b", "A": "$(exit 1)b"}},
		{name: "one failed command keeps the whole value", src: "A=$(echo a)$(exit 3)", env: nil, overload: false, want: map[string]string{"A": "$(echo a)$(exit 3)"}},
		{name: "substitution turns off expansion", src: "B=b\nA=$(echo hi)$B", env: nil, overload: false, want: map[string]string{"B": "b", "A": "hi$B"}},
		{name: "commands see parsed values", src: "B=parsed\nA=$(printf %s \"$B\")", env: nil, overload: false, want: map[string]string{"B": "parsed", "A": "parsed"}},
		{name: "commands see the environment", src: "A=$(printf %s \"$E\")", env: []string{"E=env"}, overload: false, want: map[string]string{"A": "env"}},
		{name: "parsed values beat the environment for commands", src: "B=file\nA=$(printf %s \"$B\")", env: []string{"B=env"}, overload: true, want: map[string]string{"B": "file", "A": "file"}},
		{name: "existing values are evaluated", src: "A=file", env: []string{"A=$(echo hi)"}, overload: false, want: map[string]string{"A": "hi"}},
		{name: "overload: an existing key is not evaluated", src: "A=$(echo hi)", env: []string{"A=e"}, overload: true, want: map[string]string{"A": "$(echo hi)"}},
		{name: "overload: an existing key with the same value is evaluated", src: "A=$(echo hi)", env: []string{"A=$(echo hi)"}, overload: true, want: map[string]string{"A": "hi"}},
		{name: "output is not expanded", src: "A=$(printf '%s' '$HOME')", env: []string{"HOME=h"}, overload: false, want: map[string]string{"A": "$HOME"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

// Node substitutes command output with String.prototype.replace, so "$&" in
// the output becomes the command itself: "x$(printf %s \"x$&y\")y". The port
// inserts the output as it is.
func TestParseCommandSubstitutionDeviations(t *testing.T) {
	parseCase{src: `A=$(printf %s "x$&y")`, want: map[string]string{"A": "x$&y"}}.run(t)
}

func TestParseDisableCommandSubstitution(t *testing.T) {
	opts := Options{Env: []string{"B=b"}, DisableCommandSubstitution: true}
	got, err := opts.Parse(strings.NewReader("A=$(echo hi)$B"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "$(echo hi)b"; got["A"] != want {
		t.Errorf("A = %q, want %q", got["A"], want)
	}
}
