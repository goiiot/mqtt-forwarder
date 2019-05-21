package mqtt

type TLSConfig struct {
	CAFile   string `yaml:"ca_file"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type ConnectPacket struct {
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	CleanSession bool   `yaml:"clean_session"`
	ClientID     string `yaml:"client_id"`
	Keepalive    uint16 `yaml:"keepalive"`
}

type Config struct {
	BrokerAddr string `yaml:"broker"`

	// TLS options
	TLS           *TLSConfig     `yaml:"tls"`
	// ConnectPacket sent when connecting mqtt broker
	ConnectPacket *ConnectPacket `yaml:"connect_packet"`

	// Sub the topic to subscribe with QoS
	Sub struct {
		Topic string `yaml:"topic"`
		QoS   int    `yaml:"qos"`
	} `yaml:"sub"`

	// Pub the topic to publish messages with QoS
	Publish struct {
		Topic string `yaml:"topic"`
		QoS   int    `yaml:"qos"`
	} `yaml:"pub"`
}
