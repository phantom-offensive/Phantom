package implant

import (
	"encoding/binary"
	"fmt"
)

// BOF (Beacon Object File) execution engine.
// All execution happens in-memory — nothing touches disk.
//
// Windows: In-memory COFF loader — parses sections, resolves imports via
//          GetProcAddress, allocates RWX memory, and calls the entry point.
// Linux:   memfd_create + fexecve — creates an anonymous file descriptor
//          in memory and executes from it. No file is written to the filesystem.

// ExecuteBOF executes a Beacon Object File entirely in memory.
// bofData: raw COFF .o bytes (Windows) or ELF shared object bytes (Linux)
// args: packed BOF arguments (Cobalt Strike format)
// Returns captured output and any error.
func ExecuteBOF(bofData []byte, args []byte) ([]byte, error) {
	if len(bofData) == 0 {
		return nil, fmt.Errorf("empty BOF data")
	}

	return executeBOFPlatform(bofData, args)
}

// PackBOFArgs packs arguments in Cobalt Strike BOF argument format.
// Format: [size:4][type:4][length:4][data:N] per argument
// Compatible with CS BeaconDataParse API.
func PackBOFArgs(args ...BOFArg) []byte {
	if len(args) == 0 {
		return nil
	}

	var inner []byte
	for _, arg := range args {
		typeBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(typeBuf, uint32(arg.Type))
		inner = append(inner, typeBuf...)

		lenBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(lenBuf, uint32(len(arg.Data)))
		inner = append(inner, lenBuf...)

		inner = append(inner, arg.Data...)
	}

	// Prepend total size
	sizeBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(sizeBuf, uint32(len(inner)))
	return append(sizeBuf, inner...)
}

// BOFArg represents a single BOF argument.
type BOFArg struct {
	Type uint32
	Data []byte
}

// BOF argument types (Cobalt Strike compatible).
const (
	BOFArgShort   uint32 = 1
	BOFArgInt     uint32 = 2
	BOFArgString  uint32 = 3
	BOFArgWString uint32 = 4
	BOFArgBinary  uint32 = 5
)

// NewBOFStringArg creates a null-terminated string BOF argument.
func NewBOFStringArg(s string) BOFArg {
	return BOFArg{Type: BOFArgString, Data: append([]byte(s), 0)}
}

// NewBOFIntArg creates an integer BOF argument.
func NewBOFIntArg(i int) BOFArg {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(i))
	return BOFArg{Type: BOFArgInt, Data: buf}
}

// NewBOFWStringArg creates a null-terminated wide string BOF argument.
func NewBOFWStringArg(s string) BOFArg {
	// Convert to UTF-16LE
	wdata := make([]byte, 0, (len(s)+1)*2)
	for _, r := range s {
		wdata = append(wdata, byte(r), byte(r>>8))
	}
	wdata = append(wdata, 0, 0) // null terminator
	return BOFArg{Type: BOFArgWString, Data: wdata}
}

// NewBOFBinaryArg creates a binary data BOF argument.
func NewBOFBinaryArg(data []byte) BOFArg {
	return BOFArg{Type: BOFArgBinary, Data: data}
}
