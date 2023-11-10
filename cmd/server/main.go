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
		log.Sugar().Fatalln(err)
		return
	}
	// init depend on inited confg
	log.Init()

	ch := make(chan error)
	go authserver.Run(ch)
	go signalserver.Run(ch)
	n := 2
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
