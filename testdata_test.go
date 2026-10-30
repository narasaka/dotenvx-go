package dotenvx

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// specScenario is one scenario of the dotenvx conformance suite in
// testdata/spec.
type specScenario struct {
	name     string
	src      []byte
	expected map[string]string
}

func loadSpec(t *testing.T) []specScenario {
	t.Helper()
	var expected map[string]map[string]string
	readJSON(t, filepath.Join("testdata", "spec", "expected.json"), &expected)
	files, err := filepath.Glob(filepath.Join("testdata", "spec", "*.env"))
	if err != nil {
		t.Fatal(err)
	}
	var scenarios []specScenario
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".env")
		values, ok := expected[name]
		if !ok {
			t.Fatalf("%s has no entry in expected.json", file)
		}
		delete(expected, name)
		scenarios = append(scenarios, specScenario{name: name, src: readFile(t, file), expected: values})
	}
	for name := range expected {
		t.Fatalf("expected.json has %s but there is no %s.env", name, name)
	}
	return scenarios
}

// nodeGolden is one fixture in testdata/node with the values Node's
// @dotenvx/primitives returned for it.
type nodeGolden struct {
	name     string
	src      []byte
	overload bool
	expected map[string]string
}

func loadNodeGoldens(t *testing.T) []nodeGolden {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "node", "*.env"))
	if err != nil {
		t.Fatal(err)
	}
	var goldens []nodeGolden
	for _, file := range files {
		base := strings.TrimSuffix(file, ".env")
		for _, overload := range []bool{false, true} {
			golden := nodeGolden{name: filepath.Base(base), src: readFile(t, file), overload: overload}
			path := base + ".json"
			if overload {
				golden.name += "/overload"
				path = base + ".overload.json"
			}
			readJSON(t, path, &golden.expected)
			goldens = append(goldens, golden)
		}
	}
	return goldens
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := json.Unmarshal(readFile(t, path), v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func TestFixtures(t *testing.T) {
	if got := len(loadSpec(t)); got != 88 {
		t.Errorf("spec has %d scenarios, want 88", got)
	}
	if got := len(loadNodeGoldens(t)); got != 10 {
		t.Errorf("node has %d goldens, want 10", got)
	}
}
