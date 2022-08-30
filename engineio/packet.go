package engineio

import (
	"bytes"
	"encoding/base64"
	"io"
)

type eioPacketType byte

const (
	PACKET_OPEN    eioPacketType = '0'
	PACKET_CLOSE   eioPacketType = '1'
	PACKET_PING    eioPacketType = '2'
	PACKET_PONG    eioPacketType = '3'
	PACKET_MESSAGE eioPacketType = '4'
	PACKET_UPGRADE eioPacketType = '5'
	PACKET_NOOP    eioPacketType = '6'
	PACKET_PAYLOAD eioPacketType = 'b'
)

const DELIMITER byte = 0x1E

type Packet struct {
	Type     eioPacketType
	Data     []byte
	callback chan bool
}

func NewPacket(packetType eioPacketType, data []byte) *Packet {
	var packet = &Packet{
		Type: packetType,
		Data: data,
	}
	return packet
}

func (packet *Packet) encode(isBase64 ...bool) []byte {
	buf := bytes.Buffer{}
	if packet.Type != PACKET_PAYLOAD {
		buf.WriteByte(byte(packet.Type))
		buf.Write(packet.Data)

	} else {
		if len(isBase64) > 0 && isBase64[0] {
			buf.WriteByte(byte(packet.Type))
			buf.WriteString(base64.StdEncoding.EncodeToString(packet.Data))

		} else {
			buf.Write(packet.Data)
		}
	}
	return buf.Bytes()
}

// Decode stream buffer to engineIOPacket.
// isPayload default is false.
func decodeAsEngineIOPacket(buf *bytes.Buffer, isPayloads ...bool) (*Packet, error) {
	var packet *Packet

	if len(isPayloads) != 0 && isPayloads[0] {
		packet = &Packet{
			Type: PACKET_PAYLOAD,
			Data: buf.Bytes(),
		}
		return packet, nil
	}

	packetType, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}

	packet = &Packet{
		Type: eioPacketType(packetType),
	}

	packet.Data, err = buf.ReadBytes(DELIMITER)

	if err != nil && err != io.EOF {
		return nil, err
	}

	// remove delimiter
	if lenbytes := len(packet.Data); lenbytes != 0 && packet.Data[lenbytes-1] == DELIMITER {
		packet.Data = packet.Data[:lenbytes-1]
	}

	// decode base64
	if packet.Type == PACKET_PAYLOAD {
		packet.Data, err = base64.StdEncoding.DecodeString(string(packet.Data))
		if err != nil {
			return nil, err
		}
	}
	return packet, nil
}
