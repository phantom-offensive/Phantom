package protocol

import (
	"bytes"
	"testing"
	"time"

	"github.com/phantom-c2/phantom/internal/crypto"
)

func TestMarshalUnmarshal(t *testing.T) {
	req := RegisterRequest{
		Hostname:    "DESKTOP-TEST",
		Username:    "admin",
		OS:          "windows",
		Arch:        "amd64",
		PID:         1234,
		ProcessName: "svchost.exe",
		InternalIP:  "10.0.1.42",
	}

	data, err := Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got RegisterRequest
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Hostname != req.Hostname || got.Username != req.Username || got.PID != req.PID {
		t.Fatalf("round trip mismatch: got %+v, want %+v", got, req)
	}
}

func TestEnvelopeSealOpen(t *testing.T) {
	key, err := crypto.GenerateAESKey()
	if err != nil {
		t.Fatalf("GenerateAESKey: %v", err)
	}

	req := CheckInRequest{
		AgentID: "test-agent-id",
		Results: []TaskResult{
			{TaskID: "task-1", AgentID: "test-agent-id", Output: []byte("whoami output")},
		},
	}

	payload, err := Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	env, err := SealEnvelope(MsgCheckIn, key, payload)
	if err != nil {
		t.Fatalf("SealEnvelope: %v", err)
	}

	if env.Version != ProtocolVersion {
		t.Fatalf("wrong version: got %d, want %d", env.Version, ProtocolVersion)
	}
	if env.Type != MsgCheckIn {
		t.Fatalf("wrong type: got %d, want %d", env.Type, MsgCheckIn)
	}

	decrypted, err := OpenEnvelope(env, key)
	if err != nil {
		t.Fatalf("OpenEnvelope: %v", err)
	}

	var got CheckInRequest
	if err := Unmarshal(decrypted, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.AgentID != req.AgentID {
		t.Fatalf("AgentID mismatch: got %q, want %q", got.AgentID, req.AgentID)
	}
	if len(got.Results) != 1 || got.Results[0].TaskID != "task-1" {
		t.Fatal("Results mismatch after round trip")
	}
}

func TestEnvelopeBytes(t *testing.T) {
	key, _ := crypto.GenerateAESKey()
	env, _ := SealEnvelope(MsgCheckIn, key, []byte("test payload"))

	raw := EnvelopeToBytes(env)
	restored, err := EnvelopeFromBytes(raw)
	if err != nil {
		t.Fatalf("EnvelopeFromBytes: %v", err)
	}

	if restored.Version != env.Version || restored.Type != env.Type {
		t.Fatal("envelope header mismatch")
	}
	if restored.KeyID != env.KeyID {
		t.Fatal("KeyID mismatch")
	}
	if !bytes.Equal(restored.Payload, env.Payload) {
		t.Fatal("payload mismatch")
	}
}

func TestHTTPWrapUnwrap(t *testing.T) {
	key, _ := crypto.GenerateAESKey()
	env, _ := SealEnvelope(MsgRegisterResponse, key, []byte("response data"))

	ts := time.Now().Unix()
	httpBody, err := WrapForHTTP(env, ts)
	if err != nil {
		t.Fatalf("WrapForHTTP: %v", err)
	}

	// Verify it looks like JSON
	if httpBody[0] != '{' {
		t.Fatal("HTTP body should be JSON")
	}

	restored, err := UnwrapFromHTTP(httpBody)
	if err != nil {
		t.Fatalf("UnwrapFromHTTP: %v", err)
	}

	if restored.Type != MsgRegisterResponse {
		t.Fatalf("type mismatch: got %d, want %d", restored.Type, MsgRegisterResponse)
	}

	// Decrypt and verify
	pt, err := OpenEnvelope(restored, key)
	if err != nil {
		t.Fatalf("OpenEnvelope: %v", err)
	}
	if !bytes.Equal(pt, []byte("response data")) {
		t.Fatal("decrypted payload mismatch")
	}
}

func TestTaskTypeName(t *testing.T) {
	if TaskTypeName(TaskShell) != "shell" {
		t.Fatal("TaskShell name wrong")
	}
	if TaskTypeName(TaskScreenshot) != "screenshot" {
		t.Fatal("TaskScreenshot name wrong")
	}
	if TaskTypeName(255) != "unknown" {
		t.Fatal("unknown type should return 'unknown'")
	}
}
