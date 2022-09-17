package app

import (
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	// "github.com/rs/zerolog/log".
)

func init() {
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().UTC()
	}

	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		short := file
		for i := len(file) - 1; i > 0; i-- {
			if file[i] == '/' {
				short = file[i+1:]
				break
			}
		}
		file = short
		return file + ":" + strconv.Itoa(line)
	}
}

func (a *App) setLogger() error {
	logger := zerolog.New(os.Stdout).With().Caller().Timestamp().Logger()
	defer func(a *App, logger zerolog.Logger) {
		a.Logger = &logger
		// log.Logger = logger
	}(a, logger)

	val, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		return nil
	}

	lvl, err := zerolog.ParseLevel(val)
	if err == nil {
		logger = logger.Level(lvl)
	} else {
		l := &logger
		l.Err(err).Msgf("unknown LOG_LEVEL value: \"%s\"", val)
	}

	return nil
}
