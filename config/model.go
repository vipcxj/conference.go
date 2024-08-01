package config

import (
	"fmt"
	"strings"

	"github.com/pion/webrtc/v4"
	"github.com/vipcxj/conference.go/errors"
)

type RedisConfigure struct {
	Mode       string `mapstructure:"mode" json:"mode" default:"${MODE | auto}"`
	Addrs      string `mapstructure:"addrs" json:"addrs" default:"${ADDRS}"`
	Users      string `mapstructure:"users" json:"users" default:"${USERS}"`
	Passes     string `mapstructure:"passes" json:"passes" default:"${PASSES}"`
	KeyPrefix  string `mapstructure:"keyPrefix" json:"keyPrefix" default:"${KEY_PREFIX | cfgo:}"`
	MasterName string `mapstructure:"masterName" json:"masterName" default:"${MASTER_NAME}"`
}
type PrometheusConfigure struct {
	Enable       bool   `mapstructure:"enable" json:"enable" default:"${ENABLE | false}"`
	Namespace    string `mapstructure:"namespace" json:"namespace" default:"${NAMESPACE | cfgo}"`
	Subsystem    string `mapstructure:"subsystem" json:"subsystem" default:"${SUBSYSTEM | common}"`
	GoCollectors bool   `mapstructure:"goCollectors" json:"goCollectors" default:"${GO_COLLECTORS | false}"`
}

type KafkaConfigure struct {
	Addrs       string `mapstructure:"addrs" json:"addrs" default:"${ADDRS}"`
	TopicPrefix string `mapstructure:"topicPrefix" json:"topicPrefix" default:"${TOPIC_PREFIX | cfgo_}"`
	Sasl        struct {
		Enable bool   `mapstructure:"enable" json:"enable" default:"${ENABLE | false}"`
		Method string `mapstructure:"method" json:"method" default:"${METHOD}"`
		User   string `mapstructure:"user" json:"user" default:"${USER}"`
		Pass   string `mapstructure:"pass" json:"pass" default:"${PASS}"`
	} `mapstructure:"sasl" json:"sasl" default:"" defaultenvprefix:"SASL_"`
	Tls struct {
		Enable bool   `mapstructure:"enable" json:"enable" default:"${ENABLE | false}"`
		Ca     string `mapstructure:"ca" json:"ca" default:"${CA}"`
		Cert   string `mapstructure:"cert" json:"cert" default:"${CERT}"`
		Key    string `mapstructure:"key" json:"key" default:"${KEY}"`
	} `mapstructure:"tls" json:"tls" default:"" defaultenvprefix:"TLS_"`
	Prometheus           PrometheusConfigure `mapstructure:"prometheus" json:"prometheus" default:"" defaultenvprefix:"PROMETHEUS_"`
	MaxBufferedRecords   int                 `mapstructure:"maxBufferedRecords" json:"maxBufferedRecords" default:"${MAX_BUFFERED_RECORDS | 0}"`
	MaxBufferedBytes     int                 `mapstructure:"maxBufferedBytes" json:"maxBufferedBytes" default:"${MAX_BUFFERED_BYTES | 0}"`
	MaxConcurrentFetches int                 `mapstructure:"maxConcurrentFetches" json:"maxConcurrentFetches" default:"${MAX_CONCURRENT_FETCHES | 0}"`
	FetchMaxBytes        int                 `mapstructure:"fetchMaxBytes" json:"fetchMaxBytes" default:"${FETCH_MAX_BYTES | 0}"`
	BatchMaxBytes        int                 `mapstructure:"batchMaxBytes" json:"batchMaxBytes" default:"${FBATCH_MAX_BYTES | 0}"`
	NoIdempotency        bool                `mapstructure:"noIdempotency" json:"noIdempotency" default:"${NO_IDEMPOTENCY | false}"`
	LingerMs             int                 `mapstructure:"lingerMs" json:"lingerMs" default:"${LINGER_MS | 0}"`
	Compression          string              `mapstructure:"compression" json:"compression" default:"${COMPRESSION | none}"`
}

func (c *KafkaConfigure) Validate() error {
	err := requiredString("cluster.addrs", c.Addrs, " in cluster mode")
	return err
}

func (c *KafkaConfigure) GetProm() *PrometheusConfigure {
	return &c.Prometheus
}

type ClusterConfigure struct {
	Enable   bool           `mapstructure:"enable" json:"enable" default:"${ENABLE | false}"`
	Kafka    KafkaConfigure `mapstructure:"kafka" json:"kafka" default:"" defaultenvprefix:"KAFKA_"`
	Redis    RedisConfigure `mapstructure:"redis" json:"redis" default:"" defaultenvprefix:"REDIS_"`
	NodeName string         `mapstructure:"nodeName" json:"nodeName" default:"${NODE_NAME}"`
	Debug    struct {
		Enable     bool `mapstructure:"enable" json:"enable" default:"${ENABLE | false}"`
		Replicas   int  `mapstructure:"replicas" json:"replicas" default:"${REPLICAS | 0}"`
		PortOffset int  `mapstructure:"portOffset" json:"portOffset" default:"${PORT_OFFSET | 0}"`
	} `mapstructure:"debug" json:"debug" default:"" defaultenvprefix:"DEBUG_"`
}

func (c *ClusterConfigure) GetKafka() *KafkaConfigure {
	return &c.Kafka
}

func (c *ClusterConfigure) GetRedis() *RedisConfigure {
	return &c.Redis
}

func (c *ClusterConfigure) GetNodeName() string {
	if c.NodeName == "" {
		return "node0"
	} else {
		return c.NodeName
	}
}

type ConferenceConfigure struct {
	HostOrIp string `json:"hostOrIp" mapstructure:"hostOrIp" default:"${CONF_HOST_OR_IP}"`
	Router   struct {
		HostOrIp string `json:"hostOrIp" mapstructure:"hostOrIp" default:"${CONF_ROUTER_HOST_OR_IP}"`
		Port     int    `json:"port" mapstructure:"port" default:"${CONF_ROUTER_PORT | 0}"`
	} `mapstructure:"router" default:""`
	WebRTC struct {
		JitterBuffer bool `json:"jitterBuffer" mapstructure:"jitterBuffer" default:"${CONF_WEBRTC_JITTER_BUFFER | true}"`
		ICEServer    struct {
			URLs           string `json:"urls" mapstructure:"urls" default:"${CONF_WEBRTC_ICESERVER_URLS}"`
			Username       string `json:"username,omitempty" mapstructure:"username" default:"${CONF_WEBRTC_ICESERVER_USERNAME}"`
			Credential     string `json:"credential,omitempty" mapstructure:"credential" default:"${CONF_WEBRTC_ICESERVER_CREDENTIAL}"`
			CredentialType string `json:"credentialType,omitempty" mapstructure:"credentialType" default:"${CONF_WEBRTC_ICESERVER_CREDENTIAL_TYPE}"`
		} `json:"iceServer,omitempty" mapstructure:"iceServer" default:""`
		ICETransportPolicy string `json:"iceTransportPolicy,omitempty" mapstructure:"iceTransportPolicy" default:"${CONF_WEBRTC_ICETRANSPORT_POLICY}"`
	} `json:"webrtc,omitempty" mapstructure:"webrtc" default:""`
	Prometheus PrometheusConfigure `mapstructure:"prometheus" json:"prometheus" default:"" defaultenvprefix:"CONF_PROMETHEUS_"`
	AuthServer struct {
		Enable   bool   `json:"enable" mapstructure:"enable" default:"${CONF_AUTH_SERVER_ENABLE | true}"`
		Host     string `json:"host" mapstructure:"host" default:"${CONF_AUTH_SERVER_HOST | localhost}"`
		Port     int    `json:"port" mapstructure:"port" default:"${CONF_AUTH_SERVER_PORT | 3100}"`
		Ssl      bool   `json:"ssl" mapstructure:"ssl" default:"${CONF_AUTH_SERVER_SSL | false}"`
		CertPath string `json:"certPath" mapstructure:"certPath" default:"${CONF_AUTH_SERVER_CERT_PATH}"`
		KeyPath  string `json:"keyPath" mapstructure:"keyPath" default:"${CONF_AUTH_SERVER_KEY_PATH}"`
		Cors     string `json:"cors" mapstructure:"cors" default:"${CONF_AUTH_SERVER_CORS}"`
		Healthy  struct {
			Enable           bool   `mapstructure:"enable" json:"enable" default:"${CONF_AUTH_SERVER_HEALTHY_ENABLE | true}"`
			FailureThreshold int    `mapstructure:"failureThreshold" json:"failureThreshold" default:"${CONF_AUTH_SERVER_HEALTHY_FAILURE_THRESHOLD | 3}"`
			Path             string `mapstructure:"path" json:"path" default:"${CONF_AUTH_SERVER_HEALTHY_PATH | /healthz}"`
		} `mapstructure:"healthy" json:"healthy" default:""`
	} `mapstructure:"authServer" json:"authServer" default:""`
	SecretKey string `mapstructure:"secretKey" default:"${CONF_SECRET_KEY | 123456}"`
	Log       struct {
		Profile string `mapstructure:"profile" json:"profile" default:"${CONF_LOG_PROFILE | production}"`
		Level   string `mapstructure:"level" json:"level" default:"${CONF_LOG_LEVEL | info}"`
	} `mapstructure:"log" json:"log" default:""`
	Record struct {
		Enable          bool   `mapstructure:"enable" json:"enable" default:"${CONF_RECORD_ENABLE | false}"`
		BasePath        string `mapstructure:"basePath" json:"basePath" default:"${CONF_RECORD_BASE_PATH}"`
		DirPath         string `mapstructure:"dirPath" json:"dirPath" default:"${CONF_RECORD_DIR_PATH}"`
		IndexName       string `mapstructure:"indexName" json:"indexName" default:"${CONF_RECORD_INDEX_NAME}"`
		SegmentDuration int    `mapstructure:"segmentDuration" json:"segmentDuration" default:"${CONF_RECORD_SEGMENT_DURATION | 6}"`
		GopSize         int    `mapstructure:"gopSize" json:"gopSize" default:"${CONF_RECORD_GOP_SIZE | 3}"`
		DBIndex         struct {
			Enable     bool   `mapstructure:"enable" json:"enable" default:"${CONF_RECORD_DBINDEX_ENABLE | false}"`
			MongoUrl   string `mapstructure:"mongoUrl" json:"mongoUrl" default:"${CONF_RECORD_DBINDEX_MONGO_URL}"`
			Database   string `mapstructure:"database" json:"database" default:"${CONF_RECORD_DBINDEX_DATABASE}"`
			Collection string `mapstructure:"collection" json:"collection" default:"${CONF_RECORD_DBINDEX_COLLECTION}"`
			Key        string `mapstructure:"key" json:"key" default:"${CONF_RECORD_DBINDEX_KEY}"`
			Auth       struct {
				User string `mapstructure:"user" json:"user" default:"${CONF_RECORD_DBINDEX_AUTH_USER}"`
				Pass string `mapstructure:"pass" json:"pass" default:"${CONF_RECORD_DBINDEX_AUTH_PASS}"`
			} `mapstructure:"auth" json:"auth" default:""`
		} `mapstructure:"dbIndex" json:"dbIndex" default:""`
	} `mapstructure:"record" json:"record" default:""`
	Callback struct {
		OnStart      string `mapstructure:"onStart" json:"onStart" default:"${CONF_CALLBACK_ONSTART}"`
		OnClose      string `mapstructure:"onClose" json:"onClose" default:"${CONF_CALLBACK_ONCLOSE}"`
		OnConnecting string `mapstructure:"onConnecting" json:"onConnecting" default:"${CONF_CALLBACK_ONCONNECTING}"`
		IntervalMs   int    `mapstructure:"intervalMs" json:"intervalMs" default:"${CONF_CALLBACK_INTERVAL_MS | 60000}"`
	} `mapstructure:"callback" json:"callback" default:""`
	Signal struct {
		Enable       bool   `mapstructure:"enable" json:"enable" default:"${CONF_SIGNAL_ENABLE | true}"`
		HostOrIp     string `mapstructure:"hostOrIp" json:"hostOrIp" default:"${CONF_SIGNAL_HOST_OR_IP}"`
		Port         int    `mapstructure:"port" json:"port" default:"${CONF_SIGNAL_PORT | 0}"`
		Cors         string `mapstructure:"cors" json:"cors" default:"${CONF_SIGNAL_CORS}"`
		AsyncSendMsg bool   `mapstructure:"asyncSendMsg" json:"asyncSendMsg" default:"${CONF_SIGNAL_ASYNC_SEND_MSG | false}"`
		Tls          struct {
			Enable bool   `mapstructure:"enable" json:"enable" default:"${CONF_SIGNAL_TLS_ENABLE | false}"`
			Cert   string `mapstructure:"cert" json:"cert" default:"${CONF_SIGNAL_TLS_CERT}"`
			Key    string `mapstructure:"key" json:"key" default:"${CONF_SIGNAL_TLS_KEY}"`
		} `mapstructure:"tls" json:"tls" default:""`
		MsgTimeoutMs      int `mapstructure:"msgTimeoutMs" json:"msgTimeoutMs" default:"${CONF_SIGNAL_MSG_TIMEOUT_MS | 6000}"`
		MsgTimeoutRetries int `mapstructure:"msgTimeoutRetries" json:"msgTimeoutRetries" default:"${CONF_SIGNAL_MSG_TIMEOUT_RETRIES | 10}"`
		Gin               struct {
			Debug        bool `mapstructure:"debug" json:"debug" default:"${CONF_SIGNAL_GIN_DEBUG | false}"`
			NoRequestLog bool `mapstructure:"noRequestLog" json:"noRequestLog" default:"${CONF_SIGNAL_GIN_NO_REQUEST_LOG | false}"`
		} `mapstructure:"gin" json:"gin" default:""`
		Healthy struct {
			Enable           bool   `mapstructure:"enable" json:"enable" default:"${CONF_SIGNAL_HEALTHY_ENABLE | true}"`
			FailureThreshold int    `mapstructure:"failureThreshold" json:"failureThreshold" default:"${CONF_SIGNAL_HEALTHY_FAILURE_THRESHOLD | 3}"`
			Path             string `mapstructure:"path" json:"path" default:"${CONF_SIGNAL_HEALTHY_PATH | /healthz}"`
		} `mapstructure:"healthy" json:"healthy" default:""`
		Prometheus PrometheusConfigure `mapstructure:"prometheus" json:"prometheus" default:"" defaultenvprefix:"CONF_SIGNAL_PROMETHEUS_"`
	} `mapstructure:"signal" json:"signal" default:""`
	Cluster ClusterConfigure `mapstructure:"cluster" json:"cluster" default:"" defaultenvprefix:"CONF_CLUSTER_"`
}

type LogProfile int

const (
	LOG_PROFILE_DEVELOPMENT LogProfile = iota
	LOG_PROFILE_PRODUCTION
)

func NewLogProfile(s string) LogProfile {
	switch strings.ToLower(s) {
	case "development":
		return LOG_PROFILE_DEVELOPMENT
	case "production", "":
		return LOG_PROFILE_PRODUCTION
	default:
		panic(errors.InvalidParam("invalid log profile %s", s))
	}
}

func (me LogProfile) String() string {
	switch me {
	case LOG_PROFILE_DEVELOPMENT:
		return "development"
	case LOG_PROFILE_PRODUCTION:
		return "production"
	default:
		panic(errors.InvalidParam("invalid log profile %d", me))
	}
}

// This is done this way because of a linter.
const (
	iceCredentialTypePasswordStr = "password"
	iceCredentialTypeOauthStr    = "oauth"
)

func parseCredentialType(ct string) webrtc.ICECredentialType {
	switch ct {
	case iceCredentialTypePasswordStr, "":
		return webrtc.ICECredentialTypePassword
	case iceCredentialTypeOauthStr:
		return webrtc.ICECredentialTypeOauth
	default:
		panic(errors.InvalidParam("invalid webrtc credential type %s", ct))
	}
}

func (c *ConferenceConfigure) GetWebrtcConfiguration() webrtc.Configuration {
	var iceServers []webrtc.ICEServer
	var urls []string
	if c.WebRTC.ICEServer.URLs != "" {
		urls = strings.Split(c.WebRTC.ICEServer.URLs, " ")
		ct := parseCredentialType(c.WebRTC.ICEServer.CredentialType)
		var credential any
		rawCredential := c.WebRTC.ICEServer.Credential
		if ct == webrtc.ICECredentialTypeOauth {
			var macKey, accessToken string
			if rawCredential != "" {
				parts := strings.SplitN(rawCredential, " ", 2)
				macKey = parts[0]
				if len(parts) > 1 {
					accessToken = parts[1]
				}
			}
			credential = webrtc.OAuthCredential{
				MACKey:      macKey,
				AccessToken: accessToken,
			}
		} else {
			credential = rawCredential
		}
		iceServers = []webrtc.ICEServer{{
			URLs:           urls,
			Username:       c.WebRTC.ICEServer.Username,
			Credential:     credential,
			CredentialType: ct,
		}}
	}
	return webrtc.Configuration{
		ICEServers:         iceServers,
		ICETransportPolicy: webrtc.NewICETransportPolicy(c.WebRTC.ICETransportPolicy),
	}
}

func (c *ConferenceConfigure) LogProfile() LogProfile {
	return NewLogProfile(c.Log.Profile)
}

func (c *ConferenceConfigure) PromEnable() bool {
	return c.Prometheus.Enable || c.Signal.Prometheus.Enable || c.Cluster.Kafka.Prometheus.Enable
}

func (c *ConferenceConfigure) GetProm() *PrometheusConfigure {
	return &c.Prometheus
}

func (c *ConferenceConfigure) GetCluster() *ClusterConfigure {
	return &c.Cluster
}

func (c *ConferenceConfigure) ExternalHostOrIp() string {
	return c.HostOrIp
}

func (c *ConferenceConfigure) RouterListenHostOrIp() string {
	if c.Router.HostOrIp != "" {
		return c.Router.HostOrIp
	} else {
		return c.ExternalHostOrIp()
	}
}

func (c *ConferenceConfigure) RouterPort() int {
	return c.Router.Port
}

func (c *ConferenceConfigure) RouterListenAddress() string {
	return fmt.Sprintf("%s:%d", c.RouterListenHostOrIp(), c.RouterPort())
}

func (c *ConferenceConfigure) RouterExternalAddress() string {
	return fmt.Sprintf("%s:%d", c.ExternalHostOrIp(), c.RouterPort())
}

func (c *ConferenceConfigure) SignalListenHostOrIp() string {
	if c.Signal.HostOrIp != "" {
		return c.Signal.HostOrIp
	} else {
		return c.ExternalHostOrIp()
	}
}

func (c *ConferenceConfigure) SignalPort() int {
	port := c.Signal.Port
	if port == 0 {
		if c.Signal.Tls.Enable {
			return 4430
		} else {
			return 8080
		}
	} else {
		return port
	}
}

func (c *ConferenceConfigure) SignalListenAddress() string {
	return fmt.Sprintf("%s:%d", c.SignalListenHostOrIp(), c.SignalPort())
}

func (c *ConferenceConfigure) SignalExternalAddress() string {
	port := c.SignalPort()
	if c.Signal.Tls.Enable {
		if port != 443 {
			return fmt.Sprintf("https://%s:%d", c.ExternalHostOrIp(), port)
		} else {
			return fmt.Sprintf("https://%s", c.ExternalHostOrIp())
		}
	} else {
		if port != 80 {
			return fmt.Sprintf("http://%s:%d", c.ExternalHostOrIp(), port)
		} else {
			return fmt.Sprintf("http://%s", c.ExternalHostOrIp())
		}
	}
}

func (c *ConferenceConfigure) ClusterEnable() bool {
	return c.Cluster.Enable || c.Cluster.Debug.Enable
}

func (c *ConferenceConfigure) ClusterNodeName() string {
	return c.Cluster.GetNodeName()
}

func requiredString(name string, value string, postfix string) error {
	if value == "" {
		return errors.InvalidConfig("invalid config, %s is required%s", name, postfix)
	}
	return nil
}

func requiredInt(name string, value int, postfix string) error {
	if value == 0 {
		return errors.InvalidConfig("invalid config, %s is required%s", name, postfix)
	}
	return nil
}

func (c *ConferenceConfigure) Validate() error {
	err := requiredString("hostOrIp", c.HostOrIp, "")
	if err != nil {
		return err
	}
	if c.ClusterEnable() {
		err = requiredInt("router.port", c.RouterPort(), " in cluster mode")
		if err != nil {
			return err
		}
		if c.Cluster.Enable {
			err = requiredString("cluster.nodeName", c.ClusterNodeName(), " in cluster mode")
			if err != nil {
				return err
			}
		}
		err = c.Cluster.Kafka.Validate()
		if err != nil {
			return err
		}
	}
	if c.Record.Enable && c.Record.DBIndex.Enable {
		err = requiredString("record.dbIndex.mongoUrl", c.Record.DBIndex.MongoUrl, " when record enabled and record.dbIndex enabled")
		if err != nil {
			return err
		}
		err = requiredString("record.dbIndex.database", c.Record.DBIndex.Database, " when record enabled and record.dbIndex enabled")
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *ConferenceConfigure) Split() []*ConferenceConfigure {
	if !c.Cluster.Debug.Enable {
		cfg := *c
		return []*ConferenceConfigure{&cfg}
	} else {
		replicas := c.Cluster.Debug.Replicas
		if replicas == 0 {
			replicas = 2
		}
		offsetMult := c.Cluster.Debug.PortOffset
		if offsetMult == 0 {
			offsetMult = 1
		}
		cfgs := make([]*ConferenceConfigure, replicas)
		for i := 0; i < replicas; i++ {
			cfg := *c
			cfg.Cluster.Enable = true
			if cfg.Cluster.NodeName == "" {
				cfg.Cluster.NodeName = fmt.Sprintf("node%d", i)
			} else {
				cfg.Cluster.NodeName = fmt.Sprintf("%s-%d", cfg.Cluster.NodeName, i)
			}
			cfg.Router.Port = cfg.RouterPort() + offsetMult*i
			cfg.Signal.Port = cfg.SignalPort() + offsetMult*i
			cfgs[i] = &cfg
		}
		return cfgs
	}
}

var KEYS = []string{
	"hostOrIp:string",
	"prometheus.enable:bool",
	"prometheus.namespace:string",
	"prometheus.subsystem:string",
	"prometheus.goCollectors:bool",
	"signal.enable:bool",
	"signal.hostOrIp:string",
	"signal.port:int",
	"signal.tls.enable:bool",
	"signal.tls.cert:string",
	"signal.tls.key:string",
	"signal.cors:string",
	"signal.asyncSendMsg:bool",
	"signal.msgTimeoutMs:int",
	"signal.msgTimeoutRetries:int",
	"signal.gin.debug:bool",
	"signal.gin.noRequestLog:bool",
	"signal.healthy.enable:bool",
	"signal.healthy.failureThreshold:int",
	"signal.healthy.path:string",
	"signal.prometheus.enable:bool",
	"signal.prometheus.namespace:string",
	"signal.prometheus.subsystem:string",
	"router.hostOrIp:string",
	"router.port:int",
	"webrtc.iceServer.urls:string",
	"webrtc.iceServer.username:string",
	"webrtc.iceServer.credential:string",
	"webrtc.iceServer.credentialType:string",
	"webrtc.iceTransportPolicy:string",
	"authServer.enable:bool",
	"authServer.host:string",
	"authServer.port:int",
	"authServer.ssl:bool",
	"authServer.certPath:string",
	"authServer.keyPath:string",
	"authServer.cors:string",
	"authServer.healthy.enable:bool",
	"authServer.healthy.failureThreshold:int",
	"authServer.healthy.path:string",
	"secretKey:string",
	"log.profile:string",
	"log.level:string",
	"record.enable:bool",
	"record.basePath:string",
	"record.dirPath:string",
	"record.indexName:string",
	"record.segmentDuration:int",
	"record.gopSize:int",
	"record.dbIndex.enable:bool",
	"record.dbIndex.mongoUrl:string",
	"record.dbIndex.database:string",
	"record.dbIndex.collection:string",
	"record.dbIndex.key:string",
	"record.dbIndex.auth.user:string",
	"record.dbIndex.auth.pass:string",
	"callback.onStart:string",
	"callback.onClose:string",
	"callback.onConnecting:string",
	"callback.intervalMs:int",
	"cluster.enable:bool",
	"cluster.nodeName:string",
	"cluster.kafka.addrs:string",
	"cluster.kafka.topicPrefix:string",
	"cluster.kafka.sasl.enable:bool",
	"cluster.kafka.sasl.method:string",
	"cluster.kafka.sasl.user:string",
	"cluster.kafka.sasl.pass:string",
	"cluster.kafka.tls.enable:bool",
	"cluster.kafka.tls.ca:string",
	"cluster.kafka.tls.cert:string",
	"cluster.kafka.tls.key:string",
	"cluster.kafka.prometheus.enable:bool",
	"cluster.kafka.prometheus.namespace:string",
	"cluster.kafka.prometheus.subsystem:string",
	"cluster.kafka.maxBufferedRecords:int",
	"cluster.kafka.maxBufferedBytes:int",
	"cluster.kafka.maxConcurrentFetches:int",
	"cluster.kafka.fetchMaxBytes:int",
	"cluster.kafka.batchMaxBytes:int",
	"cluster.kafka.noIdempotency:bool",
	"cluster.kafka.lingerMs:int",
	"cluster.kafka.compression:string",
	"cluster.redis.mode:string",
	"cluster.redis.addrs:string",
	"cluster.redis.users:string",
	"cluster.redis.passes:string",
	"cluster.redis.keyPrefix:string",
	"cluster.redis.masterName:string",
	"cluster.debug.enable:bool",
	"cluster.debug.replicas:int",
	"cluster.debug.portOffset:int",
}
