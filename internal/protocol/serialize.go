package protocol

import (
	"github.com/vmihailenco/msgpack/v5"
)

// Marshal serializes a message struct to msgpack bytes.
func Marshal(v interface{}) ([]byte, error) {
	return msgpack.Marshal(v)
}

// Unmarshal deserializes msgpack bytes into a message struct.
func Unmarshal(data []byte, v interface{}) error {
	return msgpack.Unmarshal(data, v)
}
