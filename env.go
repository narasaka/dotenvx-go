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
	// fold makes get ignore case, as names in the Windows process
	// environment do. It doesn't affect expansion: dotenvx expands from a
	// copy of the process environment, whose names are case-sensitive.
	fold bool
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
