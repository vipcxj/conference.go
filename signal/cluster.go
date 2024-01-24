package signal

import (
	"context"
	"fmt"

	"github.com/vipcxj/conference.go/config"
)

func roomManage(ctx context.Context, errChan chan error) {
	client, err := MakeRedisClient(config.Conf().Cluster.Redis)
	if err != nil {
		errChan <- err
		return
	}
	ps := client.PSubscribe(ctx, MakeRedisKey("room:*"))
	defer ps.Close()
	ch := ps.Channel()
	for msg := range ch {
		fmt.Println(msg.Channel, msg.Payload)
	}
}

func InitCluster(ctx context.Context, errChan chan error) {
	go roomManage(ctx, errChan)
}