package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/phantom-c2/phantom/internal/db"
	"github.com/phantom-c2/phantom/internal/protocol"
	"github.com/phantom-c2/phantom/internal/server"
	"github.com/phantom-c2/phantom/internal/util"
)

// ReportGenerator creates engagement reports from session data.
type ReportGenerator struct {
	server *server.Server
}

// NewReportGenerator creates a new report generator.
func NewReportGenerator(srv *server.Server) *ReportGenerator {
	return &ReportGenerator{server: srv}
}

// GenerateReport creates a full engagement report in Markdown format.
func (rg *ReportGenerator) GenerateReport(outputPath string) error {
	var sb strings.Builder

	// Header
	sb.WriteString("# Phantom C2 — Engagement Report\n\n")
	sb.WriteString(fmt.Sprintf("**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 MST")))
	sb.WriteString("---\n\n")

	// Executive Summary
	agents, _ := rg.server.AgentMgr.List()
	listeners := rg.server.ListenerMgr.List()

	sb.WriteString("## Executive Summary\n\n")
	sb.WriteString(fmt.Sprintf("| Metric | Value |\n"))
	sb.WriteString(fmt.Sprintf("|--------|-------|\n"))
	sb.WriteString(fmt.Sprintf("| Total Agents | %d |\n", len(agents)))
	sb.WriteString(fmt.Sprintf("| Active Agents | %d |\n", countAgentsByStatus(agents, "active")))
	sb.WriteString(fmt.Sprintf("| Active Listeners | %d |\n", len(listeners)))
	sb.WriteString(fmt.Sprintf("| Report Generated | %s |\n\n", time.Now().Format("2006-01-02 15:04")))

	// Agents
	sb.WriteString("## Compromised Hosts\n\n")
	if len(agents) > 0 {
		sb.WriteString("| Name | OS | Hostname | User | IP | First Seen | Status |\n")
		sb.WriteString("|------|----|---------|----- |----|-----------|--------|\n")
		for _, a := range agents {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s |\n",
				a.Name, a.OS, a.Hostname, a.Username, a.ExternalIP,
				a.FirstSeen.Format("2006-01-02 15:04"), a.Status))
		}
	} else {
		sb.WriteString("No agents registered during this engagement.\n")
	}
	sb.WriteString("\n")

	// Per-agent activity
	sb.WriteString("## Activity Timeline\n\n")
	for _, a := range agents {
		sb.WriteString(fmt.Sprintf("### Agent: %s (%s@%s)\n\n", a.Name, a.Username, a.Hostname))
		sb.WriteString(fmt.Sprintf("- **OS:** %s/%s\n", a.OS, a.Arch))
		sb.WriteString(fmt.Sprintf("- **PID:** %d (%s)\n", a.PID, a.ProcessName))
		sb.WriteString(fmt.Sprintf("- **Internal IP:** %s\n", a.InternalIP))
		sb.WriteString(fmt.Sprintf("- **External IP:** %s\n", a.ExternalIP))
		sb.WriteString(fmt.Sprintf("- **First Seen:** %s\n", a.FirstSeen.Format("2006-01-02 15:04:05")))
		sb.WriteString(fmt.Sprintf("- **Last Seen:** %s\n", a.LastSeen.Format("2006-01-02 15:04:05")))
		sb.WriteString(fmt.Sprintf("- **Sleep:** %ds / %d%% jitter\n\n", a.Sleep, a.Jitter))

		// Task history
		tasks, _ := rg.server.TaskDisp.GetTaskHistory(a.ID)
		if len(tasks) > 0 {
			sb.WriteString("#### Commands Executed\n\n")
			sb.WriteString("| Time | Type | Command | Status |\n")
			sb.WriteString("|------|------|---------|--------|\n")
			for _, t := range tasks {
				argsStr := strings.Join(t.Args, " ")
				if len(argsStr) > 50 {
					argsStr = argsStr[:47] + "..."
				}
				status := protocol.StatusName(uint8(t.Status))
				sb.WriteString(fmt.Sprintf("| %s | %s | `%s` | %s |\n",
					t.CreatedAt.Format("15:04:05"),
					protocol.TaskTypeName(uint8(t.Type)),
					argsStr, status))
			}
			sb.WriteString("\n")

			// Include command outputs
			sb.WriteString("#### Command Output\n\n")
			for _, t := range tasks {
				result, _ := rg.server.TaskDisp.GetResult(t.ID)
				if result != nil && len(result.Output) > 0 {
					argsStr := strings.Join(t.Args, " ")
					sb.WriteString(fmt.Sprintf("**%s %s** (%s)\n",
						protocol.TaskTypeName(uint8(t.Type)), argsStr,
						t.CreatedAt.Format("15:04:05")))
					sb.WriteString("```\n")
					output := string(result.Output)
					if len(output) > 1000 {
						output = output[:1000] + "\n... (truncated)"
					}
					sb.WriteString(output)
					sb.WriteString("\n```\n\n")
				}
				if result != nil && result.Error != "" {
					sb.WriteString(fmt.Sprintf("**Error:** `%s`\n\n", result.Error))
				}
			}
		} else {
			sb.WriteString("No commands executed on this agent.\n\n")
		}
	}

	// Listeners
	sb.WriteString("## Infrastructure\n\n")
	sb.WriteString("### Listeners\n\n")
	sb.WriteString("| Name | Type | Bind Address | Status |\n")
	sb.WriteString("|------|------|-------------|--------|\n")
	for _, l := range listeners {
		status := "Stopped"
		if l.IsRunning() {
			status = "Running"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			l.GetName(), strings.ToUpper(l.GetType()), l.GetBindAddr(), status))
	}
	sb.WriteString("\n")

	// Loot
	loot, _ := rg.server.DB.ListLoot("")
	if len(loot) > 0 {
		sb.WriteString("### Captured Loot\n\n")
		sb.WriteString("| Type | Name | Agent | Time |\n")
		sb.WriteString("|------|------|-------|------|\n")
		for _, l := range loot {
			agentName := util.ShortID(l.AgentID)
			a, _ := rg.server.AgentMgr.Get(l.AgentID)
			if a != nil {
				agentName = a.Name
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				l.Type, l.Name, agentName, l.CreatedAt.Format("15:04:05")))
		}
		sb.WriteString("\n")
	}

	// Event log
	if len(rg.server.EventLog) > 0 {
		sb.WriteString("## Event Log\n\n")
		sb.WriteString("```\n")
		for _, e := range rg.server.EventLog {
			sb.WriteString(e + "\n")
		}
		sb.WriteString("```\n\n")
	}

	// Footer
	sb.WriteString("---\n\n")
	sb.WriteString("*Generated by Phantom C2 Framework*\n")

	// Write to file
	os.MkdirAll("reports", 0755)
	if outputPath == "" {
		outputPath = fmt.Sprintf("reports/engagement_%s.md", time.Now().Format("2006-01-02_150405"))
	}

	if err := os.WriteFile(outputPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	return nil
}

// GenerateCSV exports task data as CSV.
func (rg *ReportGenerator) GenerateCSV(outputPath string) error {
	agents, _ := rg.server.AgentMgr.List()

	var sb strings.Builder
	sb.WriteString("Timestamp,Agent,Hostname,User,OS,TaskType,Command,Status,Output\n")

	for _, a := range agents {
		tasks, _ := rg.server.TaskDisp.GetTaskHistory(a.ID)
		for _, t := range tasks {
			result, _ := rg.server.TaskDisp.GetResult(t.ID)
			output := ""
			if result != nil && len(result.Output) > 0 {
				output = strings.ReplaceAll(string(result.Output), "\"", "\"\"")
				if len(output) > 500 {
					output = output[:500]
				}
			}
			argsStr := strings.Join(t.Args, " ")

			sb.WriteString(fmt.Sprintf("\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\"\n",
				t.CreatedAt.Format("2006-01-02 15:04:05"),
				a.Name, a.Hostname, a.Username, a.OS,
				protocol.TaskTypeName(uint8(t.Type)),
				argsStr,
				protocol.StatusName(uint8(t.Status)),
				output))
		}
	}

	os.MkdirAll("reports", 0755)
	if outputPath == "" {
		outputPath = fmt.Sprintf("reports/engagement_%s.csv", time.Now().Format("2006-01-02_150405"))
	}

	return os.WriteFile(outputPath, []byte(sb.String()), 0644)
}

func countAgentsByStatus(agents []*db.Agent, status string) int {
	count := 0
	for _, a := range agents {
		if a.Status == status {
			count++
		}
	}
	return count
}
