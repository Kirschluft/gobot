package gobot

import (
	"os"

	"github.com/sirupsen/logrus"
)

var (
	Logger = logrus.New()
)

func init() {
	// Initial set up for the logger
	Logger.Out = os.Stdout
	Logger.Formatter = new(logrus.TextFormatter)
	Logger.Formatter.(*logrus.TextFormatter).FullTimestamp = true
}

func SetLoggerConfig(conf Configuration) {
	switch conf.LogFormat {
	case "text":
		// default already set
	case "json":
		Logger.Formatter = new(logrus.JSONFormatter)
	case "plain":
		Logger.Formatter = new(logrus.TextFormatter)
		Logger.Formatter.(*logrus.TextFormatter).DisableColors = true
	default:
		Logger.Warn("Logging format not supported. Falling back to default.")
	}

	switch conf.LogLevel {
	case "debug":
		Logger.SetLevel(logrus.DebugLevel)
	case "prod":
		Logger.SetLevel(logrus.WarnLevel)
	case "info":
		Logger.SetLevel(logrus.InfoLevel)
	default:
		Logger.Warn("Log level not supported. Falling back to info default.")
	}

	if _, ok := Logger.Formatter.(*logrus.TextFormatter); ok {
		switch conf.LogTimeStamp {
		case "on":
			Logger.Formatter.(*logrus.TextFormatter).DisableTimestamp = false
		case "off":
			Logger.Formatter.(*logrus.TextFormatter).DisableTimestamp = true
		default:
			Logger.Warn("Log time stamp not supported. Falling back to on default.")
		}
	} else {
		switch conf.LogTimeStamp {
		case "on":
			Logger.Formatter.(*logrus.JSONFormatter).DisableTimestamp = false
		case "off":
			Logger.Formatter.(*logrus.JSONFormatter).DisableTimestamp = true
		default:
			Logger.Warn("Log time stamp not supported. Falling back to on default.")
		}
	}

	if conf.LogFile != "none" {
		if file, err := os.OpenFile(conf.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err != nil {
			Logger.Warn("Failed to log to specified file + " + conf.LogFile + ". Falling back to default stdout.")
			Logger.Out = os.Stdout
		} else {
			Logger.Info("Successfully accessed log file " + conf.LogFile + ".")
			Logger.Out = file
		}
	} else {
		Logger.Warn("Log file not specified. Falling back to default stdout.")
		Logger.Out = os.Stdout
	}
}
