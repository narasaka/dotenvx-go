package dotenvx

import "strings"

// environ turns "KEY=value" entries, as returned by os.Environ, into a map.
// Later entries win, and entries without a key are dropped.
func environ(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, kv := range env {
		i := strings.Index(kv, "=")
		if i == 0 {
			// Windows keeps per-drive working directories in variables
			// whose names start with "=", like "=C:=C:\dir".
			i = strings.Index(kv[1:], "=") + 1
		}
		if i <= 0 {
			continue
		}
		m[kv[:i]] = kv[i+1:]
	}
	return m
}

// environment is the existing environment that source is resolved against.
type environment struct {
	vars map[string]string
	// names lists the names in vars in their original order, as far as it
	// is known; dotenvx goes through some of them in that order.
	names []string
	// fold makes get ignore case, as names in the Windows process
	// environment do. It doesn't affect expansion: dotenvx expands from a
	// copy of the process environment, whose names are case-sensitive.
	fold bool
}

// newEnvironment makes the environment of "KEY=value" entries.
func newEnvironment(env []string, fold bool) environment {
	vars := environ(env)
	var names []string
	seen := map[string]bool{}
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if _, ok := vars[name]; ok && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return environment{vars: vars, names: names, fold: fold}
}

// get looks up whether key already exists, and its value.
func (e environment) get(key string) (string, bool) {
	if v, ok := e.vars[key]; ok || !e.fold {
		return v, ok
	}
	for k, v := range e.vars {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return "", false
}

// set sets key to value, as the process environment would: with fold, it
// updates an existing key that differs only in case, keeping its spelling.
// A new key goes at the end of names.
func (e *environment) set(key, value string) {
	if _, ok := e.vars[key]; !ok && e.fold {
		for k := range e.vars {
			if strings.EqualFold(k, key) {
				key = k
				break
			}
		}
	}
	if _, ok := e.vars[key]; !ok {
		e.names = append(e.names, key)
	}
	e.vars[key] = value
}

// commandEnv is the environment commands run with: the existing environment
// overlaid with the values parsed so far.
func commandEnv(existing, parsed map[string]string) []string {
	env := make([]string, 0, len(existing)+len(parsed))
	for k, v := range existing {
		if _, ok := parsed[k]; !ok {
			env = append(env, k+"="+v)
		}
	}
	for k, v := range parsed {
		env = append(env, k+"="+v)
	}
	return env
}
