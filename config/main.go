package config

import (
	"os"

	"github.com/gookit/config/v2"
	"github.com/gookit/config/v2/ini"
	"github.com/gookit/config/v2/json5"
	"github.com/gookit/config/v2/toml"
	"github.com/gookit/config/v2/yamlv3"
	"github.com/vipcxj/conference.go/errors"
)

var CONFIGURE = &ConferenceConfigure{}

func Init() error {
	config.WithOptions(config.ParseEnv, config.ParseTime, config.ParseDefault)
	config.AddDriver(ini.Driver)
	config.AddDriver(json5.Driver)
	config.AddDriver(yamlv3.Driver)
	config.AddDriver(toml.Driver)
	var err error
	config.LoadOSEnvs(ENVS)
	confPath, ok := os.LookupEnv("CONF_CONFIG_PATH")
	if ok && confPath != "" {
		err = config.LoadFiles(confPath)
	} else {
		err = config.LoadExists("conference.yml", "conference.yaml", "conference.json", "conference.toml", "conference.ini")
	}
	if err != nil {
		return err
	}
	secretPath, ok := os.LookupEnv("CONF_SECRET_PATH")
	if ok && secretPath != "" {
		err = config.LoadFiles(secretPath)
	} else {
		err = config.LoadExists("secret.yml", "secret.yaml", "secret.json", "secret.toml", "secret.ini")
	}
	if err != nil {
		return err
	}
	err = config.LoadFlags(KEYS)
	if err != nil {
		return err
	}
	err = config.Decode(CONFIGURE)
	if err != nil {
		return err
	}
	if CONFIGURE.Ip == "" {
		return errors.FatalError("The configure property ip is required.")
	}
	return nil
}

func Conf() *ConferenceConfigure {
	return CONFIGURE
}
