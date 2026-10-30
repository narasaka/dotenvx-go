package dotenvx

import (
	"regexp"
	"strings"
)

// The dotenv syntax is defined by the regular expression that dotenv and
// dotenvx use in JavaScript. lineRE is that expression translated to Go,
// with JavaScript's \s spelled out as jsSpace: it matches more than Go's \s,
// including Unicode spaces and the byte order mark. Go's regexp picks the
// same match a backtracking engine like JavaScript's would.
//
// One difference remains: in JavaScript, ^ and $ also match around U+2028
// and U+2029, which Go treats as ordinary characters.
const jsSpace = "[\t\n\v\f\r \u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000\ufeff]"

var lineRE = regexp.MustCompile(`(?m)^` + jsSpace + `*(?:export` + jsSpace + `+)?` +
	// The key, then "=" or ": ".
	`([\w.-]+)(?:` + jsSpace + `*=` + jsSpace + `*?|:` + jsSpace + `+?)` +
	// A quoted value, which may span lines, or the rest of the line up to a #.
	`(` + jsSpace + `*'(?:\\'|[^'])*'|` +
	jsSpace + `*"(?:\\"|[^"])*"|` +
	jsSpace + "*`(?:\\\\`|[^`])*`|" +
	`[^#\r\n]+)?` +
	// An optional comment.
	jsSpace + `*(?:#.*)?$`)

var newlines = strings.NewReplacer("\r\n", "\n", "\r", "\n")

var doubleQuoteEscapes = strings.NewReplacer(`\n`, "\n", `\r`, "\r", `\t`, "\t")

// assignment is one KEY=value pair in dotenv source.
type assignment struct {
	key   string
	value string
	// quote is the character the raw value started with if it is one of
	// ', " or `, and 0 otherwise.
	quote byte
}

// scan returns the assignments in src in order.
func scan(src string) []assignment {
	src = newlines.Replace(src)
	var out []assignment
	for _, m := range lineRE.FindAllStringSubmatchIndex(src, -1) {
		raw := ""
		if m[4] >= 0 {
			raw = src[m[4]:m[5]]
		}
		value, quote := clean(raw)
		out = append(out, assignment{key: src[m[2]:m[3]], value: value, quote: quote})
	}
	return out
}

// clean trims a raw value, removes its surrounding quotes and handles the
// escapes its quote style supports. A value that opens with a quote but
// doesn't end with the same one keeps both its text and its quote style.
func clean(raw string) (string, byte) {
	value := strings.TrimFunc(raw, isJSSpace)
	var quote byte
	if value != "" && strings.IndexByte("'\"`", value[0]) >= 0 {
		quote = value[0]
	}
	if quote != 0 && len(value) >= 2 && value[len(value)-1] == quote {
		value = value[1 : len(value)-1]
	}
	switch quote {
	case '\'':
		value = strings.ReplaceAll(value, `\'`, "'")
	case '"':
		value = doubleQuoteEscapes.Replace(value)
	}
	return value, quote
}

// isJSSpace reports whether JavaScript's \s matches r.
func isJSSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', '\u00a0', '\u1680', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000', '\ufeff':
		return true
	}
	return '\u2000' <= r && r <= '\u200a'
}
