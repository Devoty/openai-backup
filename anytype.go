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
)

type anytypeClient struct {
	httpClient *http.Client
	baseURL    string
	version    string
	spaceID    string
	typeKey    string
	token      string
}

type anytypeObjectResponse struct {
	ID string `json:"id"`
}

type anytypeErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type createAnytypeObjectRequest struct {
	Body    string `json:"body,omitempty"`
	Name    string `json:"name,omitempty"`
	TypeKey string `json:"type_key"`
}

func newAnytypeClient(cfg *cliConfig, httpClient *http.Client) (*anytypeClient, error) {
	if cfg.AnytypeToken == "" {
		return nil, fmt.Errorf("缺少 Anytype API Key: 请提供 --anytype-token 或设置环境变量 %s", anytypeTokenEnvVar)
	}
	if cfg.AnytypeSpaceID == "" {
		return nil, fmt.Errorf("缺少 Anytype 空间 ID: 请提供 --anytype-space-id 或设置环境变量 %s", anytypeSpaceIDEnvVar)
	}
	base := strings.TrimRight(cfg.AnytypeBaseURL, "/")
	if base == "" {
		base = defaultAnytypeBaseURL
	}
	return &anytypeClient{
		httpClient: httpClient,
		baseURL:    base,
		version:    cfg.AnytypeVersion,
		spaceID:    cfg.AnytypeSpaceID,
		typeKey:    cfg.AnytypeTypeKey,
		token:      cfg.AnytypeToken,
	}, nil
}

func (c *anytypeClient) createConversationObject(ctx context.Context, conv exportConversation, body string) (string, error) {
	name := strings.TrimSpace(conv.Title)
	if name == "" {
		name = fmt.Sprintf("对话 %s", conv.ID)
	}

	payload := createAnytypeObjectRequest{
		Body:    body,
		Name:    name,
		TypeKey: c.typeKey,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("序列化 Anytype 请求失败: %w", err)
	}

	target := fmt.Sprintf("%s/v1/spaces/%s/objects", c.baseURL, url.PathEscape(c.spaceID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("构造 Anytype 请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if c.version != "" {
		req.Header.Set("Anytype-Version", c.version)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用 Anytype 接口失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		msg := readBodyForLog(resp.Body)
		var apiErr anytypeErrorResponse
		if err := json.Unmarshal([]byte(msg), &apiErr); err == nil && apiErr.Message != "" {
			msg = apiErr.Message
		}
		return "", fmt.Errorf("创建 Anytype 对象失败: status=%d message=%s", resp.StatusCode, strings.TrimSpace(msg))
	}

	var result anytypeObjectResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析 Anytype 响应失败: %w", err)
	}

	return result.ID, nil
}

func syncConversationsToAnytype(ctx context.Context, client *anytypeClient, conversations []exportConversation, timezone string) (int, error) {
	var created int
	for _, conv := range conversations {
		body := renderConversationMarkdown(conv, timezone)
		objectID, err := client.createConversationObject(ctx, conv, body)
		if err != nil {
			return created, fmt.Errorf("对话 %s 创建 Anytype 对象失败: %w", conv.ID, err)
		}
		created++
		logInfo("Anytype 对象创建成功: conversation=%s object=%s", conv.ID, objectID)
	}
	return created, nil
}

func readBodyForLog(r io.Reader) string {
	const limit = 4 << 10
	buf, err := io.ReadAll(io.LimitReader(r, limit))
	if err != nil {
		return fmt.Sprintf("读取响应失败: %v", err)
	}
	return string(buf)
}
