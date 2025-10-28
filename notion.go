package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const notionRichTextChunkLimit = 1800

type notionClient struct {
	httpClient       *http.Client
	baseURL          string
	version          string
	token            string
	parentType       string
	parentID         string
	titlePropertyKey string
}

type notionPageRequest struct {
	Parent     notionParent              `json:"parent"`
	Properties map[string]notionProperty `json:"properties"`
	Children   []notionBlock             `json:"children,omitempty"`
}

type notionParent struct {
	Type       string `json:"type"`
	DatabaseID string `json:"database_id,omitempty"`
	PageID     string `json:"page_id,omitempty"`
}

type notionProperty struct {
	Title []notionRichText `json:"title"`
}

type notionRichText struct {
	Type        string             `json:"type"`
	PlainText   string             `json:"plain_text"`
	Text        *notionText        `json:"text,omitempty"`
	Annotations *notionAnnotations `json:"annotations,omitempty"`
}

type notionText struct {
	Content string `json:"content"`
}

type notionAnnotations struct {
	Bold   bool `json:"bold,omitempty"`
	Italic bool `json:"italic,omitempty"`
}

type notionBlock struct {
	Object           string           `json:"object"`
	Type             string           `json:"type"`
	Paragraph        *notionParagraph `json:"paragraph,omitempty"`
	Heading3         *notionHeading   `json:"heading_3,omitempty"`
	BulletedListItem *notionParagraph `json:"bulleted_list_item,omitempty"`
	Divider          *struct{}        `json:"divider,omitempty"`
}

type notionParagraph struct {
	RichText []notionRichText `json:"rich_text"`
}

type notionHeading struct {
	RichText []notionRichText `json:"rich_text"`
}

type notionPageResponse struct {
	ID string `json:"id"`
}

type notionErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func newNotionClient(cfg *cliConfig, httpClient *http.Client) (*notionClient, error) {
	token := strings.TrimSpace(cfg.NotionToken)
	if token == "" {
		return nil, fmt.Errorf("缺少 Notion API Key: 请提供 --notion-token 或设置环境变量 %s", notionTokenEnvVar)
	}
	parentID := strings.TrimSpace(cfg.NotionParentID)
	if parentID == "" {
		return nil, fmt.Errorf("缺少 Notion 父级 ID: 请提供 --notion-parent-id 或设置环境变量 %s", notionParentIDEnvVar)
	}
	parentType := strings.ToLower(strings.TrimSpace(cfg.NotionParentType))
	if parentType == "" {
		parentType = "page"
	}
	if parentType != "page" && parentType != "database" {
		return nil, fmt.Errorf("不支持的 Notion 父级类型: %s", parentType)
	}
	titleProperty := strings.TrimSpace(cfg.NotionTitleProperty)
	if titleProperty == "" {
		if parentType == "database" {
			titleProperty = defaultNotionDatabaseTitleProp
		} else {
			titleProperty = defaultNotionPageTitleProp
		}
	}
	baseURL := strings.TrimSpace(cfg.NotionBaseURL)
	if baseURL == "" {
		baseURL = defaultNotionBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	version := strings.TrimSpace(cfg.NotionVersion)
	if version == "" {
		version = defaultNotionVersion
	}

	return &notionClient{
		httpClient:       httpClient,
		baseURL:          baseURL,
		version:          version,
		token:            token,
		parentType:       parentType,
		parentID:         parentID,
		titlePropertyKey: titleProperty,
	}, nil
}

func (c *notionClient) createConversationPage(ctx context.Context, conv exportConversation, loc *time.Location) (string, error) {
	payload := c.buildPageRequest(conv, loc)
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("序列化 Notion 请求失败: %w", err)
	}

	target := c.baseURL + "/v1/pages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("构造 Notion 请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if c.version != "" {
		req.Header.Set("Notion-Version", c.version)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用 Notion 接口失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body := readBodyForLog(resp.Body)
		var apiErr notionErrorResponse
		if err := json.Unmarshal([]byte(body), &apiErr); err == nil && apiErr.Message != "" {
			body = apiErr.Message
		}
		return "", fmt.Errorf("创建 Notion 页面失败: status=%d message=%s", resp.StatusCode, strings.TrimSpace(body))
	}

	var result notionPageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析 Notion 响应失败: %w", err)
	}

	return result.ID, nil
}

func (c *notionClient) buildPageRequest(conv exportConversation, loc *time.Location) notionPageRequest {
	title := strings.TrimSpace(conv.Title)
	if title == "" {
		title = fmt.Sprintf("对话 %s", conv.ID)
	}

	parent := notionParent{Type: c.parentType}
	if c.parentType == "database" {
		parent.DatabaseID = c.parentID
	} else {
		parent.PageID = c.parentID
	}

	properties := map[string]notionProperty{
		c.titlePropertyKey: {Title: []notionRichText{newNotionPlainText(title, nil)}},
	}

	children := make([]notionBlock, 0, len(conv.Messages)*2+4)
	metadata := []string{
		fmt.Sprintf("对话 ID: %s", conv.ID),
		fmt.Sprintf("创建时间: %s", formatTimestamp(conv.CreateTime, loc)),
		fmt.Sprintf("最近更新: %s", formatTimestamp(conv.UpdateTime, loc)),
	}
	for _, line := range metadata {
		children = append(children, newNotionBulletedParagraph(line))
	}
	children = append(children, newNotionDivider())

	for idx, msg := range conv.Messages {
		role := strings.ToUpper(firstNonEmpty(msg.Role, "UNKNOWN"))
		heading := fmt.Sprintf("%d. %s · %s", idx+1, role, formatTimestamp(msg.CreateTime, loc))
		children = append(children, newNotionHeading3(heading))

		annotations := determineAnnotations(msg.Role)
		text := strings.TrimSpace(msg.Text)
		if text == "" {
			text = "(空内容)"
		}
		for _, block := range notionParagraphBlocksFromText(text, annotations) {
			children = append(children, block)
		}
	}

	return notionPageRequest{
		Parent:     parent,
		Properties: properties,
		Children:   children,
	}
}

func determineAnnotations(role string) *notionAnnotations {
	if strings.EqualFold(role, "user") {
		return &notionAnnotations{Bold: true}
	}
	if strings.EqualFold(role, "system") {
		return &notionAnnotations{Italic: true}
	}
	return nil
}

func notionParagraphBlocksFromText(text string, annotations *notionAnnotations) []notionBlock {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	segments := strings.Split(normalized, "\n\n")
	if len(segments) == 0 {
		segments = []string{""}
	}
	blocks := make([]notionBlock, 0, len(segments))
	for _, segment := range segments {
		parts := chunkText(segment, notionRichTextChunkLimit)
		richTexts := make([]notionRichText, 0, len(parts))
		for idx, part := range parts {
			var ann *notionAnnotations
			if idx == 0 {
				ann = annotations
			}
			richTexts = append(richTexts, newNotionPlainText(part, ann))
		}
		if len(richTexts) == 0 {
			richTexts = append(richTexts, newNotionPlainText("", annotations))
		}
		blocks = append(blocks, notionBlock{
			Object: "block",
			Type:   "paragraph",
			Paragraph: &notionParagraph{
				RichText: richTexts,
			},
		})
	}
	return blocks
}

func newNotionPlainText(content string, annotations *notionAnnotations) notionRichText {
	if content == "" {
		content = " "
	}
	return notionRichText{
		Type:        "text",
		PlainText:   content,
		Text:        &notionText{Content: content},
		Annotations: annotations,
	}
}

func newNotionBulletedParagraph(content string) notionBlock {
	return notionBlock{
		Object: "block",
		Type:   "bulleted_list_item",
		BulletedListItem: &notionParagraph{
			RichText: []notionRichText{newNotionPlainText(content, nil)},
		},
	}
}

func newNotionHeading3(content string) notionBlock {
	return notionBlock{
		Object: "block",
		Type:   "heading_3",
		Heading3: &notionHeading{
			RichText: []notionRichText{newNotionPlainText(content, nil)},
		},
	}
}

func newNotionDivider() notionBlock {
	return notionBlock{
		Object:  "block",
		Type:    "divider",
		Divider: &struct{}{},
	}
}

func chunkText(text string, limit int) []string {
	if limit <= 0 {
		return []string{text}
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}
	parts := make([]string, 0, (len(runes)/limit)+1)
	for start := 0; start < len(runes); start += limit {
		end := start + limit
		if end > len(runes) {
			end = len(runes)
		}
		parts = append(parts, string(runes[start:end]))
	}
	return parts
}

func syncConversationsToNotion(ctx context.Context, client *notionClient, conversations []exportConversation, timezone string) (int, []string, error) {
	loc := resolveLocation(timezone)
	var created int
	var pageIDs []string
	for _, conv := range conversations {
		pageID, err := client.createConversationPage(ctx, conv, loc)
		if err != nil {
			return created, pageIDs, fmt.Errorf("对话 %s 创建 Notion 页面失败: %w", conv.ID, err)
		}
		created++
		pageIDs = append(pageIDs, pageID)
		logInfo("Notion 页面创建成功: conversation=%s page=%s", conv.ID, pageID)
	}
	return created, pageIDs, nil
}
