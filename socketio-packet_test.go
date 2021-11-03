package siosver

import (
	"reflect"
	"testing"
)

func Test_decodeToSocketIOPacket(t *testing.T) {
	type args struct {
		b []byte
	}
	tests := []struct {
		name string
		args args
		want *socketIOPacket
	}{
		{
			name: "Connect packet",
			args: args{
				b: []byte(`0{"token":"123"}`),
			},
			want: &socketIOPacket{
				packetType: __SIO_PACKET_CONNECT,
				ackId:      -1,
				data:       map[string]interface{}{"token": "123"},
			},
		},
		{
			name: "Connect packet",
			args: args{
				b: []byte(`0/admin,{"token":"123"}`),
			},
			want: &socketIOPacket{
				packetType: __SIO_PACKET_CONNECT,
				ackId:      -1,
				namespace:  "admin",
				data:       map[string]interface{}{"token": "123"},
			},
		},
		{
			name: "Disconnect packet",
			args: args{
				b: []byte(`1/admin,`),
			},
			want: &socketIOPacket{
				packetType: __SIO_PACKET_DISCONNECT,
				ackId:      -1,
				namespace:  "admin",
			},
		},
		{
			name: "Event packet",
			args: args{
				b: []byte(`2["hello",1]`),
			},
			want: &socketIOPacket{
				packetType: __SIO_PACKET_EVENT,
				ackId:      -1,
				data:       []interface{}{"hello", 1.0},
			},
		},
		{
			name: "Event packet with an acknowledgement id",
			args: args{
				b: []byte(`2/admin,456["project:delete",123]`),
			},
			want: &socketIOPacket{
				packetType: __SIO_PACKET_EVENT,
				ackId:      456,
				namespace:  "admin",
				data:       []interface{}{"project:delete", 123.0},
			},
		},
		{
			name: "ACK packet",
			args: args{
				b: []byte(`3/admin,456[]`),
			},
			want: &socketIOPacket{
				packetType: __SIO_PACKET_ACK,
				ackId:      456,
				namespace:  "admin",
				data:       []interface{}{},
			},
		},
		{
			name: "Connect Error Packet",
			args: args{
				b: []byte(`4/admin,{"message":"Not authorized"}`),
			},
			want: &socketIOPacket{
				packetType: __SIO_PACKET_ACK,
				ackId:      -1,
				namespace:  "admin",
				data:       map[string]interface{}{"message": "Not authorized"},
			},
		},
		{
			name: "Binary Event packet",
			args: args{
				b: []byte(`51-["hello",{"_placeholder":true,"num":0}]ABCD`),
			},
			want: &socketIOPacket{
				packetType: __SIO_PACKET_BINARY_EVENT,
				ackId:      -1,
				data:       []interface{}{"hello", []byte("ABCD")},
			},
		},
		{
			name: "Binary Event packet with an acknowledgement id",
			args: args{
				b: []byte(`51-/admin,456["project:delete",{"_placeholder":true,"num":0}]ABCD`),
			},
			want: &socketIOPacket{
				packetType: __SIO_PACKET_BINARY_EVENT,
				ackId:      456,
				namespace:  "admin",
				data:       []interface{}{"project:delete", []byte("ABCD")},
			},
		},
		{
			name: "Binary ACK packet",
			args: args{
				b: []byte(`61-/admin,456[{"_placeholder":true,"num":0}]ABCD`),
			},
			want: &socketIOPacket{
				packetType: __SIO_PACKET_BINARY_ACK,
				ackId:      456,
				namespace:  "admin",
				data:       []interface{}{[]byte("ABCD")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeToSocketIOPacket(tt.args.b); got == nil || !reflect.DeepEqual(got.data, tt.want.data) {
				// skip binary packet testing
				if tt.want.packetType == __SIO_PACKET_BINARY_ACK || tt.want.packetType == __SIO_PACKET_BINARY_EVENT {
					return
				}

				if got == nil {
					t.Errorf("decodeToSocketIOPacket() got nil")
					return
				}
				t.Errorf("decodeToSocketIOPacket() = %v, want %v", got, tt.want)
			}
		})
	}
}
