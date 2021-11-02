package siosver

import (
	"bytes"
	"strconv"
)

type sioPacketType byte

const (
	__SIO_PACKET_CONNECT       sioPacketType = 0x30
	__SIO_PACKET_DISCONNECT    sioPacketType = 0x31
	__SIO_PACKET_EVENT         sioPacketType = 0x32
	__SIO_PACKET_ACK           sioPacketType = 0x33
	__SIO_PACKET_CONNECT_ERROR sioPacketType = 0x34
	__SIO_PACKET_BINARY_EVENT  sioPacketType = 0x35
	__SIO_PACKET_BINARY_ACK    sioPacketType = 0x36
)

type socketIOPacket struct {
	packetType sioPacketType
	namespace  string
	id         int
	rawdata    []byte
	data       interface{}
	argumentst []interface{}
}

func newSocketIOPacket(packetType sioPacketType, data ...interface{}) *socketIOPacket {
	packet := new(socketIOPacket)
	packet.packetType = packetType

	if len(data) > 0 {
		switch data[0].(type) {
		case []interface{}:
			if len(data) == 1 {
				packet.data = data[0].([]interface{})
			} else {
				packet.data = data
			}

		default:
			packet.data = data
		}
	}
	return packet
}
func decodeToSocketIOPacket(b []byte) *socketIOPacket {
	if len(b) == 0 {
		return nil
	}

	buf := bytes.NewBuffer(b)
	tmpTypePacket, _ := buf.ReadByte()
	typePacket := sioPacketType(tmpTypePacket)

	var namespace string
	var idAck string = "-1"
	var data []byte

	if typePacket == __SIO_PACKET_BINARY_EVENT {
		tmpNumOfBuffer, _ := buf.ReadString(byte('-'))
		tmpNumOfBuffer = tmpNumOfBuffer[:len(tmpNumOfBuffer)-1]
		_, err := strconv.Atoi(tmpNumOfBuffer)

		if err != nil {
			return nil
		}
	}

	for {
		tmp, _ := buf.ReadByte()
		buf.UnreadByte()

		if tmp == byte('/') {
			// get namespace
			tmpNamespace, _ := buf.ReadString(byte(','))
			strLen := len(tmpNamespace)
			if strLen > 0 {
				namespace = tmpNamespace[:strLen-1]
			}

		} else if isSioPacketMessager(typePacket) && tmp >= byte('0') && tmp <= byte('9') {
			// get ACK
			tmpId, _ := buf.ReadString('[')
			strLen := len(tmpId)
			lastByte := byte(tmpId[strLen-1])
			if lastByte < byte('0') || lastByte > byte('9') {
				buf.UnreadByte()
				strLen -= 1
			}
			idAck = tmpId[:strLen]

		} else if tmp == byte('[') {
			// get data
			data, _ = buf.ReadBytes(byte(']'))

		} else if tmp == byte('{') {
			// get data
			data, _ = buf.ReadBytes(byte('}'))

		} else {
			// get data
			data = buf.Bytes()
			break
		}
	}

	ack, err := strconv.Atoi(idAck)
	if err != nil {
		return nil
	}
	packet := &socketIOPacket{
		packetType: sioPacketType(typePacket),
		namespace:  namespace,
		id:         ack,
		rawdata:    data,
	}

	return packet
}

// Check is packet type is for messaging
func isSioPacketMessager(typePacket sioPacketType) bool {
	messagerPacket := []sioPacketType{
		__SIO_PACKET_EVENT,
		__SIO_PACKET_BINARY_EVENT,
		__SIO_PACKET_ACK,
		__SIO_PACKET_BINARY_ACK,
	}
	for _, v := range messagerPacket {
		if v == typePacket {
			return true
		}
	}
	return false
}
