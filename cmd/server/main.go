package main

import (
	"context"

	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/entry"
	"github.com/vipcxj/conference.go/log"
)

func main() {
	err := config.Init()
	if err != nil {
		log.Sugar().Panicf("unable to init config, %w", err)
	}
	conf := config.Conf()
	err = conf.Validate()
	if err != nil {
		log.Sugar().Panicf("validate config failed. %w", err)
	}
	// init depend on inited confg
	log.Init(conf.Log.Level, conf.LogProfile())

	entry.Run(context.Background())
}
