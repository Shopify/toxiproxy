package app

import (
	"fmt"
	"math/rand"
	"net"

	"github.com/rs/zerolog"

	"github.com/Shopify/toxiproxy/v2/collectors"
)

type ServerOptions struct {
	Host           string
	Port           string
	Config         string
	Seed           int64
	PrintVersion   bool
	ProxyMetrics   bool
	RuntimeMetrics bool
}

// App is used for keep central location of configuration and resources.
type App struct {
	Addr                  string
	Logger                *zerolog.Logger
	Metrics               *collectors.MetricsContainer
	Config                string
	EnabledProxyMetrics   bool
	EnabledRuntimeMetrics bool
}

// NewApp initialize App instance.
func NewApp(options ServerOptions) (*App, error) {
	rand.Seed(options.Seed)

	app := &App{
		Addr:                  net.JoinHostPort(options.Host, options.Port),
		Config:                options.Config,
		EnabledProxyMetrics:   options.ProxyMetrics,
		EnabledRuntimeMetrics: options.RuntimeMetrics,
	}

	start([]unit{
		{"Logger", app.setLogger},
		{"Metrics", app.setMetrics},
	})

	return app, nil
}

// unit keeps initialization tasks.
// could be used later for graceful stop function per service if it is required.
type unit struct {
	name  string
	start func() error
}

// start run initialized step for resource.
// could be wrapped with debug information and resource usage.
func start(units []unit) error {
	for _, unit := range units {
		err := unit.start()
		if err != nil {
			return fmt.Errorf("initialization %s failed: %w", unit.name, err)
		}
	}
	return nil
}
