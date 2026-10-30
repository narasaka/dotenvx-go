package dotenvx

import (
	"maps"
	"strings"
	"testing"
)

type parseCase struct {
	name     string
	src      string
	env      []string
	overload bool
	want     map[string]string
}

func (tt parseCase) run(t *testing.T) {
	t.Helper()
	got, err := Options{Env: append([]string{}, tt.env...), Overload: tt.overload}.Parse(strings.NewReader(tt.src))
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(got, tt.want) {
		t.Errorf("Parse(%q) = %q, want %q", tt.src, got, tt.want)
	}
}

// Every case here was checked against @dotenvx/primitives 4.3.0.
func TestParseExpansion(t *testing.T) {
	tests := []parseCase{
		{name: "simple", src: "A=a\nB=$A", env: nil, overload: false, want: map[string]string{"A": "a", "B": "a"}},
		{name: "braced", src: "A=a\nB=${A}", env: nil, overload: false, want: map[string]string{"A": "a", "B": "a"}},
		{name: "adjacent", src: "A=a\nB=b\nC=$A$B${A}${B}x", env: nil, overload: false, want: map[string]string{"A": "a", "B": "b", "C": "ababx"}},
		{name: "undefined is empty", src: "B=[$A][${A}]", env: nil, overload: false, want: map[string]string{"B": "[][]"}},
		{name: "progressive (and $A3 is the name A3)", src: "A=1\nA=${A}2\nA=$A3\nB=1\nB=${B}2\nB=${B}3", env: nil, overload: false, want: map[string]string{"A": "", "B": "123"}},
		{name: "self reference", src: "A=$A", env: nil, overload: false, want: map[string]string{"A": ""}},
		{name: "later keys are not visible", src: "B=$A\nA=a", env: nil, overload: false, want: map[string]string{"A": "a", "B": ""}},
		{name: "default operators when set", src: "A=x\nB=${A:-y}|${A:+z}|${A-w}|${A+v}", env: nil, overload: false, want: map[string]string{"A": "x", "B": "x|z|x|v"}},
		{name: "default operators when unset", src: "B=${U:-1}|${U:+2}|${U-3}|${U+4}", env: nil, overload: false, want: map[string]string{"B": "1||3|"}},
		{name: "default operators when empty", src: "E=\nB=${E:-y}|${E:+z}|${E-w}|${E+v}", env: nil, overload: false, want: map[string]string{"E": "", "B": "y|||v"}},
		{name: "first operator wins", src: "B=${A-b:-c}|${A:-b-c}|${A:-}|${A-}", env: nil, overload: false, want: map[string]string{"B": "b:-c|b-c||"}},
		{name: "nested defaults", src: "B=${U:-${V:-${W:-deep}}}", env: nil, overload: false, want: map[string]string{"B": "deep"}},
		{name: "braced names are taken literally", src: "A.B=dot\nB=${A.B}|${ A }|${1}", env: nil, overload: false, want: map[string]string{"A.B": "dot", "B": "dot||"}},
		{name: "unbraced names stop at other characters", src: "A=a\nB=$A.b|$A-b|$1|$|$$", env: nil, overload: false, want: map[string]string{"A": "a", "B": "a.b|a-b|$1|$|$$"}},
		{name: "stray closing brace", src: "A=a\nB=${A}}", env: nil, overload: false, want: map[string]string{"A": "a", "B": "a}"}},
		{name: "unterminated brace", src: "A=a\nB=${A", env: nil, overload: false, want: map[string]string{"A": "a", "B": "${A"}},
		{name: "escaped dollar", src: "A=a\nB=\\$A|\\${A}", env: nil, overload: false, want: map[string]string{"A": "a", "B": "$A|${A}"}},
		{name: "escaped and unescaped reference", src: "A=a\nB=$A|\\${A}", env: nil, overload: false, want: map[string]string{"A": "a", "B": "a|${A}"}},
		{name: "escaped backslash still escapes", src: "A=v\nB=\\\\$A", env: nil, overload: false, want: map[string]string{"A": "v", "B": "\\$A"}},
		{name: "double quotes expand", src: "A=a\nB=\"x $A\"", env: nil, overload: false, want: map[string]string{"A": "a", "B": "x a"}},
		{name: "backticks expand", src: "A=a\nB=`x $A`", env: nil, overload: false, want: map[string]string{"A": "a", "B": "x a"}},
		{name: "single quotes do not expand", src: "A=a\nB='x $A \\$A'", env: nil, overload: false, want: map[string]string{"A": "a", "B": "x $A \\$A"}},
		{name: "single-quoted reference stays literal", src: "A='$C'\nC=c\nD=${A}", env: nil, overload: false, want: map[string]string{"A": "$C", "C": "c", "D": "$C"}},
		{name: "expansion stops after a literal with references", src: "A='$C'\nC=c\nD=x${A}y$C", env: nil, overload: false, want: map[string]string{"A": "$C", "C": "c", "D": "x$Cy$C"}},
		{name: "a stale literal still stops expansion", src: "A='$X'\nA=plain\nD=$A$D2", env: nil, overload: false, want: map[string]string{"A": "plain", "D": "plain$D2"}},
		{name: "single-quoted value without references", src: "A='a'\nC=c\nD=$A$C", env: nil, overload: false, want: map[string]string{"A": "a", "C": "c", "D": "ac"}},
		{name: "environment value is used", src: "B=$A", env: []string{"A=env"}, overload: false, want: map[string]string{"B": "env"}},
		{name: "environment beats the file", src: "A=file\nB=$A", env: []string{"A=env"}, overload: false, want: map[string]string{"A": "env", "B": "env"}},
		{name: "overload: the file beats the environment", src: "A=file\nB=$A", env: []string{"A=env"}, overload: true, want: map[string]string{"A": "file", "B": "file"}},
		{name: "existing values are not expanded", src: "A=file", env: []string{"A=$B", "B=b"}, overload: false, want: map[string]string{"A": "$B"}},
		{name: "existing empty values are expanded", src: "A=$B", env: []string{"A=", "B=b"}, overload: false, want: map[string]string{"A": ""}},
		{name: "overload expands the file value", src: "A=$B", env: []string{"A=$C", "B=b"}, overload: true, want: map[string]string{"A": "b"}},
		{name: "references inside an environment value expand", src: "A=x$B", env: []string{"B=$C", "C=c"}, overload: false, want: map[string]string{"A": "xc"}},
		{name: "a value that becomes a variable's value stops", src: "A=$B", env: []string{"B=$C", "C=c"}, overload: false, want: map[string]string{"A": "$C"}},
		{name: "defaults with special characters", src: "B=${U:-/default/path:with/colon}", env: nil, overload: false, want: map[string]string{"B": "/default/path:with/colon"}},
		{name: "default containing a reference", src: "C=c\nB=${U:-$C}", env: nil, overload: false, want: map[string]string{"C": "c", "B": "c"}},
		{name: "many references", src: "A=1\nB=" + strings.Repeat("$A", 150), want: map[string]string{"A": "1", "B": strings.Repeat("1", 150)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

// Node expands with String.prototype.replace and String.prototype.split,
// and these cases hit their JavaScript-specific behaviour. The port does
// what the Rust implementation of dotenvx does instead: it replaces the
// reference it matched with the referenced value, as is.
func TestParseExpansionDeviations(t *testing.T) {
	tests := []parseCase{
		// Node gives "xpa$wordy": "$$" in a replacement string means "$".
		{name: "dollars in a value", src: "P='pa$$word'\nURL=x${P}y", want: map[string]string{"P": "pa$$word", "URL": "xpa$$wordy"}},
		// Node gives "\\v v" and "\\v|v": it replaces the first occurrence
		// of the reference's text, which is the escaped one.
		{name: "escaped copy of a braced reference", src: "A=v\nB=\\${A} ${A}", want: map[string]string{"A": "v", "B": "${A} v"}},
		{name: "escaped copy of a reference", src: "A=v\nB=\\$A|$A", want: map[string]string{"A": "v", "B": "$A|v"}},
		// Node gives "|": without an operator it splits the name on the
		// string "null" and looks up what comes before it.
		{name: "names containing null", src: "B=${nullable}|$nullable", env: []string{"nullable=n"}, want: map[string]string{"B": "n|n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

// Node loops forever on this one.
func TestParseExpansionCycle(t *testing.T) {
	got, err := Options{Env: []string{"B=x$B"}}.Parse(strings.NewReader("A=y$B"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "y" + strings.Repeat("x", maxReexpansions) + "$B"; got["A"] != want {
		t.Errorf("A = %q, want %q", got["A"], want)
	}
}
