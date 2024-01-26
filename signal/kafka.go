package signal

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/aws"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	"github.com/twmb/franz-go/plugin/kprom"
	"github.com/twmb/tlscfg"
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/log"
	"github.com/vipcxj/conference.go/utils"
)

type tp struct {
	t string
	p int32
}

type pconsumer struct {
	cl * kgo.Client
	topic string
	partition int32
	worker func(record *kgo.Record)

	quit chan struct{}
	done chan struct{}
	recs chan kgo.FetchTopicPartition
}

func (pc *pconsumer) consume(ctx context.Context) {
	defer close(pc.done)
	log.Sugar().Infof("start consuming kafka topic %s of partition %d", pc.topic, pc.partition)
	defer log.Sugar().Infof("stop consuming kafka topic %s of partition %d", pc.topic, pc.partition)

	for {
		select {
		case <-ctx.Done():
			return
		case <- pc.quit:
			return
		case p := <- pc.recs:
			p.EachRecord(pc.worker)
			pc.cl.MarkCommitRecords(p.Records...)
		}
	}
}

type KafkaClient struct {
	cl *kgo.Client
	consumers map[tp]*pconsumer
	workers map[string]func(record *kgo.Record)
	metrics *kprom.Metrics
	group string
	topics []string
}

func parseKafkaAddrs(addrs string) []string {
	return utils.MapSlice(strings.Split(addrs, ","), func(addr string) (mapped string, remove bool) {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			return addr, true
		} else {
			return addr, false
		}
	})
}

type KafkaOpt func(client *KafkaClient)

func MakeKafkaTopic(topic string) string {
	prefix := config.Conf().Cluster.Kafka.TopicPrefix
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return topic
	} else {
		return fmt.Sprintf("%s%s", prefix, topic)
	}
}

func KafkaOptInstallMetrics(installer func(metrics *kprom.Metrics)) KafkaOpt {
	return func(client *KafkaClient) {
		if client.metrics != nil {
			installer(client.metrics)
		}
	}
}

func KafkaOptGroup(group string) KafkaOpt {
	return func(client *KafkaClient) {
		client.group = group
	}
}

func KafkaOptTopics(topics... string) KafkaOpt {
	return func(client *KafkaClient) {
		client.topics = topics
	}
}

func KafkaOptWorkers(workers map[string]func(*kgo.Record)) KafkaOpt {
	return func(client *KafkaClient) {
		client.workers = workers
	}
}

func NewKafkaClient(copts... KafkaOpt) (*KafkaClient, error) {
	conf := &config.Conf().Cluster.Kafka
	var opts []kgo.Opt

	addrs := parseKafkaAddrs(conf.Addrs)
	if len(addrs) == 0 {
		return nil, nil
	}
	opts = append(opts, kgo.SeedBrokers(addrs...))
	if conf.MaxBufferedRecords > 0 {
		opts = append(opts, kgo.MaxBufferedRecords(conf.MaxBufferedRecords))
	}
	if conf.MaxBufferedBytes > 0 {
		opts = append(opts, kgo.MaxBufferedBytes(conf.MaxBufferedBytes))
	}
	if conf.FetchMaxBytes > 0 {
		opts = append(opts, kgo.FetchMaxBytes(int32(conf.FetchMaxBytes)))
	}
	if conf.BatchMaxBytes > 0 {
		opts = append(opts, kgo.ProducerBatchMaxBytes(int32(conf.BatchMaxBytes)))
	}
	if conf.NoIdempotency {
		opts = append(opts, kgo.DisableIdempotentWrite())
	}
	var metrics *kprom.Metrics
	if conf.Prometheus.Enable {
		metrics = kprom.NewMetrics(conf.Prometheus.Namespace)
		opts = append(opts, kgo.WithHooks(metrics))
	}
	if conf.LingerMs > 0 {
		opts = append(opts, kgo.ProducerLinger(time.Duration(conf.LingerMs * 1000 * 1000)))
	}
	switch strings.TrimSpace(strings.ToLower(conf.Compression)) {
	case "", "none":
		opts = append(opts, kgo.ProducerBatchCompression(kgo.NoCompression()))
	case "gzip":
		opts = append(opts, kgo.ProducerBatchCompression(kgo.GzipCompression()))
	case "snappy":
		opts = append(opts, kgo.ProducerBatchCompression(kgo.SnappyCompression()))
	case "lz4":
		opts = append(opts, kgo.ProducerBatchCompression(kgo.Lz4Compression()))
	case "zstd":
		opts = append(opts, kgo.ProducerBatchCompression(kgo.ZstdCompression()))
	default:
		return nil, errors.InvalidConfig("invalid kafka config, invalid compression mode: %s", conf.Compression)
	}
	if conf.Tls.Enable {
		if conf.Tls.Ca != "" || conf.Tls.Cert != "" || conf.Tls.Key != "" {
			tc, err := tlscfg.New(
				tlscfg.MaybeWithDiskCA(conf.Tls.Ca, tlscfg.ForClient),
				tlscfg.MaybeWithDiskKeyPair(conf.Tls.Cert, conf.Tls.Key),
			)
			if err != nil {
				return nil, errors.InvalidConfig("invalid kafka config, unable to init tls config, %v", err)
			}
			opts = append(opts, kgo.DialTLSConfig(tc))
		} else {
			opts = append(opts, kgo.DialTLS())
		}
	}
	if conf.Sasl.Enable {
		method := utils.NormStringOpt(conf.Sasl.Method)
		user := utils.NormStringOpt(conf.Sasl.User)
		pass := utils.NormStringOpt(conf.Sasl.Pass)
		if method == "" || user == "" || pass == "" {
			return nil, errors.InvalidConfig("invalid kafka config, method, user and pass are all required for sasl config")
		}
		method = strings.ReplaceAll(method, "-", "")
		method = strings.ReplaceAll(method, "_", "")
		switch method {
		case "plain":
			opts = append(opts, kgo.SASL(plain.Auth{
				User: user,
				Pass: pass,
			}.AsMechanism()))
		case "scramsha256":
			opts = append(opts, kgo.SASL(scram.Auth{
				User: user,
				Pass: pass,
			}.AsSha256Mechanism()))
		case "scramsha512":
			opts = append(opts, kgo.SASL(scram.Auth{
				User: user,
				Pass: pass,
			}.AsSha512Mechanism()))
		case "awsmskiam":
			opts = append(opts, kgo.SASL(aws.Auth{
				AccessKey: user,
				SecretKey: pass,
			}.AsManagedStreamingIAMMechanism()))
		default:
			return nil, errors.InvalidConfig("invalid kafka config, unrecognized sasl method %s", method)
		}
	}
	client := &KafkaClient {
		consumers: make(map[tp]*pconsumer),
	}
	opts = append(
		opts,
		kgo.OnPartitionsAssigned(client.assigned),
		kgo.OnPartitionsRevoked(client.revoked),
		kgo.OnPartitionsLost(client.lost),
		kgo.AutoCommitMarks(),
		kgo.BlockRebalanceOnPoll(),
	)
	
	for _, copt := range copts {
		copt(client)
	}
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, errors.FatalError("unable to create the kafka client, %v", err)
	}
	client.cl = cl
	return client, nil
}

func (s *KafkaClient) assigned(ctx context.Context, cl *kgo.Client, assigned map[string][]int32) {
	if s.workers == nil {
		log.Sugar().Warnf("did you forget to set workers to kafka client")
		return
	}
	for topic, partitions := range assigned {
		worker, exist := s.workers[topic]
		if exist {
			for _, partition := range partitions {
				pc := &pconsumer{
					cl: cl,
					topic: topic,
					partition: partition,
					worker: worker,
	
					quit: make(chan struct{}),
					done: make(chan struct{}),
					recs: make(chan kgo.FetchTopicPartition, 5),
				}
				s.consumers[tp{topic, partition}] = pc
				go pc.consume(ctx)
			}
		}
	}
}

func (s *KafkaClient) revoked(ctx context.Context, cl *kgo.Client, revoked map[string][]int32) {
	s.killConsumers(revoked)
	if err := cl.CommitMarkedOffsets(ctx); err != nil {
		log.Sugar().Errorf("Revoke commit failed: %v", err)
	}
}

func (s *KafkaClient) lost(_ context.Context, cl *kgo.Client, lost map[string][]int32) {
	s.killConsumers(lost)
	// Losing means we cannot commit: an error happened.
}

func (s *KafkaClient) killConsumers(lost map[string][]int32) {
	var wg sync.WaitGroup
	defer wg.Wait()

	for topic, partitions := range lost {
		for _, partition := range partitions {
			tp := tp{topic, partition}
			pc := s.consumers[tp]
			delete(s.consumers, tp)
			close(pc.quit)
			wg.Add(1)
			go func() {
				<- pc.done
				wg.Done()
			}()
		}
	}
}

func (s *KafkaClient) Check(ctx context.Context) error {
	return s.cl.Ping(ctx)
}

func (s *KafkaClient) Close() {
	s.cl.CloseAllowingRebalance()
}

func (s *KafkaClient) Poll(ctx context.Context) {
	for {
		// PollRecords is strongly recommended when using
		// BlockRebalanceOnPoll. You can tune how many records to
		// process at once (upper bound -- could all be on one
		// partition), ensuring that your processor loops complete fast
		// enough to not block a rebalance too long.
		fetches := s.cl.PollRecords(ctx, 10000)
		if fetches.IsClientClosed() {
			return
		}
		if fetches.Err() == context.Canceled {
			return
		}
		fetches.EachError(func(_ string, _ int32, err error) {
			// Note: you can delete this block, which will result
			// in these errors being sent to the partition
			// consumers, and then you can handle the errors there.
			panic(err)
		})
		fetches.EachPartition(func(p kgo.FetchTopicPartition) {
			tp := tp{p.Topic, p.Partition}

			// Since we are using BlockRebalanceOnPoll, we can be
			// sure this partition consumer exists:
			//
			// * onAssigned is guaranteed to be called before we
			// fetch offsets for newly added partitions
			//
			// * onRevoked waits for partition consumers to quit
			// and be deleted before re-allowing polling.
			s.consumers[tp].recs <- p
		})
		s.cl.AllowRebalance()
	}
}

func (s *KafkaClient) Produce(ctx context.Context, record *kgo.Record) error {
	return s.cl.ProduceSync(ctx, record)[0].Err
}