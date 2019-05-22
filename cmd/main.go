package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/goiiot/libmqtt"
	"github.com/goiiot/mqtt-forwarder/pkg/mqtt"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Listen     string      `yaml:"listen"`
	SendTo     string      `yaml:"send_to"`
	LogData    bool        `yaml:"log_data"`
	MaxMsgSize int         `yaml:"max_msg_size"`
	MQTTConfig mqtt.Config `yaml:"mqtt"`
}

var (
	configFile string
	config     Config
	listenConn net.PacketConn
)

var cmd = &cobra.Command{
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		configBytes, err := ioutil.ReadFile(configFile)
		if err != nil {
			return err
		}

		if err = yaml.Unmarshal(configBytes, &config); err != nil {
			return err
		}

		cmd.SetArgs(os.Args[1:])
		if err = cmd.ParseFlags(os.Args[1:]); err != nil {
			return err
		}

		ctx, exit := context.WithCancel(context.Background())
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGKILL, syscall.SIGTERM)
		go func() {
			count := 0
			for range sigCh {
				count++
				switch count {
				case 1:
					if listenConn != nil {
						_ = listenConn.Close()
					}

					exit()
				case 2:
					os.Exit(1)
				}
			}
		}()

		return run(ctx, &config)
	},
}

func init() {
	flags := cmd.Flags()

	flags.StringVarP(&configFile, "config", "c", "", "set config file path")

	flags.StringVarP(&config.Listen, "listen", "l", "udp://localhost:1234", "set listening address to receive payloads")
	flags.StringVarP(&config.SendTo, "send-to", "t", "", "set endpoint to send received mqtt messages (udp/unixgram)")
	flags.BoolVar(&config.LogData, "log-data", false, "whether log data received or sending")
	flags.IntVar(&config.MaxMsgSize, "max-msg-size", 1500, "max packet message size (the size of the buffer used to receive udp/unixgram messages)")

	flags.StringVarP(&config.MQTTConfig.BrokerAddr, "mqtt-broker-addr", "b", "", "set mqtt broker address e.g. `tcp://example.com:1883`")
	flags.StringVarP(&config.MQTTConfig.Sub.Topic, "mqtt-sub-topic", "s", "", "set topic to publish messages received")
	flags.StringVarP(&config.MQTTConfig.Publish.Topic, "mqtt-pub-topic", "p", "", "set topic to receive broker messages")

	_ = cmd.MarkFlagRequired("config")
}

func main() {
	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to execute cmd: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, config *Config) error {
	if config.SendTo == "" {
		return fmt.Errorf("no send_to addr provided")
	}

	pfURL, err := url.Parse(config.SendTo)
	if err != nil {
		return fmt.Errorf("failed to parse send_to addr: %v", err)
	}

	switch pfURL.Scheme {
	case "unixgram":
		_, err = net.ResolveUnixAddr(pfURL.Scheme, pfURL.Host)
	case "udp", "udp4", "udp6":
		_, err = net.ResolveUDPAddr(pfURL.Scheme, pfURL.Host)
	default:
		return fmt.Errorf("unsupported send_to addr: %s", config.SendTo)
	}
	if err != nil {
		return fmt.Errorf("failed to resolve send_to addr [%s]: %v", config.SendTo, err)
	}

	d := &net.Dialer{}
	pfConn, err := d.DialContext(ctx, pfURL.Scheme, pfURL.Host)
	if err != nil {
		return err
	}

	if err != nil {
		return fmt.Errorf("failed to resolve send_to addr: %v", err)
	}

	if config.Listen == "" {
		return fmt.Errorf("no listen addr provided")
	}

	listenURL, err := url.Parse(config.Listen)
	if err != nil {
		return fmt.Errorf("failed to parse listen addr: %v", err)
	}

	listenConn, err = net.ListenPacket(listenURL.Scheme, listenURL.Host)
	if err != nil {
		return fmt.Errorf("failed to listen addr: %v", err)
	}

	client, err := mqtt.CreateMQTTClient(&config.MQTTConfig)
	if err != nil {
		return fmt.Errorf("failed to create mqtt client: %v", err)
	}

	pubQos, err := mqtt.TranslateQosLevel(config.MQTTConfig.Publish.QoS)
	if err != nil {
		return err
	}

	client.Handle(config.MQTTConfig.Sub.Topic, func(topic string, qos libmqtt.QosLevel, msg []byte) {
		log.Printf("recv msg from topic [%v]", topic)
		if config.LogData {
			log.Printf("sending data to send_to [%s]: %s", config.SendTo, base64.StdEncoding.EncodeToString(msg))
		}
		_, err := pfConn.Write(msg)
		if err != nil {
			log.Printf("failed to send message to send_to [%s]: %v", config.SendTo, err)
		}
	})

	if config.MaxMsgSize < 0 {
		config.MaxMsgSize = 1500
	}

	log.Printf("listening at [%s], will send mqtt msg to [%s]", config.Listen, config.SendTo)

	buf := make([]byte, config.MaxMsgSize)
	for {
		n, rAddr, err := listenConn.ReadFrom(buf[0:])
		log.Printf("recv %d bytes", n)
		if n > 0 {
			data := make([]byte, n)
			_ = copy(data, buf)

			if config.LogData {
				log.Printf("publishing data to topic [%s]: %s", config.MQTTConfig.Publish.Topic, base64.StdEncoding.EncodeToString(data))
			}

			client.Publish(&libmqtt.PublishPacket{
				TopicName: config.MQTTConfig.Publish.Topic,
				Qos:       pubQos,
				Payload:   data,
			})
		}

		select {
		case <-ctx.Done():
			client.Destroy(true)
			return nil
		default:
			if err != nil {
				log.Printf("exception in readFrom %v: %v", rAddr, err)
			}
		}
	}
}
