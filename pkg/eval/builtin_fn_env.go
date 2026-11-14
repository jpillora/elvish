package eval

import (
	"errors"
	"os"
)

// ErrNonExistentEnvVar is raised by the get-env command when the environment
// variable does not exist.
var ErrNonExistentEnvVar = errors.New("non-existent environment variable")

func init() {
	addBuiltinFns(map[string]any{
		"has-env": func(fm *Frame, key string) bool { _, ok := fm.Evaler.Process.LookupEnv(key); return ok },
		"get-env": func(fm *Frame, key string) (string, error) {
			v, ok := fm.Evaler.Process.LookupEnv(key)
			if !ok {
				return "", ErrNonExistentEnvVar
			}
			return v, nil
		},
		"set-env":   func(fm *Frame, k, v string) error { return fm.Evaler.Process.Setenv(k, v) },
		"unset-env": func(fm *Frame, k string) error { return fm.Evaler.Process.Unsetenv(k) },
	})
}

func hasEnv(key string) bool {
	_, ok := os.LookupEnv(key)
	return ok
}

func getEnv(key string) (string, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return "", ErrNonExistentEnvVar
	}
	return value, nil
}
