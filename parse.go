package dotenvx

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// The dotenv syntax is defined by the regular expression that dotenv and
// dotenvx use in JavaScript. lineRE is that expression translated to Go.
// Go's regexp picks the same match a backtracking engine like JavaScript's
// would, given the same expression, so the translation only has to spell
// out what JavaScript means by a few constructs:
//
//   - \s matches more than Go's \s, including Unicode spaces and the byte
//     order mark. jsSpace lists exactly what it matches.
//   - In multiline mode, lines end at U+2028 and U+2029 as well as at \n
//     (scan removes \r before lineRE runs). Go's ^ and $ only know \n, so
//     scan tries lineRE at each line start itself, and lineRE ends by
//     matching the end of the input or the character that ends the line,
//     which scan then leaves out of the match. That is equivalent to
//     JavaScript's $, because nothing in the expression comes after it.
//   - . doesn't match the characters that end lines.
const (
	jsSpace      = "[\t\n\v\f\r \u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000\ufeff]"
	lineEnd      = "\n\u2028\u2029"
	notLineEnd   = "[^\r" + lineEnd + "]"
	lineEndOrEOF = `(\z|[` + lineEnd + `])`
)

var lineRE = regexp.MustCompile(`\A` + jsSpace + `*(?:export` + jsSpace + `+)?` +
	// The key, then "=" or ": ".
	`([\w.-]+)(?:` + jsSpace + `*=` + jsSpace + `*?|:` + jsSpace + `+?)` +
	// A quoted value, which may span lines, or the rest of the line up to a #.
	`(` + jsSpace + `*'(?:\\'|[^'])*'|` +
	jsSpace + `*"(?:\\"|[^"])*"|` +
	jsSpace + "*`(?:\\\\`|[^`])*`|" +
	`[^#\r\n]+)?` +
	// An optional comment, then the end of the line.
	jsSpace + `*(?:#` + notLineEnd + `*)?` + lineEndOrEOF)

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
	for p := 0; p >= 0; {
		m := lineRE.FindStringSubmatchIndex(src[p:])
		if m == nil {
			// Had there been a match at a later line start within the run of
			// whitespace that starts here, there would have been one here
			// too, with the leading \s* taking the extra whitespace. So skip
			// the whole run, which keeps blank lines from costing quadratic
			// time.
			p += len(src[p:]) - len(strings.TrimLeftFunc(src[p:], isJSSpace))
			p = nextLineStart(src, p)
			continue
		}
		// The submatches are the key, the raw value and the line end.
		key, rawValue, end := m[2:4], m[4:6], m[6]
		raw := ""
		if rawValue[0] >= 0 {
			raw = src[p+rawValue[0] : p+rawValue[1]]
		}
		value, quote := clean(raw)
		out = append(out, assignment{key: src[p+key[0] : p+key[1]], value: value, quote: quote})
		// Carry on where the match ended, before the character that ended
		// its line, as JavaScript's global matching does.
		p += end
		if !isLineStart(src, p) {
			p = nextLineStart(src, p)
		}
	}
	return out
}

// clean trims a raw value, removes its quotes and handles the escapes its
// quote style supports. The quote style is the value's first character.
func clean(raw string) (string, byte) {
	value := strings.TrimFunc(raw, isJSSpace)
	var quote byte
	if value != "" && quoteKind(value[0]) >= 0 {
		quote = value[0]
	}
	value = unquote(value)
	switch quote {
	case '\'':
		value = strings.ReplaceAll(value, `\'`, "'")
	case '"':
		value = doubleQuoteEscapes.Replace(value)
	}
	return value, quote
}

// unquote does what dotenvx's value.replace(/^(['"`])([\s\S]*)\1$/mg, "$2")
// does: wherever a line starts with a quote, it removes that quote and the
// last matching quote that ends a line, keeping what is between them.
// Values usually span one line, or start and end with their quote, and then
// this simply removes the surrounding quotes.
func unquote(s string) string {
	// The greedy match for an opening quote ends at the last quote of the
	// same kind that ends a line, if that comes after it.
	closing := [3]int{-1, -1, -1}
	for i := len(s) - 1; i >= 0; i-- {
		if k := quoteKind(s[i]); k >= 0 && closing[k] < 0 && isLineEnd(s, i+1) {
			closing[k] = i
		}
	}
	var b strings.Builder
	last := 0
	for p := 0; p >= 0; p = nextLineStart(s, p) {
		if p == len(s) {
			continue
		}
		k := quoteKind(s[p])
		if k < 0 || closing[k] <= p {
			continue
		}
		end := closing[k]
		b.WriteString(s[last:p])
		b.WriteString(s[p+1 : end])
		last = end + 1
		p = end
	}
	b.WriteString(s[last:])
	return b.String()
}

// quoteKind returns 0, 1 or 2 for ', " and `, and -1 for anything else.
func quoteKind(c byte) int {
	return strings.IndexByte("'\"`", c)
}

// isLineStart reports whether a line starts at s[p:].
func isLineStart(s string, p int) bool {
	r, _ := utf8.DecodeLastRuneInString(s[:p])
	return p == 0 || strings.ContainsRune(lineEnd, r)
}

// isLineEnd reports whether a line ends at s[p], at the end of s or at a
// character that ends lines.
func isLineEnd(s string, p int) bool {
	r, _ := utf8.DecodeRuneInString(s[p:])
	return p == len(s) || strings.ContainsRune(lineEnd, r)
}

// nextLineStart returns the index of the next line start after s[p], or -1
// if there is none.
func nextLineStart(s string, p int) int {
	i := strings.IndexAny(s[p:], lineEnd)
	if i < 0 {
		return -1
	}
	_, size := utf8.DecodeRuneInString(s[p+i:])
	return p + i + size
}

// isJSSpace reports whether JavaScript's \s matches r.
func isJSSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', '\u00a0', '\u1680', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000', '\ufeff':
		return true
	}
	return '\u2000' <= r && r <= '\u200a'
}
