package vars

import (
	"errors"
	"os"
)

var errEnvMustBeString = errors.New("environment variable can only be set string values")

type envVariable struct {
	name string
	env  Environment
}

// Environment supplies variables without requiring process-global mutation.
type Environment interface {
	LookupEnv(string) (string, bool)
	Setenv(string, string) error
	Unsetenv(string) error
}

type osEnvironment struct{}

func (osEnvironment) LookupEnv(k string) (string, bool) { return os.LookupEnv(k) }
func (osEnvironment) Setenv(k, v string) error          { return os.Setenv(k, v) }
func (osEnvironment) Unsetenv(k string) error           { return os.Unsetenv(k) }

func (ev envVariable) Set(val any) error {
	if s, ok := val.(string); ok {
		return ev.env.Setenv(ev.name, s)
	}
	return errEnvMustBeString
}

func (ev envVariable) Get() any {
	v, _ := ev.env.LookupEnv(ev.name)
	return v
}

func (ev envVariable) Unset() error {
	return ev.env.Unsetenv(ev.name)
}

func (ev envVariable) IsSet() bool {
	_, ok := ev.env.LookupEnv(ev.name)
	return ok
}

// FromEnv returns a Var corresponding to the named environment variable.
func FromEnv(name string) UnsettableVar {
	return FromEnvWithEnv(name, osEnvironment{})
}

func FromEnvWithEnv(name string, env Environment) UnsettableVar {
	return envVariable{name, env}
}
