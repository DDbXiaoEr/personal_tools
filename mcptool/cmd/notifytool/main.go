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
	wecomKey       string
	dingtalkKey    string
	dingtalkSecret string
	timeout        time.Duration

	// 启动时配置的默认 @ 名单，调用工具时会与工具参数合并。
	wecomMentionedList       []string
	wecomMentionedMobileList []string
	dingtalkAtUserIDs        []string
	dingtalkAtMobiles        []string
	dingtalkAtAll            bool
}

var appConfig config
var defaultSender *sender

func main() {
	wecomKey := flag.String("wecom-key", "", "企业微信群机器人 key（也可直接填完整 webhook URL）")
	dingtalkKey := flag.String("dingtalk-key", "", "钉钉群机器人 access_token（也可直接填完整 webhook URL）")
	dingtalkSecret := flag.String("dingtalk-secret", "", "钉钉加签机器人的 secret（key 为完整 URL 含 secret 时可省略）")
	wecomMentioned := flag.String("wecom-mentioned-list", "", "默认 @ 的企业微信 userid，多个用英文逗号分隔；@all 表示 @所有人")
	wecomMentionedMobile := flag.String("wecom-mentioned-mobile-list", "", "默认 @ 的手机号（企业微信），多个用英文逗号分隔；@all 表示 @所有人")
	dingtalkAtUserIDs := flag.String("dingtalk-at-user-ids", "", "默认 @ 的钉钉 userid，多个用英文逗号分隔")
	dingtalkAtMobiles := flag.String("dingtalk-at-mobiles", "", "默认 @ 的手机号（钉钉），多个用英文逗号分隔")
	dingtalkAtAll := flag.Bool("dingtalk-at-all", false, "钉钉消息默认 @所有人")
	timeout := flag.Duration("timeout", 10*time.Second, "HTTP 请求超时时间")
	flag.Parse()

	appConfig = config{
		wecomKey:                 strings.TrimSpace(*wecomKey),
		dingtalkKey:              strings.TrimSpace(*dingtalkKey),
		dingtalkSecret:           strings.TrimSpace(*dingtalkSecret),
		timeout:                  *timeout,
		wecomMentionedList:       cleanList(splitAccounts(*wecomMentioned)),
		wecomMentionedMobileList: cleanList(splitAccounts(*wecomMentionedMobile)),
		dingtalkAtUserIDs:        cleanList(splitAccounts(*dingtalkAtUserIDs)),
		dingtalkAtMobiles:        cleanList(splitAccounts(*dingtalkAtMobiles)),
		dingtalkAtAll:            *dingtalkAtAll,
	}
	defaultSender = newSender(appConfig.timeout, appConfig.wecomKey, appConfig.dingtalkKey, appConfig.dingtalkSecret)

	s := server.NewMCPServer(
		"notify-mcp",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	s.AddTool(
		mcp.NewTool("send_wecom_message",
			mcp.WithDescription("发送文本消息到企业微信群机器人，可按 userid 或手机号 @ 指定人员。机器人 key 在服务启动时通过 -wecom-key 指定；启动时用 -wecom-mentioned-list / -wecom-mentioned-mobile-list 配置的默认名单会和调用参数自动合并。"),
			mcp.WithString("content",
				mcp.Required(),
				mcp.Description("要发送的文本内容"),
			),
			mcp.WithString("mentioned_list",
				mcp.Description("要 @ 的企业微信 userid，多个用英文逗号分隔；传 @all 表示 @所有人"),
			),
			mcp.WithString("mentioned_mobile_list",
				mcp.Description("要 @ 的手机号，多个用英文逗号分隔；传 @all 表示 @所有人"),
			),
		),
		wecomHandler,
	)

	s.AddTool(
		mcp.NewTool("send_dingtalk_message",
			mcp.WithDescription("发送文本消息到钉钉群机器人，可按手机号或 userid @ 指定人员。机器人 key 在服务启动时通过 -dingtalk-key 指定；启动时用 -dingtalk-at-user-ids / -dingtalk-at-mobiles / -dingtalk-at-all 配置的默认名单会和调用参数自动合并。"),
			mcp.WithString("content",
				mcp.Required(),
				mcp.Description("要发送的文本内容"),
			),
			mcp.WithString("at_mobiles",
				mcp.Description("要 @ 的手机号，多个用英文逗号分隔"),
			),
			mcp.WithString("at_user_ids",
				mcp.Description("要 @ 的钉钉 userid，多个用英文逗号分隔"),
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
		mergeAccounts(appConfig.wecomMentionedList, accountList(request, "mentioned_list")),
		mergeAccounts(appConfig.wecomMentionedMobileList, accountList(request, "mentioned_mobile_list")),
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
		mergeAccounts(appConfig.dingtalkAtMobiles, accountList(request, "at_mobiles")),
		mergeAccounts(appConfig.dingtalkAtUserIDs, accountList(request, "at_user_ids")),
		appConfig.dingtalkAtAll || request.GetBool("is_at_all", false),
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

// accountList 读取逗号分隔的账号参数，同时兼容数组写法。
func accountList(request mcp.CallToolRequest, key string) []string {
	raw, ok := request.GetArguments()[key]
	if !ok {
		return nil
	}
	var items []string
	switch v := raw.(type) {
	case string:
		items = splitAccounts(v)
	case []string:
		for _, s := range v {
			items = append(items, splitAccounts(s)...)
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				items = append(items, splitAccounts(s)...)
			}
		}
	}
	return cleanList(items)
}

// mergeAccounts 合并启动参数里的默认名单和本次调用传入的名单。
// 默认名单在前，重复项（忽略大小写）只保留第一次出现的写法。
func mergeAccounts(base, extra []string) []string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(base)+len(extra))
	out := make([]string, 0, len(base)+len(extra))
	for _, list := range [][]string{base, extra} {
		for _, item := range cleanList(list) {
			key := strings.ToLower(item)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func splitAccounts(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case ',', '，', ';', '；':
			return true
		}
		return false
	})
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
