package siosver

import (
	"bytes"
	"reflect"
)

var typeOfBuffer = reflect.ValueOf(bytes.Buffer{}).Type()

var socketIOBufferIndex = map[string]interface{}{
	"_placeholder": true,
	"num":          0,
}
