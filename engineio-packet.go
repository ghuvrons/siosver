package siosver

import "bytes"

type eioPacketType byte

const (
	__EIO_PACKET_OPEN    eioPacketType = '0'
	__EIO_PACKET_CLOSE   eioPacketType = '1'
	__EIO_PACKET_PING    eioPacketType = '2'
	__EIO_PACKET_PONG    eioPacketType = '3'
	__EIO_PACKET_MESSAGE eioPacketType = '4'
	__EIO_PACKET_UPGRADE eioPacketType = '5'
	__EIO_PACKET_NOOP    eioPacketType = '6'
)

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

func decodeAsEngineIOPacket(b []byte) (*engineIOPacket, error) {
	var packet = &engineIOPacket{
		packetType: eioPacketType(b[0]),
		data:       b[1:],
	}
	return packet, nil
}
