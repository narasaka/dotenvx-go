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
