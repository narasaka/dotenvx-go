package dotenvx

import (
	"errors"
	"regexp"
	"slices"
	"strings"
)

var (
	// ErrReferenceCycle is reported, wrapped, when a value can't be
	// expanded because a variable refers back to itself, as in A=$B with
	// B=x$B.
	ErrReferenceCycle = errors.New("reference cycle")

	// ErrExpansionLimit is reported, wrapped, when expanding a value would
	// take more than 10,000 substitutions of values that contain "$", or
	// make it longer than 16 MiB, as happens when each of a chain of
	// variables refers to the next one twice.
	ErrExpansionLimit = errors.New("expansion limit exceeded")
)

// maxInsertions and maxExpandedLen bound the work of expanding one value.
// They are far beyond what real values need. Only substitutions of values
// containing "$" count towards maxInsertions, because any other
// substitution removes a "$" for good.
const (
	maxInsertions  = 10000
	maxExpandedLen = 16 << 20
)

var (
	bracedRef = regexp.MustCompile(`^\$\{([^{}]+)\}`)
	simpleRef = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*)`)
	// anyRef is what dotenvx looks for in single-quoted values to decide
	// that expansion must stop once one of them has been substituted.
	anyRef = regexp.MustCompile(`\$\{[^}]+\}|\$[A-Za-z_][A-Za-z0-9_]*`)
)

// origins records, for each byte of a value being expanded, which
// variables' values it came from: the variable whose value it was, then the
// one whose value contained the reference to that, and so on. Each byte
// holds an index into nodes, where index 0 stands for the source itself.
type origins struct {
	bytes []int32
	nodes []originNode
	fill  []int32
}

type originNode struct {
	name   string
	parent int32
	depth  int32
}

func newOrigins(n int) *origins {
	return &origins{bytes: make([]int32, n), nodes: []originNode{{}}}
}

// has reports whether the chain starting at node i includes name.
func (o *origins) has(i int32, name string) bool {
	for ; i != 0; i = o.nodes[i].parent {
		if o.nodes[i].name == name {
			return true
		}
	}
	return false
}

// common returns the deepest node shared by the chains of bytes [start:end].
func (o *origins) common(start, end int) int32 {
	c := o.bytes[start]
	for _, b := range o.bytes[start+1 : end] {
		for o.nodes[b].depth > o.nodes[c].depth {
			b = o.nodes[b].parent
		}
		for o.nodes[c].depth > o.nodes[b].depth {
			c = o.nodes[c].parent
		}
		for b != c {
			b, c = o.nodes[b].parent, o.nodes[c].parent
		}
	}
	return c
}

// insert replaces bytes [start:end] with n bytes that came from the value
// of name, looked up by a reference whose own chain is parent.
func (o *origins) insert(start, end, n int, name string, parent int32) {
	o.nodes = append(o.nodes, originNode{name: name, parent: parent, depth: o.nodes[parent].depth + 1})
	o.fill = slices.Grow(o.fill[:0], n)[:n]
	if n > 0 {
		// Fill by doubling, which copies in bulk rather than byte by byte.
		o.fill[0] = int32(len(o.nodes) - 1)
		for filled := 1; filled < n; filled *= 2 {
			copy(o.fill[filled:], o.fill[:filled])
		}
	}
	o.bytes = slices.Replace(o.bytes, start, end, o.fill...)
}

// keep replaces bytes [start:end] with the n bytes just before end, which
// keep their chains. That is where the alternative of a braced reference is,
// before its closing brace.
func (o *origins) keep(start, end, n int) {
	o.bytes = slices.Delete(o.bytes, end-1, end)
	o.bytes = slices.Delete(o.bytes, start, end-1-n)
}

// expand replaces $NAME, ${NAME} and ${NAME<op>alt} references in value,
// innermost first, the way dotenvx does. lookup resolves names. literals
// holds the values of keys that were assigned single-quoted values.
//
// dotenvx loops forever when expanding a variable's value leads back to the
// same variable. expand tracks where each byte of the value came from, and
// returns ErrReferenceCycle instead of inserting a variable's value for a
// reference that came entirely from that value. That stops every such loop.
// It also stops a few contrived inputs that dotenvx does finish, where the
// text around the repeated copy changes what the copy means: when its
// reference sits in an alternative that is discarded, or runs into a name
// character that follows it.
func expand(value string, lookup func(string) (string, bool), literals map[string]string) (string, error) {
	// o stays nil while all of value came from the source.
	var o *origins
	for insertions := 0; ; {
		start, end, ref := nextRef(value)
		if ref == "" {
			return value, nil
		}
		name, op, alt := splitOperator(ref)
		v, ok := lookup(name)
		replacement, isValue := resolve(op, alt, v, ok)
		if len(value)-(end-start)+len(replacement) > maxExpandedLen {
			return value, ErrExpansionLimit
		}
		next := value[:start] + replacement + value[end:]
		// dotenvx stops right after substituting a reference whose result
		// is that variable's value, or that names a single-quoted value
		// containing references.
		if ok && v == next || anyRef.MatchString(literals[name]) {
			return next, nil
		}
		if isValue {
			var parent int32
			if o != nil {
				parent = o.common(start, end)
			}
			if o != nil && o.has(parent, name) {
				return value, ErrReferenceCycle
			}
			if strings.Contains(replacement, "$") {
				if insertions++; insertions > maxInsertions {
					return value, ErrExpansionLimit
				}
			}
			if o == nil {
				o = newOrigins(len(value))
			}
			o.insert(start, end, len(replacement), name, parent)
		} else if o != nil {
			o.keep(start, end, len(replacement))
		}
		value = next
	}
}

// nextRef finds the first reference in s whose "$" isn't escaped with a
// backslash. It returns where the reference is and what is inside it: the
// name, plus the operator and alternative if it is braced.
func nextRef(s string) (start, end int, ref string) {
	for i := 0; ; i++ {
		j := strings.IndexByte(s[i:], '$')
		if j < 0 {
			return 0, 0, ""
		}
		i += j
		if i > 0 && s[i-1] == '\\' || i+1 == len(s) || !startsRef(s[i+1]) {
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
}

// startsRef reports whether c can follow the "$" of a reference: it starts
// a braced reference or a name.
func startsRef(c byte) bool {
	return c == '{' || c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
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
