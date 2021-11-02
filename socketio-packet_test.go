package siosver

import (
	"fmt"
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
			name: "Test Connect packet",
			args: args{
				b: []byte(`0{"token":"123"}`),
			},
			want: newSocketIOPacket(__SIO_PACKET_CONNECT, map[string]interface{}{"token": "123"}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeToSocketIOPacket(tt.args.b); !reflect.DeepEqual((*got).data, (*tt.want).data) {
				fmt.Println((*got).data);
				t.Errorf("decodeToSocketIOPacket() = %v, want %v", *got, *tt.want)
			}
		})
	}
}