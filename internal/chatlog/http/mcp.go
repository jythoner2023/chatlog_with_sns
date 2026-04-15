package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog/log"

	"github.com/sjzar/chatlog/internal/chatlog/conf"
	"github.com/sjzar/chatlog/internal/errors"
	"github.com/sjzar/chatlog/internal/model"
	"github.com/sjzar/chatlog/pkg/util"
	"github.com/sjzar/chatlog/pkg/util/dat2img"
	"github.com/sjzar/chatlog/pkg/util/silk"
	"github.com/sjzar/chatlog/pkg/version"
)

func (s *Service) initMCPServer() {
	s.mcpServer = server.NewMCPServer(conf.AppName, version.Version,
		server.WithResourceCapabilities(false, false),
		server.WithToolCapabilities(true),
		server.WithPromptCapabilities(true),
	)
	s.mcpServer.AddTool(ContactTool, s.handleMCPContact)
	s.mcpServer.AddTool(ChatRoomTool, s.handleMCPChatRoom)
	s.mcpServer.AddTool(RecentChatTool, s.handleMCPRecentChat)
	s.mcpServer.AddTool(ChatLogTool, s.handleMCPChatLog)
	s.mcpServer.AddTool(CurrentTimeTool, s.handleMCPCurrentTime)
	s.mcpServer.AddTool(GetMediaContentTool, s.handleMCPGetMediaContent)
	s.mcpServer.AddTool(OCRImageMessageTool, s.handleMCPOCRImageMessage)
	s.mcpServer.AddTool(SendWebhookNotificationTool, s.handleMCPSendWebhookNotification)
	s.mcpServer.AddTool(AnalyzeChatActivityTool, s.handleMCPAnalyzeChatActivity)
	s.mcpServer.AddTool(GetUserProfileTool, s.handleMCPGetUserProfile)
	s.mcpServer.AddTool(SearchSharedFilesTool, s.handleMCPSearchSharedFiles)
	s.mcpServer.AddPrompt(ChatSummaryDailyPrompt, s.handleMCPChatSummaryDaily)
	s.mcpServer.AddPrompt(ConflictDetectorPrompt, s.handleMCPConflictDetector)
	s.mcpServer.AddPrompt(RelationshipMilestonesPrompt, s.handleMCPRelationshipMilestones)
	s.mcpSSEServer = server.NewSSEServer(s.mcpServer,
		server.WithSSEEndpoint("/sse"),
		server.WithMessageEndpoint("/message"),
	)
	s.mcpStreamableServer = server.NewStreamableHTTPServer(s.mcpServer)
}

var ChatSummaryDailyPrompt = mcp.NewPrompt(
	"chat_summary_daily",
	mcp.WithPromptDescription("生成每日聊天摘要模板。"),
	mcp.WithArgument("date", mcp.ArgumentDescription("摘要日期 (YYYY-MM-DD)"), mcp.RequiredArgument()),
	mcp.WithArgument("talker", mcp.ArgumentDescription("对话方 ID"), mcp.RequiredArgument()),
)

var ConflictDetectorPrompt = mcp.NewPrompt(
	"conflict_detector",
	mcp.WithPromptDescription("情绪与冲突检测模板。"),
	mcp.WithArgument("talker", mcp.ArgumentDescription("对话方 ID"), mcp.RequiredArgument()),
)

var RelationshipMilestonesPrompt = mcp.NewPrompt(
	"relationship_milestones",
	mcp.WithPromptDescription("关系里程碑回顾模板。"),
	mcp.WithArgument("talker", mcp.ArgumentDescription("对话方 ID"), mcp.RequiredArgument()),
)

var SearchSharedFilesTool = mcp.NewTool(
	"search_shared_files",
	mcp.WithDescription(`专门搜索聊天记录中发送的文件元数据。当用户想找某个特定的共享文件时使用。`),
	mcp.WithString("talker", mcp.Description("对话方 ID"), mcp.Required()),
	mcp.WithString("keyword", mcp.Description("文件名搜索关键词")),
)

var AnalyzeChatActivityTool = mcp.NewTool(
	"analyze_chat_activity",
	mcp.WithDescription(`统计特定时间段内对话方的活跃度，包括发言频率、活跃时段等。用于分析某人的社交习惯或群聊热度。`),
	mcp.WithString("time", mcp.Description("时间范围 (例如: 2023-04-01~2023-04-18)"), mcp.Required()),
	mcp.WithString("talker", mcp.Description("对话方 ID"), mcp.Required()),
)

var GetUserProfileTool = mcp.NewTool(
	"get_user_profile",
	mcp.WithDescription(`获取联系人或群组的详细资料，包括备注、属性、群成员（如果是群组）等背景信息。用于更深入地了解对话方。`),
	mcp.WithString("key", mcp.Description("联系人或群组的 ID 或名称"), mcp.Required()),
)

var SendWebhookNotificationTool = mcp.NewTool(
	"send_webhook_notification",
	mcp.WithDescription(`触发外部 Webhook 通知。当模型完成聊天记录分析、发现重要事项或需要提醒外部系统时使用此工具。`),
	mcp.WithString("url", mcp.Description("Webhook 接收地址"), mcp.Required()),
	mcp.WithString("message", mcp.Description("要发送的通知内容或分析结果"), mcp.Required()),
	mcp.WithString("level", mcp.Description("通知级别 (info, warn, error)")),
)

var OCRImageMessageTool = mcp.NewTool(
	"ocr_image_message",
	mcp.WithDescription(`对特定图片消息进行 OCR 解析以提取其中的文字。`),
	mcp.WithString("talker", mcp.Description("消息所在的对话方（联系人 ID 或群 ID）"), mcp.Required()),
	mcp.WithNumber("message_id", mcp.Description("消息的唯一 ID (Seq)"), mcp.Required()),
)

var GetMediaContentTool = mcp.NewTool(
	"get_media_content",
	mcp.WithDescription(`根据消息 ID 获取解码后的媒体文件内容（图片或语音）。当聊天记录中显示 [图片] 或 [语音] 且用户需要查看具体内容或进行分析时使用此工具。`),
	mcp.WithString("talker", mcp.Description("消息所在的对话方（联系人 ID 或群 ID）"), mcp.Required()),
	mcp.WithNumber("message_id", mcp.Description("消息的唯一 ID (Seq)"), mcp.Required()),
)

var ContactTool = mcp.NewTool(
	"query_contact",
	mcp.WithDescription(`查询用户的联系人信息。可以通过姓名、备注名、ID 或多个标签进行查询，返回匹配的联系人列表。当用户询问某人的联系方式、想了解联系人信息、需要按标签筛选联系人时使用此工具。参数为空时，将返回联系人列表`),
	mcp.WithString("keyword", mcp.Description("联系人的搜索关键词，可以是姓名、备注名或ID。")),
	mcp.WithString("tags", mcp.Description("联系人标签，多个标签用英文逗号分隔，例如：投资人,AI圈")),
	mcp.WithString("tag_mode", mcp.Description("多标签匹配模式：all=同时满足全部标签，any=满足任一标签")),
)

var ChatRoomTool = mcp.NewTool(
	"query_chat_room",
	mcp.WithDescription(`查询用户参与的群聊信息。可以通过群名称、群ID或相关关键词进行查询，返回匹配的群聊列表。当用户询问群聊信息、想了解某个群的详情或需要查找特定群聊时使用此工具。`),
	mcp.WithString("keyword", mcp.Description("群聊的搜索关键词，可以是群名称、群ID或相关描述")),
)

var RecentChatTool = mcp.NewTool(
	"query_recent_chat",
	mcp.WithDescription(`查询最近会话列表，包括个人聊天和群聊。当用户想了解最近的聊天记录、查看最近联系过的人或群组时使用此工具。不需要参数，直接返回最近的会话列表。`),
)

var ChatLogTool = mcp.NewTool(
	"query_chat_log",
	mcp.WithDescription(`检索历史聊天记录，可根据时间、对话方、发送者和关键词等条件进行精确查询。当用户需要查找特定信息或想了解与某人/某群的历史交流时使用此工具。

【强制多步查询流程!】
当查询特定话题或特定发送者发言时，必须严格按照以下流程使用，任何偏离都会导致错误的结果：

步骤1: 初步定位相关消息
- 使用keyword参数查找特定话题
- 使用sender参数查找特定发送者的消息
- 使用较宽时间范围初步查询

步骤2: 【必须执行】针对每个关键结果点分别获取上下文
- 必须对步骤1返回的每个时间点T1, T2, T3...分别执行独立查询（时间范围接近的消息可以合并为一个查询）
- 每次独立查询必须移除keyword参数
- 每次独立查询必须移除sender参数
- 每次独立查询使用"Tn前后15-30分钟"的窄范围
- 每次独立查询仅保留talker参数

步骤3: 【必须执行】综合分析所有上下文
- 必须等待所有步骤2的查询结果返回后再进行分析
- 必须综合考虑所有上下文信息后再回答用户

【严格执行规则！】
- 禁止仅凭步骤1的结果直接回答用户
- 禁止在步骤2使用过大的时间范围一次性查询所有上下文
- 禁止跳过步骤2或步骤3
- 必须对每个关键结果点分别执行独立的上下文查询

【执行示例】
正确流程示例:
1. 步骤1: chatlog(time="2023-04-01~2023-04-30", talker="工作群", keyword="项目进度")
返回结果: 4月5日、4月12日、4月20日有相关消息
2. 步骤2:
- 查询1: chatlog(time="2023-04-05/09:30~2023-04-05/10:30", talker="工作群") // 注意没有keyword
- 查询2: chatlog(time="2023-04-12/14:00~2023-04-12/15:00", talker="工作群") // 注意没有keyword
- 查询3: chatlog(time="2023-04-20/16:00~2023-04-20/17:00", talker="工作群") // 注意没有keyword
3. 步骤3: 综合分析所有上下文后回答用户

错误流程示例:
- 仅执行步骤1后直接回答
- 步骤2使用time="2023-04-01~2023-04-30"一次性查询
- 步骤2仍然保留keyword或sender参数

【自我检查】回答用户前必须自问:
- 我是否对每个关键时间点都执行了独立的上下文查询?
- 我是否在上下文查询中移除了keyword和sender参数?
- 我是否分析了所有上下文后再回答?
- 如果上述任一问题答案为"否"，则必须纠正流程

返回格式："昵称(ID) [MessageID] 时间\n消息内容\n昵称(ID) [MessageID] 时间\n消息内容"
当消息内容包含 [图片] 或 [语音] 时，可以使用 get_media_content 或 ocr_image_message 工具，并传入对应的 [MessageID] 来获取具体内容。
当查询多个Talker时，返回格式为："昵称(ID)\n[TalkerName(Talker)] [MessageID] 时间\n消息内容"

重要提示：
1. 当用户询问特定时间段内的聊天记录时，必须使用正确的时间格式，特别是包含小时和分钟的查询
2. 对于"今天下午4点到5点聊了啥"这类查询，正确的时间参数格式应为"2023-04-18/16:00~2023-04-18/17:00"
3. 当用户询问具体群聊中某人的聊天记录时，使用"sender"参数
4. 当用户询问包含特定关键词的聊天记录时，使用"keyword"参数`),
	mcp.WithString("time", mcp.Description(`指定查询的时间点或时间范围，格式必须严格遵循以下规则：

【单一时间点格式】
- 精确到日："2023-04-18"或"20230418"
- 精确到分钟（必须包含斜杠和冒号）："2023-04-18/14:30"或"20230418/14:30"（表示2023年4月18日14点30分）

【时间范围格式】（使用"~"分隔起止时间）
- 日期范围："2023-04-01~2023-04-18"
- 同一天的时间段："2023-04-18/14:30~2023-04-18/15:45"
* 表示2023年4月18日14点30分到15点45分之间

【重要提示】包含小时分钟的格式必须使用斜杠和冒号："/"和":"
正确示例："2023-04-18/16:30"（4月18日下午4点30分）
错误示例："2023-04-18 16:30"、"2023-04-18T16:30"

【其他支持的格式】
- 年份："2023"
- 月份："2023-04"或"202304"`), mcp.Required()),
	mcp.WithString("talker", mcp.Description(`指定对话方（联系人或群组）
- 可使用ID、昵称或备注名
- 多个对话方用","分隔，如："张三,李四,工作群"
- 【重要】这是多步查询中唯一应保留的参数`), mcp.Required()),
	mcp.WithString("sender", mcp.Description(`指定群聊中的发送者
- 仅在查询群聊记录时有效
- 多个发送者用","分隔，如："张三,李四"
- 可使用ID、昵称或备注名
【重要】查询特定发送者的消息时：
1. 第一步：使用sender参数初步定位多个相关消息时间点
2. 后续步骤：必须移除sender参数，分别查询每个时间点前后的完整对话
3. 错误示例：对所有找到的消息一次性查询大范围上下文
4. 正确示例：对每个时间点T分别执行查询"T前后15-30分钟"（不带sender）`)),
	mcp.WithString("keyword", mcp.Description(`搜索内容中的关键词
- 支持正则表达式匹配
- 【重要】查询特定话题时：
1. 第一步：使用keyword参数初步定位多个相关消息时间点
2. 后续步骤：必须移除keyword参数，分别查询每个时间点前后的完整对话
3. 错误示例：对所有找到的关键词消息一次性查询大范围上下文
4. 正确示例：对每个时间点T分别执行查询"T前后15-30分钟"（不带keyword）`)),
)

var CurrentTimeTool = mcp.NewTool(
	"current_time",
	mcp.WithDescription(`获取当前系统时间，返回RFC3339格式的时间字符串（包含用户本地时区信息）。
使用场景：
- 当用户询问"总结今日聊天记录"、"本周都聊了啥"等当前时间问题
- 当用户提及"昨天"、"上周"、"本月"等相对时间概念，需要确定基准时间点
- 需要执行依赖当前时间的计算（如"上个月5号我们有开会吗"）
返回示例：2025-04-18T21:29:00+08:00
注意：此工具不需要任何输入参数，直接调用即可获取当前时间。`),
)

type ContactRequest struct {
	Keyword string `json:"keyword"`
	Tags    string `json:"tags"`
	TagMode string `json:"tag_mode"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}

func (s *Service) handleMCPContact(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var req ContactRequest
	if err := request.BindArguments(&req); err != nil {
		log.Error().Err(err).Msg("Failed to bind arguments")
		log.Error().Interface("request", request.GetRawArguments()).Msg("Failed to bind arguments")
		return errors.ErrMCPTool(err), nil
	}

	list, err := s.db.GetContacts(req.Keyword, req.Tags, req.TagMode, req.Limit, req.Offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get contacts")
		return errors.ErrMCPTool(err), nil
	}
	buf := &bytes.Buffer{}
	csvWriter := csv.NewWriter(buf)
	csvWriter.Write([]string{"UserName", "Alias", "Remark", "NickName", "Labels"})
	for _, contact := range list.Items {
		csvWriter.Write([]string{contact.UserName, contact.Alias, contact.Remark, contact.NickName, model.ContactLabelsString(contact.Labels)})
	}
	csvWriter.Flush()
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: buf.String(),
			},
		},
	}, nil
}

type ChatRoomRequest struct {
	Keyword string `json:"keyword"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}

func (s *Service) handleMCPChatRoom(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	var req ChatRoomRequest
	if err := request.BindArguments(&req); err != nil {
		log.Error().Err(err).Msg("Failed to bind arguments")
		log.Error().Interface("request", request.GetRawArguments()).Msg("Failed to bind arguments")
		return errors.ErrMCPTool(err), nil
	}

	list, err := s.db.GetChatRooms(req.Keyword, req.Limit, req.Offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chat rooms")
		return errors.ErrMCPTool(err), nil
	}
	buf := &bytes.Buffer{}
	buf.WriteString("Name,Remark,NickName,Owner,UserCount\n")
	for _, chatRoom := range list.Items {
		buf.WriteString(fmt.Sprintf("%s,%s,%s,%s,%d\n", chatRoom.Name, chatRoom.Remark, chatRoom.NickName, chatRoom.Owner, len(chatRoom.Users)))
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: buf.String(),
			},
		},
	}, nil
}

type RecentChatRequest struct {
	Keyword string `json:"keyword"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}

func (s *Service) handleMCPRecentChat(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	var req RecentChatRequest
	if err := request.BindArguments(&req); err != nil {
		log.Error().Err(err).Msg("Failed to bind arguments")
		log.Error().Interface("request", request.GetRawArguments()).Msg("Failed to bind arguments")
		return errors.ErrMCPTool(err), nil
	}

	data, err := s.db.GetSessions(req.Keyword, req.Limit, req.Offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get sessions")
		return errors.ErrMCPTool(err), nil
	}
	buf := &bytes.Buffer{}
	for _, session := range data.Items {
		buf.WriteString(session.PlainText(120))
		buf.WriteString("\n")
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: buf.String(),
			},
		},
	}, nil
}

type ChatLogRequest struct {
	Time    string `form:"time"`
	Talker  string `form:"talker"`
	Sender  string `form:"sender"`
	Keyword string `form:"keyword"`
	Limit   int    `form:"limit"`
	Offset  int    `form:"offset"`
	Format  string `form:"format"`
}

func (s *Service) handleMCPChatLog(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	var req ChatLogRequest
	if err := request.BindArguments(&req); err != nil {
		log.Error().Err(err).Msg("Failed to bind arguments")
		log.Error().Interface("request", request.GetRawArguments()).Msg("Failed to bind arguments")
		return errors.ErrMCPTool(err), nil
	}

	var err error
	start, end, ok := util.TimeRangeOf(req.Time)
	if !ok {
		log.Error().Err(err).Msg("Failed to get messages")
		return errors.ErrMCPTool(err), nil
	}
	if req.Limit < 0 {
		req.Limit = 0
	}

	if req.Offset < 0 {
		req.Offset = 0
	}

	messages, err := s.db.GetMessages(start, end, req.Talker, req.Sender, req.Keyword, req.Limit, req.Offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get messages")
		return errors.ErrMCPTool(err), nil
	}

	buf := &bytes.Buffer{}
	if len(messages) == 0 {
		buf.WriteString("未找到符合查询条件的聊天记录")
	}
	for _, m := range messages {
		buf.WriteString(m.PlainText(strings.Contains(req.Talker, ","), util.PerfectTimeFormat(start, end), ""))
		buf.WriteString("\n")
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: buf.String(),
			},
		},
	}, nil
}

func (s *Service) handleMCPCurrentTime(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: time.Now().Local().Format(time.RFC3339),
			},
		},
	}, nil
}

type GetMediaContentRequest struct {
	Talker    string `json:"talker"`
	MessageID int64  `json:"message_id"`
}

func (s *Service) handleMCPGetMediaContent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var req GetMediaContentRequest
	if err := request.BindArguments(&req); err != nil {
		return errors.ErrMCPTool(err), nil
	}

	msg, err := s.db.GetMessage(req.Talker, req.MessageID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get message")
		return errors.ErrMCPTool(err), nil
	}

	switch msg.Type {
	case model.MessageTypeImage:
		return s.handleMCPGetImage(ctx, msg)
	case model.MessageTypeVoice:
		return s.handleMCPGetVoice(ctx, msg)
	default:
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("暂不支持的消息类型: %d", msg.Type),
				},
			},
		}, nil
	}
}

func (s *Service) handleMCPOCRImageMessage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var req GetMediaContentRequest
	if err := request.BindArguments(&req); err != nil {
		return errors.ErrMCPTool(err), nil
	}

	msg, err := s.db.GetMessage(req.Talker, req.MessageID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get message")
		return errors.ErrMCPTool(err), nil
	}

	if msg.Type != model.MessageTypeImage {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "该消息不是图片消息，无法进行 OCR 解析。",
				},
			},
		}, nil
	}

	result, err := s.handleMCPGetImage(ctx, msg)
	if err != nil {
		return result, err
	}

	// 在结果中添加一条提示信息
	result.Content = append([]mcp.Content{
		mcp.TextContent{
			Type: "text",
			Text: "已提取图片数据，请直接分析该图片内容并提取文字 (OCR)。",
		},
	}, result.Content...)

	return result, nil
}

func (s *Service) handleMCPGetImage(ctx context.Context, msg *model.Message) (*mcp.CallToolResult, error) {
	key, ok := msg.Contents["md5"].(string)
	if !ok {
		// 尝试从 path 获取
		key, _ = msg.Contents["path"].(string)
	}

	if key == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "无法找到图片标识符",
				},
			},
		}, nil
	}

	media, err := s.db.GetMedia("image", key)
	if err != nil {
		return errors.ErrMCPTool(err), nil
	}

	absolutePath := filepath.Join(s.conf.GetDataDir(), media.Path)
	b, err := os.ReadFile(absolutePath)
	if err != nil {
		return errors.ErrMCPTool(err), nil
	}

	var data []byte
	var mimeType string

	if strings.HasSuffix(strings.ToLower(media.Path), ".dat") {
		out, ext, err := dat2img.Dat2Image(b)
		if err != nil {
			return errors.ErrMCPTool(err), nil
		}
		data = out
		switch ext {
		case "png":
			mimeType = "image/png"
		case "gif":
			mimeType = "image/gif"
		case "bmp":
			mimeType = "image/bmp"
		default:
			mimeType = "image/jpeg"
		}
	} else {
		data = b
		ext := strings.ToLower(filepath.Ext(media.Path))
		switch ext {
		case ".png":
			mimeType = "image/png"
		case ".gif":
			mimeType = "image/gif"
		case ".bmp":
			mimeType = "image/bmp"
		default:
			mimeType = "image/jpeg"
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.ImageContent{
				Type:     "image",
				Data:     base64.StdEncoding.EncodeToString(data),
				MIMEType: mimeType,
			},
		},
	}, nil
}

func (s *Service) handleMCPGetVoice(ctx context.Context, msg *model.Message) (*mcp.CallToolResult, error) {
	key, ok := msg.Contents["voice"].(string)
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "无法找到语音标识符",
				},
			},
		}, nil
	}

	media, err := s.db.GetMedia("voice", key)
	if err != nil {
		return errors.ErrMCPTool(err), nil
	}

	out, err := silk.Silk2MP3(media.Data)
	if err != nil {
		// 如果转换失败，返回 base64 编码的原始数据
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("语音转换失败: %v。原始语音数据(base64): %s", err, base64.StdEncoding.EncodeToString(media.Data)),
				},
			},
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("语音已转换为 MP3 格式。数据(base64): %s", base64.StdEncoding.EncodeToString(out)),
			},
		},
	}, nil
}

type SendWebhookNotificationRequest struct {
	URL     string `json:"url"`
	Message string `json:"message"`
	Level   string `json:"level"`
}

func (s *Service) handleMCPSendWebhookNotification(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var req SendWebhookNotificationRequest
	if err := request.BindArguments(&req); err != nil {
		return errors.ErrMCPTool(err), nil
	}

	payload := map[string]interface{}{
		"message":   req.Message,
		"level":     req.Level,
		"timestamp": time.Now().Format(time.RFC3339),
		"source":    "chatlog-mcp",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return errors.ErrMCPTool(err), nil
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", req.URL, bytes.NewBuffer(body))
	if err != nil {
		return errors.ErrMCPTool(err), nil
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return errors.ErrMCPTool(err), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.ErrMCPTool(fmt.Errorf("webhook returned status %d", resp.StatusCode)), nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: "Webhook 通知发送成功。",
			},
		},
	}, nil
}

type AnalyzeChatActivityRequest struct {
	Time   string `json:"time"`
	Talker string `json:"talker"`
}

func (s *Service) handleMCPAnalyzeChatActivity(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var req AnalyzeChatActivityRequest
	if err := request.BindArguments(&req); err != nil {
		return errors.ErrMCPTool(err), nil
	}

	start, end, ok := util.TimeRangeOf(req.Time)
	if !ok {
		return errors.ErrMCPTool(fmt.Errorf("invalid time format")), nil
	}

	messages, err := s.db.GetMessages(start, end, req.Talker, "", "", 0, 0)
	if err != nil {
		return errors.ErrMCPTool(err), nil
	}

	if len(messages) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "该时间段内没有聊天记录。",
				},
			},
		}, nil
	}

	// 统计逻辑
	totalCount := len(messages)
	senderStats := make(map[string]int)
	hourStats := make(map[int]int)
	typeStats := make(map[int64]int)

	for _, m := range messages {
		sender := m.SenderName
		if sender == "" {
			sender = m.Sender
		}
		senderStats[sender]++
		hourStats[m.Time.Hour()]++
		typeStats[m.Type]++
	}

	buf := &bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("分析报告 (%s - %s)\n", start.Format(time.DateOnly), end.Format(time.DateOnly)))
	buf.WriteString(fmt.Sprintf("总消息数: %d\n\n", totalCount))

	buf.WriteString("发言频率排行:\n")
	type senderStat struct {
		Name  string
		Count int
	}
	ss := make([]senderStat, 0, len(senderStats))
	for name, count := range senderStats {
		ss = append(ss, senderStat{name, count})
	}
	sort.Slice(ss, func(i, j int) bool { return ss[i].Count > ss[j].Count })
	for i, s := range ss {
		if i >= 10 {
			break
		} // 只显示前 10
		percentage := float64(s.Count) / float64(totalCount) * 100
		buf.WriteString(fmt.Sprintf("- %s: %d (%.1f%%)\n", s.Name, s.Count, percentage))
	}

	buf.WriteString("\n活跃时段分布:\n")
	for h := 0; h < 24; h++ {
		if count, ok := hourStats[h]; ok {
			buf.WriteString(fmt.Sprintf("%02d:00: %s (%d)\n", h, strings.Repeat("█", (count*20+totalCount-1)/totalCount), count))
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: buf.String(),
			},
		},
	}, nil
}

type GetUserProfileRequest struct {
	Key string `json:"key"`
}

func (s *Service) handleMCPGetUserProfile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var req GetUserProfileRequest
	if err := request.BindArguments(&req); err != nil {
		return errors.ErrMCPTool(err), nil
	}

	buf := &bytes.Buffer{}

	// 尝试作为群聊获取
	if chatRoom, err := s.db.GetChatRoom(req.Key); err == nil {
		buf.WriteString(fmt.Sprintf("【群聊资料】\n"))
		buf.WriteString(fmt.Sprintf("ID: %s\n", chatRoom.Name))
		buf.WriteString(fmt.Sprintf("名称: %s\n", chatRoom.NickName))
		if chatRoom.Remark != "" {
			buf.WriteString(fmt.Sprintf("备注: %s\n", chatRoom.Remark))
		}
		buf.WriteString(fmt.Sprintf("群主: %s\n", chatRoom.Owner))
		buf.WriteString(fmt.Sprintf("成员数: %d\n", len(chatRoom.Users)))
		buf.WriteString("\n部分成员列表:\n")
		for i, user := range chatRoom.Users {
			if i >= 20 {
				buf.WriteString("... 等等\n")
				break
			}
			displayName := chatRoom.User2DisplayName[user.UserName]
			buf.WriteString(fmt.Sprintf("- %s (%s)\n", displayName, user.UserName))
		}
	} else if contact, err := s.db.GetContact(req.Key); err == nil {
		// 尝试作为联系人获取
		buf.WriteString(fmt.Sprintf("【联系人资料】\n"))
		buf.WriteString(fmt.Sprintf("ID: %s\n", contact.UserName))
		buf.WriteString(fmt.Sprintf("昵称: %s\n", contact.NickName))
		if contact.Remark != "" {
			buf.WriteString(fmt.Sprintf("备注: %s\n", contact.Remark))
		}
		if contact.Alias != "" {
			buf.WriteString(fmt.Sprintf("微信号: %s\n", contact.Alias))
		}
		buf.WriteString(fmt.Sprintf("是否好友: %v\n", contact.IsFriend))
	} else {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("未找到相关联系人或群组: %s", req.Key),
				},
			},
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: buf.String(),
			},
		},
	}, nil
}

type SearchSharedFilesRequest struct {
	Talker  string `json:"talker"`
	Keyword string `json:"keyword"`
}

func (s *Service) handleMCPSearchSharedFiles(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var req SearchSharedFilesRequest
	if err := request.BindArguments(&req); err != nil {
		return errors.ErrMCPTool(err), nil
	}

	// 查找 MessageTypeShare (49) 且 MessageSubTypeFile (6)
	messages, err := s.db.GetMessages(time.Time{}, time.Now(), req.Talker, "", req.Keyword, 50, 0)
	if err != nil {
		return errors.ErrMCPTool(err), nil
	}

	buf := &bytes.Buffer{}
	count := 0
	for _, m := range messages {
		if m.Type == model.MessageTypeShare && m.SubType == model.MessageSubTypeFile {
			title, _ := m.Contents["title"].(string)
			buf.WriteString(fmt.Sprintf("[%d] %s - %s\n", m.Seq, m.Time.Format("2006-01-02 15:04"), title))
			count++
		}
	}

	if count == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "未找到相关共享文件。",
				},
			},
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("找到 %d 个文件:\n%s", count, buf.String()),
			},
		},
	}, nil
}

func (s *Service) handleMCPChatSummaryDaily(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	date := request.Params.Arguments["date"]
	talker := request.Params.Arguments["talker"]

	return mcp.NewGetPromptResult(
		"每日聊天摘要指令",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("请分析并在总结 %s 在 %s 的聊天内容。请先使用 query_chat_log 获取当天的完整记录，然后从关键话题、重要决策、待办事项三个维度进行总结。", talker, date),
			}),
		},
	), nil
}

func (s *Service) handleMCPConflictDetector(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	talker := request.Params.Arguments["talker"]

	return mcp.NewGetPromptResult(
		"情绪与冲突检测指令",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("请分析与 %s 最近的聊天记录，识别是否存在潜在的情绪波动或冲突。请关注语气变化、负面词汇频率以及争议性话题。", talker),
			}),
		},
	), nil
}

func (s *Service) handleMCPRelationshipMilestones(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	talker := request.Params.Arguments["talker"]

	return mcp.NewGetPromptResult(
		"关系里程碑回顾指令",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("请回顾与 %s 的历史聊天记录，找出重要的关系里程碑（如：初次相识、重大合作达成、共同解决的危机等）。", talker),
			}),
		},
	), nil
}
