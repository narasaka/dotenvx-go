package dotenvx

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile writes an env file into dir and returns its path.
func writeFile(t *testing.T, dir, name, src string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// unsetenv unsets key for the duration of the test.
func unsetenv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	os.Unsetenv(key)
}

func TestRead(t *testing.T) {
	dir := t.TempDir()
	local := writeFile(t, dir, ".env.local", "A=local\nC=${B}-local")
	env := writeFile(t, dir, ".env", "A=env\nB=env\nC=env")
	tests := []struct {
		name     string
		overload bool
		want     map[string]string
	}{
		// The first file wins, and later files see what earlier ones set.
		{"first file wins", false, map[string]string{"A": "local", "B": "env", "C": "-local"}},
		{"overload: last file wins", true, map[string]string{"A": "env", "B": "env", "C": "env"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Options{Env: []string{}, Overload: tt.overload}.Read(local, env)
			if err != nil {
				t.Fatal(err)
			}
			if !maps.Equal(got, tt.want) {
				t.Errorf("Read = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadExistingEnv(t *testing.T) {
	path := writeFile(t, t.TempDir(), ".env", "A=file\nB=$A")
	got, err := Options{Env: []string{"A=env"}}.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"A": "env", "B": "env"}; !maps.Equal(got, want) {
		t.Errorf("Read = %q, want %q", got, want)
	}
}

func TestReadDefaultFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "DOTENVX_GO_TEST=file")
	t.Chdir(dir)
	unsetenv(t, "DOTENVX_GO_TEST")
	got, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	if got["DOTENVX_GO_TEST"] != "file" {
		t.Errorf("Read() = %q, want it to read .env", got)
	}
	if _, ok := os.LookupEnv("DOTENVX_GO_TEST"); ok {
		t.Error("Read set DOTENVX_GO_TEST in the environment")
	}
}

func TestReadMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, ".env", "A=1")
	got, err := Options{Env: []string{}}.Read(filepath.Join(dir, "missing"), path)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Read error = %v, want fs.ErrNotExist", err)
	}
	if want := map[string]string{"A": "1"}; !maps.Equal(got, want) {
		t.Errorf("Read = %q, want the other files' values %q", got, want)
	}
}

func TestReadReferenceCycle(t *testing.T) {
	path := writeFile(t, t.TempDir(), ".env", `B=x\$B`+"\nA=y$B")
	got, err := Options{Env: []string{}}.Read(path)
	if !errors.Is(err, ErrReferenceCycle) || !strings.Contains(err.Error(), path) {
		t.Errorf("Read error = %v, want ErrReferenceCycle naming the file", err)
	}
	if got["A"] != "yx$B" {
		t.Errorf("A = %q, want the values read so far", got["A"])
	}
}

func TestReadUTF16(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	// "A=é\n" in UTF-16LE with a byte order mark, plus a stray odd byte.
	if err := os.WriteFile(path, []byte{0xff, 0xfe, 'A', 0, '=', 0, 0xe9, 0, '\n', 0, 'B'}, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Options{Env: []string{}}.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"A": "é"}; !maps.Equal(got, want) {
		t.Errorf("Read = %q, want %q", got, want)
	}
}

func TestLoad(t *testing.T) {
	path := writeFile(t, t.TempDir(), ".env", "DOTENVX_GO_A=file\nDOTENVX_GO_B=file")
	unsetenv(t, "DOTENVX_GO_A")
	t.Setenv("DOTENVX_GO_B", "env")
	if err := Load(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("DOTENVX_GO_A"); got != "file" {
		t.Errorf("DOTENVX_GO_A = %q, want file", got)
	}
	if got := os.Getenv("DOTENVX_GO_B"); got != "env" {
		t.Errorf("DOTENVX_GO_B = %q, want the existing value", got)
	}
	if err := Overload(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("DOTENVX_GO_B"); got != "file" {
		t.Errorf("after Overload, DOTENVX_GO_B = %q, want file", got)
	}
}

func TestLoadErrors(t *testing.T) {
	dir := t.TempDir()
	// A NUL byte can't be stored in an environment variable.
	path := writeFile(t, dir, ".env", "DOTENVX_GO_A=1\nDOTENVX_GO_NUL=a\x00b")
	unsetenv(t, "DOTENVX_GO_A")
	unsetenv(t, "DOTENVX_GO_NUL")
	err := Load(filepath.Join(dir, "missing"), path)
	if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "DOTENVX_GO_NUL") {
		t.Errorf("Load error = %v, want the missing file and the failed variable", err)
	}
	if got := os.Getenv("DOTENVX_GO_A"); got != "1" {
		t.Errorf("DOTENVX_GO_A = %q, want the other values loaded", got)
	}
}

// On Windows, Node checks for existing keys in process.env, whose names are
// case-insensitive there, but expands from a copy of it, whose names aren't.
func TestParseFoldedEnv(t *testing.T) {
	env := environment{vars: environ([]string{"Path=env"}), fold: true}
	got, _ := Options{}.parse("PATH=file\nA=$PATH|$Path|${path:-unset}", env, "")
	if want := map[string]string{"PATH": "env", "A": "env|env|unset"}; !maps.Equal(got, want) {
		t.Errorf("parse = %q, want %q", got, want)
	}
	env.fold = false
	got, _ = Options{}.parse("PATH=file", env, "")
	if want := map[string]string{"PATH": "file"}; !maps.Equal(got, want) {
		t.Errorf("without folding, parse = %q, want %q", got, want)
	}
}
