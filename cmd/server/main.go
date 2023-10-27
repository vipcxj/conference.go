package main

import (
	"log"
	"os"

	"github.com/vipcxj/conference.go/authserver"
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/signalserver"
)

func main() {
	err := config.Init()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
		return
	}
	ch := make(chan error)
	go authserver.Run(ch)
	go signalserver.Run(ch)
	n := 2
	for {
		err = <- ch
		if !errors.IsOk(err) {
			log.Fatal(err)
			n = 0
			os.Exit(1)
			return
		}
		n--
		if n == 0 {
			break
		}
	}
}
