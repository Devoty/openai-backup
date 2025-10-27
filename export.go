package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

func buildExportConversation(meta conversationMeta, detail *conversationDetail) exportConversation {
	// 将接口返回的 mapping 规整为 Markdown 友好的结构。
	export := exportConversation{
		ID:         firstNonEmpty(detail.ID, meta.ID),
		Title:      firstNonEmpty(detail.Title, meta.Title),
		CreateTime: chooseTime(detail.CreateTime.Float64(), meta.CreateTime.Float64()),
		UpdateTime: chooseTime(detail.UpdateTime.Float64(), meta.UpdateTime.Float64()),
	}

	for _, node := range detail.Mapping {
		if node.Message == nil {
			continue
		}
		msg := node.Message
		text := renderMessageContent(msg.Content)
		role := chooseRole(msg)
		normalized := normalizeContent(text)
		if normalized == "" || strings.TrimSpace(normalized) == "\"\"" {
			if strings.EqualFold(role, "system") || strings.EqualFold(role, "assistant") {
				logInfo("过滤空SYSTEM消息 node=%s", node.ID)
			}
			continue
		}
		export.Messages = append(export.Messages, exportMessage{
			Role:       role,
			CreateTime: msg.CreateTime.Float64(),
			UpdateTime: msg.UpdateTime.Float64(),
			Text:       normalized,
		})
	}

	sort.Slice(export.Messages, func(i, j int) bool {
		a := export.Messages[i].CreateTime
		b := export.Messages[j].CreateTime
		if a == 0 || b == 0 {
			return export.Messages[i].Text < export.Messages[j].Text
		}
		return a < b
	})

	return export
}

func renderConversationMarkdown(conv exportConversation, timezone string) string {
	// 拼装单个对话的 Markdown 内容，便于写入 Anytype。
	var b strings.Builder

	loc := resolveLocation(timezone)
	title := conv.Title
	if title == "" {
		title = "(未命名对话)"
	}

	b.WriteString(fmt.Sprintf("# %s\n\n", escapeMarkdownHeading(title)))
	b.WriteString(fmt.Sprintf("- 对话ID: `%s`\n", conv.ID))
	b.WriteString(fmt.Sprintf("- 创建时间: %s\n", formatTimestamp(conv.CreateTime, loc)))
	b.WriteString(fmt.Sprintf("- 最近更新: %s\n\n", formatTimestamp(conv.UpdateTime, loc)))

	for idx, msg := range conv.Messages {
		label := strings.ToUpper(msg.Role)
		if label == "" {
			label = "UNKNOWN"
		}
		b.WriteString(fmt.Sprintf("## %d. %s · %s\n\n", idx+1, label, formatTimestamp(msg.CreateTime, loc)))
		b.WriteString(blockquote(msg.Role, msg.Text))
		b.WriteString("\n")
	}

	return b.String()
}

func renderMessageContent(content messageContent) string {
	// 将 message.content.parts 解析为纯文本输出。
	var segments []string

	if trimmed := strings.TrimSpace(content.Text); trimmed != "" {
		segments = append(segments, trimmed)
	}

	for _, raw := range content.Parts {
		var str string
		if err := json.Unmarshal(raw, &str); err == nil {
			str = strings.TrimSpace(str)
			if str != "" {
				segments = append(segments, str)
				continue
			}
		}

		var withText struct {
			Text string `json:"text"`
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &withText); err == nil {
			text := strings.TrimSpace(withText.Text)
			if text != "" {
				segments = append(segments, text)
				continue
			}
		}

		rawText := strings.TrimSpace(string(raw))
		if rawText != "" && rawText != "null" {
			segments = append(segments, rawText)
		}
	}

	return strings.TrimSpace(strings.Join(segments, "\n\n"))
}

func chooseRole(msg *chatMessage) string {
	if msg.Author.Role != "" {
		return msg.Author.Role
	}
	if msg.Role != "" {
		return msg.Role
	}
	return "unknown"
}

func blockquote(role, text string) string {
	isUser := strings.EqualFold(role, "user")
	if text == "" {
		if isUser {
			return "> (空内容)\n"
		}
		return "(空内容)\n"
	}

	if !isUser {
		return text + "\n"
	}

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = "> " + line
		if line == "" {
			lines[i] = ">"
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func formatTimestamp(value float64, loc *time.Location) string {
	if value <= 0 {
		return "-"
	}
	sec := int64(value)
	nsec := int64((value - float64(sec)) * 1e9)
	t := time.Unix(sec, nsec).In(loc)
	return t.Format("2006-01-02 15:04:05 MST")
}

func resolveLocation(name string) *time.Location {
	// 解析时区字符串。
	switch strings.ToLower(name) {
	case "utc":
		return time.UTC
	case "local", "":
		return time.Local
	default:
		loc, err := time.LoadLocation(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "警告: 未能识别时区 %q, 使用本地时区\n", name)
			return time.Local
		}
		return loc
	}
}

func normalizeContent(input string) string {
	if input == "" {
		return ""
	}
	clean := strings.TrimSpace(input)
	if clean == "" {
		return ""
	}
	clean = strings.ReplaceAll(clean, "\u200B", "")
	clean = strings.ReplaceAll(clean, "\uFEFF", "")
	clean = strings.TrimSpace(clean)
	return clean
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func chooseTime(values ...float64) float64 {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func escapeMarkdownHeading(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return trimmed
	}
	trimmed = strings.ReplaceAll(trimmed, "\n", " ")
	return trimmed
}
