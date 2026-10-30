package dotenvx

import (
	"errors"
	"fmt"
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
	for _, name := range []string{"DOTENV_FILE", "DOTENV_PATH", "DOTENV_F"} {
		unsetenv(t, name)
	}
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
	// "A=é\nB=" and an unpaired surrogate in UTF-16LE with a byte order
	// mark, plus a stray odd byte.
	if err := os.WriteFile(path, []byte{0xff, 0xfe, 'A', 0, '=', 0, 0xe9, 0, '\n', 0, 'B', 0, '=', 0, 0x00, 0xd8, 'C'}, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Options{Env: []string{}}.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"A": "é", "B": "\ufffd"}; !maps.Equal(got, want) {
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
	got, _, err := Options{}.parse("PATH=file\nA=$PATH|$Path|${path:-unset}", env, "")
	if want := map[string]string{"PATH": "env", "A": "env|env|unset"}; err != nil || !maps.Equal(got, want) {
		t.Errorf("parse = %q, %v; want %q", got, err, want)
	}
	env.fold = false
	got, _, err = Options{}.parse("PATH=file", env, "")
	if want := map[string]string{"PATH": "file"}; err != nil || !maps.Equal(got, want) {
		t.Errorf("without folding, parse = %q, %v; want %q", got, err, want)
	}
}

// After each file, dotenvx sets its values in process.env, in the order
// they were first assigned. On Windows, setting a name that differs only in
// case from an existing one updates that one, keeping its spelling. Each
// case here was checked against @dotenvx/primitives 4.3.0 with process.env
// standing in for the Windows one.
func TestReadFoldedEnv(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		env      []string
		overload bool
		// want is what Read returns, and wantEnv the environment after it.
		want, wantEnv map[string]string
	}{
		{"overload across files", []string{"DXG_Foo=first", "DXG_FOO=second"}, nil, true,
			map[string]string{"DXG_Foo": "first", "DXG_FOO": "second"}, map[string]string{"DXG_Foo": "second"}},
		{"an existing spelling", []string{"PATH=new", "RESULT=$Path"}, []string{"Path=old"}, true,
			map[string]string{"PATH": "new", "RESULT": "new"}, map[string]string{"Path": "new", "RESULT": "new"}},
		{"within a file", []string{"DXG_Foo=one\nDXG_FOO=two"}, nil, false,
			map[string]string{"DXG_Foo": "one", "DXG_FOO": "two"}, map[string]string{"DXG_Foo": "two"}},
		{"a later file", []string{"DXG_Foo=one\nDXG_FOO=two", "DXG_Foo=ignored\nDXG_FOO=ignored"}, nil, false,
			map[string]string{"DXG_Foo": "two", "DXG_FOO": "two"}, map[string]string{"DXG_Foo": "two"}},
		{"in order of first assignment", []string{"DXG_FOO=two\nDXG_Foo=one\nDXG_FOO=three"}, nil, false,
			map[string]string{"DXG_FOO": "three", "DXG_Foo": "one"}, map[string]string{"DXG_FOO": "one"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			var files []string
			for i, src := range tt.files {
				files = append(files, writeFile(t, dir, fmt.Sprint(i), src))
			}
			env := environment{vars: environ(tt.env), fold: true}
			got, _, err := Options{Overload: tt.overload}.read(files, env)
			if err != nil || !maps.Equal(got, tt.want) {
				t.Errorf("read = %q, %v; want %q", got, err, tt.want)
			}
			if !maps.Equal(env.vars, tt.wantEnv) {
				t.Errorf("environment = %q, want %q", env.vars, tt.wantEnv)
			}
		})
	}
}

func TestReadDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "A=1")
	got, err := Options{Env: []string{}}.Read(dir)
	if want := map[string]string{"A": "1"}; err != nil || !maps.Equal(got, want) {
		t.Errorf("Read(dir) = %q, %v; want %q", got, err, want)
	}
}

func TestReadDefaultFiles(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.env", "A=a\nB=a")
	b := writeFile(t, dir, "b.env", "B=b\nC=b")
	writeFile(t, dir, ".env", "DEFAULT=1")
	t.Chdir(dir)
	tests := []struct {
		name string
		env  []string
		want map[string]string
	}{
		{"DOTENV_FILE", []string{"DOTENV_FILE= " + a + " ,, " + b, "DOTENV_PATH=" + b}, map[string]string{"A": "a", "B": "a", "C": "b"}},
		{"DOTENV_PATH", []string{"DOTENV_FILE=", "DOTENV_PATH=" + b}, map[string]string{"B": "b", "C": "b"}},
		{"DOTENV_F", []string{"DOTENV_F=" + a}, map[string]string{"A": "a", "B": "a"}},
		// A list without names falls back to .env, rather than to the next
		// variable.
		{"no names", []string{"DOTENV_FILE= , ", "DOTENV_PATH=" + b}, map[string]string{"DEFAULT": "1"}},
		{"only commas", []string{"DOTENV_FILE=,,"}, map[string]string{"DEFAULT": "1"}},
		// The names are case-sensitive, as in dotenvx, even on Windows.
		{"other case", []string{"dotenv_file=" + a}, map[string]string{"DEFAULT": "1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Options{Env: tt.env}.read(nil, environment{vars: environ(tt.env), fold: true})
			if err != nil || !maps.Equal(got, tt.want) {
				t.Errorf("Read() = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

// Load always resolves against the process environment it sets.
func TestLoadIgnoresEnv(t *testing.T) {
	path := writeFile(t, t.TempDir(), ".env", "DOTENVX_GO_A=file")
	t.Setenv("DOTENVX_GO_A", "process")
	if err := (Options{Env: []string{}}).Load(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("DOTENVX_GO_A"); got != "process" {
		t.Errorf("DOTENVX_GO_A = %q, want the existing value", got)
	}
}
