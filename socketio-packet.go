package siosver

type sioTypePacket byte

const (
	__SIO_PACKET_CONNECT       sioTypePacket = 0x30
	__SIO_PACKET_DISCONNECT    sioTypePacket = 0x31
	__SIO_PACKET_EVENT         sioTypePacket = 0x32
	__SIO_PACKET_ACK           sioTypePacket = 0x33
	__SIO_PACKET_CONNECT_ERROR sioTypePacket = 0x34
	__SIO_PACKET_BINARY_EVENT  sioTypePacket = 0x35
	__SIO_PACKET_BINARY_ACK    sioTypePacket = 0x36
)

type sioPacket struct {
	typePacket sioTypePacket
	namespace  string
	id         int
	rawdata    []byte
	data       interface{}
	argumentst []interface{}
}

func newSioPacket(typePacket sioTypePacket, data ...interface{}) *sioPacket {
	packet := new(sioPacket)
	packet.typePacket = typePacket
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
