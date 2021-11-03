package siosver

import (
	"bytes"
	"encoding/json"
	"strconv"
)

type sioPacketType byte

const (
	__SIO_PACKET_CONNECT       sioPacketType = '0'
	__SIO_PACKET_DISCONNECT    sioPacketType = '1'
	__SIO_PACKET_EVENT         sioPacketType = '2'
	__SIO_PACKET_ACK           sioPacketType = '3'
	__SIO_PACKET_CONNECT_ERROR sioPacketType = '4'
	__SIO_PACKET_BINARY_EVENT  sioPacketType = '5'
	__SIO_PACKET_BINARY_ACK    sioPacketType = '6'
)

type socketIOPacket struct {
	packetType sioPacketType
	namespace  string
	ackId      int
	data       interface{}
	argumentst []interface{}
}

func newSocketIOPacket(packetType sioPacketType, data ...interface{}) *socketIOPacket {
	packet := &socketIOPacket{
		packetType: packetType,
		ackId:      -1,
	}

	if len(data) > 0 {
		switch data[0].(type) {
		case []interface{}:
			if len(data) == 1 {
				packet.data = data[0].([]interface{})
			} else {
				packet.data = data
			}

		default:
			if len(data) == 1 {
				packet.data = data[0]
			} else {
				packet.data = data
			}
		}
	}
	return packet
}

func (packet *socketIOPacket) nameSpace(namespace string) *socketIOPacket {
	packet.namespace = namespace
	return packet
}

func decodeToSocketIOPacket(b []byte) *socketIOPacket {
	if len(b) == 0 {
		return nil
	}

	buf := bytes.NewBuffer(b)
	tmpTypePacket, _ := buf.ReadByte()
	typePacket := sioPacketType(tmpTypePacket)
	packet := newSocketIOPacket(typePacket)
	packetBufIdx := 0

	for {
		if buf.Len() == 0 {
			break
		}

		tmp, _ := buf.ReadByte()
		buf.UnreadByte()

		if tmp == byte('/') {
			// get namespace
			buf.ReadByte()
			tmpNamespace, _ := buf.ReadString(byte(','))
			strLen := len(tmpNamespace)
			if strLen > 0 {
				packet.namespace = tmpNamespace[:strLen-1]
			}

		} else if isSioPacketMessager(typePacket) && tmp >= byte('0') && tmp <= byte('9') {
			// get ACK or num of binary
			var tmpNumber string
			isGetNumOfBinary := false

			// get string bytes
			if packetBufIdx == 0 && (typePacket == __SIO_PACKET_BINARY_EVENT || typePacket == __SIO_PACKET_BINARY_ACK) {
				isGetNumOfBinary = true
				tmpNumber, _ = buf.ReadString('-')

			} else {
				tmpNumber, _ = buf.ReadString('[')
			}

			// convert string bytes to integer
			strLen := len(tmpNumber)
			lastByte := byte(tmpNumber[strLen-1])

			if lastByte < byte('0') || lastByte > byte('9') {
				buf.UnreadByte()
				strLen -= 1
			}

			number, err := strconv.Atoi(tmpNumber[:strLen])
			if err != nil {
				return nil
			}

			// save
			if isGetNumOfBinary {
				buf.ReadByte()
				packetBufIdx = number

			} else {
				packet.ackId = number
			}

		} else if tmp == byte('{') || tmp == byte('[') {
			// get data
			dec := json.NewDecoder(buf)
			if err := dec.Decode(&(packet.data)); err != nil {
				return nil
			}
			break

		} else {
			return nil
		}
	}

	// read buffer

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
