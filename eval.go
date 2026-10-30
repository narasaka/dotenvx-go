package dotenvx

import (
	"os"
	"os/exec"
	"strings"
)

// evaluate replaces each $(command) in value with the command's output,
// minus trailing newlines. Commands run with /bin/sh -c, as Node's execSync
// runs them, with env as their environment. If any command fails, or can't
// run at all, evaluate returns value unchanged, like dotenvx.
func evaluate(value string, env []string) string {
	var b strings.Builder
	last := 0
	for _, span := range commandSpans(value) {
		cmd := exec.Command("/bin/sh", "-c", value[span[0]+2:span[1]-1])
		cmd.Env = env
		// Like execSync, let the user see what the command reports.
		cmd.Stderr = os.Stderr
		out, err := cmd.Output()
		if err != nil {
			return value
		}
		b.WriteString(value[last:span[0]])
		b.WriteString(strings.TrimRight(string(out), "\r\n"))
		last = span[1]
	}
	b.WriteString(value[last:])
	return b.String()
}

// commandSpans returns the start and end of each $(command) in s.
func commandSpans(s string) [][2]int {
	var spans [][2]int
	for start := strings.Index(s, "$("); start >= 0; {
		next := start + 2
		if end := commandEnd(s, start+2); end >= 0 {
			spans = append(spans, [2]int{start, end})
			next = end
		}
		i := strings.Index(s[next:], "$(")
		if i < 0 {
			break
		}
		start = next + i
	}
	return spans
}

// commandEnd returns the index just past the parenthesis that closes a
// command starting at s[i], or -1 if it is never closed. It follows the
// quoting and nesting rules of dotenvx's scanner.
func commandEnd(s string, i int) int {
	// One entry per open parenthesis: the quote character open inside it,
	// or 0.
	quotes := []byte{0}
	for i < len(s) {
		top := len(quotes) - 1
		c := s[i]
		switch {
		case quotes[top] == '\'':
			// Nothing is special inside single quotes.
			if c == '\'' {
				quotes[top] = 0
			}
		case quotes[top] != 0:
			switch {
			case c == '\\':
				i++
			case c == quotes[top]:
				quotes[top] = 0
			case c == '$' && i+1 < len(s) && s[i+1] == '(':
				quotes = append(quotes, 0)
				i++
			}
		case c == '\\':
			i++
		case c == '\'' || c == '"' || c == '`':
			quotes[top] = c
		case c == '(':
			quotes = append(quotes, 0)
		case c == ')':
			quotes = quotes[:top]
			if top == 0 {
				return i + 1
			}
		}
		i++
	}
	return -1
}
