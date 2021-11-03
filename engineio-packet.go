package siosver

import (
	"bytes"
	"encoding/base64"
	"io"
)

type eioPacketType byte

const (
	__EIO_PACKET_OPEN    eioPacketType = '0'
	__EIO_PACKET_CLOSE   eioPacketType = '1'
	__EIO_PACKET_PING    eioPacketType = '2'
	__EIO_PACKET_PONG    eioPacketType = '3'
	__EIO_PACKET_MESSAGE eioPacketType = '4'
	__EIO_PACKET_UPGRADE eioPacketType = '5'
	__EIO_PACKET_NOOP    eioPacketType = '6'
	__EIO_PAYLOAD        eioPacketType = 'b'
)

const __EIO_DELIMITER byte = 0x1E

type engineIOPacket struct {
	packetType eioPacketType
	data       []byte
}

func newEngineIOPacket(typePacket eioPacketType, data []byte) *engineIOPacket {
	var packet = &engineIOPacket{typePacket, data}
	return packet
}

func (packet *engineIOPacket) encode() []byte {
	buf := bytes.Buffer{}
	buf.WriteByte(byte(packet.packetType))
	buf.Write(packet.data)
	return buf.Bytes()
}

func decodeAsEngineIOPacket(buf *bytes.Buffer) (*engineIOPacket, error) {
	packetType, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}

	packet := &engineIOPacket{
		packetType: eioPacketType(packetType),
	}

	packet.data, err = buf.ReadBytes(__EIO_DELIMITER)

	if err != nil && err != io.EOF {
		return nil, err
	}

	// remove delimiter
	if lenbytes := len(packet.data); lenbytes != 0 && packet.data[lenbytes-1] == __EIO_DELIMITER {
		packet.data = packet.data[:lenbytes-1]
	}

	if packet.packetType == __EIO_PAYLOAD {
		packet.data, err = base64.StdEncoding.DecodeString(string(packet.data))
		if err != nil {
			return nil, err
		}
	}
	return packet, nil
}
