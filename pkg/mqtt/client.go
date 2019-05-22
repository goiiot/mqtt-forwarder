package mqtt

import (
	"fmt"
	"log"
	"time"

	"github.com/goiiot/libmqtt"
)

func CreateMQTTClient(config *Config) (libmqtt.Client, error) {
	subQos, err := TranslateQosLevel(config.Sub.QoS)
	if err != nil {
		return nil, err
	}

	mqttOpt := []libmqtt.Option{
		libmqtt.WithBackoffStrategy(time.Second, 20*time.Second, 1.2),
		libmqtt.WithAutoReconnect(true),
		libmqtt.WithRouter(libmqtt.NewRegexRouter()),
	}

	if config.ConnectPacket != nil {
		libmqtt.WithIdentity(config.ConnectPacket.Username, config.ConnectPacket.Password)
		libmqtt.WithClientID(config.ConnectPacket.ClientID)
		libmqtt.WithCleanSession(config.ConnectPacket.CleanSession)
		libmqtt.WithKeepalive(config.ConnectPacket.Keepalive, 1.2)
	}

	if config.TLS != nil {
		if config.TLS.KeyFile != "" && config.TLS.CertFile != "" {
			mqttOpt = append(mqttOpt, libmqtt.WithTLS(config.TLS.CertFile, config.TLS.KeyFile, config.TLS.CAFile, "", true))
		} else {
			mqttOpt = append(mqttOpt, libmqtt.WithSecureServer(config.BrokerAddr))
		}
	} else {
		mqttOpt = append(mqttOpt, libmqtt.WithServer(config.BrokerAddr))
	}

	client, err := libmqtt.NewClient(mqttOpt...)
	if err != nil {
		return nil, err
	}

	client.HandleNet(func(server string, err error) {
		if err != nil {
			log.Printf("exception happened when connecting server [%s]: %v", server, err)
		}
	})

	client.HandlePub(func(topic string, err error) {
		if err != nil {
			log.Printf("failed to publish message to topic [%s]: %v", topic, err)
			return
		}

		log.Printf("published message to topic [%s]", topic)
	})

	client.HandleSub(func(topics []*libmqtt.Topic, err error) {
		if err != nil {
			log.Printf("failed to subscribe topic %v: %v", topics, err)
			return
		}

		log.Printf("subscribed to topic %v", topics)
	})

	client.Connect(func(server string, code byte, err error) {
		if err != nil {
			log.Printf("failed to connect server [%v]: %v", server, err)
			return
		}

		if code != libmqtt.CodeSuccess {
			log.Printf("failed to connect server [%v] with code: %v", server, code)
			return
		}

		log.Printf("connected to mqtt broker, subscribing")
		client.Subscribe(&libmqtt.Topic{Qos: subQos, Name: config.Sub.Topic})
	})

	return client, nil
}

func TranslateQosLevel(qos int) (libmqtt.QosLevel, error) {
	switch qos {
	case 0:
		return libmqtt.Qos0, nil
	case 1:
		return libmqtt.Qos1, nil
	case 2:
		return libmqtt.Qos2, nil
	}

	return 0, fmt.Errorf("invalid qos level: %d", qos)
}
