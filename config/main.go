package config

import (
	"github.com/gookit/config/v2"
	"github.com/gookit/config/v2/ini"
	"github.com/gookit/config/v2/json5"
	"github.com/gookit/config/v2/toml"
	"github.com/gookit/config/v2/yamlv3"
)

var CONFIGURE = &ConferenceConfigure{}

func Init() {
	config.WithOptions(config.ParseEnv, config.ParseTime, config.ParseDefault)
	config.AddDriver(ini.Driver)
	config.AddDriver(json5.Driver)
	config.AddDriver(yamlv3.Driver)
	config.AddDriver(toml.Driver)

	err := config.LoadExists("conference.yml", "conference.yaml", "conference.json", "conference.toml", "conference.ini")
	if err != nil {
		panic(err)
	}
	err = config.LoadExists("secret.yml", "secret.yaml", "secret.json", "secret.toml", "secret.ini")
	if err != nil {
		panic(err)
	}
	config.LoadOSEnvs(ENVS)
	err = config.LoadFlags(KEYS)
	if err != nil {
		panic(err)
	}
	err = config.Decode(CONFIGURE)
	if err != nil {
		panic(err)
	}
}

func Conf() *ConferenceConfigure {
	return CONFIGURE
}
