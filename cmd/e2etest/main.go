package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/phantom-c2/phantom/internal/protocol"
	"github.com/phantom-c2/phantom/internal/server"
)

func main() {
	sep := strings.Repeat("═", 65)

	fmt.Println()
	fmt.Println(sep)
	fmt.Println("  PHANTOM C2 — END-TO-END TEST")
	fmt.Println(sep)
	fmt.Println()

	// ── Step 1: Start server ──
	fmt.Println("  [1/6] Loading server config...")
	cfg, err := server.LoadConfig("configs/server.yaml")
	if err != nil {
		fmt.Printf("  [-] Config error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("  [1/6] Initializing server...")
	srv, err := server.New(cfg)
	if err != nil {
		fmt.Printf("  [-] Server error: %v\n", err)
		os.Exit(1)
	}
	defer srv.Shutdown()

	srv.SetupListeners()
	srv.StartListener("fallback-http")
	time.Sleep(1 * time.Second)
	fmt.Println("  [+] Server started — listener on 0.0.0.0:8080")

	// ── Step 2: Launch agent ──
	fmt.Println()
	fmt.Println("  [2/6] Launching agent...")
	agent := exec.Command("./build/agents/phantom-agent_linux_amd64")
	agent.Start()
	defer agent.Process.Kill()
	fmt.Printf("  [+] Agent process started (PID: %d)\n", agent.Process.Pid)

	// ── Step 3: Wait for registration ──
	fmt.Println()
	fmt.Println("  [3/6] Waiting for agent registration...")
	var agentID, agentName string
	for i := 0; i < 20; i++ {
		time.Sleep(1 * time.Second)
		agents, _ := srv.AgentMgr.List()
		if len(agents) > 0 {
			a := agents[0]
			agentID = a.ID
			agentName = a.Name
			fmt.Printf("  [+] Agent registered!\n")
			fmt.Printf("      Name:     %s\n", a.Name)
			fmt.Printf("      ID:       %s\n", a.ID[:8])
			fmt.Printf("      OS:       %s/%s\n", a.OS, a.Arch)
			fmt.Printf("      Host:     %s\n", a.Hostname)
			fmt.Printf("      User:     %s\n", a.Username)
			fmt.Printf("      IP:       %s\n", a.ExternalIP)
			break
		}
		fmt.Printf("      waiting... (%ds)\n", i+1)
	}

	if agentID == "" {
		fmt.Println("  [-] FAIL: No agent registered")
		os.Exit(1)
	}

	// ── Step 4: Queue tasks ──
	fmt.Println()
	fmt.Println("  [4/6] Queueing tasks for", agentName, "...")

	type taskInfo struct {
		id   string
		name string
	}

	tasks := []struct {
		name     string
		taskType uint8
		args     []string
	}{
		{"whoami", protocol.TaskShell, []string{"whoami"}},
		{"id", protocol.TaskShell, []string{"id"}},
		{"hostname", protocol.TaskShell, []string{"hostname"}},
		{"pwd", protocol.TaskShell, []string{"pwd"}},
		{"uname -a", protocol.TaskShell, []string{"uname -a"}},
		{"cat /etc/os-release | head -3", protocol.TaskShell, []string{"cat /etc/os-release | head -3"}},
		{"ps aux | head -10", protocol.TaskShell, []string{"ps aux | head -10"}},
		{"sysinfo", protocol.TaskSysinfo, nil},
	}

	var queued []taskInfo
	for _, t := range tasks {
		record, err := srv.TaskDisp.CreateTask(agentID, t.taskType, t.args, nil)
		if err != nil {
			fmt.Printf("      [-] Failed to create task '%s': %v\n", t.name, err)
			continue
		}
		queued = append(queued, taskInfo{id: record.ID, name: t.name})
		fmt.Printf("      [+] Queued: %-35s (ID: %s)\n", t.name, record.ID[:8])
	}

	// ── Step 5: Wait for results ──
	fmt.Println()
	fmt.Printf("  [5/6] Waiting for agent to execute and return results...\n")
	fmt.Printf("      Agent sleep: 5s — need 2 check-in cycles (pick up + return)\n")

	for i := 0; i < 45; i++ {
		time.Sleep(1 * time.Second)
		complete := 0
		for _, t := range queued {
			r, _ := srv.TaskDisp.GetResult(t.id)
			if r != nil && (len(r.Output) > 0 || r.Error != "") {
				complete++
			}
		}
		if complete >= len(queued) {
			fmt.Printf("      [+] All %d results received! (%ds)\n", complete, i+1)
			break
		}
		if (i+1)%5 == 0 {
			fmt.Printf("      ... %d/%d results with output (%ds)\n", complete, len(queued), i+1)
		}
	}

	// ── Step 6: Display results ──
	fmt.Println()
	fmt.Println(sep)
	fmt.Println("  INTERACTION RESULTS")
	fmt.Println(sep)

	passed := 0
	failed := 0

	for _, t := range queued {
		r, _ := srv.TaskDisp.GetResult(t.id)
		fmt.Println()

		if r == nil {
			fmt.Printf("  [TIMEOUT] %s\n", t.name)
			fmt.Printf("  phantom [%s] > %s\n", agentName, t.name)
			fmt.Printf("  (no response received)\n")
			failed++
			continue
		}

		output := ""
		if r.Output != nil {
			output = string(r.Output)
		}

		if r.Error != "" {
			fmt.Printf("  [ERROR] %s\n", t.name)
			fmt.Printf("  phantom [%s] > %s\n", agentName, t.name)
			fmt.Printf("  Error: %s\n", r.Error)
			failed++
		} else if len(output) == 0 {
			fmt.Printf("  [WAIT] %s\n", t.name)
			fmt.Printf("  phantom [%s] > %s\n", agentName, t.name)
			fmt.Printf("  (result received but output empty — agent may need another cycle)\n")
			failed++
		} else {
			fmt.Printf("  [PASS] %s\n", t.name)
			fmt.Printf("  phantom [%s] > %s\n", agentName, t.name)
			// Indent output lines
			lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
			for _, line := range lines {
				fmt.Printf("      %s\n", line)
			}
			passed++
		}
	}

	// ── Summary ──
	fmt.Println()
	fmt.Println(sep)
	fmt.Printf("  SUMMARY: %d passed, %d failed out of %d tasks\n", passed, failed, len(queued))

	// Show agent status
	a, _ := srv.AgentMgr.Get(agentID)
	if a != nil {
		fmt.Printf("  Agent '%s' status: %s (last seen: %s)\n", a.Name, a.Status, a.LastSeen.Format("15:04:05"))
	}

	if failed == 0 {
		fmt.Println("  ALL TESTS PASSED")
	} else {
		fmt.Println("  SOME TESTS FAILED")
	}
	fmt.Println(sep)
	fmt.Println()
}
