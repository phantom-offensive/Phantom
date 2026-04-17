package webui

import (
	"context"
	"encoding/json"
	"net/http"
)

// handleExChannelList returns all registered External C2 channels and their status.
func (w *WebUI) handleExChannelList(rw http.ResponseWriter, r *http.Request) {
	names := w.server.ExChannels.List()

	type channelInfo struct {
		Name    string `json:"name"`
		Running bool   `json:"running"`
		Status  string `json:"status"`
	}

	result := make([]channelInfo, 0, len(names))
	for _, name := range names {
		ch, _ := w.server.ExChannels.Get(name)
		status := "stopped"
		if ch.IsRunning() {
			status = "running"
		}
		result = append(result, channelInfo{
			Name:    name,
			Running: ch.IsRunning(),
			Status:  status,
		})
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]interface{}{
		"channels": result,
		"count":    len(result),
	})
}

// handleExChannelStart starts a named ExC2 channel.
func (w *WebUI) handleExChannelStart(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(rw, map[string]string{"error": "POST required"})
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.Name == "" {
		writeJSON(rw, map[string]string{"error": "channel name required"})
		return
	}

	ch, ok := w.server.ExChannels.Get(req.Name)
	if !ok {
		writeJSON(rw, map[string]string{"error": "channel '" + req.Name + "' not found"})
		return
	}

	if ch.IsRunning() {
		writeJSON(rw, map[string]interface{}{"success": true, "message": "channel already running"})
		return
	}

	if err := ch.Start(context.Background()); err != nil {
		writeJSON(rw, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(rw, map[string]interface{}{
		"success": true,
		"message": "ExC2 channel '" + req.Name + "' started",
	})
}

// handleExChannelStop stops a named ExC2 channel.
func (w *WebUI) handleExChannelStop(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(rw, map[string]string{"error": "POST required"})
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.Name == "" {
		writeJSON(rw, map[string]string{"error": "channel name required"})
		return
	}

	ch, ok := w.server.ExChannels.Get(req.Name)
	if !ok {
		writeJSON(rw, map[string]string{"error": "channel '" + req.Name + "' not found"})
		return
	}

	if !ch.IsRunning() {
		writeJSON(rw, map[string]interface{}{"success": true, "message": "channel already stopped"})
		return
	}

	if err := ch.Stop(); err != nil {
		writeJSON(rw, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(rw, map[string]interface{}{
		"success": true,
		"message": "ExC2 channel '" + req.Name + "' stopped",
	})
}
