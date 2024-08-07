package signal

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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
	"go.uber.org/zap"
)

type tp struct {
	t string
	p int32
}

type pconsumer struct {
	cl        *kgo.Client
	topic     string
	partition int32
	worker    func(record *kgo.Record)

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
		case <-pc.quit:
			return
		case p := <-pc.recs:
			p.EachRecord(pc.worker)
			pc.cl.MarkCommitRecords(p.Records...)
		}
	}
}

type KafkaClient struct {
	conf      *config.KafkaConfigure
	cl        *kgo.Client
	consumers map[tp]*pconsumer
	workers   map[string]func(record *kgo.Record)
	group     string
	topics    []string
	logger    *zap.Logger
	sugar     *zap.SugaredLogger
}

func parseKafkaAddrs(addrs string) []string {
	return utils.SliceMapNew(strings.Split(addrs, ","), func(addr string) (mapped string, remove bool) {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			return addr, true
		} else {
			return addr, false
		}
	})
}

type KafkaOpt func(client *KafkaClient, opts []kgo.Opt) []kgo.Opt

func MakeKafkaTopic(conf *config.KafkaConfigure, topic string) string {
	prefix := conf.TopicPrefix
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return topic
	} else {
		return fmt.Sprintf("%s%s", prefix, topic)
	}
}

func KafkaOptGroup(group string) KafkaOpt {
	return func(client *KafkaClient, opts []kgo.Opt) []kgo.Opt {
		client.group = group
		return opts
	}
}

func KafkaOptTopics(topics ...string) KafkaOpt {
	return func(client *KafkaClient, opts []kgo.Opt) []kgo.Opt {
		client.topics = topics
		return opts
	}
}

func KafkaOptWorkers(workers map[string]func(*kgo.Record)) KafkaOpt {
	return func(client *KafkaClient, opts []kgo.Opt) []kgo.Opt {
		client.workers = workers
		return opts
	}
}

func KafkaOptPromReg(reg *prometheus.Registry) KafkaOpt {
	return func(client *KafkaClient, opts []kgo.Opt) []kgo.Opt {
		var metrics *kprom.Metrics
		if client.conf.Prometheus.Enable {
			metrics = kprom.NewMetrics(client.conf.Prometheus.Namespace, kprom.Registry(reg), kprom.Subsystem(client.conf.Prometheus.Subsystem))
			opts = append(opts, kgo.WithHooks(metrics))
		}
		return opts
	}
}

func KafkaOptLogger(logger *zap.Logger) KafkaOpt {
	return func(client *KafkaClient, opts []kgo.Opt) []kgo.Opt {
		client.logger = logger
		client.sugar = logger.Sugar()
		opts = append(opts, kgo.WithLogger(log.NewKgoLogger(logger)))
		return opts
	}
}

func NewKafkaClient(conf *config.KafkaConfigure, copts ...KafkaOpt) (*KafkaClient, error) {
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
	if conf.LingerMs > 0 {
		opts = append(opts, kgo.ProducerLinger(time.Duration(conf.LingerMs*1000*1000)))
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
		user := conf.Sasl.User
		pass := conf.Sasl.Pass
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
	client := &KafkaClient{
		conf:      conf,
		consumers: make(map[tp]*pconsumer),
	}
	opts = append(
		opts,
		kgo.OnPartitionsAssigned(client.assigned),
		kgo.OnPartitionsRevoked(client.revoked),
		kgo.OnPartitionsLost(client.lost),
		kgo.AutoCommitMarks(),
		kgo.BlockRebalanceOnPoll(),
		kgo.AllowAutoTopicCreation(),
	)

	for _, copt := range copts {
		opts = copt(client, opts)
	}
	if client.group != "" {
		opts = append(opts, kgo.ConsumerGroup(client.group))
	}
	if len(client.topics) > 0 {
		opts = append(opts, kgo.ConsumeTopics(client.topics...))
	}
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, errors.FatalError("unable to create the kafka client, %v", err)
	}
	client.cl = cl
	return client, nil
}

func (s *KafkaClient) MakeTopic(topic string) string {
	if s == nil {
		return ""
	}
	return MakeKafkaTopic(s.conf, topic)
}

func (s *KafkaClient) Sugar() *zap.SugaredLogger {
	if s == nil || s.sugar == nil {
		return log.Sugar()
	}
	return s.sugar
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
					cl:        cl,
					topic:     topic,
					partition: partition,
					worker:    worker,

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
				<-pc.done
				wg.Done()
			}()
		}
	}
}

func (s *KafkaClient) Check(ctx context.Context) error {
	if s != nil {
		return s.cl.Ping(ctx)
	}
	return nil
}

func (s *KafkaClient) Close() {
	if s != nil {
		s.cl.CloseAllowingRebalance()
	}
}

func (s *KafkaClient) Poll(ctx context.Context) {
	if s == nil {
		return
	}
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
		fetches.EachPartition(func(p kgo.FetchTopicPartition) {
			if p.Err != nil {
				s.Sugar().Errorf("err happened in topic %s of partition %d: %v", p.Topic, p.Partition, p.Err)
				return
			}

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
	if s == nil {
		return nil
	}
	return s.cl.ProduceSync(ctx, record)[0].Err
}
