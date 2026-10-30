package dotenvx

import (
	"fmt"
	"io"
	"os"
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
	for _, a := range scan(string(src)) {
		value := a.value
		if v, ok := existing[a.key]; ok && !o.Overload {
			value = v
		}
		parsed[a.key] = value
	}
	return parsed, nil
}

// Parse reads dotenv source from r with the default Options.
func Parse(r io.Reader) (map[string]string, error) {
	return Options{}.Parse(r)
}
