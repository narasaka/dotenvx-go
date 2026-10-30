package dotenvx_test

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
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
	// Files are read in order, and the first to set a key wins, so the
	// local override comes first. The later file sees its values.
	values, err := dotenvx.Options{Env: []string{}}.Read("testdata/load/.env.local", "testdata/load/.env")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(values["HELLO"])
	fmt.Println(values["GREETING"])
	// Output:
	// local
	// Hello, local
}

func ExampleLoad() {
	// Load .env from the current directory, if there is one, keeping
	// variables that are already set.
	if err := dotenvx.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Fatal(err)
	}
	fmt.Println(os.Getenv("HELLO"))
}

func ExampleOverload() {
	// Load the shared defaults, then let .env.local replace any of them and
	// any variables that are already set.
	if err := dotenvx.Overload(".env", ".env.local"); err != nil {
		log.Fatal(err)
	}
}
