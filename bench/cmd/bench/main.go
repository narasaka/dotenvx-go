// Command bench measures what it costs a Go server to get its configuration
// from a dotenvx-encrypted file:
//
//   - plain: the variables are already in the environment, unencrypted (the
//     baseline, as if the platform held every secret);
//   - run: `dotenvx run -- app`, which keeps dotenvx running beside it;
//   - get: `dotenvx get` decrypts to JSON and exits, then the server applies
//     the JSON;
//   - lib: the server decrypts the file itself with dotenvx-go.
//
// A fifth setup, linked, starts the lib build with plaintext variables, which
// shows what linking dotenvx-go costs apart from loading a file.
//
// For each env file in env/ and each setup, it starts the dummy server in app/
// several times, and reports how long it took until /health answered and the
// memory (PSS) of every process involved once it was serving. It runs on
// Linux, from the bench directory:
//
//	sh install-dotenvx.sh
//	go run ./cmd/bench
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"maps"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/narasaka/dotenvx-go"
)

// Run is one start of the server.
type Run struct {
	ReadyMs   float64        `json:"readyMs"`
	PSSKiB    map[string]int `json:"pssKiB"` // by process name
	Processes int            `json:"processes"`
	Total     int            `json:"totalPssKiB"`
	// Anon is the anonymous part of Total. The rest is file-backed, mostly
	// the executables, which processes mapping the same file share.
	Anon int `json:"anonPssKiB"`
}

// Result is every run of one setup with one env file.
type Result struct {
	File  string `json:"file"`
	Setup string `json:"setup"`
	Keys  int    `json:"keys"`
	Runs  []Run  `json:"runs"`
}

func main() {
	dotenvxBin := flag.String("dotenvx", "bin/dotenvx", "dotenvx CLI to compare against")
	runs := flag.Int("runs", 5, "starts per setup and file")
	settle := flag.Duration("settle", time.Second, "wait after /health answers before measuring memory")
	jsonOut := flag.String("json", "", "also write the results as JSON to this file")
	flag.Parse()
	if *runs < 1 {
		fmt.Fprintln(os.Stderr, "bench: -runs must be at least 1")
		os.Exit(2)
	}
	// Ctrl-C stops the server being measured, which runs in its own process
	// group and so doesn't get the terminal's signal, and lets cleanup run.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, *dotenvxBin, *runs, *settle, *jsonOut); err != nil {
		fmt.Fprintln(os.Stderr, "bench:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, dotenvxBin string, runs int, settle time.Duration, jsonOut string) error {
	rollup, err := os.ReadFile("/proc/self/smaps_rollup")
	if err != nil {
		return fmt.Errorf("needs Linux's /proc/<pid>/smaps_rollup: %w", err)
	}
	if !strings.Contains(string(rollup), "\nPss_Anon:") {
		return errors.New("needs a kernel that reports Pss_Anon in /proc/<pid>/smaps_rollup")
	}
	dotenvxBin, err = filepath.Abs(dotenvxBin)
	if err != nil {
		return err
	}
	version, err := exec.Command(dotenvxBin, "--version").Output()
	if err != nil {
		return fmt.Errorf("%s: %w (run sh install-dotenvx.sh)", dotenvxBin, err)
	}
	fmt.Printf("dotenvx %s", version)

	tmp, err := os.MkdirTemp("", "dotenvx-bench")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	app, appLib := filepath.Join(tmp, "app"), filepath.Join(tmp, "app-lib")
	for _, b := range [][]string{{app}, {appLib, "-tags", "dotenvxgo"}} {
		args := append([]string{"build", "-trimpath", "-ldflags=-s -w", "-o", b[0]}, b[1:]...)
		if out, err := exec.Command("go", append(args, "./app")...).CombinedOutput(); err != nil {
			return fmt.Errorf("go build: %w\n%s", err, out)
		}
	}
	for _, b := range []string{app, appLib} {
		if info, err := os.Stat(b); err == nil {
			fmt.Printf("%s: %.1f MiB\n", filepath.Base(b), float64(info.Size())/(1<<20))
		}
	}

	keys, err := dotenvx.Options{Env: []string{}, DisableCommandSubstitution: true}.Read("env/.env.keys")
	if err != nil {
		return err
	}
	var results []Result
	for _, name := range []string{"small", "large"} {
		file := "env/.env." + name
		keyName := "DOTENV_PRIVATE_KEY_" + strings.ToUpper(name)
		key := keyName + "=" + keys[keyName]
		values, err := dotenvx.Options{Env: []string{key}}.Read(file)
		if err != nil {
			return err
		}
		expect := 0
		var plain []string
		for k, v := range values {
			if strings.HasPrefix(k, "BENCH_VALUE_") {
				expect++
				plain = append(plain, k+"="+v)
			}
		}
		base := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "BENCH_EXPECT=" + strconv.Itoa(expect)}
		setups := []struct {
			name string
			argv []string
			env  []string
		}{
			{"plain", []string{app}, plain},
			{"linked", []string{appLib}, plain},
			{"run", []string{dotenvxBin, "run", "--strict", "-f", file, "--", app}, []string{key}},
			{"get", []string{"/bin/sh", "-c", `DOTENVX_ENV_JSON=$("$0" get --strict -f "$1") && export DOTENVX_ENV_JSON && exec "$2"`, dotenvxBin, file, app}, []string{key}},
			{"lib", []string{appLib}, []string{key, "BENCH_ENV_FILE=" + file}},
		}
		for _, s := range setups {
			r := Result{File: filepath.Base(file), Setup: s.name, Keys: expect}
			for range runs {
				if err := ctx.Err(); err != nil {
					return err
				}
				one, err := start(ctx, s.argv, append(slices.Clone(base), s.env...), settle)
				if err != nil {
					return fmt.Errorf("%s with %s: %w", s.name, file, err)
				}
				r.Runs = append(r.Runs, one)
			}
			results = append(results, r)
		}
	}

	report(results)
	if jsonOut != "" {
		out, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(jsonOut, append(out, '\n'), 0o644)
	}
	return nil
}

// start runs argv in its own process group until /health answers, measures
// it and stops it.
func start(ctx context.Context, argv, env []string, settle time.Duration) (Run, error) {
	port, err := freePort()
	if err != nil {
		return Run{}, err
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(env, "PORT="+strconv.Itoa(port))
	var stderr strings.Builder
	cmd.Stderr = &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	begin := time.Now()
	if err := cmd.Start(); err != nil {
		return Run{}, err
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	stop := func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-exited
		}
	}

	client := http.Client{Timeout: 100 * time.Millisecond}
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	deadline := begin.Add(30 * time.Second)
	for {
		select {
		case err := <-exited:
			// Its own process may be gone while a child it started lives on.
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			return Run{}, fmt.Errorf("exited before serving (%v): %s", err, strings.TrimSpace(stderr.String()))
		case <-ctx.Done():
			stop()
			return Run{}, ctx.Err()
		default:
		}
		if resp, err := client.Get(url); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			stop()
			return Run{}, errors.New("/health didn't answer within 30s")
		}
		time.Sleep(time.Millisecond)
	}
	r := Run{ReadyMs: float64(time.Since(begin).Microseconds()) / 1000, PSSKiB: map[string]int{}}

	select {
	case <-time.After(settle):
	case <-ctx.Done():
		stop()
		return Run{}, ctx.Err()
	}
	if err := measure(cmd.Process.Pid, &r); err != nil {
		stop()
		return Run{}, err
	}
	stop()
	return r, nil
}

// measure adds up the PSS of every process in the process group pgid.
func measure(pgid int, r *Run) error {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		stat, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat"))
		if err != nil {
			continue
		}
		// The fields after the parenthesized name: state ppid pgrp ...
		fields := strings.Fields(string(stat[strings.LastIndexByte(string(stat), ')')+1:]))
		if len(fields) < 3 || fields[2] != strconv.Itoa(pgid) {
			continue
		}
		rollup, err := os.ReadFile(filepath.Join("/proc", e.Name(), "smaps_rollup"))
		if err != nil {
			continue
		}
		comm, _ := os.ReadFile(filepath.Join("/proc", e.Name(), "comm"))
		name := strings.TrimSpace(string(comm))
		if name == "app-lib" {
			name = "app"
		}
		for line := range strings.SplitSeq(string(rollup), "\n") {
			key, rest, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			kib, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(rest), " kB"))
			switch key {
			case "Pss":
				r.PSSKiB[name] += kib
				r.Total += kib
			case "Pss_Anon":
				r.Anon += kib
			}
		}
		r.Processes++
	}
	if r.Processes == 0 {
		return errors.New("no processes to measure")
	}
	return nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func median(xs []float64) float64 {
	s := slices.Sorted(slices.Values(xs))
	if len(s)%2 == 0 {
		return (s[len(s)/2-1] + s[len(s)/2]) / 2
	}
	return s[len(s)/2]
}

func report(results []Result) {
	fmt.Println()
	fmt.Println("| file | setup | processes | PSS (MiB) | anonymous (MiB) | by process (MiB) | until /health (ms) |")
	fmt.Println("| --- | --- | --- | --- | --- | --- | --- |")
	for _, r := range results {
		var ready, total, anon []float64
		byName := map[string][]float64{}
		for _, one := range r.Runs {
			ready = append(ready, one.ReadyMs)
			total = append(total, float64(one.Total)/1024)
			anon = append(anon, float64(one.Anon)/1024)
			for name, kib := range one.PSSKiB {
				byName[name] = append(byName[name], float64(kib)/1024)
			}
		}
		var parts []string
		for _, name := range slices.Sorted(maps.Keys(byName)) {
			parts = append(parts, fmt.Sprintf("%s %.1f", name, median(byName[name])))
		}
		fmt.Printf("| %s (%d values) | %s | %d | %.1f | %.1f | %s | %.1f |\n",
			r.File, r.Keys, r.Setup, r.Runs[0].Processes, median(total), median(anon), strings.Join(parts, ", "), median(ready))
	}
	fmt.Println()
	fmt.Println("Medians of", len(results[0].Runs), "runs each.")
}
