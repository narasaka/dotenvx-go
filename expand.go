package dotenvx

import (
	"errors"
	"regexp"
)

// ErrReferenceCycle is reported, wrapped, when a value can't be expanded
// because a variable refers back to itself, as in A=$B with B=x$B.
var ErrReferenceCycle = errors.New("reference cycle")

var (
	bracedRef = regexp.MustCompile(`^\$\{([^{}]+)\}`)
	simpleRef = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*)`)
	// anyRef is what dotenvx looks for in single-quoted values to decide
	// that expansion must stop once one of them has been substituted.
	anyRef = regexp.MustCompile(`\$\{[^}]+\}|\$[A-Za-z_][A-Za-z0-9_]*`)
)

// origin records which variables' values a piece of text came from: the
// variable whose value it was, then the one whose value contained the
// reference to that, and so on. Text written in the source has a nil origin.
type origin struct {
	name   string
	parent *origin
}

func (o *origin) has(name string) bool {
	for ; o != nil; o = o.parent {
		if o.name == name {
			return true
		}
	}
	return false
}

// expand replaces $NAME, ${NAME} and ${NAME<op>alt} references in value,
// innermost first, the way dotenvx does. lookup resolves names. literals
// holds the values of keys that were assigned single-quoted values.
//
// dotenvx loops forever when a variable's value leads back to a reference
// to the same variable. expand tracks where each byte of the value came
// from, and returns ErrReferenceCycle instead of inserting a variable's
// value into text that came from that same value.
func expand(value string, lookup func(string) (string, bool), literals map[string]string) (string, error) {
	origins := make([]*origin, len(value))
	for {
		start, end, ref := nextRef(value)
		if ref == "" {
			return value, nil
		}
		name, op, alt := splitOperator(ref)
		v, ok := lookup(name)
		replacement, isValue := resolve(op, alt, v, ok)
		var inserted []*origin
		if isValue {
			parent := origins[start]
			if parent.has(name) {
				return value, ErrReferenceCycle
			}
			o := &origin{name: name, parent: parent}
			inserted = make([]*origin, len(replacement))
			for i := range inserted {
				inserted[i] = o
			}
		} else {
			// The alternative is the end of the braced reference, before its
			// closing brace, and keeps the origin it had there.
			inserted = origins[end-1-len(replacement) : end-1]
		}
		value = value[:start] + replacement + value[end:]
		origins = append(append(append(make([]*origin, 0, len(value)), origins[:start]...), inserted...), origins[end:]...)
		if v, ok := lookup(name); ok && v == value {
			return value, nil
		}
		if anyRef.MatchString(literals[name]) {
			return value, nil
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

// resolve returns what a reference expands to, given the value of its name
// and whether the name is set. isValue reports whether that is the value,
// rather than the alternative or nothing.
func resolve(op, alt, v string, ok bool) (replacement string, isValue bool) {
	switch op {
	case ":-":
		if v == "" {
			return alt, false
		}
	case "-":
		if !ok {
			return alt, false
		}
	case ":+":
		if v != "" {
			return alt, false
		}
		return "", false
	case "+":
		if ok {
			return alt, false
		}
		return "", false
	}
	return v, true
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
