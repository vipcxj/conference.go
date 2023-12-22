package config

import (
	"strings"

	"github.com/pion/webrtc/v4"
	"github.com/vipcxj/conference.go/errors"
)

type ConferenceConfigure struct {
	Ip             string `mapstructure:"ip" default:"${CONF_IP}"`
	Port           int    `mapstructure:"port" default:"${CONF_PORT | 0}"`
	SignalEnable   bool   `mapstructure:"signalEnable" default:"${CONF_SIGNAL_ENABLE | true}"`
	SignalHost     string `mapstructure:"signalHost" default:"${CONF_SIGNAL_HOST | localhost}"`
	SignalPort     int    `mapstructure:"signalPort" default:"${CONF_SIGNAL_PORT | 8080}"`
	SignalSsl      bool   `mapstructure:"signalSsl" default:"${CONF_SIGNAL_SSL | false}"`
	SignalCertPath string `mapstructure:"signalCertPath" default:"${CONF_SIGNAL_CERT_PATH}"`
	SignalKeyPath  string `mapstructure:"signalKeyPath" default:"${CONF_SIGNAL_KEY_PATH}"`
	SignalCors     string `mapstructure:"signalCors" default:"${CONF_SIGNAL_CORS}"`
	WebRTC         struct {
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
		Enable          bool   `mapstructure:"enable" json:"enable" default:"${CONF_RECORD_ENABLE}"`
		DirPath         string `mapstructure:"dirPath" json:"dirPath" default:"${CONF_RECORD_DIR_PATH}"`
		IndexName       string `mapstructure:"indexName" json:"indexName" default:"${CONF_RECORD_INDEX_NAME}"`
		SegmentDuration int    `mapstructure:"segmentDuration" json:"segmentDuration" default:"${CONF_RECORD_SEGMENT_DURATION | 6}"`
		GopSize         int    `mapstructure:"gopSize" json:"gopSize" default:"${CONF_RECORD_GOP_SIZE | 3}"`
		DBIndex         struct {
			Enable     bool   `mapstructure:"enable" json:"enable" default:"${CONF_RECORD_DBINDEX_ENABLE}"`
			MongoUrl   string `mapstructure:"mongoUrl" json:"mongoUrl" default:"${CONF_RECORD_DBINDEX_MONGO_URL}"`
			Database   string `mapstructure:"database" json:"database" default:"${CONF_RECORD_DBINDEX_DATABASE}"`
			Collection string `mapstructure:"collection" json:"collection" default:"${CONF_RECORD_DBINDEX_COLLECTION}"`
			Auth       struct {
				User string `mapstructure:"user" json:"user" default:"${CONF_RECORD_DBINDEX_AUTH_USER}"`
				Pass string `mapstructure:"pass" json:"pass" default:"${CONF_RECORD_DBINDEX_AUTH_PASS}"`
			} `mapstructure:"auth" json:"auth" default:""`
		} `mapstructure:"dbIndex" json:"dbIndex" default:""`
	} `mapstructure:"record" json:"record" default:""`
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
	"record.dirPath:string",
	"record.indexName:string",
	"record.segmentDuration:int",
	"record.gopSize:int",
	"record.dbIndex.enable:bool",
	"record.dbIndex.mongoUrl:string",
	"record.dbIndex.database:string",
	"record.dbIndex.collection:string",
	"record.dbIndex.auth.user:string",
	"record.dbIndex.auth.pass:string",
}
