package app

import (
	"fmt"

	"github.com/rs/zerolog"
)

// App is used for keep central location of configuration and resources.
type App struct {
	Logger zerolog.Logger
}

// NewApp initialize App instance.
func NewApp() (*App, error) {
	app := &App{}

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
