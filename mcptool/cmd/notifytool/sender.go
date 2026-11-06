package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	wecomWebhookBase    = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send"
	dingtalkWebhookBase = "https://oapi.dingtalk.com/robot/send"
	maxResponseBytes    = 1 << 20
)

type webhookResult struct {
	Platform string `json:"platform"`
	Errcode  int    `json:"errcode"`
	Errmsg   string `json:"errmsg"`
}

type sender struct {
	client      *http.Client
	wecomKey    string
	dingtalkKey string
}

func newSender(timeout time.Duration, wecomKey, dingtalkKey string) *sender {
	return &sender{
		client:      &http.Client{Timeout: timeout},
		wecomKey:    strings.TrimSpace(wecomKey),
		dingtalkKey: strings.TrimSpace(dingtalkKey),
	}
}

type wecomText struct {
	Content             string   `json:"content"`
	MentionedList       []string `json:"mentioned_list,omitempty"`
	MentionedMobileList []string `json:"mentioned_mobile_list,omitempty"`
}

type wecomPayload struct {
	MsgType string    `json:"msgtype"`
	Text    wecomText `json:"text"`
}

func (s *sender) sendWeCom(ctx context.Context, content string, mentionedList, mentionedMobiles []string) (webhookResult, error) {
	payload := wecomPayload{
		MsgType: "text",
		Text: wecomText{
			Content:             content,
			MentionedList:       mentionedList,
			MentionedMobileList: mentionedMobiles,
		},
	}
	return s.post(ctx, webhookURL(s.wecomKey, wecomWebhookBase, "key"), payload, "wecom", s.wecomKey)
}

type dingtalkAt struct {
	AtMobiles []string `json:"atMobiles,omitempty"`
	AtUserIDs []string `json:"atUserIds,omitempty"`
	IsAtAll   bool     `json:"isAtAll,omitempty"`
}

type dingtalkText struct {
	Content string `json:"content"`
}

type dingtalkPayload struct {
	MsgType string       `json:"msgtype"`
	Text    dingtalkText `json:"text"`
	At      dingtalkAt   `json:"at"`
}

func (s *sender) sendDingTalk(ctx context.Context, content string, atMobiles, atUserIDs []string, isAtAll bool) (webhookResult, error) {
	payload := dingtalkPayload{
		MsgType: "text",
		Text: dingtalkText{
			Content: appendDingTalkMentions(content, atMobiles, atUserIDs),
		},
		At: dingtalkAt{
			AtMobiles: atMobiles,
			AtUserIDs: atUserIDs,
			IsAtAll:   isAtAll,
		},
	}
	return s.post(ctx, webhookURL(s.dingtalkKey, dingtalkWebhookBase, "access_token"), payload, "dingtalk", s.dingtalkKey)
}

func (s *sender) post(ctx context.Context, endpoint string, payload any, platform, secret string) (webhookResult, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return webhookResult{}, fmt.Errorf("encode payload failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return webhookResult{}, fmt.Errorf("create request failed: %s", redact(err.Error(), secret))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return webhookResult{}, fmt.Errorf("request failed: %s", redact(err.Error(), secret))
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return webhookResult{}, fmt.Errorf("read response failed: %s", redact(err.Error(), secret))
	}

	var result webhookResult
	jsonErr := json.Unmarshal(data, &result)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return result, fmt.Errorf("webhook returned HTTP %d: %s", resp.StatusCode, redact(responseSnippet(data), secret))
	}
	if jsonErr != nil {
		return result, fmt.Errorf("decode response failed: %s", redact(responseSnippet(data), secret))
	}

	result.Platform = platform
	if result.Errcode != 0 {
		return result, fmt.Errorf("%s webhook error %d: %s", platform, result.Errcode, redact(result.Errmsg, secret))
	}
	return result, nil
}

func webhookURL(key, base, param string) string {
	key = strings.TrimSpace(key)
	if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
		return key
	}
	return base + "?" + param + "=" + url.QueryEscape(key)
}

func appendDingTalkMentions(content string, atMobiles, atUserIDs []string) string {
	tokens := make([]string, 0, len(atMobiles)+len(atUserIDs))
	for _, mobile := range atMobiles {
		if mobile = strings.TrimSpace(mobile); mobile != "" {
			tokens = append(tokens, "@"+mobile)
		}
	}
	for _, userID := range atUserIDs {
		if userID = strings.TrimSpace(userID); userID != "" {
			tokens = append(tokens, "@"+userID)
		}
	}

	missing := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if !strings.Contains(content, token) {
			missing = append(missing, token)
		}
	}
	if len(missing) == 0 {
		return content
	}
	if content != "" && !strings.HasSuffix(content, " ") && !strings.HasSuffix(content, "\n") {
		content += " "
	}
	return content + strings.Join(missing, " ")
}

func redact(text, secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return text
	}
	return strings.ReplaceAll(text, secret, "***")
}

func responseSnippet(data []byte) string {
	const limit = 300
	text := strings.TrimSpace(string(data))
	if len(text) > limit {
		return text[:limit] + "..."
	}
	return text
}
