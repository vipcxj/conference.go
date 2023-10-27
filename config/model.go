package config

type ConferenceConfigure struct {
	SignalEnable       bool   `mapstructure:"signalEnable" default:"${CONF_SIGNAL_ENABLE | true}"`
	SignalHost         string `mapstructure:"signalHost" default:"${CONF_SIGNAL_HOST | localhost}"`
	SignalPort         uint16 `mapstructure:"signalPort" default:"${CONF_SIGNAL_PORT | 8080}"`
	SignalSsl          bool   `mapstructure:"signalSsl" default:"${CONF_SIGNAL_SSL | false}"`
	SignalCertPath     string `mapstructure:"signalCertPath" default:"${CONF_SIGNAL_CERT_PATH}"`
	SignalKeyPath      string `mapstructure:"signalKeyPath" default:"${CONF_SIGNAL_KEY_PATH}"`
	SignalCors         string `mapstructure:"signalCors" default:"${CONF_SIGNAL_CORS}"`
	AuthServerEnable   bool   `mapstructure:"authServerEnable" default:"true"`
	AuthServerHost     string `mapstructure:"authServerHost" default:"localhost"`
	AuthServerPort     uint16 `mapstructure:"authServerPort" default:"3100"`
	AuthServerSsl      bool   `mapstructure:"authServerSsl" default:"false"`
	AuthServerCertPath string `mapstructure:"authServerCertPath" default:""`
	AuthServerKeyPath  string `mapstructure:"authServerKeyPath" default:""`
	AuthServerCors     string `mapstructure:"authServerCors" default:"${CONF_AUTH_SERVER_CORS}"`
	SecretKey          string `mapstructure:"secretKey"`
}

var KEYS = []string{
	"signalEnable:bool",
	"signalHost:string",
	"signalPort:uint16",
	"signalSsl:bool",
	"signalCertPath:string",
	"signalKeyPath:string",
	"signalCors:string",
	"authServerEnable:bool",
	"authServerHost:string",
	"authServerPort:uint16",
	"authServerSsl:bool",
	"authServerCertPath:string",
	"authServerKeyPath:string",
	"authServerCors:string",
	"secretKey:string",
}
var ENVS = map[string]string{
	"CONF_SIGNAL_ENABLE":         "signalEnable",
	"CONF_SIGNAL_HOST":           "signalHost",
	"CONF_SIGNAL_PORT":           "signalPort",
	"CONF_SIGNAL_SSL":            "signalSsl",
	"CONF_SIGNAL_CERT_PATH":      "signalCertPath",
	"CONF_SIGNAL_KEY_PATH":       "signalKeyPath",
	"CONF_AUTH_SERVER_ENABLE":    "authServerEnable",
	"CONF_AUTH_SERVER_HOST":      "authServerHost",
	"CONF_AUTH_SERVER_PORT":      "authServerPort",
	"CONF_AUTH_SERVER_SSL":       "authServerSsl",
	"CONF_AUTH_SERVER_CERT_PATH": "authServerCertPath",
	"CONF_AUTH_SERVER_KEY_PATH":  "authServerKeyPath",
	"CONF_SECRET_KEY":            "secretKey",
}
