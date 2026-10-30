package dotenvx

import (
	"os"
	"syscall"
	"testing"
)

// Without file descriptors to spare, a command can't run, and the value is
// kept as it is.
func TestEvaluateWithoutFileDescriptors(t *testing.T) {
	// With none, making the pipes fails; with four, starting the shell.
	for _, spare := range []int{0, 4} {
		got := withSpareFileDescriptors(t, spare, func() string {
			return evaluate("$(echo hi)", nil)
		})
		if got != "$(echo hi)" {
			t.Errorf("with %d spare descriptors, evaluate = %q, want the value as it was", spare, got)
		}
	}
}

// withSpareFileDescriptors runs f with the limit on open files lowered so
// that only spare more can be opened.
func withSpareFileDescriptors(t *testing.T, spare int, f func() string) string {
	t.Helper()
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		t.Fatal(err)
	}
	// New files get the lowest free descriptors. Opening spare+1 of them
	// finds the first descriptor that a limit can then leave out.
	files := make([]*os.File, spare+1)
	for i := range files {
		var err error
		if files[i], err = os.Open(os.DevNull); err != nil {
			t.Fatal(err)
		}
	}
	tight := limit
	tight.Cur = uint64(files[spare].Fd())
	for _, file := range files {
		file.Close()
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &tight); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
			t.Fatal(err)
		}
	}()
	return f()
}
