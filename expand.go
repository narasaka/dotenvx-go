package dotenvx

import (
	"regexp"
	"strings"
)

// maxReexpansions caps how many times expanding one value may substitute
// text that itself contains a "$". Only those substitutions can repeat
// forever, as in A=$B with B=x$B. Every other one removes at least one "$",
// so a value with any number of plain references still expands fully.
const maxReexpansions = 100

var (
	bracedRef = regexp.MustCompile(`^\$\{([^{}]+)\}`)
	simpleRef = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*)`)
	// anyRef is what dotenvx looks for in single-quoted values to decide
	// that expansion must stop once one of them has been substituted.
	anyRef = regexp.MustCompile(`\$\{[^}]+\}|\$[A-Za-z_][A-Za-z0-9_]*`)
)

// expand replaces $NAME, ${NAME} and ${NAME<op>alt} references in value,
// innermost first, the way dotenvx does. lookup resolves names. literals
// holds the values of keys that were assigned single-quoted values.
func expand(value string, lookup func(string) (string, bool), literals map[string]string) string {
	for reexpansions := 0; ; {
		start, end, ref := nextRef(value)
		if ref == "" {
			return value
		}
		name, replacement := resolve(ref, lookup)
		value = value[:start] + replacement + value[end:]
		if v, ok := lookup(name); ok && v == value {
			return value
		}
		if anyRef.MatchString(literals[name]) {
			return value
		}
		if strings.Contains(replacement, "$") {
			if reexpansions++; reexpansions == maxReexpansions {
				return value
			}
		}
	}
}

// nextRef finds the first reference in s whose "$" isn't escaped with a
// backslash. It returns where the reference is and what is inside it: the
// name, plus the operator and alternative if it is braced.
func nextRef(s string) (start, end int, ref string) {
	for i := 0; i < len(s); i++ {
		if s[i] != '$' || i > 0 && s[i-1] == '\\' {
			continue
		}
		m := bracedRef.FindStringSubmatch(s[i:])
		if m == nil {
			m = simpleRef.FindStringSubmatch(s[i:])
		}
		if m != nil {
			return i, i + len(m[0]), m[1]
		}
	}
	return 0, 0, ""
}

// resolve returns the name a reference looks up and what it expands to.
func resolve(ref string, lookup func(string) (string, bool)) (name, replacement string) {
	name, op, alt := splitOperator(ref)
	v, ok := lookup(name)
	switch op {
	case ":-":
		if v == "" {
			v = alt
		}
	case "-":
		if !ok {
			v = alt
		}
	case ":+":
		if v != "" {
			v = alt
		}
	case "+":
		if ok {
			v = alt
		}
	}
	return name, v
}

// splitOperator splits a reference at its first ":-", "-", ":+" or "+".
func splitOperator(ref string) (name, op, alt string) {
	for i := 0; i < len(ref); i++ {
		switch {
		case ref[i] == '-' || ref[i] == '+':
			return ref[:i], ref[i : i+1], ref[i+1:]
		case ref[i] == ':' && i+1 < len(ref) && (ref[i+1] == '-' || ref[i+1] == '+'):
			return ref[:i], ref[i : i+2], ref[i+2:]
		}
	}
	return ref, "", ""
}
