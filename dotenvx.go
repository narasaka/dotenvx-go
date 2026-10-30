package dotenvx

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Options configures how dotenv source is resolved. The zero value is ready
// to use and matches the defaults of dotenvx.
type Options struct {
	// Env is the existing environment, in the "KEY=value" form of
	// os.Environ. A key that already exists in Env keeps its existing value
	// unless Overload is set. If Env is nil, Parse uses the current process
	// environment; use an empty slice for no existing environment at all.
	Env []string

	// Overload lets values from the source replace values that already
	// exist in Env.
	Overload bool
}

// Parse reads dotenv source from r and returns its values. It doesn't modify
// the process environment. When a key is assigned more than once, the last
// assignment wins.
//
// Values that aren't single-quoted have their $NAME and ${NAME} references
// expanded, as do ${NAME:-default}, ${NAME-default}, ${NAME:+alternative}
// and ${NAME+alternative}, the way dotenvx expands them. Names resolve to
// existing values first, then to values parsed earlier, or the other way
// round with Overload. A backslash keeps a "$" from starting a reference.
// Existing values that aren't empty are used as they are, unexpanded.
//
// Expansion differs from dotenvx where dotenvx relies on accidents of
// JavaScript: a value is inserted as it is, even if it contains "$$" or
// "$&"; the reference that matched is the one replaced, even if an escaped
// copy of it comes first; and names containing "null", or naming properties
// of Object.prototype, resolve like any other. Where dotenvx would loop
// forever, because a variable refers back to itself, Parse leaves the
// rest of that value unexpanded and returns its values with an error that
// wraps ErrReferenceCycle.
//
// The source is read as UTF-8. Bytes that aren't valid UTF-8 are kept as
// they are, where dotenvx, which only sees decoded text, would have replaced
// them with U+FFFD. Keys such as constructor or __proto__, which make
// dotenvx throw, are parsed like any other key.
func (o Options) Parse(r io.Reader) (map[string]string, error) {
	src, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("dotenvx: %w", err)
	}
	env := o.Env
	if env == nil {
		env = os.Environ()
	}
	existing := environ(env)
	parsed := map[string]string{}
	// literals holds the values of single-quoted assignments.
	literals := map[string]string{}
	// lookup resolves a reference: existing values win, unless overloading.
	lookup := func(name string) (string, bool) {
		first, second := existing, parsed
		if o.Overload {
			first, second = parsed, existing
		}
		if v, ok := first[name]; ok {
			return v, true
		}
		v, ok := second[name]
		return v, ok
	}
	var errs []error
	for _, a := range scan(string(src)) {
		value := a.value
		if v, ok := existing[a.key]; ok && !o.Overload {
			value = v
		}
		// Existing values are taken as they are, unless they are empty.
		if a.quote != '\'' && (existing[a.key] == "" || o.Overload) {
			expanded, err := expand(value, lookup, literals)
			if err != nil {
				errs = append(errs, fmt.Errorf("dotenvx: expanding %s: %w", a.key, err))
			}
			value = strings.ReplaceAll(expanded, `\$`, "$")
		}
		if a.quote == '\'' {
			literals[a.key] = value
		}
		parsed[a.key] = value
	}
	return parsed, errors.Join(errs...)
}

// Parse reads dotenv source from r with the default Options.
func Parse(r io.Reader) (map[string]string, error) {
	return Options{}.Parse(r)
}
