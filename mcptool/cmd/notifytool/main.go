package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type config struct {
	wecomKey    string
	dingtalkKey string
	timeout     time.Duration
}

var appConfig config
var defaultSender *sender

func main() {
	wecomKey := flag.String("wecom-key", "", "企业微信群机器人 key（也可直接填完整 webhook URL）")
	dingtalkKey := flag.String("dingtalk-key", "", "钉钉群机器人 access_token（也可直接填完整 webhook URL）")
	timeout := flag.Duration("timeout", 10*time.Second, "HTTP 请求超时时间")
	flag.Parse()

	appConfig = config{
		wecomKey:    strings.TrimSpace(*wecomKey),
		dingtalkKey: strings.TrimSpace(*dingtalkKey),
		timeout:     *timeout,
	}
	defaultSender = newSender(appConfig.timeout, appConfig.wecomKey, appConfig.dingtalkKey)

	s := server.NewMCPServer(
		"notify-mcp",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	s.AddTool(
		mcp.NewTool("send_wecom_message",
			mcp.WithDescription("发送文本消息到企业微信群机器人，可按 userid 或手机号 @ 指定人员。机器人 key 在服务启动时通过 -wecom-key 指定。"),
			mcp.WithString("content",
				mcp.Required(),
				mcp.Description("要发送的文本内容"),
			),
			mcp.WithArray("mentioned_list",
				mcp.Description("要 @ 的企业微信 userid 列表；传 @all 表示 @所有人"),
				mcp.WithStringItems(),
			),
			mcp.WithArray("mentioned_mobile_list",
				mcp.Description("要 @ 的手机号列表；传 @all 表示 @所有人"),
				mcp.WithStringItems(),
			),
		),
		wecomHandler,
	)

	s.AddTool(
		mcp.NewTool("send_dingtalk_message",
			mcp.WithDescription("发送文本消息到钉钉群机器人，可按手机号或 userid @ 指定人员。机器人 key 在服务启动时通过 -dingtalk-key 指定。"),
			mcp.WithString("content",
				mcp.Required(),
				mcp.Description("要发送的文本内容"),
			),
			mcp.WithArray("at_mobiles",
				mcp.Description("要 @ 的手机号列表"),
				mcp.WithStringItems(),
			),
			mcp.WithArray("at_user_ids",
				mcp.Description("要 @ 的钉钉 userid 列表"),
				mcp.WithStringItems(),
			),
			mcp.WithBoolean("is_at_all",
				mcp.Description("是否 @所有人（默认 false）"),
			),
		),
		dingtalkHandler,
	)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func wecomHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	content := strings.TrimSpace(request.GetString("content", ""))
	if content == "" {
		return mcp.NewToolResultError("content is required"), nil
	}
	if appConfig.wecomKey == "" {
		return mcp.NewToolResultError("wecom key is not configured; start the server with -wecom-key"), nil
	}

	result, err := defaultSender.sendWeCom(
		ctx,
		content,
		cleanList(request.GetStringSlice("mentioned_list", nil)),
		cleanList(request.GetStringSlice("mentioned_mobile_list", nil)),
	)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(marshalResult(result)), nil
}

func dingtalkHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	content := strings.TrimSpace(request.GetString("content", ""))
	if content == "" {
		return mcp.NewToolResultError("content is required"), nil
	}
	if appConfig.dingtalkKey == "" {
		return mcp.NewToolResultError("dingtalk key is not configured; start the server with -dingtalk-key"), nil
	}

	result, err := defaultSender.sendDingTalk(
		ctx,
		content,
		cleanList(request.GetStringSlice("at_mobiles", nil)),
		cleanList(request.GetStringSlice("at_user_ids", nil)),
		request.GetBool("is_at_all", false),
	)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(marshalResult(result)), nil
}

func marshalResult(result webhookResult) string {
	b, _ := json.Marshal(result)
	return string(b)
}

func cleanList(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
