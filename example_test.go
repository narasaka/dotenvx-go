package dotenvx_test

import (
	"fmt"
	"strings"

	"github.com/narasaka/dotenvx-go"
)

func ExampleOptions_Parse() {
	src := `# comments and blank lines are skipped
HELLO=World
GREETING="Hello, ${HELLO}
on two lines"
EXISTING=from the file
`
	opts := dotenvx.Options{Env: []string{"EXISTING=from the environment"}}
	values, err := opts.Parse(strings.NewReader(src))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(values["HELLO"])
	fmt.Println(values["GREETING"])
	fmt.Println(values["EXISTING"])

	opts.Overload = true
	values, _ = opts.Parse(strings.NewReader(src))
	fmt.Println(values["EXISTING"])
	// Output:
	// World
	// Hello, World
	// on two lines
	// from the environment
	// from the file
}

func ExampleOptions_Read() {
	// Files are read in order, and the first to set a key wins, so a local
	// override file comes first.
	values, err := dotenvx.Options{Env: []string{}}.Read("testdata/node/export.env", "testdata/node/eval.env")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(values["KEY"], values["HELLO"])
	// Output:
	// value world
}
