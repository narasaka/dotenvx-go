package dotenvx

import (
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"unicode/utf16"
)

// Read reads the given env files, or ".env" if none are given, and returns
// their values without modifying the process environment.
//
// Files are read in order, and each one sees the values set by the files
// before it as existing values. So unless Overload is set, the first file to
// set a key wins, and with Overload the last one does.
//
// A file that can't be read doesn't stop the others: Read returns the values
// it could read together with the errors.
func (o Options) Read(files ...string) (map[string]string, error) {
	if len(files) == 0 {
		files = []string{".env"}
	}
	existing := o.environment()
	values := map[string]string{}
	var errs []error
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			errs = append(errs, fmt.Errorf("dotenvx: %w", err))
			continue
		}
		parsed, err := o.parse(decode(src), existing, "dotenvx: "+file+": ")
		if err != nil {
			errs = append(errs, err)
		}
		for k, v := range parsed {
			existing.vars[k] = v
			values[k] = v
		}
	}
	return values, errors.Join(errs...)
}

// Load reads the given env files, or ".env" if none are given, as Read
// does, and sets their values in the process environment. Like Read, it
// sets every value it could read even if some files fail.
func (o Options) Load(files ...string) error {
	values, err := o.Read(files...)
	errs := []error{err}
	for _, k := range slices.Sorted(maps.Keys(values)) {
		if err := os.Setenv(k, values[k]); err != nil {
			errs = append(errs, fmt.Errorf("dotenvx: set %s: %w", k, err))
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
// odd byte, and takes anything else as UTF-8.
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
