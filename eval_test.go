package dotenvx

import (
	"os"
	"strings"
	"testing"
	"time"
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

// Node substitutes each command's output with String.prototype.replace,
// which treats the output as a replacement pattern and replaces the first
// copy of the command's text, wherever it is. The port inserts each output
// as it is, where the command was.
func TestParseCommandSubstitutionDeviations(t *testing.T) {
	tests := []parseCase{
		// Node gives "x$(printf %s \"x$&y\")y" and "x$y".
		{name: "$&", src: `A=$(printf %s "x$&y")`, want: map[string]string{"A": "x$&y"}},
		{name: "$$", src: `A=$(printf %s 'x$$y')`, want: map[string]string{"A": "x$$y"}},
		// Node gives "b$(echo b)": it replaces the copy of $(echo b) that the
		// first command printed.
		{name: "output containing a later command", src: `A=$(printf %s '$(echo b)')$(echo b)`, want: map[string]string{"A": "$(echo b)b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

// As in Node, a command's output includes what a child it leaves running in
// the background writes, until the child exits.
func TestParseCommandBackgroundChild(t *testing.T) {
	parseCase{src: "A=$( (sleep 0.2; echo late) & echo now)", want: map[string]string{"A": "now\nlate"}}.run(t)
}

// Node's execSync treats more than 1 MiB of output, on stdout and stderr
// together, as a failure, which keeps the value as it is.
func TestParseCommandOutputLimit(t *testing.T) {
	// Keep the stderr of these commands out of the test output.
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	stderr := os.Stderr
	os.Stderr = devNull
	defer func() { os.Stderr = stderr }()

	tests := []struct {
		name, src, want string
	}{
		{"1 MiB", "A=$(printf %01048576d 0)", strings.Repeat("0", 1<<20)},
		{"more", "A=$(printf %01048577d 0)", "$(printf %01048577d 0)"},
		{"1 MiB with stderr", "A=$(printf hi; printf %01048574d 0 >&2)", "hi"},
		{"more with stderr", "A=$(printf hi; printf %01048575d 0 >&2)", "$(printf hi; printf %01048575d 0 >&2)"},
		{"more on stderr alone", "A=$(printf %01048577d 0 >&2)", "$(printf %01048577d 0 >&2)"},
		// Like Node, give up at once, without waiting for a child that
		// still holds the other stream open.
		{"more with a child holding stderr", "A=$(sleep 5 & printf %01048577d 0)", "$(sleep 5 & printf %01048577d 0)"},
		{"more on stderr with a child holding stdout", "A=$(sleep 5 & printf %01048577d 0 >&2)", "$(sleep 5 & printf %01048577d 0 >&2)"},
		{"more from a background child", "A=$( (printf %01048577d 0; sleep 5) & echo now)", "$( (printf %01048577d 0; sleep 5) & echo now)"},
		{"more on stderr from a background child", "A=$( (printf %01048577d 0 >&2; sleep 5) & echo now)", "$( (printf %01048577d 0 >&2; sleep 5) & echo now)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			got, err := Options{Env: []string{}}.Parse(strings.NewReader(tt.src))
			if err != nil || got["A"] != tt.want {
				t.Errorf("A has %d bytes, error %v; want %d bytes", len(got["A"]), err, len(tt.want))
			}
			if elapsed := time.Since(start); elapsed > 2500*time.Millisecond {
				t.Errorf("took %v", elapsed)
			}
		})
	}
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
