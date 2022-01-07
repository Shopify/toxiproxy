package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Shopify/toxiproxy/v2"
)

type cliArguments struct {
	host         string
	port         string
	config       string
	seed         int64
	printVersion bool
}

func parseArguments() cliArguments {
	result := cliArguments{}
	flag.StringVar(&result.host, "host", "localhost",
		"Host for toxiproxy's API to listen on")
	flag.StringVar(&result.port, "port", "8474",
		"Port for toxiproxy's API to listen on")
	flag.StringVar(&result.config, "config", "",
		"JSON file containing proxies to create on startup")
	flag.Int64Var(&result.seed, "seed", time.Now().UTC().UnixNano(),
		"Seed for randomizing toxics with")
	flag.BoolVar(&result.printVersion, "version", false,
		`print the version (default "false")`)
	flag.Parse()

	return result
}

func main() {
	// Handle SIGTERM to exit cleanly
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	go func() {
		<-signals
		os.Exit(0)
	}()

	cli := parseArguments()
	run(cli)
}

func run(cli cliArguments) {
	if cli.printVersion {
		fmt.Printf("toxiproxy-server version %s\n", toxiproxy.Version)
		return
	}

	setupLogger()

	rand.Seed(cli.seed)

	server := toxiproxy.NewServer()
	if len(cli.config) > 0 {
		server.PopulateConfig(cli.config)
	}

	server.Listen(cli.host, cli.port)
}

func setupLogger() {
	val, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		return
	}

	lvl, err := logrus.ParseLevel(val)
	if err == nil {
		logrus.SetLevel(lvl)
		return
	}

	valid_levels := make([]string, len(logrus.AllLevels))
	for i, level := range logrus.AllLevels {
		valid_levels[i] = level.String()
	}
	levels := strings.Join(valid_levels, ",")

	logrus.Errorf("unknown LOG_LEVEL value: \"%s\", use one of: %s", val, levels)
}
