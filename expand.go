package dotenvx

import (
	"errors"
	"regexp"
)

var (
	// ErrReferenceCycle is reported, wrapped, when a value can't be
	// expanded because a variable refers back to itself, as in A=$B with
	// B=x$B.
	ErrReferenceCycle = errors.New("reference cycle")

	// ErrExpansionLimit is reported, wrapped, when expanding a value would
	// take more than 10,000 substitutions of variables' values, as when
	// each of a chain of variables refers to the next one twice.
	ErrExpansionLimit = errors.New("too many substitutions")
)

// maxInsertions bounds the work of expanding one value. It is far beyond
// what real values need, and keeps definitions that double at each step
// from exhausting memory.
const maxInsertions = 10000

var (
	bracedRef = regexp.MustCompile(`^\$\{([^{}]+)\}`)
	simpleRef = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*)`)
	// anyRef is what dotenvx looks for in single-quoted values to decide
	// that expansion must stop once one of them has been substituted.
	anyRef = regexp.MustCompile(`\$\{[^}]+\}|\$[A-Za-z_][A-Za-z0-9_]*`)
)

// origin records which variables' values a piece of text came from: the
// variable whose value it was, then the one whose value contained the
// reference to that, and so on. Text from the source has a nil origin.
type origin struct {
	name   string
	parent *origin
	depth  int
}

func (o *origin) has(name string) bool {
	for ; o != nil; o = o.parent {
		if o.name == name {
			return true
		}
	}
	return false
}

func (o *origin) level() int {
	if o == nil {
		return 0
	}
	return o.depth
}

// commonOrigin returns the deepest origin that all the given ones share.
func commonOrigin(origins []*origin) *origin {
	common := origins[0]
	for _, o := range origins[1:] {
		for o.level() > common.level() {
			o = o.parent
		}
		for common.level() > o.level() {
			common = common.parent
		}
		for o != common {
			o, common = o.parent, common.parent
		}
	}
	return common
}

// expand replaces $NAME, ${NAME} and ${NAME<op>alt} references in value,
// innermost first, the way dotenvx does. lookup resolves names. literals
// holds the values of keys that were assigned single-quoted values.
//
// dotenvx loops forever when expanding a variable's value leads back to the
// same variable. expand tracks where each byte of the value came from, and
// returns ErrReferenceCycle instead of inserting a variable's value for a
// reference that came entirely from that value. That stops every such loop.
// A few contrived inputs that dotenvx does finish stop too: ones where the
// repeated reference sits in an alternative that is about to be discarded.
func expand(value string, lookup func(string) (string, bool), literals map[string]string) (string, error) {
	// origins holds the origin of each byte of value, or is nil while all
	// of value came from the source. spare is reused to build the next one.
	var origins, spare []*origin
	for insertions := 0; ; {
		start, end, ref := nextRef(value)
		if ref == "" {
			return value, nil
		}
		name, op, alt := splitOperator(ref)
		v, ok := lookup(name)
		replacement, isValue := resolve(op, alt, v, ok)
		next := value[:start] + replacement + value[end:]
		// dotenvx stops right after substituting a reference whose result
		// is that variable's value, or that names a single-quoted value
		// containing references.
		if ok && v == next || anyRef.MatchString(literals[name]) {
			return next, nil
		}
		if isValue {
			var parent *origin
			if origins != nil {
				parent = commonOrigin(origins[start:end])
			}
			if parent.has(name) {
				return value, ErrReferenceCycle
			}
			if insertions++; insertions > maxInsertions {
				return value, ErrExpansionLimit
			}
			if origins == nil {
				origins = make([]*origin, len(value))
			}
			spare = append(spare[:0], origins[:start]...)
			o := &origin{name: name, parent: parent, depth: parent.level() + 1}
			for range replacement {
				spare = append(spare, o)
			}
			origins, spare = append(spare, origins[end:]...), origins
		} else if origins != nil {
			// The alternative is the end of the braced reference, before
			// its closing brace, and keeps the origin it had there.
			spare = append(append(spare[:0], origins[:start]...), origins[end-1-len(replacement):end-1]...)
			origins, spare = append(spare, origins[end:]...), origins
		}
		value = next
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
