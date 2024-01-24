package config

import (
	"strings"

	"github.com/pion/webrtc/v4"
	"github.com/vipcxj/conference.go/errors"
)

type RedisConfigure struct {
	Mode       string `mapstructure:"mode" json:"mode" default:"${CONF_CLUSTER_REDIS_MODE | auto}"`
	Addrs      string `mapstructure:"addrs" json:"addrs" default:"${CONF_CLUSTER_REDIS_ADDRS}"`
	Users      string `mapstructure:"users" json:"users" default:"${CONF_CLUSTER_REDIS_USERS}"`
	Passes     string `mapstructure:"passes" json:"passes" default:"${CONF_CLUSTER_REDIS_PASSES}"`
	KeyPrefix  string `mapstructure:"keyPrefix" json:"keyPrefix" default:"${CONF_CLUSTER_REDIS_KEY_PREFIX | cfgo}"`
	MasterName string `mapstructure:"masterName" json:"masterName" default:"${CONF_CLUSTER_REDIS_MASTER_NAME}"`
}

type KafkaConfigure struct {
	Addrs string `mapstructure:"addrs" json:"addrs" default:"${CONF_CLUSTER_KAFKA_ADDRS}"`
	Sasl  struct {
		Enable bool   `mapstructure:"enable" json:"enable" default:"${CONF_CLUSTER_KAFKA_SASL_ENABLE | false}"`
		Method string `mapstructure:"method" json:"method" default:"${CONF_CLUSTER_KAFKA_SASL_METHOD}"`
		User   string `mapstructure:"user" json:"user" default:"${CONF_CLUSTER_KAFKA_SASL_USER}"`
		Pass   string `mapstructure:"pass" json:"pass" default:"${CONF_CLUSTER_KAFKA_SASL_PASS}"`
	} `mapstructure:"sasl" json:"sasl" default:""`
	Tls struct {
		Enable bool   `mapstructure:"enable" json:"enable" default:"${CONF_CLUSTER_KAFKA_TLS_ENABLE | false}"`
		Ca     string `mapstructure:"ca" json:"ca" default:"${CONF_CLUSTER_KAFKA_TLS_CA}"`
		Cert   string `mapstructure:"cert" json:"cert" default:"${CONF_CLUSTER_KAFKA_TLS_CERT}"`
		Key    string `mapstructure:"key" json:"key" default:"${CONF_CLUSTER_KAFKA_TLS_KEY}"`
	} `mapstructure:"tls" json:"tls" default:""`
	Prometheus struct {
		Enable    bool   `mapstructure:"enable" json:"enable" default:"${CONF_CLUSTER_KAFKA_PROMETHEUS_ENABLE | false}"`
		Namespace string `mapstructure:"namespace" json:"namespace" default:"${CONF_CLUSTER_KAFKA_PROMETHEUS_NAMESPACE | kgo}"`
		Path      string `mapstructure:"path" json:"path" default:"${CONF_CLUSTER_KAFKA_PROMETHEUS_PATH | /metrics}"`
	} `mapstructure:"prometheus" json:"prometheus" default:""`
	MaxBufferedRecords   int    `mapstructure:"maxBufferedRecords" json:"maxBufferedRecords" default:"${CONF_CLUSTER_KAFKA_MAX_BUFFERED_RECORDS | 0}"`
	MaxBufferedBytes     int    `mapstructure:"maxBufferedBytes" json:"maxBufferedBytes" default:"${CONF_CLUSTER_KAFKA_MAX_BUFFERED_BYTES | 0}"`
	MaxConcurrentFetches int    `mapstructure:"maxConcurrentFetches" json:"maxConcurrentFetches" default:"${CONF_CLUSTER_KAFKA_MAX_CONCURRENT_FETCHES | 0}"`
	FetchMaxBytes        int    `mapstructure:"fetchMaxBytes" json:"fetchMaxBytes" default:"${CONF_CLUSTER_KAFKA_FETCH_MAX_BYTES | 0}"`
	BatchMaxBytes        int    `mapstructure:"batchMaxBytes" json:"batchMaxBytes" default:"${CONF_CLUSTER_KAFKA_FBATCH_MAX_BYTES | 0}"`
	NoIdempotency        bool   `mapstructure:"noIdempotency" json:"noIdempotency" default:"${CONF_CLUSTER_KAFKA_NO_IDEMPOTENCY | false}"`
	LingerMs             int    `mapstructure:"lingerMs" json:"lingerMs" default:"${CONF_CLUSTER_KAFKA_LINGER_MS | 0}"`
	Compression          string `mapstructure:"compression" json:"compression" default:"${CONF_CLUSTER_KAFKA_COMPRESSION | none}"`
}

type ConferenceConfigure struct {
	Ip                string `mapstructure:"ip" default:"${CONF_IP}"`
	Port              int    `mapstructure:"port" default:"${CONF_PORT | 0}"`
	SignalEnable      bool   `mapstructure:"signalEnable" default:"${CONF_SIGNAL_ENABLE | true}"`
	SignalHost        string `mapstructure:"signalHost" default:"${CONF_SIGNAL_HOST | localhost}"`
	SignalPort        int    `mapstructure:"signalPort" default:"${CONF_SIGNAL_PORT | 8080}"`
	SignalSsl         bool   `mapstructure:"signalSsl" default:"${CONF_SIGNAL_SSL | false}"`
	SignalExternalUrl string `mapstructure:"signalExternalUrl" default:"${CONF_SIGNAL_EXTERNAL_URL}"`
	SignalCertPath    string `mapstructure:"signalCertPath" default:"${CONF_SIGNAL_CERT_PATH}"`
	SignalKeyPath     string `mapstructure:"signalKeyPath" default:"${CONF_SIGNAL_KEY_PATH}"`
	SignalCors        string `mapstructure:"signalCors" default:"${CONF_SIGNAL_CORS}"`
	WebRTC            struct {
		ICEServer struct {
			URLs           string `json:"urls" mapstructure:"urls" default:"${CONF_WEBRTC_ICESERVER_URLS}"`
			Username       string `json:"username,omitempty" mapstructure:"username" default:"${CONF_WEBRTC_ICESERVER_USERNAME}"`
			Credential     string `json:"credential,omitempty" mapstructure:"credential" default:"${CONF_WEBRTC_ICESERVER_CREDENTIAL}"`
			CredentialType string `json:"credentialType,omitempty" mapstructure:"credentialType" default:"${CONF_WEBRTC_ICESERVER_CREDENTIAL_TYPE}"`
		} `json:"iceServer,omitempty" mapstructure:"iceServer" default:""`
		ICETransportPolicy string `json:"iceTransportPolicy,omitempty" mapstructure:"iceTransportPolicy" default:"${CONF_WEBRTC_ICETRANSPORT_POLICY}"`
	} `json:"webrtc,omitempty" mapstructure:"webrtc" default:""`
	AuthServerEnable   bool   `mapstructure:"authServerEnable" default:"${CONF_AUTH_SERVER_ENABLE | true}"`
	AuthServerHost     string `mapstructure:"authServerHost" default:"${CONF_AUTH_SERVER_HOST | localhost}"`
	AuthServerPort     int    `mapstructure:"authServerPort" default:"${CONF_AUTH_SERVER_PORT | 3100}"`
	AuthServerSsl      bool   `mapstructure:"authServerSsl" default:"${CONF_AUTH_SERVER_SSL | false}"`
	AuthServerCertPath string `mapstructure:"authServerCertPath" default:"${CONF_AUTH_SERVER_CERT_PATH}"`
	AuthServerKeyPath  string `mapstructure:"authServerKeyPath" default:"${CONF_AUTH_SERVER_KEY_PATH}"`
	AuthServerCors     string `mapstructure:"authServerCors" default:"${CONF_AUTH_SERVER_CORS}"`
	SecretKey          string `mapstructure:"secretKey" default:"${CONF_SECRET_KEY}"`
	Log                struct {
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
		Gin struct {
			Debug        bool `mapstructure:"debug" json:"debug" default:"${CONF_SIGNAL_GIN_DEBUG | false}"`
			NoRequestLog bool `mapstructure:"noRequestLog" json:"noRequestLog" default:"${CONF_SIGNAL_GIN_NO_REQUEST_LOG | false}"`
		} `mapstructure:"gin" json:"gin" default:""`
		Healthy struct {
			Enable           bool   `mapstructure:"enable" json:"enable" default:"${CONF_SIGNAL_HEALTHY_ENABLE | true}"`
			FailureThreshold int    `mapstructure:"failureThreshold" json:"failureThreshold" default:"${CONF_SIGNAL_HEALTHY_FAILURE_THRESHOLD | 3}"`
			Path             string `mapstructure:"path" json:"path" default:"${CONF_SIGNAL_HEALTHY_PATH | /healthz}"`
		} `mapstructure:"healthy" json:"healthy" default:""`
	} `mapstructure:"signal" json:"signal" default:""`
	Cluster struct {
		Enable   bool           `mapstructure:"enable" json:"enable" default:"${CONF_CLUSTER_ENABLE | true}"`
		Kafka    KafkaConfigure `mapstructure:"kafka" json:"kafka" default:""`
		Redis    RedisConfigure `mapstructure:"redis" json:"redis" default:""`
		NodeName string         `mapstructure:"nodeName" json:"nodeName" default:"${CONF_CLUSTER_NODE_NAME}"`
	} `mapstructure:"cluster" json:"cluster" default:""`
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

var KEYS = []string{
	"ip:string",
	"port:int",
	"signalEnable:bool",
	"signalHost:string",
	"signalPort:int",
	"signalSsl:bool",
	"signalExternalUrl:string",
	"signalCertPath:string",
	"signalKeyPath:string",
	"signalCors:string",
	"webrtc.iceServer.urls:string",
	"webrtc.iceServer.username:string",
	"webrtc.iceServer.credential:string",
	"webrtc.iceServer.credentialType:string",
	"webrtc.iceTransportPolicy:string",
	"authServerEnable:bool",
	"authServerHost:string",
	"authServerPort:int",
	"authServerSsl:bool",
	"authServerCertPath:string",
	"authServerKeyPath:string",
	"authServerCors:string",
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
	"signal.gin.debug:bool",
	"signal.gin.noRequestLog:bool",
	"signal.healthy.enable:bool",
	"signal.healthy.failureThreshold:int",
	"signal.healthy.path:string",
	"cluster.enable:bool",
	"cluster.nodeName:string",
	"cluster.kafka.addrs:string",
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
	"cluster.kafka.prometheus.path:string",
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
}
