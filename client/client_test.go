package client_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/lesismal/nbio/nbhttp"
	"github.com/vipcxj/conference.go/client"
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/entry"
	"github.com/vipcxj/conference.go/log"
	"github.com/vipcxj/conference.go/utils"
)

func WaitHealthy(host string, path string, ctx context.Context) error {
	for {
		resp, err := http.Get(fmt.Sprintf("%s%s", host, path))
		if err == nil && resp.StatusCode == http.StatusOK {
			return nil
		}
		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return err
		}
	}
}

func setup(ctx context.Context) {
	os.Args = append(
		os.Args,
		"--log.level=debug",
		"--hostOrIp=localhost",
		"--authServer.enable",
		"--authServer.host=localhost",
		"--authServer.port=3105",
		"--authServer.cors=*",
		"--authServer.healthy.enable",
		"--signal.hostOrIp=localhost",
		"--signal.port=8188",
		"--signal.cors=*",
		"--signal.healthy.enable",
		"--router.port=43211",
		"--record.enable=false",
		"--cluster.enable=false",
	)
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
	go entry.Run(ctx)
	ctx1, cancel1 := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
	defer cancel1()
	err = WaitHealthy("http://localhost:3105", "/healthz", ctx1)
	if err != nil {
		panic(err)
	}
	ctx2, cancel2 := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
	defer cancel2()
	err = WaitHealthy("http://localhost:8188", "/healthz", ctx2)
	if err != nil {
		panic(err)
	}
}

func TestMain(m *testing.M) {
	ctx, cancel := context.WithCancel(context.Background())
	setup(ctx)
	code := m.Run()
	cancel()
	os.Exit(code)
}

func createClient(ctx context.Context, uid string, room string, autoJoin bool) (c client.Client, cancel context.CancelCauseFunc, err error) {
	engine := nbhttp.NewEngine(nbhttp.Config{})
	err = engine.Start()
	if err != nil {
		return
	}
	ctx, ctxCancel := context.WithCancelCause(ctx)
	cancel = func(cause error) {
		engine.Cancel()
		ctxCancel(cause)
	}
	token, err := client.GetToken("http://localhost:3105", uid, uid, fmt.Sprintf("user%s", uid), room, "test", autoJoin)
	if err != nil {
		cancel(err)
		return
	}
	c = client.NewWebsocketClient(ctx, &client.WebsocketClientConfigure{
		WebSocketSignalConfigure: client.WebSocketSignalConfigure{
			Url:   "ws://localhost:8188/ws",
			Token: token,
		},
	}, engine)
	return
}

func TestJoin(t *testing.T) {
	ctx := context.Background()
	c, cancel, err := createClient(ctx, "1", "root.*", false)
	defer cancel(nil)
	if err != nil {
		t.Fatalf("unable to create client, %v", err)
	}
	err = c.Join(ctx, "root.room1")
	if err != nil {
		t.Errorf("unable to join root.room1, %v", err)
	}
	inRoom, err := c.IsInRoom(ctx, "root.room1")
	if err != nil {
		t.Errorf("unable to predicate whether is in room root.room1")
	}
	utils.AssertTrue(t, inRoom)
	err = c.Join(ctx, "root.room2")
	if err != nil {
		t.Errorf("unable to join root.room2, %v", err)
	}
	inRoom, err = c.IsInRoom(ctx, "root.room2")
	if err != nil {
		t.Errorf("unable to predicate whether is in room root.room2")
	}
	utils.AssertTrue(t, inRoom)
	err = c.Join(ctx, "room1")
	if err == nil {
		t.Errorf("room1 should not be joined")
	} else {
		t.Logf("failed to join room1 with err \"%v\", this is expect result", err)
	}
	inRoom, err = c.IsInRoom(ctx, "room1")
	if err != nil {
		t.Errorf("unable to predicate whether is in room room1")
	}
	utils.AssertFalse(t, inRoom)
	err = c.Join(ctx, "room2")
	if err == nil {
		t.Errorf("room2 should not be joined")
	} else {
		t.Logf("failed to join room2 with err \"%v\", this is expect result", err)
	}
	inRoom, err = c.IsInRoom(ctx, "room2")
	if err != nil {
		t.Errorf("unable to predicate whether is in room room2")
	}
	utils.AssertFalse(t, inRoom)
}

func TestKeepAlive(t *testing.T) {
	ctx := context.Background()
	c1, cancel1, err := createClient(ctx, "1", "room", true)
	if err != nil {
		t.Errorf("unable to create client 1, %v", err)
		return
	}
	defer cancel1(nil)
	rc1, err := c1.Roomed(ctx, "room")
	if err != nil {
		t.Errorf("unable to create roomed client 1, %v", err)
		return
	}
	c2, cancel2, err := createClient(ctx, "2", "room", true)
	if err != nil {
		t.Errorf("unable to create client 2, %v", err)
		return
	}
	defer cancel2(nil)
	rc2, err := c2.Roomed(ctx, "room")
	if err != nil {
		t.Errorf("unable to create roomed client 2, %v", err)
		return
	}
	stop1, err := rc1.KeepAlive(ctx, "2", client.KEEP_ALIVE_MODE_ACTIVE, time.Second, func(kaCtx *client.KeepAliveContext) (stop bool) {
		if kaCtx.Err != nil {
			t.Errorf("client 1 failed to keep alive with client 2, %v", kaCtx.Err)
			return true
		}
		if kaCtx.TimeoutNum > 1 {
			t.Errorf("pong timeout")
			return true
		}
		return false
	})
	if err != nil {
		t.Errorf("client 1 failed to keep alive, %v", err)
		return
	}
	defer stop1()
	stop2, err := rc2.KeepAlive(ctx, "1", client.KEEP_ALIVE_MODE_PASSIVE, time.Second*2, func(kaCtx *client.KeepAliveContext) (stop bool) {
		if kaCtx.Err != nil {
			t.Errorf("client 2 failed to keep alive with client 1, %v", kaCtx.Err)
			return true
		}
		if kaCtx.TimeoutNum > 0 {
			t.Errorf("ping timeout")
			return true
		}
		return false
	})
	if err != nil {
		t.Errorf("client 2 failed to keep alive, %v", err)
		return
	}
	defer stop2()
	time.Sleep(time.Second * 10)
}
