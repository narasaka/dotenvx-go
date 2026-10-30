package dotenvx

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
)

// maxOutput is the most a command may write to stdout and stderr together.
// Like execSync's default maxBuffer, more counts as a failure.
const maxOutput = 1 << 20

// evaluate replaces each $(command) in value with the command's output,
// minus trailing newlines. Commands run with /bin/sh -c, as Node's execSync
// runs them, with env as their environment. If any command fails, can't
// run at all, or writes more than maxOutput bytes, evaluate returns value
// unchanged, like dotenvx. What commands write to stderr goes to os.Stderr
// once they are done, as with execSync, whatever happens to it.
func evaluate(value string, env []string) string {
	var b strings.Builder
	last := 0
	for _, span := range commandSpans(value) {
		out, errOut, ok := run(value[span[0]+2:span[1]-1], env)
		os.Stderr.Write(errOut)
		if !ok {
			return value
		}
		b.WriteString(value[last:span[0]])
		b.WriteString(strings.TrimRight(out, "\r\n"))
		last = span[1]
	}
	b.WriteString(value[last:])
	return b.String()
}

// run runs command and returns what it writes to stdout and stderr, and
// false if it fails, can't run, or writes more than maxOutput bytes to the
// two together. It doesn't write to os.Stderr itself, so the command never
// waits on whoever reads that.
//
// Like execSync, it waits for children the command leaves running in the
// background until they close stdout and stderr, unless the output is too
// long. Then it kills the command and closes both streams at once. That's
// why it makes the pipes itself, rather than leave them to os/exec, which
// would wait for them to close.
func run(command string, env []string) (out string, errOut []byte, ok bool) {
	var readers, writers [2]*os.File
	defer func() {
		for _, f := range append(readers[:], writers[:]...) {
			if f != nil {
				f.Close()
			}
		}
	}()
	for i := range readers {
		var err error
		if readers[i], writers[i], err = os.Pipe(); err != nil {
			return "", nil, false
		}
	}
	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = writers[0], writers[1]
	if err := cmd.Start(); err != nil {
		return "", nil, false
	}
	budget := &outputBudget{left: maxOutput, cmd: cmd, readers: readers}
	var stdout strings.Builder
	var stderr bytes.Buffer
	var copying sync.WaitGroup
	for i, w := range []io.Writer{&stdout, &stderr} {
		copying.Add(1)
		go func() {
			defer copying.Done()
			io.Copy(budgetWriter{budget, w}, readers[i])
		}()
	}
	err := cmd.Wait()
	// The streams end once the command, and any children holding them, are
	// done with them.
	writers[0].Close()
	writers[1].Close()
	copying.Wait()
	return stdout.String(), stderr.Bytes(), err == nil && !budget.spent
}

// outputBudget is what a command may still write to stdout and stderr,
// which are copied concurrently. Overspending it kills the command and
// closes both streams.
type outputBudget struct {
	mu      sync.Mutex
	left    int
	spent   bool
	cmd     *exec.Cmd
	readers [2]*os.File
}

// budgetWriter writes to w, which can't fail, what fits in the budget.
type budgetWriter struct {
	budget *outputBudget
	w      io.Writer
}

func (b budgetWriter) Write(p []byte) (int, error) {
	b.budget.mu.Lock()
	defer b.budget.mu.Unlock()
	if b.budget.spent || len(p) > b.budget.left {
		if !b.budget.spent {
			b.budget.spent = true
			// SIGTERM, like execSync's default killSignal.
			b.budget.cmd.Process.Signal(syscall.SIGTERM)
			b.budget.readers[0].Close()
			b.budget.readers[1].Close()
		}
		return 0, errOutputTooLong
	}
	b.budget.left -= len(p)
	return b.w.Write(p)
}

var errOutputTooLong = errors.New("command output too long")

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
