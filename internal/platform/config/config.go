// Package config is the service's settings, from the environment only
// (twelve-factor): each a field here, a line in .env.example, and its
// default beside it. A required one missing stops the start with its name.
package config

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config -.
type Config struct {
	// Addr is where the server listens.
	Addr string `env:"HTTP_ADDR" envDefault:":8080"`
	// APIPrefix is where the client's contract is served: the client's base URL ends with it.
	APIPrefix string `env:"API_PREFIX" envDefault:"/api"`
	// LogLevel is debug, info, warn or error.
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
	// CORSOrigins may call the API from a browser with its cookies: named, never "*".
	CORSOrigins []string `env:"CORS_ORIGINS" envSeparator:","`
	// ShutdownTimeout is how long requests in flight get to finish on a stop.
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"10s"`
}

// Load reads the environment. A value that can't be read is refused by
// the variable's name, which is what the person starting the service sets.
func Load() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		if parse, ok := errors.AsType[env.ParseError](err); ok {
			return Config{}, fmt.Errorf("config: %s: %w", variable(parse.Name), parse.Err)
		}

		return Config{}, fmt.Errorf("config: %w", err)
	}

	return cfg, nil
}

// variable is the environment variable of a Config field.
func variable(field string) string {
	f, ok := reflect.TypeFor[Config]().FieldByName(field)
	if !ok {
		return field
	}

	name, _, _ := strings.Cut(f.Tag.Get("env"), ",")

	return name
}
