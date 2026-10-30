package dotenvx

import (
	"errors"
	"maps"
	"regexp"
	"strings"
	"testing"
	"testing/iotest"
	"unicode/utf8"
)

// Every case here was checked against @dotenvx/primitives 4.3.0, the Node
// implementation of dotenvx, with an empty environment and no key files.
func TestParseSyntax(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want map[string]string
	}{
		{"basic", "A=1", map[string]string{"A": "1"}},
		{"empty", "A=", map[string]string{"A": ""}},
		{"empty with comment", "A= # note", map[string]string{"A": ""}},
		{"export", "export A=1", map[string]string{"A": "1"}},
		{"export with extra spaces", "export   A=1", map[string]string{"A": "1"}},
		{"colon separator", "A: 1", map[string]string{"A": "1"}},
		{"colon without space is skipped", "A:1", map[string]string{}},
		{"spaces around separator", "  A  =  1  ", map[string]string{"A": "1"}},
		{"key split across lines", "A\n=1", map[string]string{"A": "1"}},
		{"key with dots and hyphens", "a.b-c_D=1", map[string]string{"a.b-c_D": "1"}},
		{"non-ascii key is skipped", "K\u00c9Y=1\nOK=2", map[string]string{"OK": "2"}},
		{"line without separator is skipped", "JUSTAKEY\nA=1", map[string]string{"A": "1"}},
		{"comment line", "# A=1\nB=2", map[string]string{"B": "2"}},
		{"inline comment", "A=one # two", map[string]string{"A": "one"}},
		{"hash without space", "A=one#two", map[string]string{"A": "one"}},
		{"equals in value", "A=b=c==", map[string]string{"A": "b=c=="}},
		{"last assignment wins", "A=1\nA=2", map[string]string{"A": "2"}},
		{"single quotes", "A='  x # y  '", map[string]string{"A": "  x # y  "}},
		{"double quotes", "A=\"  x # y  \"", map[string]string{"A": "  x # y  "}},
		{"backticks", "A=`  x # y  `", map[string]string{"A": "  x # y  "}},
		{"quoted value then comment", "A=\"x\" # y", map[string]string{"A": "x"}},
		{"double quotes translate escapes", "A=\"a\\nb\\rc\\td\"", map[string]string{"A": "a\nb\rc\td"}},
		{"double quotes keep other escapes", "A=\"a\\\"b\\\\c\"", map[string]string{"A": "a\\\"b\\\\c"}},
		{"escaped backslash before n", "A=\"a\\\\nb\"", map[string]string{"A": "a\\\nb"}},
		{"single quotes unescape quote", "A='a\\'b'", map[string]string{"A": "a'b"}},
		{"single quotes keep newline escape", "A='a\\nb'", map[string]string{"A": "a\\nb"}},
		{"backticks keep escapes", "A=`a\\nb\\`c`", map[string]string{"A": "a\\nb\\`c"}},
		{"unquoted keeps escapes", "A=a\\nb", map[string]string{"A": "a\\nb"}},
		{"multiline double quotes", "A=\"one\ntwo\"\nB=3", map[string]string{"A": "one\ntwo", "B": "3"}},
		{"multiline single quotes", "A='one\n# two'\nB=3", map[string]string{"A": "one\n# two", "B": "3"}},
		{"multiline backticks", "A=`one\n\"two'`", map[string]string{"A": "one\n\"two'"}},
		{"unclosed double quote", "A=\"abc\\n\nB=1", map[string]string{"A": "\"abc\n", "B": "1"}},
		{"unclosed single quote", "A='abc\nB=1", map[string]string{"A": "'abc", "B": "1"}},
		{"text after closing quote", "A=\"abc\" def", map[string]string{"A": "\"abc\" def"}},
		{"inner quote of the same kind", "A='abc'x'", map[string]string{"A": "abc'x"}},
		{"inner quotes are kept", "A={\"foo\": \"bar\"}", map[string]string{"A": "{\"foo\": \"bar\"}"}},
		{"crlf", "A=1\r\nB=\"x\r\ny\"\r\n", map[string]string{"A": "1", "B": "x\ny"}},
		{"lone cr", "A=1\rB=2", map[string]string{"A": "1", "B": "2"}},
		{"blank lines", "\n\n  \nA=1\n\n", map[string]string{"A": "1"}},
		{"utf-8 bom", "\ufeffA=1", map[string]string{"A": "1"}},
		{"unicode whitespace is trimmed", "A=\u00a0x\u3000", map[string]string{"A": "x"}},
		{"vertical tab is whitespace", "A=\u000bx\u000b", map[string]string{"A": "x"}},
		{"next line is not trimmed", "A=\u0085x\u0085", map[string]string{"A": "\u0085x\u0085"}},
		{"tab separated", "A\t=\tx", map[string]string{"A": "x"}},
		// JavaScript also ends lines at U+2028 and U+2029.
		{"LS ends a quoted value", "A=\"x\"\u2028B=2", map[string]string{"A": "x", "B": "2"}},
		{"PS ends a quoted value", "A='x'\u2029B=2", map[string]string{"A": "x", "B": "2"}},
		{"LS ends a comment", "# c\u2028A=1", map[string]string{"A": "1"}},
		{"PS ends an inline comment", "A=x # note\u2029B=2", map[string]string{"A": "x", "B": "2"}},
		{"LS stays inside an unquoted value", "A=one\u2028B=two", map[string]string{"A": "one\u2028B=two"}},
		{"quotes after LS are removed", "A=x\u2028\"y\"", map[string]string{"A": "x\u2028y"}},
		{"quotes after PS are removed", "A=x\u2029'y'", map[string]string{"A": "x\u2029y"}},
		{"several quoted segments", "A=x\u2028\"y\"\u2028`z`", map[string]string{"A": "x\u2028y\u2028z"}},
		{"quoted segments with text between", "A=x\u2028\"y\" w\u2029'z'", map[string]string{"A": "x\u2028\"y\" w\u2029z"}},
		{"greedy to the last closing quote", "A=x\u2028'a'\u2028'b'", map[string]string{"A": "x\u2028a'\u2028'b"}},
		{"quote style comes from the first character", "A='x\u2028\"y\\'\"", map[string]string{"A": "'x\u2028y'"}},
		{"unquoted values run past separators", "\u2028A=1\u2029\u2029B=2", map[string]string{"A": "1\u2029\u2029B=2"}},
		{"key on a line after LS", "A=\"1\" \u2028 export B=2", map[string]string{"A": "1", "B": "2"}},
		{"line start after a separator", "\u2028A=\"1\"\u2029\u2029B=2", map[string]string{"A": "1", "B": "2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Options{Env: []string{}}.Parse(strings.NewReader(tt.src))
			if err != nil {
				t.Fatal(err)
			}
			if !maps.Equal(got, tt.want) {
				t.Errorf("Parse(%q) = %q, want %q", tt.src, got, tt.want)
			}
		})
	}
}

// Node only ever sees decoded text, in which invalid UTF-8 has already become
// U+FFFD. The port passes the bytes through, the way a shell would.
func TestParseInvalidUTF8(t *testing.T) {
	got, err := Options{Env: []string{}}.Parse(strings.NewReader("A=\xff\nB=\"\xfe\""))
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"A": "\xff", "B": "\xfe"}; !maps.Equal(got, want) {
		t.Errorf("Parse = %q, want %q", got, want)
	}
}

// Node throws "u[p].push is not a function" for keys that name a property
// of Object.prototype. The port parses them like any other key.
func TestParseObjectPrototypeKeys(t *testing.T) {
	got, err := Options{Env: []string{}}.Parse(strings.NewReader("constructor=1\n__proto__=2\ntoString=3\nhasOwnProperty=4\nvalueOf=5"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"constructor": "1", "__proto__": "2", "toString": "3", "hasOwnProperty": "4", "valueOf": "5"}
	if !maps.Equal(got, want) {
		t.Errorf("Parse = %q, want %q", got, want)
	}
}

func TestParseExistingEnv(t *testing.T) {
	tests := []struct {
		name     string
		env      []string
		overload bool
		want     string
	}{
		{"missing", nil, false, "file"},
		{"existing wins", []string{"A=env"}, false, "env"},
		{"existing empty value wins", []string{"A="}, false, ""},
		{"overload", []string{"A=env"}, true, "file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := append([]string{}, tt.env...)
			got, err := Options{Env: env, Overload: tt.overload}.Parse(strings.NewReader("A=file"))
			if err != nil {
				t.Fatal(err)
			}
			if got["A"] != tt.want {
				t.Errorf("A = %q, want %q", got["A"], tt.want)
			}
		})
	}
}

func TestParseProcessEnv(t *testing.T) {
	t.Setenv("DOTENVX_GO_TEST", "process")
	got, err := Parse(strings.NewReader("DOTENVX_GO_TEST=file"))
	if err != nil {
		t.Fatal(err)
	}
	if got["DOTENVX_GO_TEST"] != "process" {
		t.Errorf("DOTENVX_GO_TEST = %q, want the process environment's value", got["DOTENVX_GO_TEST"])
	}
}

func TestParseReadError(t *testing.T) {
	readErr := iotest.ErrReader(iotest.ErrTimeout)
	if _, err := Parse(readErr); err == nil || !errors.Is(err, iotest.ErrTimeout) {
		t.Errorf("Parse error = %v, want it to wrap %v", err, iotest.ErrTimeout)
	}
}

func TestIsJSSpace(t *testing.T) {
	// The code points JavaScript's \s matches (and String.prototype.trim
	// removes), listed by running /\s/ over the BMP in Node.
	want := map[rune]bool{}
	for _, r := range []rune{0x9, 0xa, 0xb, 0xc, 0xd, 0x20, 0xa0, 0x1680, 0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007, 0x2008, 0x2009, 0x200a, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff} {
		want[r] = true
	}
	class := regexp.MustCompile("^" + jsSpace + "$")
	for r := rune(0); r <= 0x10ffff; r++ {
		if got := isJSSpace(r); got != want[r] {
			t.Errorf("isJSSpace(%U) = %v", r, got)
		}
		if got := class.MatchString(string(r)); got != want[r] && utf8.ValidRune(r) {
			t.Errorf("jsSpace matches %U = %v", r, got)
		}
	}
}

// The pathological cases took quadratic time in earlier versions of scan.
func BenchmarkScan(b *testing.B) {
	for _, bc := range []struct{ name, src string }{
		{"typical", strings.Repeat("KEY=value # comment\nQUOTED=\"a b\nc\"\n\n", 1000)},
		{"blank lines", strings.Repeat("\n", 32000)},
		{"unmatched quotes after separators", "A=x" + strings.Repeat("\u2028\"z", 16000)},
	} {
		b.Run(bc.name, func(b *testing.B) {
			for b.Loop() {
				scan(bc.src)
			}
		})
	}
}
