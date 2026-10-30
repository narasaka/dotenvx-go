// Command app is the dummy server the benchmark starts. It loads its
// configuration one of three ways, checks that every BENCH_VALUE_* variable
// arrived, and answers /health:
//
//   - with DOTENVX_ENV_JSON set, it applies that JSON object, which is what
//     `dotenvx get` printed, the way a server avoiding `dotenvx run` might;
//   - built with the dotenvxgo tag and BENCH_ENV_FILE set, it decrypts that
//     file itself with dotenvx-go;
//   - otherwise it uses the environment as it is, as under `dotenvx run` or
//     with plaintext variables.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func main() {
	if err := loadConfig(); err != nil {
		log.Fatal(err)
	}
	if err := checkConfig(); err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	log.Fatal(http.ListenAndServe("127.0.0.1:"+os.Getenv("PORT"), nil))
}

func loadConfig() error {
	if raw, ok := os.LookupEnv("DOTENVX_ENV_JSON"); ok {
		var vars map[string]string
		if err := json.Unmarshal([]byte(raw), &vars); err != nil {
			return fmt.Errorf("read DOTENVX_ENV_JSON: %w", err)
		}
		for key, value := range vars {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
		return os.Unsetenv("DOTENVX_ENV_JSON")
	}
	return loadFile()
}

// checkConfig makes sure the benchmark measured a server that really got its
// configuration: BENCH_EXPECT variables named BENCH_VALUE_*, none of them
// still encrypted.
func checkConfig() error {
	want, err := strconv.Atoi(os.Getenv("BENCH_EXPECT"))
	if err != nil {
		return fmt.Errorf("BENCH_EXPECT: %w", err)
	}
	got := 0
	for _, kv := range os.Environ() {
		key, value, _ := strings.Cut(kv, "=")
		if !strings.HasPrefix(key, "BENCH_VALUE_") {
			continue
		}
		if value == "" || strings.HasPrefix(value, "encrypted:") {
			return fmt.Errorf("%s wasn't decrypted", key)
		}
		got++
	}
	if got != want {
		return fmt.Errorf("got %d BENCH_VALUE_* variables, want %d", got, want)
	}
	return nil
}
