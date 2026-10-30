package dotenvx

import (
	"maps"
	"testing"
)

func TestEnviron(t *testing.T) {
	got := environ([]string{
		"A=1",
		"EMPTY=",
		"EQUALS=x=y",
		"A=last wins",
		`=C:=C:\dir`, // Windows keeps per-drive directories in names like this.
		"NOEQUALS",
		"=",
		"",
	})
	want := map[string]string{"A": "last wins", "EMPTY": "", "EQUALS": "x=y", "=C:": `C:\dir`}
	if !maps.Equal(got, want) {
		t.Errorf("environ = %q, want %q", got, want)
	}
}
