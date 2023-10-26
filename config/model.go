package config

type ConferenceConfigure struct {
	SignalHost string `mapstructure:"signalHost" default:"localhost"`
	SignalPort uint16 `mapstructure:"signalPort" default:"8080"`
	SignalSsl bool `mapstructure:"signalSsl" default:"false"`
	SignalCertPath string `mapstructure:"signalCertPath" default:""`
	SignalKeyPath string `mapstructure:"signalKeyPath" default:""`
	AuthServerHost string `mapstructure:"authServerHost" default:"localhost"`
	AuthServerPort uint16 `mapstructure:"authServerPort" default:"3100"`
	AuthServerSsl bool `mapstructure:"authServerSsl" default:"false"`
	AuthServerCertPath string `mapstructure:"authServerCertPath" default:""`
	AuthServerKeyPath string `mapstructure:"authServerKeyPath" default:""`
	SecretKey string `mapstructure:"secretKey"`
}

var KEYS = []string{ 
	"signalHost:string", 
	"signalPort:uint16",
	"signalSsl:bool",
	"signalCertPath:string",
	"signalKeyPath:string",
	"authServerHost:string", 
	"authServerPort:uint16",
	"authServerSsl:bool",
	"authServerCertPath:string",
	"authServerKeyPath:string",
	"secretKey:string",
}
var ENVS = map[string]string{
	"SIGNAL_HOST": "signalHost",
	"SIGNAL_PORT": "signalPort",
	"SIGNAL_SSL": "signalSsl",
	"SIGNAL_CERT_PATH": "signalCertPath",
	"SIGNAL_KEY_PATH": "signalKeyPath",
	"AUTH_SERVER_HOST": "authServerHost",
	"AUTH_SERVER_PORT": "authServerPort",
	"AUTH_SERVER_SSL": "authServerSsl",
	"AUTH_SERVER_CERT_PATH": "authServerCertPath",
	"AUTH_SERVER_KEY_PATH": "authServerKeyPath",
	"SECRET_KEY": "secretKey",
 }