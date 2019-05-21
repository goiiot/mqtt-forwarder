/*
 * Copyright Go-IIoT (https://github.com/goiiot)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package libmqtt

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"io/ioutil"
	"time"
)

var (
	ErrNotSupportedVersion = errors.New("mqtt version not supported ")
)

// Option is client option for connection options
type Option func(*AsyncClient) error

// WithCleanSession will set clean flag in connect packet
func WithCleanSession(f bool) Option {
	return func(c *AsyncClient) error {
		c.options.cleanSession = f
		return nil
	}
}

// WithIdentity for username and password
func WithIdentity(username, password string) Option {
	return func(c *AsyncClient) error {
		c.options.username = username
		c.options.password = password
		return nil
	}
}

// WithKeepalive set the keepalive interval (time in second)
func WithKeepalive(keepalive uint16, factor float64) Option {
	return func(c *AsyncClient) error {
		c.options.keepalive = time.Duration(keepalive) * time.Second
		if factor > 1 {
			c.options.keepaliveFactor = factor
		} else {
			factor = 1.2
		}
		return nil
	}
}

// WithAutoReconnect set client to auto reconnect to server when connection failed
func WithAutoReconnect(autoReconnect bool) Option {
	return func(c *AsyncClient) error {
		c.options.autoReconnect = autoReconnect
		return nil
	}
}

// WithBackoffStrategy will set reconnect backoff strategy
// firstDelay is the time to wait before retrying after the first failure
// maxDelay defines the upper bound of backoff delay
// factor is applied to the backoff after each retry.
//
// e.g. FirstDelay = 1s and Factor = 2
// 		then the SecondDelay is 2s, the ThirdDelay is 4s
func WithBackoffStrategy(firstDelay, maxDelay time.Duration, factor float64) Option {
	return func(c *AsyncClient) error {
		if firstDelay < time.Millisecond {
			firstDelay = time.Millisecond
		}

		if maxDelay < firstDelay {
			maxDelay = firstDelay
		}

		if factor < 1 {
			factor = 1
		}

		c.options.firstDelay = firstDelay
		c.options.maxDelay = maxDelay
		c.options.backOffFactor = factor
		return nil
	}
}

// WithClientID set the client id for connection
func WithClientID(clientID string) Option {
	return func(c *AsyncClient) error {
		c.options.clientID = clientID
		return nil
	}
}

// WithWill mark this connection as a will teller
func WithWill(topic string, qos QosLevel, retain bool, payload []byte) Option {
	return func(c *AsyncClient) error {
		c.options.isWill = true
		c.options.willTopic = topic
		c.options.willQos = qos
		c.options.willRetain = retain
		c.options.willPayload = payload
		return nil
	}
}

// WithSecureServer use server certificate for verification
// won't apply `WithTLS`, `WithCustomTLS`, `WithTLSReader` options
// when connecting to these servers
func WithSecureServer(servers ...string) Option {
	return func(c *AsyncClient) error {
		c.options.secureServers = append(c.options.secureServers, servers...)
		return nil
	}
}

// WithServer set client servers
// addresses should be in form of `ip:port` or `domain.name:port`,
// only TCP connection supported for now
func WithServer(servers ...string) Option {
	return func(c *AsyncClient) error {
		c.options.servers = append(c.options.servers, servers...)
		return nil
	}
}

// WithTLSReader set tls from client cert, key, ca reader, apply to all servers
// listed in `WithServer` Option
func WithTLSReader(certReader, keyReader, caReader io.Reader, serverNameOverride string, skipVerify bool) Option {
	return func(c *AsyncClient) error {
		b, err := ioutil.ReadAll(certReader)
		if err != nil {
			return err
		}

		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(b) {
			return err
		}

		// load cert-key pair
		certBytes, err := ioutil.ReadAll(certReader)
		if err != nil {
			return err
		}
		keyBytes, err := ioutil.ReadAll(keyReader)
		if err != nil {
			return err
		}
		cert, err := tls.X509KeyPair(certBytes, keyBytes)
		if err != nil {
			return err
		}

		c.options.tlsConfig = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: skipVerify,
			ClientCAs:          cp,
			ServerName:         serverNameOverride,
		}

		return nil
	}
}

// WithTLS set client tls from cert, key and ca file, apply to all servers
// listed in `WithServer` Option
func WithTLS(certFile, keyFile, caCert, serverNameOverride string, skipVerify bool) Option {
	return func(c *AsyncClient) error {
		b, err := ioutil.ReadFile(caCert)
		if err != nil {
			return err
		}

		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(b) {
			return err
		}

		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return err
		}

		c.options.tlsConfig = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: skipVerify,
			ClientCAs:          cp,
			ServerName:         serverNameOverride,
		}
		return nil
	}
}

// WithCustomTLS replaces the TLS options with a custom tls.Config
func WithCustomTLS(config *tls.Config) Option {
	return func(c *AsyncClient) error {
		c.options.tlsConfig = config
		return nil
	}
}

// WithDialTimeout for connection time out (time in second)
func WithDialTimeout(timeout uint16) Option {
	return func(c *AsyncClient) error {
		c.options.dialTimeout = time.Duration(timeout) * time.Second
		return nil
	}
}

// WithBuf is the alias of WithBufSize
var WithBuf = WithBufSize

// WithBufSize designate the channel size of send and recv
func WithBufSize(sendBuf, recvBuf int) Option {
	return func(c *AsyncClient) error {
		if sendBuf < 1 {
			sendBuf = 1
		}
		if recvBuf < 1 {
			recvBuf = 1
		}

		c.options.sendChanSize = sendBuf
		c.options.recvChanSize = recvBuf
		return nil
	}
}

// WithLog will create basic logger for the client
func WithLog(l LogLevel) Option {
	return func(c *AsyncClient) error {
		c.log = newLogger(l)
		return nil
	}
}

// WithVersion defines the mqtt protocol ProtoVersion in use
func WithVersion(version ProtoVersion, compromise bool) Option {
	return func(c *AsyncClient) error {
		switch version {
		case V311, V5:
			c.options.protoVersion = version
			c.options.protoCompromise = compromise
			return nil
		}

		return ErrNotSupportedVersion
	}
}

// WithRouter set the router for topic dispatch
func WithRouter(r TopicRouter) Option {
	return func(c *AsyncClient) error {
		if r != nil {
			c.router = r
		}
		return nil
	}
}

// WithPersist defines the persist method to be used
func WithPersist(method PersistMethod) Option {
	return func(c *AsyncClient) error {
		if method != nil {
			c.persist = method
		}
		return nil
	}
}

// clientOptions is the options for client to connect, reconnect, disconnect
type clientOptions struct {
	protoVersion     ProtoVersion  // mqtt protocol ProtoVersion
	protoCompromise  bool          // compromise to server protocol ProtoVersion
	sendChanSize     int           // send channel size
	recvChanSize     int           // recv channel size
	servers          []string      // server address strings
	secureServers    []string      // servers with valid tls certificates
	dialTimeout      time.Duration // dial timeout in second
	clientID         string        // used by ConnPacket
	username         string        // used by ConnPacket
	password         string        // used by ConnPacket
	keepalive        time.Duration // used by ConnPacket (time in second)
	keepaliveFactor  float64       // used for reasonable amount time to close conn if no ping resp
	cleanSession     bool          // used by ConnPacket
	isWill           bool          // used by ConnPacket
	willTopic        string        // used by ConnPacket
	willPayload      []byte        // used by ConnPacket
	willQos          byte          // used by ConnPacket
	willRetain       bool          // used by ConnPacket
	tlsConfig        *tls.Config   // tls config with client side cert
	maxDelay         time.Duration
	firstDelay       time.Duration
	backOffFactor    float64
	autoReconnect    bool
	defaultTlsConfig *tls.Config
}
