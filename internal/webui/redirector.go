package webui

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/phantom-c2/phantom/internal/listener"
)

type redirectorRequest struct {
	Domain      string `json:"domain"`
	C2Host      string `json:"c2_host"`
	C2Port      string `json:"c2_port"`
	RedirPort   string `json:"redir_port"`
	Profile     string `json:"profile"`
	LetsEncrypt bool   `json:"lets_encrypt"`
}

// handleRedirectorGenerate generates redirector configs (nginx/apache/caddy/etc.)
func (w *WebUI) handleRedirectorGenerate(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(rw, map[string]interface{}{"success": false, "message": "POST required"})
		return
	}

	var req redirectorRequest
	json.NewDecoder(r.Body).Decode(&req)

	if req.Domain == "" || req.C2Host == "" {
		writeJSON(rw, map[string]interface{}{"success": false, "message": "domain and c2_host are required"})
		return
	}
	if req.C2Port == "" {
		req.C2Port = "8080"
	}
	if req.RedirPort == "" {
		req.RedirPort = "443"
	}

	cfg := listener.RedirectorConfig{
		C2Host:      req.C2Host,
		C2Port:      req.C2Port,
		RedirDomain: req.Domain,
		RedirPort:   req.RedirPort,
		ProfileName: req.Profile,
		LetsEncrypt: req.LetsEncrypt,
	}

	output, err := listener.GenerateRedirectorConfigs(cfg, "build/redirector")
	if err != nil {
		writeJSON(rw, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	writeJSON(rw, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Redirector configs generated for %s → %s:%s", req.Domain, req.C2Host, req.C2Port),
		"output":  output,
		"dir":     "build/redirector",
	})
}
