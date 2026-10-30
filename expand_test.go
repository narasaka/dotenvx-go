package dotenvx

import (
	"errors"
	"fmt"
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
		{name: "simple", src: "A=a\nB=$A", want: map[string]string{"A": "a", "B": "a"}},
		{name: "braced", src: "A=a\nB=${A}", want: map[string]string{"A": "a", "B": "a"}},
		{name: "adjacent", src: "A=a\nB=b\nC=$A$B${A}${B}x", want: map[string]string{"A": "a", "B": "b", "C": "ababx"}},
		{name: "undefined is empty", src: "B=[$A][${A}]", want: map[string]string{"B": "[][]"}},
		{name: "progressive (and $A3 is the name A3)", src: "A=1\nA=${A}2\nA=$A3\nB=1\nB=${B}2\nB=${B}3", want: map[string]string{"A": "", "B": "123"}},
		{name: "self reference", src: "A=$A", want: map[string]string{"A": ""}},
		{name: "later keys are not visible", src: "B=$A\nA=a", want: map[string]string{"A": "a", "B": ""}},
		{name: "default operators when set", src: "A=x\nB=${A:-y}|${A:+z}|${A-w}|${A+v}", want: map[string]string{"A": "x", "B": "x|z|x|v"}},
		{name: "default operators when unset", src: "B=${U:-1}|${U:+2}|${U-3}|${U+4}", want: map[string]string{"B": "1||3|"}},
		{name: "default operators when empty", src: "E=\nB=${E:-y}|${E:+z}|${E-w}|${E+v}", want: map[string]string{"E": "", "B": "y|||v"}},
		{name: "first operator wins", src: "B=${A-b:-c}|${A:-b-c}|${A:-}|${A-}", want: map[string]string{"B": "b:-c|b-c||"}},
		{name: "nested defaults", src: "B=${U:-${V:-${W:-deep}}}", want: map[string]string{"B": "deep"}},
		{name: "braced names are taken literally", src: "A.B=dot\nB=${A.B}|${ A }|${1}", want: map[string]string{"A.B": "dot", "B": "dot||"}},
		{name: "unbraced names stop at other characters", src: "A=a\nB=$A.b|$A-b|$1|$|$$", want: map[string]string{"A": "a", "B": "a.b|a-b|$1|$|$$"}},
		{name: "stray closing brace", src: "A=a\nB=${A}}", want: map[string]string{"A": "a", "B": "a}"}},
		{name: "unterminated brace", src: "A=a\nB=${A", want: map[string]string{"A": "a", "B": "${A"}},
		{name: "escaped dollar", src: "A=a\nB=\\$A|\\${A}", want: map[string]string{"A": "a", "B": "$A|${A}"}},
		{name: "escaped and unescaped reference", src: "A=a\nB=$A|\\${A}", want: map[string]string{"A": "a", "B": "a|${A}"}},
		{name: "escaped backslash still escapes", src: "A=v\nB=\\\\$A", want: map[string]string{"A": "v", "B": "\\$A"}},
		{name: "double quotes expand", src: "A=a\nB=\"x $A\"", want: map[string]string{"A": "a", "B": "x a"}},
		{name: "backticks expand", src: "A=a\nB=`x $A`", want: map[string]string{"A": "a", "B": "x a"}},
		{name: "single quotes do not expand", src: "A=a\nB='x $A \\$A'", want: map[string]string{"A": "a", "B": "x $A \\$A"}},
		{name: "single-quoted reference stays literal", src: "A='$C'\nC=c\nD=${A}", want: map[string]string{"A": "$C", "C": "c", "D": "$C"}},
		{name: "expansion stops after a literal with references", src: "A='$C'\nC=c\nD=x${A}y$C", want: map[string]string{"A": "$C", "C": "c", "D": "x$Cy$C"}},
		{name: "a stale literal still stops expansion", src: "A='$X'\nA=plain\nD=$A$D2", want: map[string]string{"A": "plain", "D": "plain$D2"}},
		{name: "single-quoted value without references", src: "A='a'\nC=c\nD=$A$C", want: map[string]string{"A": "a", "C": "c", "D": "ac"}},
		{name: "environment value is used", src: "B=$A", env: []string{"A=env"}, want: map[string]string{"B": "env"}},
		{name: "environment beats the file", src: "A=file\nB=$A", env: []string{"A=env"}, want: map[string]string{"A": "env", "B": "env"}},
		{name: "overload: the file beats the environment", src: "A=file\nB=$A", env: []string{"A=env"}, overload: true, want: map[string]string{"A": "file", "B": "file"}},
		{name: "existing values are not expanded", src: "A=file", env: []string{"A=$B", "B=b"}, want: map[string]string{"A": "$B"}},
		{name: "existing empty values are expanded", src: "A=$B", env: []string{"A=", "B=b"}, want: map[string]string{"A": ""}},
		{name: "overload expands the file value", src: "A=$B", env: []string{"A=$C", "B=b"}, overload: true, want: map[string]string{"A": "b"}},
		{name: "references inside an environment value expand", src: "A=x$B", env: []string{"B=$C", "C=c"}, want: map[string]string{"A": "xc"}},
		{name: "a value that becomes a variable's value stops", src: "A=$B", env: []string{"B=$C", "C=c"}, want: map[string]string{"A": "$C"}},
		{name: "defaults with special characters", src: "B=${U:-/default/path:with/colon}", want: map[string]string{"B": "/default/path:with/colon"}},
		{name: "default containing a reference", src: "C=c\nB=${U:-$C}", want: map[string]string{"C": "c", "B": "c"}},
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
		// Node never finishes the first one and throws "Cannot convert
		// undefined or null to object" on the second: it looks names up in a
		// plain object, where they find the methods of Object.prototype.
		{name: "names of Object.prototype properties", src: "B=$toString|${constructor}\nC=x${hasOwnProperty:-d}", want: map[string]string{"B": "|", "C": "xd"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

// These expand fully in Node, so a guard against cycles mustn't stop them.
// Every case here was checked against @dotenvx/primitives 4.3.0.
func TestParseExpansionWithoutCycles(t *testing.T) {
	chain := []string{"B101=done"}
	for i := range 101 {
		chain = append(chain, fmt.Sprintf("B%d=$B%d", i, i+1))
	}
	tests := []parseCase{
		{name: "references to a dollar", src: "A=" + strings.Repeat("$B|", 100) + "$B", env: []string{"B=$"}, want: map[string]string{"A": strings.Repeat("$|", 100) + "$"}},
		{name: "defaults of a dollar", src: "A=" + strings.Repeat("${U:-$}|", 100) + "${U:-$}", want: map[string]string{"A": strings.Repeat("$|", 100) + "$"}},
		{name: "references to values with references", src: "A=" + strings.Repeat("${B}|", 100), env: []string{"B=$Z", "Z=z"}, want: map[string]string{"A": strings.Repeat("z|", 100)}},
		{name: "a long chain", src: "A=x$B0", env: chain, want: map[string]string{"A": "xdone"}},
		{name: "a self-reference that resolves to its alternative", src: "B=y${A}", env: []string{"A=${A+z}"}, want: map[string]string{"B": "yz"}},
		{name: "a self-reference that resolves to nothing", src: "B=y${A}${A:-q}", env: []string{"A=${A:+}"}, want: map[string]string{"B": "y"}},
		// References assembled from a substituted value and the text after it.
		{name: "a dollar followed by a name", src: "A=x${D}D", env: []string{"D=$"}, want: map[string]string{"A": "x$"}},
		{name: "text and a dollar followed by a name", src: "A=x${D}D", env: []string{"D=foo$"}, want: map[string]string{"A": "xfoofoo$"}},
		{name: "a dollar followed by a braced name", src: "A=x${D}{D}", env: []string{"D=$"}, want: map[string]string{"A": "x$"}},
		{name: "a dollar followed by a substituted name", src: "A=${L}${M}", env: []string{"L=$", "M=L"}, want: map[string]string{"A": "$"}},
		{name: "text, a dollar and a substituted name", src: "A=x${L}${M}", env: []string{"L=$", "M=L"}, want: map[string]string{"A": "x$"}},
		// dotenvx stops here because the result equals a variable's value.
		{name: "a result equal to a value", src: "A=${L:++${B-}", env: []string{"B=$L}$B", "L="}, want: map[string]string{"A": "$L}$B"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

// Node loops forever on these. The port stops expanding the value where it
// would insert a variable's value into text that came from that value, and
// reports the cycle.
func TestParseExpansionCycle(t *testing.T) {
	tests := []struct {
		name string
		env  []string
		want string
	}{
		{"growing", []string{"B=x$B"}, "yx$B"},
		{"not growing", []string{"B=$B"}, "y$B"},
		{"through another variable", []string{"B=${C}", "C=-$B"}, "y-$B"},
		// Node gives "y" here: it discards the repeated reference along with
		// the alternative that contains it, before it is substituted.
		{"inside an alternative", []string{"B=$M", "M=${L:+${B}}"}, "y${L:+${B}}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Options{Env: tt.env}.Parse(strings.NewReader("A=y$B\nOTHER=$B"))
			if !errors.Is(err, ErrReferenceCycle) {
				t.Errorf("Parse error = %v, want ErrReferenceCycle", err)
			}
			if got["A"] != tt.want {
				t.Errorf("A = %q, want %q", got["A"], tt.want)
			}
		})
	}
}

// Node would eventually produce 2^15 x's here, or run out of memory with a
// few more variables. The port gives up after maxInsertions substitutions.
func TestParseExpansionLimit(t *testing.T) {
	env := []string{"X15=x"}
	for i := range 15 {
		env = append(env, fmt.Sprintf("X%d=$X%d$X%d", i, i+1, i+1))
	}
	_, err := Options{Env: env}.Parse(strings.NewReader("A=y$X0"))
	if !errors.Is(err, ErrExpansionLimit) {
		t.Errorf("Parse error = %v, want ErrExpansionLimit", err)
	}
}

func BenchmarkExpand(b *testing.B) {
	lookup := func(string) (string, bool) { return "1", true }
	for _, bc := range []struct{ name, value string }{
		{"no references", strings.Repeat("x", 1<<20)},
		{"many references", strings.Repeat("$A", 8000)},
		{"many references to dollars", strings.Repeat("${A:-$}", 2000)},
	} {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				expand(bc.value, lookup, nil)
			}
		})
	}
}
