package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func fetchAllConversations(ctx context.Context, client *http.Client, cfg *cliConfig, token string) ([]conversationMeta, error) {
	// 拉取分页对话列表并拼接完整集合。
	var result []conversationMeta
	offset := cfg.InitialOffset

	for {
		logInfo("请求对话列表 offset=%d limit=%d", offset, cfg.PageSize)
		page, err := fetchConversationPage(ctx, client, cfg, token, offset, cfg.PageSize)
		if err != nil {
			return nil, err
		}

		if len(page.Items) == 0 {
			break
		}

		for _, item := range page.Items {
			result = append(result, item)
			if cfg.MaxConversations > 0 && len(result) >= cfg.MaxConversations {
				return result, nil
			}
		}

		if !page.HasMore {
			logInfo("对话列表已读完, has_more=false")
			break
		}
		nextOffset := offset + cfg.PageSize
		if nextOffset <= offset {
			break
		}
		offset = nextOffset
	}

	return result, nil
}

func fetchConversationPage(ctx context.Context, client *http.Client, cfg *cliConfig, token string, offset, limit int) (*conversationListResponse, error) {
	// 构造列表接口请求。
	endpoint, err := url.Parse(cfg.BaseURL + "/conversations")
	if err != nil {
		return nil, err
	}

	query := endpoint.Query()
	query.Set("offset", fmt.Sprintf("%d", offset))
	query.Set("limit", fmt.Sprintf("%d", limit))
	query.Set("order", cfg.Order)
	if cfg.IncludeArchived {
		query.Set("is_archived", "true")
	} else {
		query.Set("is_archived", "false")
	}
	query.Set("is_starred", "false")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	applyCommonHeaders(req, cfg, token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("请求对话列表失败: %s - %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var parsed conversationListResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("解析对话列表响应失败: %w", err)
	}

	return &parsed, nil
}

func fetchConversationDetail(ctx context.Context, client *http.Client, cfg *cliConfig, token, conversationID string) (*conversationDetail, error) {
	// 请求单个对话的详细消息结构。
	endpoint := fmt.Sprintf("%s/conversation/%s", strings.TrimSuffix(cfg.BaseURL, "/"), url.PathEscape(conversationID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	applyCommonHeaders(req, cfg, token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("请求对话详情失败: %s - %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var parsed conversationDetail
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("解析对话详情响应失败: %w", err)
	}

	return &parsed, nil
}

func applyCommonHeaders(req *http.Request, cfg *cliConfig, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", cfg.UserAgent)
	if cfg.DeviceID != "" {
		req.Header.Set("oai-device-id", cfg.DeviceID)
	}
	if cfg.OaiLanguage != "" {
		req.Header.Set("oai-language", cfg.OaiLanguage)
	}
	if cfg.AcceptLanguage != "" {
		req.Header.Set("Accept-Language", cfg.AcceptLanguage)
	}
	if cfg.Referer != "" {
		req.Header.Set("Referer", cfg.Referer)
	}
	if cfg.Cookie != "" {
		req.Header.Set("Cookie", cfg.Cookie)
	}
	if cfg.Origin != "" {
		req.Header.Set("Origin", cfg.Origin)
	}
	if cfg.SecChUA != "" {
		req.Header.Set("sec-ch-ua", cfg.SecChUA)
	}
	if cfg.SecChUAMobile != "" {
		req.Header.Set("sec-ch-ua-mobile", cfg.SecChUAMobile)
	}
	if cfg.SecChUAPlatform != "" {
		req.Header.Set("sec-ch-ua-platform", cfg.SecChUAPlatform)
	}
	if cfg.SecFetchDest != "" {
		req.Header.Set("sec-fetch-dest", cfg.SecFetchDest)
	}
	if cfg.SecFetchMode != "" {
		req.Header.Set("sec-fetch-mode", cfg.SecFetchMode)
	}
	if cfg.SecFetchSite != "" {
		req.Header.Set("sec-fetch-site", cfg.SecFetchSite)
	}
	if cfg.ChatGPTAccountID != "" {
		req.Header.Set("chatgpt-account-id", cfg.ChatGPTAccountID)
	}
	if cfg.OAIClientVersion != "" {
		req.Header.Set("oai-client-version", cfg.OAIClientVersion)
	}
	if cfg.Priority != "" {
		req.Header.Set("priority", cfg.Priority)
	}
}

func deleteConversation(ctx context.Context, client *http.Client, cfg *cliConfig, token, conversationID string) error {
	if strings.TrimSpace(conversationID) == "" {
		return errors.New("缺少对话 ID")
	}

	endpoint := fmt.Sprintf("%s/conversation/%s", strings.TrimSuffix(cfg.BaseURL, "/"), url.PathEscape(conversationID))
	payload := map[string]any{
		"is_visible": false,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("构造删除请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	applyCommonHeaders(req, cfg, token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("删除对话失败: %s - %s", resp.Status, strings.TrimSpace(string(body)))
	}

	return nil
}
