package webui

import (
	"encoding/json"
	"net/http"
)

type webhookRequest struct {
	SlackURL   string `json:"slack_url"`
	DiscordURL string `json:"discord_url"`
}

// handleWebhook returns the current webhook config (GET) or updates it (POST).
func (w *WebUI) handleWebhook(rw http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		slack, discord := "", ""
		if w.server.Webhook != nil {
			slack = w.server.Webhook.SlackURL()
			discord = w.server.Webhook.DiscordURL()
		}
		writeJSON(rw, map[string]interface{}{
			"slack_url":   slack,
			"discord_url": discord,
		})
		return
	}

	if r.Method != http.MethodPost {
		writeJSON(rw, map[string]interface{}{"success": false, "message": "GET or POST required"})
		return
	}

	var req webhookRequest
	json.NewDecoder(r.Body).Decode(&req)

	if err := w.server.SetWebhook(req.SlackURL, req.DiscordURL); err != nil {
		writeJSON(rw, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	if req.SlackURL == "" && req.DiscordURL == "" {
		writeJSON(rw, map[string]interface{}{"success": true, "message": "Webhook cleared"})
		return
	}
	writeJSON(rw, map[string]interface{}{"success": true, "message": "Webhook configured"})
}

// handleWebhookTest sends a test notification.
func (w *WebUI) handleWebhookTest(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(rw, map[string]interface{}{"success": false, "message": "POST required"})
		return
	}
	if w.server.Webhook == nil {
		writeJSON(rw, map[string]interface{}{"success": false, "message": "No webhook configured"})
		return
	}
	w.server.Webhook.NotifyAgentRegistered("test-agent", "windows", "TEST-PC", "admin", "10.0.0.1")
	writeJSON(rw, map[string]interface{}{"success": true, "message": "Test notification sent"})
}
