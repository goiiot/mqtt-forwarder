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

import "bytes"

// ConnPacket is the first packet sent by Client to Server
type ConnPacket struct {
	BasePacket
	ProtoName string

	// Flags

	CleanSession bool
	IsWill       bool
	WillQos      QosLevel
	WillRetain   bool

	// Properties
	Props *ConnProps

	// Payloads
	Username    string
	Password    string
	ClientID    string
	Keepalive   uint16
	WillTopic   string
	WillMessage []byte
}

// Type ConnPacket's type is CtrlConn
func (c *ConnPacket) Type() CtrlType {
	return CtrlConn
}

func (c *ConnPacket) Bytes() []byte {
	if c == nil {
		return nil
	}

	w := &bytes.Buffer{}
	c.WriteTo(w)
	return w.Bytes()
}

func (c *ConnPacket) WriteTo(w BufferedWriter) error {
	if c == nil {
		return ErrEncodeBadPacket
	}

	switch c.ProtoVersion {
	case 0, V311:
		w.WriteByte(byte(CtrlConn << 4))
		payload := c.payload()
		if err := writeVarInt(len(payload)+10, w); err != nil {
			return err
		}
		w.Write(mqtt)
		w.WriteByte(byte(V311))
		w.WriteByte(c.flags())
		w.WriteByte(byte(c.Keepalive >> 8))
		w.WriteByte(byte(c.Keepalive))
		_, err := w.Write(payload)
		return err
	case V5:
		w.WriteByte(byte(CtrlConn << 4))

		props := c.Props.props()
		propLen := len(props)
		payload := c.payload()

		if err := writeVarInt(len(payload)+propLen+10, w); err != nil {
			return err
		}
		w.Write(mqtt)
		w.WriteByte(byte(V5))
		w.WriteByte(c.flags())
		w.WriteByte(byte(c.Keepalive >> 8))
		w.WriteByte(byte(c.Keepalive))

		writeVarInt(propLen, w)
		w.Write(props)

		_, err := w.Write(payload)
		return err
	default:
		return ErrUnsupportedVersion
	}
}

func (c *ConnPacket) flags() byte {
	var flag byte
	if c.ClientID == "" {
		c.CleanSession = true
	}

	if c.CleanSession {
		flag |= 0x02
	}

	if c.IsWill {
		flag |= 0x04
		flag |= c.WillQos << 3

		if c.WillRetain {
			flag |= 0x20
		}
	}

	if c.Password != "" {
		flag |= 0x40
	}

	if c.Username != "" {
		flag |= 0x80
	}

	return flag
}

func (c *ConnPacket) payload() []byte {
	// client id
	result := encodeStringWithLen(c.ClientID)

	// will topic and message
	if c.IsWill {
		result = append(result, encodeStringWithLen(c.WillTopic)...)
		result = append(result, encodeBytesWithLen(c.WillMessage)...)
	}

	if c.Username != "" {
		result = append(result, encodeStringWithLen(c.Username)...)
	}

	if c.Password != "" {
		result = append(result, encodeStringWithLen(c.Password)...)
	}

	return result
}

// ConnProps defines connect packet properties
type ConnProps struct {
	// If the Session Expiry Interval is absent the value 0 is used.
	// If it is set to 0, or is absent, the Session ends when the Network Connection is closed.
	// If the Session Expiry Interval is 0xFFFFFFFF (UINT_MAX), the Session does not expire.
	SessionExpiryInterval uint32

	// The Client uses this value to limit the number of QoS 1 and QoS 2 publications
	// that it is willing to process concurrently.
	//
	// There is no mechanism to limit the QoS 0 publications that the Server might try to send.
	//
	// The value of Receive Maximum applies only to the current Network Connection.
	// If the Receive Maximum value is absent then its value defaults to 65,535
	MaxRecv uint16

	// The Maximum Packet Size the Client is willing to accept
	//
	// If the Maximum Packet Size is not present,
	// no limit on the packet size is imposed beyond the limitations in the protocol as a result of the remaining length encoding and the protocol header sizes
	MaxPacketSize uint32

	// This value indicates the highest value that the Client will accept
	// as a Topic Alias sent by the Server.
	//
	// The Client uses this value to limit the number of Topic Aliases that
	// it is willing to hold on this Connection.
	MaxTopicAlias uint16

	// The Client uses this value to request the Server to return Response
	// Information in the ConnAckPacket
	ReqRespInfo bool

	// The Client uses this value to indicate whether the Reason String
	// or User Properties are sent in the case of failures.
	ReqProblemInfo bool

	// User defined Properties
	UserProps UserProps

	// If Authentication Method is absent, extended authentication is not performed.
	//
	// If a Client sets an Authentication Method in the ConnPacket,
	// the Client MUST NOT send any packets other than AuthPacket or DisConn packets
	// until it has received a ConnAck packet
	AuthMethod string

	// The contents of this data are defined by the authentication method.
	AuthData []byte
}

func (c *ConnProps) props() []byte {
	if c == nil {
		return nil
	}

	result := make([]byte, 0)
	if c.SessionExpiryInterval != 0 {
		data := []byte{propKeySessionExpiryInterval, 0, 0, 0, 0}
		putUint32(data[1:], c.SessionExpiryInterval)
		result = append(result, data...)
	}

	if c.MaxRecv != 0 {
		data := []byte{propKeyMaxRecv, 0, 0}
		putUint16(data[1:], c.MaxRecv)
		result = append(result, data...)
	}

	if c.MaxPacketSize != 0 {
		data := []byte{propKeyMaxPacketSize, 0, 0, 0, 0}
		putUint32(data[1:], c.MaxPacketSize)
		result = append(result, data...)
	}

	if c.MaxTopicAlias != 0 {
		data := []byte{propKeyMaxTopicAlias, 0, 0}
		putUint16(data[1:], c.MaxTopicAlias)
		result = append(result, data...)
	}

	if c.ReqRespInfo {
		result = append(result, propKeyReqRespInfo, 1)
	}

	if c.ReqProblemInfo {
		result = append(result, propKeyReqProblemInfo, 1)
	}

	if c.UserProps != nil {
		c.UserProps.encodeTo(result)
	}

	if c.AuthMethod != "" {
		result = append(result, propKeyAuthMethod)
		result = append(result, encodeStringWithLen(c.AuthMethod)...)
	}

	if c.AuthData != nil {
		result = append(result, propKeyAuthData)
		result = append(result, encodeBytesWithLen(c.AuthData)...)
	}

	return result
}

func (c *ConnProps) setProps(props map[byte][]byte) {
	if c == nil {
		return
	}

	if v, ok := props[propKeySessionExpiryInterval]; ok {
		c.SessionExpiryInterval = getUint32(v)
	}

	if v, ok := props[propKeyMaxRecv]; ok {
		c.MaxRecv = getUint16(v)
	}

	if v, ok := props[propKeyMaxPacketSize]; ok {
		c.MaxPacketSize = getUint32(v)
	}

	if v, ok := props[propKeyMaxTopicAlias]; ok {
		c.MaxTopicAlias = getUint16(v)
	}

	if v, ok := props[propKeyReqRespInfo]; ok && len(v) == 1 {
		c.ReqRespInfo = v[0] == 1
	}

	if v, ok := props[propKeyReqProblemInfo]; ok && len(v) == 1 {
		c.ReqProblemInfo = v[0] == 1
	}

	if v, ok := props[propKeyUserProps]; ok {
		c.UserProps = getUserProps(v)
	}

	if v, ok := props[propKeyAuthMethod]; ok {
		c.AuthMethod, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyAuthData]; ok {
		c.AuthData, _, _ = getBinaryData(v)
	}
}

// ConnAckPacket is the packet sent by the Server in response to a ConnPacket
// received from a Client.
//
// The first packet sent from the Server to the Client MUST be a ConnAckPacket
type ConnAckPacket struct {
	BasePacket
	Present bool
	Code    byte
	Props   *ConnAckProps
}

// Type ConnAckPacket's type is CtrlConnAck
func (c *ConnAckPacket) Type() CtrlType {
	return CtrlConnAck
}

func (c *ConnAckPacket) Bytes() []byte {
	if c == nil {
		return nil
	}

	w := &bytes.Buffer{}
	c.WriteTo(w)
	return w.Bytes()
}

func (c *ConnAckPacket) WriteTo(w BufferedWriter) error {
	if c == nil {
		return ErrEncodeBadPacket
	}

	switch c.ProtoVersion {
	case 0, V311:
		w.WriteByte(byte(CtrlConnAck << 4))
		w.WriteByte(2)
		w.WriteByte(boolToByte(c.Present))
		return w.WriteByte(c.Code)
	case V5:
		w.WriteByte(byte(CtrlConnAck << 4))

		props := c.Props.props()
		propLen := len(props)

		if err := writeVarInt(propLen+2, w); err != nil {
			return err
		}

		w.WriteByte(boolToByte(c.Present))
		w.WriteByte(c.Code)

		writeVarInt(propLen, w)
		_, err := w.Write(props)
		return err
	default:
		return ErrUnsupportedVersion
	}
}

// ConnAckProps defines connect acknowledge properties
type ConnAckProps struct {
	// If the Session Expiry Interval is absent the value in the ConnPacket used.
	// The server uses this property to inform the Client that it is using
	// a value other than that sent by the Client in the ConnAck
	SessionExpiryInterval uint32

	// The Server uses this value to limit the number of QoS 1 and QoS 2 publications
	// that it is willing to process concurrently for the Client.
	//
	// It does not provide a mechanism to limit the QoS 0 publications that
	// the Client might try to send
	MaxRecv uint16

	MaxQos QosLevel

	// Declares whether the Server supports retained messages.
	// true means that retained messages are not supported.
	// false means retained messages are supported
	RetainAvail bool

	// Maximum Packet Size the Server is willing to accept.
	// If the Maximum Packet Size is not present, there is no limit on the
	// packet size imposed beyond the limitations in the protocol as a
	// result of the remaining length encoding and the protocol header sizes
	MaxPacketSize uint32

	// The Client Identifier which was assigned by the Server
	// because a zero length Client Identifier was found in the ConnPacket
	AssignedClientID string

	// This value indicates the highest value that the Server will accept
	// as a Topic Alias sent by the Client.
	//
	// The Server uses this value to limit the number of Topic Aliases
	// that it is willing to hold on this Connection.
	MaxTopicAlias uint16

	// Human readable string designed for diagnostics
	Reason string

	// User defines Properties
	UserProps UserProps

	// Whether the Server supports Wildcard Subscriptions.
	// false means that Wildcard Subscriptions are not supported.
	// true means Wildcard Subscriptions are supported.
	//
	// default is true
	WildcardSubAvail bool // 40

	// Whether the Server supports Subscription Identifiers.
	// false means that Subscription Identifiers are not supported.
	// true means Subscription Identifiers are supported.
	//
	// default is true
	SubIDAvail bool

	// Whether the Server supports Shared Subscriptions.
	// false means that Shared Subscriptions are not supported.
	// true means Shared Subscriptions are supported
	//
	// default is true
	SharedSubAvail bool

	// Keep Alive time assigned by the Server
	ServerKeepalive uint16

	// Response Information
	RespInfo string

	// Can be used by the Client to identify another Server to use
	ServerRef string

	// The name of the authentication method
	AuthMethod string

	// The contents of this data are defined by the authentication method.
	AuthData []byte
}

func (c *ConnAckProps) props() []byte {
	if c == nil {
		return nil
	}

	result := make([]byte, 0)
	if c.SessionExpiryInterval != 0 {
		data := []byte{propKeySessionExpiryInterval, 0, 0, 0, 0}
		putUint32(data[1:], c.SessionExpiryInterval)
		result = append(result, data...)
	}

	if c.MaxRecv != 0 {
		data := []byte{propKeyMaxRecv, 0, 0}
		putUint16(data[1:], c.MaxRecv)
		result = append(result, data...)
	}

	if c.MaxQos != Qos2 {
		result = append(result, propKeyMaxQos, c.MaxQos)
	}

	if c.RetainAvail {
		result = append(result, propKeyRetainAvail, 1)
	}

	if c.MaxPacketSize != 0 {
		data := []byte{propKeyMaxPacketSize, 0, 0, 0, 0}
		putUint32(data[1:], c.MaxPacketSize)
		result = append(result, data...)
	}

	if c.AssignedClientID != "" {
		result = append(result, propKeyAssignedClientID)
		result = append(result, encodeStringWithLen(c.AssignedClientID)...)
	}

	if c.MaxTopicAlias != 0 {
		data := []byte{propKeyMaxTopicAlias, 0, 0}
		putUint16(data[1:], c.MaxTopicAlias)
		result = append(result, data...)
	}

	if c.Reason != "" {
		result = append(result, propKeyReasonString)
		result = append(result, encodeStringWithLen(c.Reason)...)
	}

	if c.UserProps != nil {
		c.UserProps.encodeTo(result)
	}

	if c.WildcardSubAvail {
		result = append(result, propKeyWildcardSubAvail, 1)
	}

	if c.SubIDAvail {
		result = append(result, propKeySubIDAvail, 1)
	}

	if c.SharedSubAvail {
		result = append(result, propKeySharedSubAvail, 1)
	}

	if c.ServerKeepalive != 0 {
		data := []byte{propKeyServerKeepalive, 0, 0}
		putUint16(data[1:], c.ServerKeepalive)
		result = append(result, data...)
	}

	if c.RespInfo != "" {
		result = append(result, propKeyRespInfo)
		result = append(result, encodeStringWithLen(c.RespInfo)...)
	}

	if c.ServerRef != "" {
		result = append(result, propKeyServerRef)
		result = append(result, encodeStringWithLen(c.ServerRef)...)
	}

	if c.AuthMethod != "" {
		result = append(result, propKeyAuthMethod)
		result = append(result, encodeStringWithLen(c.AuthMethod)...)
	}

	if c.AuthData != nil {
		result = append(result, propKeyAuthData)
		result = append(result, encodeBytesWithLen(c.AuthData)...)
	}

	return result
}

func (c *ConnAckProps) setProps(props map[byte][]byte) {
	if v, ok := props[propKeySessionExpiryInterval]; ok {
		c.SessionExpiryInterval = getUint32(v)
	}

	if v, ok := props[propKeyMaxRecv]; ok {
		c.MaxRecv = getUint16(v)
	}

	if v, ok := props[propKeyMaxQos]; ok && len(v) == 1 {
		c.MaxQos = v[0]
	}

	if v, ok := props[propKeyRetainAvail]; ok && len(v) == 1 {
		c.RetainAvail = v[0] == 1
	}

	if v, ok := props[propKeyMaxPacketSize]; ok {
		c.MaxPacketSize = getUint32(v)
	}

	if v, ok := props[propKeyAssignedClientID]; ok {
		c.AssignedClientID, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyMaxTopicAlias]; ok {
		c.MaxTopicAlias = getUint16(v)
	}

	if v, ok := props[propKeyReasonString]; ok {
		c.Reason, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyUserProps]; ok {
		c.UserProps = getUserProps(v)
	}

	if v, ok := props[propKeyWildcardSubAvail]; ok && len(v) == 1 {
		c.WildcardSubAvail = v[0] == 1
	}

	if v, ok := props[propKeyServerKeepalive]; ok {
		c.ServerKeepalive = getUint16(v)
	}

	if v, ok := props[propKeyRespInfo]; ok {
		c.RespInfo, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyServerRef]; ok {
		c.ServerRef, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyAuthMethod]; ok {
		c.AuthMethod, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyAuthData]; ok {
		c.AuthData, _, _ = getBinaryData(v)
	}
}

// DisConnPacket is the final Control Packet sent from the Client to the Server.
// It indicates that the Client is disconnecting cleanly.
type DisConnPacket struct {
	BasePacket
	Code  byte
	Props *DisConnProps
}

// Type of DisConnPacket is CtrlDisConn
func (d *DisConnPacket) Type() CtrlType {
	return CtrlDisConn
}

func (d *DisConnPacket) Bytes() []byte {
	if d == nil {
		return nil
	}

	w := &bytes.Buffer{}
	d.WriteTo(w)
	return w.Bytes()
}

func (d *DisConnPacket) WriteTo(w BufferedWriter) error {
	if d == nil {
		return ErrEncodeBadPacket
	}

	switch d.ProtoVersion {
	case 0, V311:
		w.WriteByte(byte(CtrlDisConn << 4))
		return w.WriteByte(0x00)
	case V5:
		w.WriteByte(byte(CtrlDisConn << 4))
		props := d.Props.props()

		tmpBuf := &bytes.Buffer{}
		writeVarInt(len(props), tmpBuf)

		if err := writeVarInt(len(props)+1+tmpBuf.Len(), w); err != nil {
			return err
		}

		w.WriteByte(d.Code)
		tmpBuf.WriteTo(w)
		_, err := w.Write(props)
		return err
	default:
		return ErrUnsupportedVersion
	}
}

// DisConnProps properties for DisConnPacket
type DisConnProps struct {
	// Session Expiry Interval in seconds
	// If the Session Expiry Interval is absent, the Session Expiry Interval in the CONNECT packet is used
	//
	// The Session Expiry Interval MUST NOT be sent on a DISCONNECT by the Server
	SessionExpiryInterval uint32

	// Human readable, designed for diagnostics and SHOULD NOT be parsed by the receiver
	Reason string

	// User defines Properties
	UserProps UserProps

	// Used by the Client to identify another Server to use
	ServerRef string
}

func (d *DisConnProps) props() []byte {
	if d == nil {
		return nil
	}

	result := make([]byte, 0)
	if d.SessionExpiryInterval != 0 {
		data := []byte{propKeySessionExpiryInterval, 0, 0, 0, 0}
		putUint32(data[1:], d.SessionExpiryInterval)
		result = append(result, data...)
	}

	if d.Reason != "" {
		result = append(result, propKeyReasonString)
		result = append(result, encodeStringWithLen(d.Reason)...)
	}

	if d.UserProps != nil {
		d.UserProps.encodeTo(result)
	}

	if d.ServerRef != "" {
		result = append(result, propKeyServerRef)
		result = append(result, encodeStringWithLen(d.ServerRef)...)
	}

	return result
}

func (d *DisConnProps) setProps(props map[byte][]byte) {
	if d == nil {
		return
	}

	if v, ok := props[propKeySessionExpiryInterval]; ok {
		d.SessionExpiryInterval = getUint32(v)
	}

	if v, ok := props[propKeyReasonString]; ok {
		d.Reason, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyUserProps]; ok {
		d.UserProps = getUserProps(v)
	}

	if v, ok := props[propKeyServerRef]; ok {
		d.ServerRef, _, _ = getStringData(v)
	}
}
