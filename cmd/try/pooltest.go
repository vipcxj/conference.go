package main

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/alphadose/itogami"
	"lukechampine.com/frand"
)

func readChan(ch chan []byte, waited *int64) {
	start := time.Now()
	bs := <-ch
	if bs == nil {
		panic(errors.New("nil bs"))
	}
	l := len(bs)
	if l <= 0 {
		panic(fmt.Errorf("wrong size %d", l))
	}
	buf := make([]byte, l)
	repeat := frand.Intn(1000)
	for i := 0; i < repeat; i++ {
		copy(buf, bs)
	}
	elapsed := time.Since(start)
	atomic.AddInt64(waited, elapsed.Nanoseconds())
}

type Context struct {
	ch      chan []byte
	elapsed int64
}

func main() {
	pool := itogami.NewPoolWithFunc(100, func(ctx *Context) {
		readChan(ctx.ch, &ctx.elapsed)
	})
	ch := make(chan []byte, 1024)
	closeCh := make(chan bool)
	outCh := make(chan *Context)
	go func(ch chan []byte, closeCh chan bool) {
		ctx := &Context{
			ch: ch,
		}
		for {
			select {
			case <-closeCh:
				outCh <- ctx
				return
			default:
				for i := 0; i < 100; i++ {
					pool.Invoke(ctx)
				}
			}
		}
	}(ch, closeCh)
	timeout := time.After(time.Second * 10)
	bytes := make([]byte, 4096)
	for {
		stop := false
		select {
		case ch <- bytes:
		case <-timeout:
			stop = true
		}
		if stop {
			break
		}
	}
	closeCh <- true
	ctx := <-outCh
	fmt.Println("In goroutine use time ", (ctx.elapsed / 1000_000), "ms")
}
