package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSendWeCom(t *testing.T) {
	var got wecomPayload
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer srv.Close()

	s := newSender(2*time.Second, srv.URL, "", "")
	result, err := s.sendWeCom(context.Background(), "hello", []string{"user1"}, []string{"13800000000"})
	if err != nil {
		t.Fatalf("sendWeCom: %v", err)
	}
	if result.Platform != "wecom" || result.Errcode != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotContentType != "application/json" {
		t.Errorf("content-type = %q", gotContentType)
	}
	if got.MsgType != "text" || got.Text.Content != "hello" {
		t.Errorf("unexpected payload: %+v", got)
	}
	if len(got.Text.MentionedList) != 1 || got.Text.MentionedList[0] != "user1" {
		t.Errorf("mentioned_list = %v", got.Text.MentionedList)
	}
	if len(got.Text.MentionedMobileList) != 1 || got.Text.MentionedMobileList[0] != "13800000000" {
		t.Errorf("mentioned_mobile_list = %v", got.Text.MentionedMobileList)
	}
}

func TestSendDingTalkAppendsMentions(t *testing.T) {
	var got dingtalkPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer srv.Close()

	s := newSender(2*time.Second, "", srv.URL, "")
	result, err := s.sendDingTalk(context.Background(), "deploy done", []string{"13800000000"}, []string{"user1"}, false)
	if err != nil {
		t.Fatalf("sendDingTalk: %v", err)
	}
	if result.Platform != "dingtalk" || result.Errcode != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got.MsgType != "text" {
		t.Errorf("msgtype = %q", got.MsgType)
	}
	if !strings.Contains(got.Text.Content, "@13800000000") || !strings.Contains(got.Text.Content, "@user1") {
		t.Errorf("content should include mentions: %q", got.Text.Content)
	}
	if len(got.At.AtMobiles) != 1 || got.At.AtMobiles[0] != "13800000000" {
		t.Errorf("atMobiles = %v", got.At.AtMobiles)
	}
	if len(got.At.AtUserIDs) != 1 || got.At.AtUserIDs[0] != "user1" {
		t.Errorf("atUserIds = %v", got.At.AtUserIDs)
	}
}

func TestSendDingTalkDoesNotDuplicateMentions(t *testing.T) {
	got := appendDingTalkMentions("hi @13800000000", []string{"13800000000"}, nil)
	if got != "hi @13800000000" {
		t.Fatalf("content = %q", got)
	}
}

func TestSendWebhookError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":93000,"errmsg":"invalid token"}`))
	}))
	defer srv.Close()

	s := newSender(2*time.Second, srv.URL, "", "")
	_, err := s.sendWeCom(context.Background(), "hello", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "93000") || !strings.Contains(err.Error(), "invalid token") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSignDingTalkURL(t *testing.T) {
	now := time.UnixMilli(1700000000000)
	got, err := signDingTalkURL(dingtalkWebhookBase+"?access_token=tok", "SECtest", now)
	if err != nil {
		t.Fatalf("signDingTalkURL: %v", err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := u.Query()
	if q.Get("access_token") != "tok" {
		t.Errorf("access_token = %q", q.Get("access_token"))
	}
	if q.Get("timestamp") != "1700000000000" {
		t.Errorf("timestamp = %q", q.Get("timestamp"))
	}

	mac := hmac.New(sha256.New, []byte("SECtest"))
	mac.Write([]byte("1700000000000\nSECtest"))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if q.Get("sign") != want {
		t.Errorf("sign = %q, want %q", q.Get("sign"), want)
	}
}

func TestSignDingTalkURLWithoutSecret(t *testing.T) {
	endpoint := dingtalkWebhookBase + "?access_token=tok"
	got, err := signDingTalkURL(endpoint, "", time.UnixMilli(1700000000000))
	if err != nil {
		t.Fatalf("signDingTalkURL: %v", err)
	}
	if got != endpoint {
		t.Fatalf("got = %q, want %q", got, endpoint)
	}
}

func TestSendDingTalkSignsRequest(t *testing.T) {
	var query url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer srv.Close()

	s := newSender(2*time.Second, "", srv.URL+"?access_token=tok&secret=SECtest", "")
	s.now = func() time.Time { return time.UnixMilli(1700000000000) }
	if s.dingtalkSecret != "SECtest" {
		t.Fatalf("secret = %q", s.dingtalkSecret)
	}
	if _, err := s.sendDingTalk(context.Background(), "hi", nil, nil, false); err != nil {
		t.Fatalf("sendDingTalk: %v", err)
	}
	if query.Get("timestamp") != "1700000000000" || query.Get("sign") == "" {
		t.Fatalf("missing signature: %v", query)
	}
	if query.Get("secret") != "" {
		t.Fatalf("secret should be removed from URL: %v", query)
	}
	if query.Get("access_token") != "tok" {
		t.Fatalf("access_token = %q", query.Get("access_token"))
	}
}

func TestSplitDingTalkTarget(t *testing.T) {
	key, secret := splitDingTalkTarget(" tok ", "")
	if key != "tok" || secret != "" {
		t.Fatalf("key=%q secret=%q", key, secret)
	}

	key, secret = splitDingTalkTarget("https://example.com/hook?access_token=tok&secret=SEC", "")
	if key != "https://example.com/hook?access_token=tok" || secret != "SEC" {
		t.Fatalf("key=%q secret=%q", key, secret)
	}

	key, secret = splitDingTalkTarget("https://example.com/hook?access_token=tok&secret=SEC", "EXPLICIT")
	if key != "https://example.com/hook?access_token=tok" || secret != "EXPLICIT" {
		t.Fatalf("key=%q secret=%q", key, secret)
	}
}

func TestWebhookURLEscapesKey(t *testing.T) {
	got := webhookURL("a b&c", wecomWebhookBase, "key")
	want := wecomWebhookBase + "?key=a+b%26c"
	if got != want {
		t.Fatalf("webhookURL = %q, want %q", got, want)
	}

	full := "https://example.com/hook?token=abc"
	if got := webhookURL(full, dingtalkWebhookBase, "access_token"); got != full {
		t.Fatalf("webhookURL full = %q", got)
	}
}

func TestRedact(t *testing.T) {
	got := redact("Post https://example.com?key=secret: failed", "secret")
	if strings.Contains(got, "secret") || !strings.Contains(got, "***") {
		t.Fatalf("redact = %q", got)
	}
}
