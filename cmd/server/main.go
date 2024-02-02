package main

import (
	"github.com/vipcxj/conference.go/authserver"
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/log"
	"github.com/vipcxj/conference.go/signalserver"
)

func main() {
	// init depend on empty confg
	log.Init()
	err := config.Init()
	if err != nil {
		log.Sugar().Fatal(err)
		return
	}
	err = config.Conf().Validate()
	if err != nil {
		log.Sugar().Fatal(err)
	}
	// init depend on inited confg
	log.Init()

	ch := make(chan error)
	n := 1
	go authserver.Run(ch)
	cfgs := config.Conf().Split()
	for _, cfg := range cfgs {
		n ++
		go signalserver.Run(cfg, ch)
	}
	for {
		err = <-ch
		if !errors.IsOk(err) {
			log.Sugar().Fatalln(err)
			return
		}
		n--
		if n == 0 {
			break
		}
	}
}
