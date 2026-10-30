package dotenvx

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

// Read reads the given env files and returns their values without
// modifying the process environment. A file that is a directory stands for
// the .env in it. Paths are used as given: unlike the path option of
// dotenvx's config, a leading "~" isn't expanded and commas don't separate
// paths. With no files, like dotenvx, Read reads the comma-separated files
// named by DOTENV_FILE, DOTENV_PATH or DOTENV_F, the first that isn't
// empty; or else, if DOTENV_PRIVATE_KEY* variables are set, the files they
// stand for, in the environment's order (.env.production for
// DOTENV_PRIVATE_KEY_PRODUCTION); or else ".env". It looks for those in
// the existing environment only: with Env set, that is Env, where dotenvx
// would also check the process environment.
//
// Files are read in order, and each one sees the values set by the files
// before it as existing values. So unless Overload is set, the first file to
// set a key wins, and with Overload the last one does.
//
// A file that can't be read doesn't stop the others: Read returns the values
// it could read together with the errors. To make a single file optional,
// ignore an error that wraps fs.ErrNotExist. With several files, that would
// also ignore errors joined with it.
func (o Options) Read(files ...string) (map[string]string, error) {
	values, _, err := o.read(files, o.environment())
	return values, err
}

// setting is a value to set in the environment.
type setting struct{ key, value string }

// read reads files into env, updating it as dotenvx updates the process
// environment after each file. It returns the values read, and every
// setting it made to env, in order, for Load to repeat.
func (o Options) read(files []string, env environment) (map[string]string, []setting, error) {
	if len(files) == 0 {
		files = defaultFiles(env)
	}
	values := map[string]string{}
	var settings []setting
	var errs []error
	for _, file := range files {
		if info, err := os.Stat(file); err == nil && info.IsDir() {
			file = filepath.Join(file, ".env")
		}
		src, err := os.ReadFile(file)
		if err != nil {
			errs = append(errs, fmt.Errorf("dotenvx: %w", err))
			continue
		}
		parsed, keys, err := o.parse(decode(src), env, o.keyFilesFor(file, env), "dotenvx: "+file+": ")
		if err != nil {
			errs = append(errs, err)
		}
		for _, k := range keys {
			env.set(k, parsed[k])
			values[k] = parsed[k]
			settings = append(settings, setting{k, parsed[k]})
		}
	}
	return values, settings, errors.Join(errs...)
}

// defaultFiles returns the files to read when none are given. Like
// dotenvx, it looks the names up case-sensitively, even on Windows.
func defaultFiles(env environment) []string {
	var files []string
	for _, name := range []string{"DOTENV_FILE", "DOTENV_PATH", "DOTENV_F"} {
		if v := env.vars[name]; v != "" {
			for _, file := range strings.Split(v, ",") {
				if file = strings.TrimFunc(file, isJSSpace); file != "" {
					files = append(files, file)
				}
			}
			break
		}
	}
	if len(files) > 0 {
		return files
	}
	// Set private keys name their files, in the environment's order.
	for _, name := range env.names {
		if strings.HasPrefix(name, "DOTENV_PRIVATE_KEY") {
			files = append(files, guessFile(name))
		}
	}
	if len(files) == 0 {
		return []string{".env"}
	}
	return files
}

// Load reads env files as Read does, and sets their values in the process
// environment, in the order dotenvx would set them. It ignores Env. Like
// Read, it sets every value it could read even if some files fail.
func (o Options) Load(files ...string) error {
	o.Env = nil
	_, settings, err := o.read(files, o.environment())
	errs := []error{err}
	failed := map[string]bool{}
	for _, s := range settings {
		if err := os.Setenv(s.key, s.value); err != nil && !failed[s.key] {
			failed[s.key] = true
			errs = append(errs, fmt.Errorf("dotenvx: set %s: %w", s.key, err))
		}
	}
	return errors.Join(errs...)
}

// Read reads env files with the default Options.
func Read(files ...string) (map[string]string, error) {
	return Options{}.Read(files...)
}

// Load loads env files into the process environment with the default
// Options. Variables that are already set keep their values.
func Load(files ...string) error {
	return Options{}.Load(files...)
}

// Overload loads env files into the process environment, replacing
// variables that are already set.
func Overload(files ...string) error {
	return Options{Overload: true}.Load(files...)
}

// decode turns dotenv source into text. Like dotenvx, it decodes source that
// starts with a UTF-16LE byte order mark from UTF-16LE, dropping a trailing
// odd byte, and takes anything else as UTF-8. Unpaired surrogates, which
// dotenvx keeps in its JavaScript strings, become U+FFFD.
func decode(src []byte) string {
	if len(src) < 2 || src[0] != 0xff || src[1] != 0xfe {
		return string(src)
	}
	units := make([]uint16, (len(src)-2)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(src[2+2*i:])
	}
	return string(utf16.Decode(units))
}
