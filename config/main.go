package config

import (
	"fmt"
	"os"

	"github.com/gookit/config/v2"
	"github.com/gookit/config/v2/ini"
	"github.com/gookit/config/v2/json5"
	"github.com/gookit/config/v2/toml"
	"github.com/gookit/config/v2/yamlv3"
)

var CONFIGURE = &ConferenceConfigure{}

func InitConfig(conf any, keys []string, conf_name string, secret_conf_name string) error {
	config.WithOptions(config.ParseEnv, config.ParseTime, config.ParseDefault)
	config.AddDriver(ini.Driver)
	config.AddDriver(json5.Driver)
	config.AddDriver(yamlv3.Driver)
	config.AddDriver(toml.Driver)
	var err error
	confPath, ok := os.LookupEnv("CONF_CONFIG_PATH")
	if ok && confPath != "" {
		err = config.LoadFiles(confPath)
	} else {
		err = config.LoadExists(
			fmt.Sprintf("%s.yml", conf_name),
			fmt.Sprintf("%s.yaml", conf_name),
			fmt.Sprintf("%s.json", conf_name),
			fmt.Sprintf("%s.toml", conf_name),
			fmt.Sprintf("%s.ini", conf_name),
		)
	}
	if err != nil {
		return err
	}
	secretPath, ok := os.LookupEnv("CONF_SECRET_PATH")
	if ok && secretPath != "" {
		err = config.LoadFiles(secretPath)
	} else {
		err = config.LoadExists(
			fmt.Sprintf("%s.yml", secret_conf_name),
			fmt.Sprintf("%s.yaml", secret_conf_name),
			fmt.Sprintf("%s.json", secret_conf_name),
			fmt.Sprintf("%s.toml", secret_conf_name),
			fmt.Sprintf("%s.ini", secret_conf_name),
		)
	}
	if err != nil {
		return err
	}
	err = config.LoadFlags(keys)
	if err != nil {
		return err
	}
	err = config.Decode(conf)
	if err != nil {
		return err
	}
	return nil
}

func Init() error {
	return InitConfig(CONFIGURE, KEYS, "conference", "secret")
}

func Conf() *ConferenceConfigure {
	return CONFIGURE
}
