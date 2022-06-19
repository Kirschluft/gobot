package gobot

import (
	"reflect"

	"github.com/tkanos/gonfig"
)

type Configuration struct {
	LogFile       string
	LogLevel      string
	LogFormat     string
	LogTimeStamp  string
	DiscordToken  string
	LavalinkPW    string
	LavalinkHost  string
	LavalinkPort  string
	LavalinkNode  string
	ResumeKey     string
	ResumeTimeOut int
	Secure        bool
}

func getconfig(file string) (Configuration, error) {
	conf := Configuration{}
	err := gonfig.GetConf(file, &conf)
	return conf, err
}

func ReadConfig(configFile string) Configuration {
	Logger.Info("Setting up configurations")

	// Read configurations from file or set to default values
	conf, err := getconfig(configFile)
	if err != nil {
		Logger.Fatal("Could not load configuration file " + configFile)
	}

	values := reflect.ValueOf(conf)
	for i := 0; i < values.NumField(); i++ {
		if v := values.Field(i).Interface(); v == "" {
			Logger.Fatal("Value not set for environment variable " + values.Type().Field(i).Name)
		}
	}

	return conf
}
