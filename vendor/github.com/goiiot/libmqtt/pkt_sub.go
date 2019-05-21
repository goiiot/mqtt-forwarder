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

// SubscribePacket is sent from the Client to the Server
// to create one or more Subscriptions.
//
// Each Subscription registers a Client's interest in one or more TopicNames.
// The Server sends PublishPackets to the Client in order to forward
// Application Messages that were published to TopicNames that match these Subscriptions.
// The SubscribePacket also specifies (for each Subscription)
// the maximum QoS with which the Server can send Application Messages to the Client
type SubscribePacket struct {
	BasePacket
	PacketID uint16
	Topics   []*Topic
	Props    *SubscribeProps
}

// Type of SubscribePacket is CtrlSubscribe
func (s *SubscribePacket) Type() CtrlType {
	return CtrlSubscribe
}

func (s *SubscribePacket) Bytes() []byte {
	if s == nil {
		return nil
	}

	w := &bytes.Buffer{}
	s.WriteTo(w)
	return w.Bytes()
}

func (s *SubscribePacket) WriteTo(w BufferedWriter) error {
	if s == nil {
		return ErrEncodeBadPacket
	}

	switch s.ProtoVersion {
	case 0, V311:
		w.WriteByte(byte(CtrlSubscribe<<4 | 0x02))
		payload := s.payload()
		if err := writeVarInt(len(payload)+2, w); err != nil {
			return err
		}
		w.WriteByte(byte(s.PacketID >> 8))
		w.WriteByte(byte(s.PacketID))
		_, err := w.Write(payload)
		return err
	case V5:
		w.WriteByte(byte(CtrlSubscribe<<4 | 0x02))

		props := s.Props.props()
		payload := s.payload()
		propLen := len(props)

		if err := writeVarInt(len(payload)+propLen+2, w); err != nil {
			return err
		}

		w.WriteByte(byte(s.PacketID >> 8))
		w.WriteByte(byte(s.PacketID))

		writeVarInt(propLen, w)
		w.Write(props)

		_, err := w.Write(payload)
		return err
	default:
		return ErrUnsupportedVersion
	}
}

func (s *SubscribePacket) payload() []byte {
	var result []byte
	if s.Topics != nil {
		for _, t := range s.Topics {
			result = append(result, encodeStringWithLen(t.Name)...)
			result = append(result, t.Qos)
		}
	}
	return result
}

// SubscribeProps properties for SubscribePacket
type SubscribeProps struct {
	// SubID identifier of the subscription
	SubID uint32
	// UserProps User defined Properties
	UserProps UserProps
}

func (s *SubscribeProps) props() []byte {
	if s == nil {
		return nil
	}

	result := make([]byte, 0)

	if s.SubID != 0 {
		buf := &bytes.Buffer{}
		writeVarInt(int(s.SubID), buf)
		result = append(result, propKeySubID)
		result = append(result, buf.Bytes()...)
	}

	if s.UserProps != nil {
		s.UserProps.encodeTo(result)
	}
	return result
}

func (s *SubscribeProps) setProps(props map[byte][]byte) {
	if s == nil || props == nil {
		return
	}

	if v, ok := props[propKeySubID]; ok {
		id, _ := getRemainLength(bytes.NewReader(v))
		s.SubID = uint32(id)
	}

	if v, ok := props[propKeyUserProps]; ok {
		s.UserProps = getUserProps(v)
	}
}

// SubAckPacket is sent by the Server to the Client
// to confirm receipt and processing of a SubscribePacket.
//
// SubAckPacket contains a list of return codes,
// that specify the maximum QoS level that was granted in
// each Subscription that was requested by the SubscribePacket.
type SubAckPacket struct {
	BasePacket
	PacketID uint16
	Codes    []byte
	Props    *SubAckProps
}

// Type of SubAckPacket is CtrlSubAck
func (s *SubAckPacket) Type() CtrlType {
	return CtrlSubAck
}

func (s *SubAckPacket) Bytes() []byte {
	if s == nil {
		return nil
	}

	w := &bytes.Buffer{}
	s.WriteTo(w)
	return w.Bytes()
}

func (s *SubAckPacket) WriteTo(w BufferedWriter) error {
	if s == nil {
		return ErrEncodeBadPacket
	}

	switch s.ProtoVersion {
	case 0, V311:
		w.WriteByte(byte(CtrlSubAck << 4))
		payload := s.payload()
		if err := writeVarInt(len(payload)+2, w); err != nil {
			return err
		}
		w.WriteByte(byte(s.PacketID >> 8))
		w.WriteByte(byte(s.PacketID))
		_, err := w.Write(payload)
		return err
	case V5:
		w.WriteByte(byte(CtrlSubAck << 4))

		props := s.Props.props()
		payload := s.payload()
		propLen := len(props)

		if err := writeVarInt(len(payload)+propLen+2, w); err != nil {
			return err
		}

		w.WriteByte(byte(s.PacketID >> 8))
		w.WriteByte(byte(s.PacketID))

		writeVarInt(propLen, w)
		w.Write(props)

		_, err := w.Write(payload)
		return err
	default:
		return ErrUnsupportedVersion
	}
}

func (s *SubAckPacket) payload() []byte {
	return s.Codes
}

// SubAckProps properties for SubAckPacket
type SubAckProps struct {
	// Human readable string designed for diagnostics
	Reason string

	// UserProps User defined Properties
	UserProps UserProps
}

func (p *SubAckProps) props() []byte {
	if p == nil {
		return nil
	}

	result := make([]byte, 0)
	if p.Reason != "" {
		result = append(result, propKeyReasonString)
		result = append(result, encodeStringWithLen(p.Reason)...)
	}

	if p.UserProps != nil {
		p.UserProps.encodeTo(result)
	}
	return result
}

func (p *SubAckProps) setProps(props map[byte][]byte) {
	if p == nil || props == nil {
		return
	}

	if v, ok := props[propKeyReasonString]; ok {
		p.Reason, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyUserProps]; ok {
		p.UserProps = getUserProps(v)
	}
}

// UnSubPacket is sent by the Client to the Server,
// to unsubscribe from topics.
type UnSubPacket struct {
	BasePacket
	PacketID   uint16
	TopicNames []string
	Props      *UnSubProps
}

// Type of UnSubPacket is CtrlUnSub
func (s *UnSubPacket) Type() CtrlType {
	return CtrlUnSub
}

func (s *UnSubPacket) Bytes() []byte {
	if s == nil {
		return nil
	}

	w := &bytes.Buffer{}
	s.WriteTo(w)
	return w.Bytes()
}

func (s *UnSubPacket) WriteTo(w BufferedWriter) error {
	if s == nil {
		return ErrEncodeBadPacket
	}

	switch s.ProtoVersion {
	case 0, V311:
		w.WriteByte(byte(CtrlUnSub<<4 | 0x02))
		payload := s.payload()
		if err := writeVarInt(len(payload)+2, w); err != nil {
			return err
		}
		w.WriteByte(byte(s.PacketID >> 8))
		w.WriteByte(byte(s.PacketID))
		_, err := w.Write(payload)
		return err
	case V5:
		w.WriteByte(byte(CtrlUnSub<<4 | 0x02))
		props := s.Props.props()
		payload := s.payload()
		propLen := len(props)

		if err := writeVarInt(len(payload)+propLen+2, w); err != nil {
			return err
		}

		w.WriteByte(byte(s.PacketID >> 8))
		w.WriteByte(byte(s.PacketID))

		writeVarInt(propLen, w)
		w.Write(props)

		_, err := w.Write(payload)
		return err
	default:
		return ErrUnsupportedVersion
	}
}

func (s *UnSubPacket) payload() []byte {
	result := make([]byte, 0)
	if s.TopicNames != nil {
		for _, t := range s.TopicNames {
			result = append(result, encodeStringWithLen(t)...)
		}
	}
	return result
}

// UnSubProps properties for UnSubPacket
type UnSubProps struct {
	// UserProps User defined Properties
	UserProps UserProps
}

func (p *UnSubProps) props() []byte {
	if p == nil {
		return nil
	}
	result := make([]byte, 0)
	if p.UserProps != nil {
		p.UserProps.encodeTo(result)
	}
	return result
}

func (p *UnSubProps) setProps(props map[byte][]byte) {
	if p == nil || props == nil {
		return
	}

	if v, ok := props[propKeyUserProps]; ok {
		p.UserProps = getUserProps(v)
	}
}

// UnSubAckPacket is sent by the Server to the Client to confirm
// receipt of an UnSubPacket
type UnSubAckPacket struct {
	BasePacket
	PacketID uint16
	Props    *UnSubAckProps
}

// Type of UnSubAckPacket is CtrlUnSubAck
func (s *UnSubAckPacket) Type() CtrlType {
	return CtrlUnSubAck
}

func (s *UnSubAckPacket) Bytes() []byte {
	if s == nil {
		return nil
	}

	w := &bytes.Buffer{}
	s.WriteTo(w)
	return w.Bytes()
}

func (s *UnSubAckPacket) WriteTo(w BufferedWriter) error {
	if s == nil {
		return ErrEncodeBadPacket
	}

	switch s.ProtoVersion {
	case 0, V311:
		w.WriteByte(byte(CtrlUnSubAck << 4))
		w.WriteByte(0x02)
		w.WriteByte(byte(s.PacketID >> 8))
		return w.WriteByte(byte(s.PacketID))
	case V5:
		w.WriteByte(byte(CtrlUnSubAck << 4))
		w.WriteByte(0x02)
		w.WriteByte(byte(s.PacketID >> 8))
		w.WriteByte(byte(s.PacketID))

		props := s.Props.props()
		writeVarInt(len(props), w)
		_, err := w.Write(props)
		return err
	default:
		return ErrUnsupportedVersion
	}
}

// UnSubAckProps properties for UnSubAckPacket
type UnSubAckProps struct {
	// Human readable string designed for diagnostics
	Reason string

	// UserProps User defined Properties
	UserProps UserProps
}

func (p *UnSubAckProps) props() []byte {
	if p == nil {
		return nil
	}
	result := make([]byte, 0)
	if p.Reason != "" {
		result = append(result, propKeyReasonString)
		result = append(result, encodeStringWithLen(p.Reason)...)
	}

	if p.UserProps != nil {
		p.UserProps.encodeTo(result)
	}
	return result
}

func (p *UnSubAckProps) setProps(props map[byte][]byte) {
	if p == nil || props == nil {
		return
	}

	if v, ok := props[propKeyReasonString]; ok {
		p.Reason, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyUserProps]; ok {
		p.UserProps = getUserProps(v)
	}
}
