package siosver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

type packetType byte

const (
	__SIO_PACKET_CONNECT       packetType = '0'
	__SIO_PACKET_DISCONNECT    packetType = '1'
	__SIO_PACKET_EVENT         packetType = '2'
	__SIO_PACKET_ACK           packetType = '3'
	__SIO_PACKET_CONNECT_ERROR packetType = '4'
	__SIO_PACKET_BINARY_EVENT  packetType = '5'
	__SIO_PACKET_BINARY_ACK    packetType = '6'
)

type packet struct {
	packetType  packetType
	namespace   string
	ackId       int
	data        interface{}
	numOfBuffer int
}

func newPacket(packetType packetType, data ...interface{}) *packet {
	p := &packet{
		packetType: packetType,
		ackId:      -1,
	}

	if len(data) > 0 {
		switch data[0].(type) {
		case []interface{}:
			if len(data) == 1 {
				p.data = data[0].([]interface{})
			} else {
				p.data = data
			}

		case uint, uint32, uint16, uint8, int, int32, int16, int8, float32, float64, string, bool, nil:
			p.data = data

		default:
			if len(data) == 1 {
				p.data = data[0]
			} else {
				p.data = data
			}
		}
	}
	return p
}

func (p *packet) withAck(ackId int) *packet {
	p.ackId = ackId

	switch p.data.(type) {
	case []interface{}:
		break
	default:
		p.data = []interface{}{p.data}
	}

	return p
}

func (p *packet) encode() (data string, buffers [](*bytes.Buffer)) {
	// TODO : what if packet type is binary

	buf := bytes.Buffer{}
	buffers = [](*bytes.Buffer){}
	isBinaryPacket := false

	// check buffers data
	sioPacketGetBuffer(&buffers, &(p.data))
	if len(buffers) > 0 {
		switch p.packetType {
		case __SIO_PACKET_EVENT:
			p.packetType = __SIO_PACKET_BINARY_EVENT
		case __SIO_PACKET_ACK:
			p.packetType = __SIO_PACKET_BINARY_ACK
		}
		isBinaryPacket = true
	}

	// packetType
	buf.WriteByte(byte(p.packetType))

	// num of buffers data
	if isBinaryPacket {
		fmt.Fprintf(&buf, "%d-", len(buffers))
	}

	// namespace
	if p.namespace != "" {
		fmt.Fprintf(&buf, "/%s,", p.namespace)
	}

	// ACK
	if p.ackId != -1 {
		fmt.Fprintf(&buf, "%d", p.ackId)
	}

	rawdata, _ := json.Marshal(p.data)
	buf.Write(rawdata)

	data = buf.String()
	return
}

func decodeToPacket(buf *bytes.Buffer) *packet {
	// TODO : return error

	tmpTypePacket, err := buf.ReadByte()
	if err != nil {
		return nil
	}

	typePacket := packetType(tmpTypePacket)
	packet := newPacket(typePacket)

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
			if packet.numOfBuffer == 0 && (typePacket == __SIO_PACKET_BINARY_EVENT || typePacket == __SIO_PACKET_BINARY_ACK) {
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
				packet.numOfBuffer = number

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
	// TODO : read buffer and add to packet data

	return packet
}

// Check is packet type is for messaging
func isSioPacketMessager(typePacket packetType) bool {
	switch typePacket {
	case __SIO_PACKET_EVENT, __SIO_PACKET_BINARY_EVENT, __SIO_PACKET_ACK, __SIO_PACKET_BINARY_ACK:
		return true
	}
	return false
}
