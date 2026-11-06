package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
	client         *http.Client
	wecomKey       string
	dingtalkKey    string
	dingtalkSecret string
	now            func() time.Time
}

func newSender(timeout time.Duration, wecomKey, dingtalkKey, dingtalkSecret string) *sender {
	dingtalkKey, dingtalkSecret = splitDingTalkTarget(dingtalkKey, dingtalkSecret)
	return &sender{
		client:         &http.Client{Timeout: timeout},
		wecomKey:       strings.TrimSpace(wecomKey),
		dingtalkKey:    dingtalkKey,
		dingtalkSecret: dingtalkSecret,
		now:            time.Now,
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
	endpoint, err := signDingTalkURL(webhookURL(s.dingtalkKey, dingtalkWebhookBase, "access_token"), s.dingtalkSecret, s.now())
	if err != nil {
		return webhookResult{}, err
	}
	return s.post(ctx, endpoint, payload, "dingtalk", s.dingtalkKey, s.dingtalkSecret)
}

func (s *sender) post(ctx context.Context, endpoint string, payload any, platform string, secrets ...string) (webhookResult, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return webhookResult{}, fmt.Errorf("encode payload failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return webhookResult{}, fmt.Errorf("create request failed: %s", redact(err.Error(), secrets...))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return webhookResult{}, fmt.Errorf("request failed: %s", redact(err.Error(), secrets...))
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return webhookResult{}, fmt.Errorf("read response failed: %s", redact(err.Error(), secrets...))
	}

	var result webhookResult
	jsonErr := json.Unmarshal(data, &result)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return result, fmt.Errorf("webhook returned HTTP %d: %s", resp.StatusCode, redact(responseSnippet(data), secrets...))
	}
	if jsonErr != nil {
		return result, fmt.Errorf("decode response failed: %s", redact(responseSnippet(data), secrets...))
	}

	result.Platform = platform
	if result.Errcode != 0 {
		errmsg := redact(result.Errmsg, secrets...)
		if platform == "dingtalk" && result.Errcode == 310000 {
			errmsg += "（机器人开启了加签，请用 -dingtalk-secret 提供安全设置里的 SEC 密钥）"
		}
		return result, fmt.Errorf("%s webhook error %d: %s", platform, result.Errcode, errmsg)
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

// splitDingTalkTarget 从完整 webhook URL 中提取 access_token 与 secret。
// 显式传入的 secret 优先；URL 里的 secret 参数会被移除，改由发送时动态签名。
func splitDingTalkTarget(target, secret string) (string, string) {
	target = strings.TrimSpace(target)
	secret = strings.TrimSpace(secret)
	if !strings.Contains(target, "://") {
		return target, secret
	}
	u, err := url.Parse(target)
	if err != nil {
		return target, secret
	}
	q := u.Query()
	if secret == "" {
		secret = q.Get("secret")
	}
	if q.Get("secret") != "" {
		q.Del("secret")
		u.RawQuery = q.Encode()
		target = u.String()
	}
	return target, secret
}

// signDingTalkURL 为钉钉加签机器人追加 timestamp 与 sign。
// 算法: HMAC-SHA256(secret, timestamp+"\n"+secret)，结果 base64 后 URL 编码。
func signDingTalkURL(endpoint, secret string, now time.Time) (string, error) {
	if secret == "" {
		return endpoint, nil
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse dingtalk webhook failed: %w", err)
	}
	timestamp := strconv.FormatInt(now.UnixMilli(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "\n" + secret))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	q := u.Query()
	q.Del("secret")
	q.Set("timestamp", timestamp)
	q.Set("sign", sign)
	u.RawQuery = q.Encode()
	return u.String(), nil
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

func redact(text string, secrets ...string) string {
	for _, secret := range secrets {
		if secret = strings.TrimSpace(secret); secret != "" {
			text = strings.ReplaceAll(text, secret, "***")
		}
	}
	return text
}

func responseSnippet(data []byte) string {
	const limit = 300
	text := strings.TrimSpace(string(data))
	if len(text) > limit {
		return text[:limit] + "..."
	}
	return text
}
