package main

import (
	"flag"

	"github.com/c0nvulsiv3/gobot/gobot"
)

var (
	conf gobot.Configuration
)

func init() {
	// Parse input for config file
	var configFile string
	flag.StringVar(&configFile, "config", "config.json", "Path to configuration file")
	flag.Parse()

	if len(configFile) == 0 {
		gobot.Logger.Warn("Usage: gobot -config")
		flag.PrintDefaults()
	}

	conf = gobot.ReadConfig(configFile)

	// Set up logger with the specified configurations
	gobot.Logger.Info("Setting up logger")
	gobot.SetLoggerConfig(conf)
}

func main() {
	gobot.StartBot(conf)
}
