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
	"bufio"
	"context"
	"net"
	"time"
)

// clientConn is the wrapper of connection to server
// tend to actual packet send and receive
type clientConn struct {
	protoVersion ProtoVersion       // mqtt protocol version
	parent       Client             // client which created this connection
	name         string             // server addr info
	conn         net.Conn           // connection to server
	connRW       *bufio.ReadWriter  // make buffered connection
	logicSendC   chan Packet        // logic send channel
	netRecvC     chan Packet        // received packet from server
	keepaliveC   chan int           // keepalive packet
	ctx          context.Context    // context for single connection
	exit         context.CancelFunc // terminate this connection if necessary
}

// start mqtt logic
func (c *clientConn) logic() {
	defer func() {
		c.conn.Close()
		c.parent.log.e("NET exit logic for server =", c.name)
	}()

	// start keepalive if required
	if c.parent.options.keepalive > 0 {
		c.parent.workers.Add(1)
		go c.keepalive()
	}

	for {
		select {
		case <-c.ctx.Done():
			return
		case pkt, more := <-c.netRecvC:
			if !more {
				return
			}

			switch pkt.(type) {
			case *SubAckPacket:
				p := pkt.(*SubAckPacket)
				c.parent.log.v("NET received SubAck, id =", p.PacketID)

				if originPkt, ok := c.parent.idGen.getExtra(p.PacketID); ok {
					switch originPkt.(type) {
					case *SubscribePacket:
						originSub := originPkt.(*SubscribePacket)
						N := len(p.Codes)
						for i, v := range originSub.Topics {
							if i < N {
								v.Qos = p.Codes[i]
							}
						}
						c.parent.log.d("NET subscribed topics =", originSub.Topics)
						notifySubMsg(c.parent.msgCh, originSub.Topics, nil)
						c.parent.idGen.free(p.PacketID)

						notifyPersistMsg(c.parent.msgCh, c.parent.persist.Delete(sendKey(p.PacketID)))
					}
				}
			case *UnSubAckPacket:
				p := pkt.(*UnSubAckPacket)
				c.parent.log.v("NET received UnSubAck, id =", p.PacketID)

				if originPkt, ok := c.parent.idGen.getExtra(p.PacketID); ok {
					switch originPkt.(type) {
					case *UnSubPacket:
						originUnSub := originPkt.(*UnSubPacket)
						c.parent.log.d("NET unSubscribed topics", originUnSub.TopicNames)
						notifyUnSubMsg(c.parent.msgCh, originUnSub.TopicNames, nil)
						c.parent.idGen.free(p.PacketID)

						notifyPersistMsg(c.parent.msgCh, c.parent.persist.Delete(sendKey(p.PacketID)))
					}
				}
			case *PublishPacket:
				p := pkt.(*PublishPacket)
				c.parent.log.v("NET received publish, topic =", p.TopicName, "id =", p.PacketID, "QoS =", p.Qos)
				// received server publish, send to client
				c.parent.recvCh <- p

				// tend to QoS
				switch p.Qos {
				case Qos1:
					c.parent.log.d("NET send PubAck for Publish, id =", p.PacketID)
					c.send(&PubAckPacket{PacketID: p.PacketID})

					notifyPersistMsg(c.parent.msgCh, c.parent.persist.Store(recvKey(p.PacketID), pkt))
				case Qos2:
					c.parent.log.d("NET send PubRecv for Publish, id =", p.PacketID)
					c.send(&PubRecvPacket{PacketID: p.PacketID})

					notifyPersistMsg(c.parent.msgCh, c.parent.persist.Store(recvKey(p.PacketID), pkt))
				}
			case *PubAckPacket:
				p := pkt.(*PubAckPacket)
				c.parent.log.v("NET received PubAck, id =", p.PacketID)

				if originPkt, ok := c.parent.idGen.getExtra(p.PacketID); ok {
					switch originPkt.(type) {
					case *PublishPacket:
						originPub := originPkt.(*PublishPacket)
						if originPub.Qos == Qos1 {
							c.parent.log.d("NET published qos1 packet, topic =", originPub.TopicName)
							notifyPubMsg(c.parent.msgCh, originPub.TopicName, nil)
							c.parent.idGen.free(p.PacketID)

							notifyPersistMsg(c.parent.msgCh, c.parent.persist.Delete(sendKey(p.PacketID)))
						}
					}
				}
			case *PubRecvPacket:
				p := pkt.(*PubRecvPacket)
				c.parent.log.v("NET received PubRec, id =", p.PacketID)

				if originPkt, ok := c.parent.idGen.getExtra(p.PacketID); ok {
					switch originPkt.(type) {
					case *PublishPacket:
						originPub := originPkt.(*PublishPacket)
						if originPub.Qos == Qos2 {
							c.send(&PubRelPacket{PacketID: p.PacketID})
							c.parent.log.d("NET send PubRel, id =", p.PacketID)
						}
					}
				}
			case *PubRelPacket:
				p := pkt.(*PubRelPacket)
				c.parent.log.v("NET send PubRel, id =", p.PacketID)

				if originPkt, ok := c.parent.idGen.getExtra(p.PacketID); ok {
					switch originPkt.(type) {
					case *PublishPacket:
						originPub := originPkt.(*PublishPacket)
						if originPub.Qos == Qos2 {
							c.send(&PubCompPacket{PacketID: p.PacketID})
							c.parent.log.d("NET send PubComp, id =", p.PacketID)

							notifyPersistMsg(c.parent.msgCh, c.parent.persist.Store(recvKey(p.PacketID), pkt))
						}
					}
				}
			case *PubCompPacket:
				p := pkt.(*PubCompPacket)
				c.parent.log.v("NET received PubComp, id =", p.PacketID)

				if originPkt, ok := c.parent.idGen.getExtra(p.PacketID); ok {
					switch originPkt.(type) {
					case *PublishPacket:
						originPub := originPkt.(*PublishPacket)
						if originPub.Qos == Qos2 {
							c.send(&PubRelPacket{PacketID: p.PacketID})
							c.parent.log.d("NET send PubRel, id =", p.PacketID)
							c.parent.log.d("NET published qos2 packet, topic =", originPub.TopicName)
							notifyPubMsg(c.parent.msgCh, originPub.TopicName, nil)
							c.parent.idGen.free(p.PacketID)

							notifyPersistMsg(c.parent.msgCh, c.parent.persist.Delete(sendKey(p.PacketID)))
						}
					}
				}
			default:
				c.parent.log.v("NET received packet, type =", pkt.Type())
			}
		}
	}
}

// keepalive with server
func (c *clientConn) keepalive() {
	c.parent.log.d("NET start keepalive")

	t := time.NewTicker(c.parent.options.keepalive * 3 / 4)
	timeout := time.Duration(float64(c.parent.options.keepalive) * c.parent.options.keepaliveFactor)
	timeoutTimer := time.NewTimer(timeout)

	defer func() {
		t.Stop()
		timeoutTimer.Stop()
		c.parent.log.d("NET stop keepalive for server =", c.name)
		c.parent.workers.Done()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-t.C:
			c.send(PingReqPacket)

			select {
			case <-c.ctx.Done():
				return
			case _, more := <-c.keepaliveC:
				if !more {
					return
				}

				timeoutTimer.Reset(timeout)
			case <-timeoutTimer.C:
				c.parent.log.i("NET keepalive timeout")
				// exit client connection
				c.exit()
				return
			}
		}
	}
}

// handle mqtt logic control packet send
func (c *clientConn) handleSend() {
	c.parent.log.v("NET start send handle for server = ", c.name)

	defer func() {
		c.parent.workers.Done()
		c.parent.log.e("NET exit send handler for server =", c.name)
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case pkt, more := <-c.parent.sendCh:
			if !more {
				return
			}

			if err := pkt.WriteTo(c.connRW); err != nil {
				c.parent.log.e("NET encode error", err)
				return
			}

			if err := c.connRW.Flush(); err != nil {
				c.parent.log.e("NET flush error", err)
				return
			}

			switch pkt.Type() {
			case CtrlPublish:
				p := pkt.(*PublishPacket)
				if p.Qos == 0 {
					c.parent.log.d("NET published qos0 packet, topic =", p.TopicName)
					notifyPubMsg(c.parent.msgCh, p.TopicName, nil)
				}
			case CtrlDisConn:
				// client exit with disconnect
				c.parent.exit()
				return
			}
		case pkt, more := <-c.logicSendC:
			if !more {
				return
			}

			if err := pkt.WriteTo(c.connRW); err != nil {
				c.parent.log.e("NET encode error", err)
				return
			}

			if err := c.connRW.Flush(); err != nil {
				c.parent.log.e("NET flush error", err)
				return
			}

			switch pkt.Type() {
			case CtrlPubRel:
				notifyPersistMsg(c.parent.msgCh,
					c.parent.persist.Store(sendKey(pkt.(*PubRelPacket).PacketID), pkt))
			case CtrlPubAck:
				notifyPersistMsg(c.parent.msgCh,
					c.parent.persist.Delete(sendKey(pkt.(*PubAckPacket).PacketID)))
			case CtrlPubComp:
				notifyPersistMsg(c.parent.msgCh,
					c.parent.persist.Delete(sendKey(pkt.(*PubCompPacket).PacketID)))
			case CtrlDisConn:
				// disconnect to server
				c.conn.Close()
				return
			}
		}
	}
}

// handle all message receive
func (c *clientConn) handleRecv() {
	defer func() {
		c.parent.log.e("NET exit recv handler for server =", c.name)
		close(c.netRecvC)
		close(c.keepaliveC)

		c.parent.workers.Done()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			pkt, err := Decode(c.protoVersion, c.connRW)
			if err != nil {
				c.parent.log.e("NET connection broken, server =", c.name, "err =", err)

				// TODO send proper net error to net handler

				// exit client connection
				c.exit()
				return
			}

			if pkt == PingRespPacket {
				c.parent.log.d("NET received keepalive message")
				c.keepaliveC <- 1
			} else {
				c.netRecvC <- pkt
			}
		}
	}
}

// send mqtt logic packet
func (c *clientConn) send(pkt Packet) {
	if c.parent.isClosing() {
		return
	}

	c.logicSendC <- pkt
}
