package dotenvx

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
)

// Options configures how dotenv source is resolved. The zero value is ready
// to use and matches the defaults of dotenvx.
type Options struct {
	// Env is the existing environment that Parse and Read resolve against,
	// in the "KEY=value" form of os.Environ. A key that already exists in
	// Env keeps its existing value unless Overload is set. If Env is nil,
	// they use the current process environment; use an empty slice for no
	// existing environment at all. Load ignores Env: it always resolves
	// against the process environment it sets values in.
	Env []string

	// Overload lets values from the source replace values that already
	// exist in Env.
	Overload bool

	// DisableCommandSubstitution turns off $(command) substitution. By
	// default, as in dotenvx, each $(command) in a value that isn't
	// single-quoted is replaced with the output of running the command with
	// /bin/sh. That includes the existing value of a key the source
	// assigns, unless Overload is set. The command sees Env plus the values
	// parsed before it. When a command fails, writes more than 1 MiB to
	// stdout and stderr together, or can't run because there is no /bin/sh
	// (as on Windows or in minimal container images), the value is kept as
	// it is. What a command writes to stderr is written to os.Stderr after
	// it finishes, as dotenvx does.
	DisableCommandSubstitution bool
}

// Parse reads dotenv source from r and returns its values. It doesn't modify
// the process environment. When a key is assigned more than once, the last
// assignment wins.
//
// Values that aren't single-quoted first have each $(command) replaced
// with the command's output, unless DisableCommandSubstitution is set.
// Each output is inserted as it is, where the command was. dotenvx instead
// treats it as a String.prototype.replace pattern ($$, $&, $` and $') and
// replaces the first copy of the command's text, which may be in an
// earlier command's output; it also turns invalid UTF-8 into U+FFFD, and
// runs commands with cmd.exe on Windows rather than /bin/sh.
//
// Values that aren't single-quoted, and had no command substituted, then
// have their $NAME and ${NAME} references expanded, as do
// ${NAME:-default}, ${NAME-default}, ${NAME:+alternative} and
// ${NAME+alternative}, the way dotenvx expands them. Names resolve to
// existing values first, then to values parsed earlier, or the other way
// round with Overload. A backslash keeps a "$" from starting a reference.
// Existing values that aren't empty aren't expanded, though they do go
// through command substitution.
//
// Expansion differs from dotenvx where dotenvx relies on accidents of
// JavaScript: a value is inserted as it is, even if it contains "$$" or
// "$&"; the reference that matched is the one replaced, even if an escaped
// copy of it comes first; and names containing "null", or naming properties
// of Object.prototype, resolve like any other. Where dotenvx would loop
// forever, because a variable refers back to itself, Parse leaves the
// rest of that value unexpanded and returns its values with an error that
// wraps ErrReferenceCycle. It does that as soon as a reference that came
// entirely from a variable's value would insert that value again, which
// also stops a few contrived inputs that dotenvx finishes. A value that
// would take more than 10,000 substitutions of values containing "$", or
// grow past 16 MiB, stops the same way, with ErrExpansionLimit.
//
// Source that starts with a UTF-16LE byte order mark is decoded from
// UTF-16LE; anything else is read as UTF-8. Bytes that aren't valid UTF-8
// are kept as they are, where dotenvx, which only sees decoded text, would
// have replaced them with U+FFFD. Keys such as constructor or __proto__,
// which make dotenvx throw, are parsed like any other key.
func (o Options) Parse(r io.Reader) (map[string]string, error) {
	src, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("dotenvx: %w", err)
	}
	values, _, err := o.parse(decode(src), o.environment(), "dotenvx: ")
	return values, err
}

// environment returns Env, or the process environment if Env is nil.
func (o Options) environment() environment {
	if o.Env == nil {
		return environment{vars: environ(os.Environ()), fold: runtime.GOOS == "windows"}
	}
	return environment{vars: environ(o.Env)}
}

// parse resolves the values in src against the existing environment. It
// also returns the keys in the order they were first assigned, which is the
// order dotenvx sets them in. Its errors start with prefix.
func (o Options) parse(src string, existing environment, prefix string) (map[string]string, []string, error) {
	parsed := map[string]string{}
	// literals holds the values of single-quoted assignments.
	literals := map[string]string{}
	// lookup resolves a reference: existing values win, unless overloading.
	lookup := func(name string) (string, bool) {
		first, second := existing.vars, parsed
		if o.Overload {
			first, second = parsed, existing.vars
		}
		if v, ok := first[name]; ok {
			return v, true
		}
		v, ok := second[name]
		return v, ok
	}
	var keys []string
	var errs []error
	for _, a := range scan(src) {
		value := a.value
		current, exists := existing.get(a.key)
		if exists && !o.Overload {
			value = current
		}
		// Like dotenvx, evaluate a value from the environment only if the
		// source didn't replace it.
		evaluated := false
		if a.quote != '\'' && !o.DisableCommandSubstitution &&
			strings.Contains(value, "$(") && (!exists || current == value) {
			next := evaluate(value, commandEnv(existing.vars, parsed))
			evaluated = next != value
			value = next
		}
		// Existing values are taken as they are, unless they are empty.
		if a.quote != '\'' && !evaluated && (current == "" || o.Overload) {
			expanded, err := expand(value, lookup, literals)
			if err != nil {
				errs = append(errs, fmt.Errorf("%sexpanding %s: %w", prefix, a.key, err))
			}
			value = strings.ReplaceAll(expanded, `\$`, "$")
		}
		if a.quote == '\'' {
			literals[a.key] = value
		}
		if _, ok := parsed[a.key]; !ok {
			keys = append(keys, a.key)
		}
		parsed[a.key] = value
	}
	return parsed, keys, errors.Join(errs...)
}

// Parse reads dotenv source from r with the default Options.
func Parse(r io.Reader) (map[string]string, error) {
	return Options{}.Parse(r)
}
