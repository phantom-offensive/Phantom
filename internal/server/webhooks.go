package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookConfig holds webhook notification settings.
type WebhookConfig struct {
	SlackURL   string `yaml:"slack_url"`
	DiscordURL string `yaml:"discord_url"`
	Enabled    bool   `yaml:"enabled"`
}

// WebhookNotifier sends notifications to Slack/Discord when events occur.
type WebhookNotifier struct {
	slackURL   string
	discordURL string
	client     *http.Client
}

// NewWebhookNotifier creates a new webhook notifier.
func NewWebhookNotifier(slackURL, discordURL string) *WebhookNotifier {
	return &WebhookNotifier{
		slackURL:   slackURL,
		discordURL: discordURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// NotifyAgentRegistered sends a notification when a new agent checks in.
func (w *WebhookNotifier) NotifyAgentRegistered(name, os, hostname, username, ip string) {
	msg := fmt.Sprintf("🔴 **New Agent** `%s` registered\n"+
		"```\n"+
		"OS:       %s\n"+
		"Host:     %s\n"+
		"User:     %s\n"+
		"IP:       %s\n"+
		"Time:     %s\n"+
		"```", name, os, hostname, username, ip, time.Now().Format("15:04:05 MST"))

	w.send(msg)
}

// NotifyAgentDead sends a notification when an agent goes dead.
func (w *WebhookNotifier) NotifyAgentDead(name, hostname string) {
	msg := fmt.Sprintf("⚫ **Agent Dead** `%s` (%s) — no longer checking in", name, hostname)
	w.send(msg)
}

// NotifyTaskComplete sends a notification for high-value task completions.
func (w *WebhookNotifier) NotifyTaskComplete(agentName, taskType, summary string) {
	msg := fmt.Sprintf("✅ **Task Complete** on `%s`\n"+
		"Type: `%s`\n"+
		"Result: ```%s```", agentName, taskType, truncate(summary, 500))
	w.send(msg)
}

// NotifyListenerEvent sends a notification for listener events.
func (w *WebhookNotifier) NotifyListenerEvent(action, name, bindAddr string) {
	emoji := "🟢"
	if action == "stop" {
		emoji = "🔴"
	}
	msg := fmt.Sprintf("%s **Listener %s** `%s` on `%s`", emoji, action, name, bindAddr)
	w.send(msg)
}

func (w *WebhookNotifier) send(message string) {
	if w.slackURL != "" {
		go w.sendSlack(message)
	}
	if w.discordURL != "" {
		go w.sendDiscord(message)
	}
}

func (w *WebhookNotifier) sendSlack(message string) {
	payload := map[string]string{
		"text": message,
	}
	body, _ := json.Marshal(payload)
	w.client.Post(w.slackURL, "application/json", bytes.NewReader(body))
}

func (w *WebhookNotifier) sendDiscord(message string) {
	payload := map[string]string{
		"content": message,
	}
	body, _ := json.Marshal(payload)
	w.client.Post(w.discordURL, "application/json", bytes.NewReader(body))
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
