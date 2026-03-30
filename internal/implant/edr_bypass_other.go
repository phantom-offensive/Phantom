//go:build !windows

package implant

// Stubs for non-Windows platforms

func UnhookAllDLLs() []string       { return []string{"[-] DLL unhooking is Windows-only"} }
func RemovePEHeaders() error        { return nil }
func EncryptHeap(key []byte)        {}
func PatchAllETW() []string         { return []string{"[-] ETW patching is Windows-only"} }
func SpawnWithBlockDLLs(cmd string) (uint32, error) { return 0, nil }
func SetHardwareBreakpoint(addr uintptr, idx int) error { return nil }
func ModuleStomp(dll string) error  { return nil }
func InitAdvancedEvasion() []string { return InitEvasion() }
