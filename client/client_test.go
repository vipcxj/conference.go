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
	"github.com/vipcxj/conference.go/errors"
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
	log.Init(conf.Log.Level, conf.LogProfile(), conf.Log.File)
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

func TestUserInfo(t *testing.T) {
	ctx := context.Background()
	c, cancel, err := createClient(ctx, "1", "room", true)
	defer cancel(nil)
	if err != nil {
		t.Fatalf("unable to create client, %v", err)
	}
	info, err := c.UserInfo(ctx)
	if err != nil {
		t.Fatalf("unable to get the user info, %v", err)
	}
	utils.AssertEqual(t, "1", info.Key)
	utils.AssertEqual(t, "1", info.UserId)
	utils.AssertEqual(t, "user1", info.UserName)
	utils.AssertEqual(t, "test", info.Role)
	rooms, err := c.GetRooms(ctx)
	if err != nil {
		t.Fatalf("unable to get the rooms, %v", err)
	}
	utils.AssertEqualSlice(t, []string{"room"}, rooms)
}

func TestJoinAndLeave(t *testing.T) {
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
	err = c.Leave(ctx, "root.room1")
	if err != nil {
		t.Errorf("unable to leave root.room1, %v", err)
	}
	inRoom, err = c.IsInRoom(ctx, "root.room1")
	if err != nil {
		t.Errorf("unable to predicate whether is in room root.room1")
	}
	utils.AssertFalse(t, inRoom)
	err = c.Leave(ctx, "root.room2")
	if err != nil {
		t.Errorf("unable to leave root.room2, %v", err)
	}
	inRoom, err = c.IsInRoom(ctx, "root.room2")
	if err != nil {
		t.Errorf("unable to predicate whether is in room root.room2")
	}
	utils.AssertFalse(t, inRoom)
}

func TestSendMessage(t *testing.T) {
	ctx := context.Background()
	ch1 := make(chan struct{})
	ch2 := make(chan struct{})
	c1, cancel1, err := createClient(ctx, "1", "room", true)
	if err != nil {
		t.Fatalf("unable to create client 1, %v", err)
	}
	defer cancel1(nil)
	err = c1.MakeSureConnect(ctx, "")
	if err != nil {
		t.Fatalf("failed to make sure c1 connected, %v", err)
	}

	c2, cancel2, err := createClient(ctx, "2", "room", true)
	if err != nil {
		t.Fatalf("unable to create client 2, %v", err)
	}
	defer cancel2(nil)
	err = c2.MakeSureConnect(ctx, "")
	if err != nil {
		t.Fatalf("failed to make sure c2 connected, %v", err)
	}

	c1.OnMessage("hello", func(content string, ack client.CustomAckFunc, room, from, to string) (remained bool) {
		utils.AssertEqual(t, "room", room)
		utils.AssertEqual(t, c2.Id(), from)
		utils.AssertEqual(t, c1.Id(), to)
		utils.AssertEqual(t, "hello", content)
		utils.AssertTrue(t, ack == nil)
		close(ch1)
		return false
	})
	c2.OnMessage("hello", func(content string, ack client.CustomAckFunc, room, from, to string) (remained bool) {
		utils.AssertEqual(t, "room", room)
		utils.AssertEqual(t, c1.Id(), from)
		utils.AssertEqual(t, c2.Id(), to)
		utils.AssertEqual(t, "hello", content)
		utils.AssertTrue(t, ack == nil)
		close(ch2)
		return false
	})

	c1.SendMessage(ctx, false, "hello", "hello", c2.Id(), "room")
	<-ch2
	c2.SendMessage(ctx, false, "hello", "hello", c1.Id(), "room")
	<-ch1
	c1.OnMessage("hello", func(content string, ack client.CustomAckFunc, room, from, to string) (remained bool) {
		utils.AssertEqual(t, "room", room)
		utils.AssertEqual(t, c2.Id(), from)
		utils.AssertEqual(t, c1.Id(), to)
		utils.AssertTrue(t, ack != nil)
		ack(content, nil)
		return false
	})
	res, err := c2.SendMessage(ctx, true, "hello", "world", c1.Id(), "room")
	if err != nil {
		t.Fatalf("failed to send msg to client 1, %v", err)
	}
	utils.AssertEqual(t, "world", res)
	c2.OnMessage("error", func(content string, ack client.CustomAckFunc, room, from, to string) (remained bool) {
		utils.AssertEqual(t, "room", room)
		utils.AssertEqual(t, c1.Id(), from)
		utils.AssertEqual(t, c2.Id(), to)
		utils.AssertTrue(t, ack != nil)
		ack("", errors.FatalError("test error"))
		return false
	})
	res, err = c1.SendMessage(ctx, true, "error", "error", c2.Id(), "room")
	utils.AssertTrue(t, err != nil)
	ce, ok := err.(*errors.ConferenceError)
	utils.AssertTrue(t, ok)
	utils.AssertEqual(t, "test error", ce.Error())
	utils.AssertEqual(t, "", res)
}

func TestKeepAlive(t *testing.T) {
	ctx := context.Background()
	c1, cancel1, err := createClient(ctx, "1", "room", true)
	if err != nil {
		t.Fatalf("unable to create client 1, %v", err)
	}
	defer cancel1(nil)
	id1, err := c1.MakeSureId(ctx)
	if err != nil {
		t.Fatalf("unable to get id of client 1, %v", err)
	}
	utils.AssertTrue(t, id1 != "")
	rc1, err := c1.Roomed(ctx, "room")
	if err != nil {
		t.Fatalf("unable to create roomed client 1, %v", err)
	}
	c2, cancel2, err := createClient(ctx, "2", "room", true)
	if err != nil {
		t.Fatalf("unable to create client 2, %v", err)
	}
	defer cancel2(nil)
	id2, err := c2.MakeSureId(ctx)
	if err != nil {
		t.Fatalf("unable to get id of client 2, %v", err)
	}
	utils.AssertTrue(t, id2 != "")
	rc2, err := c2.Roomed(ctx, "room")
	if err != nil {
		t.Fatalf("unable to create roomed client 2, %v", err)
	}
	stop1, err := rc1.KeepAlive(ctx, id2, client.KEEP_ALIVE_MODE_ACTIVE, time.Second, func(kaCtx *client.KeepAliveContext) (stop bool) {
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
		t.Fatalf("client 1 failed to keep alive, %v", err)
	}
	defer stop1()
	stop2, err := rc2.KeepAlive(ctx, id1, client.KEEP_ALIVE_MODE_PASSIVE, time.Second*2, func(kaCtx *client.KeepAliveContext) (stop bool) {
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
		t.Fatalf("client 2 failed to keep alive, %v", err)
	}
	defer stop2()
	time.Sleep(time.Second * 10)
}
