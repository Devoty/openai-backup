package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type flexFloat64 float64

func (f *flexFloat64) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		*f = 0
		return nil
	}

	var num float64
	if err := json.Unmarshal(b, &num); err == nil {
		*f = flexFloat64(num)
		return nil
	}

	var str string
	if err := json.Unmarshal(b, &str); err == nil {
		str = strings.TrimSpace(str)
		if str == "" {
			*f = 0
			return nil
		}
		if parsed, err := strconv.ParseFloat(str, 64); err == nil {
			*f = flexFloat64(parsed)
			return nil
		}
		if t, err := time.Parse(time.RFC3339Nano, str); err == nil {
			*f = flexFloat64(float64(t.UnixNano()) / 1e9)
			return nil
		}
		if t, err := time.Parse(time.RFC3339, str); err == nil {
			*f = flexFloat64(float64(t.UnixNano()) / 1e9)
			return nil
		}
		return fmt.Errorf("无法解析字符串时间戳: %s", str)
	}

	return fmt.Errorf("无法解析数值: %s", s)
}

func (f flexFloat64) Float64() float64 {
	return float64(f)
}

type conversationListResponse struct {
	Items   []conversationMeta `json:"items"`
	Total   int                `json:"total"`
	Limit   int                `json:"limit"`
	Offset  int                `json:"offset"`
	HasMore bool               `json:"has_more"`
}

type conversationMeta struct {
	ID         string      `json:"id"`
	Title      string      `json:"title"`
	CreateTime flexFloat64 `json:"create_time"`
	UpdateTime flexFloat64 `json:"update_time"`
}

type conversationDetail struct {
	ID         string                      `json:"id"`
	Title      string                      `json:"title"`
	CreateTime flexFloat64                 `json:"create_time"`
	UpdateTime flexFloat64                 `json:"update_time"`
	Mapping    map[string]conversationNode `json:"mapping"`
}

type conversationNode struct {
	ID       string          `json:"id"`
	Message  *chatMessage    `json:"message"`
	Parent   string          `json:"parent"`
	Children []string        `json:"children"`
	Metadata json.RawMessage `json:"metadata"`
}

type chatMessage struct {
	ID          string          `json:"id"`
	Author      messageAuthor   `json:"author"`
	CreateTime  flexFloat64     `json:"create_time"`
	UpdateTime  flexFloat64     `json:"update_time"`
	Content     messageContent  `json:"content"`
	Metadata    json.RawMessage `json:"metadata"`
	Status      string          `json:"status"`
	EndTurn     *bool           `json:"end_turn"`
	Weight      *float64        `json:"weight"`
	Recipient   string          `json:"recipient"`
	Role        string          `json:"role"`
	Extras      json.RawMessage `json:"extra_metadata"`
	Attachments json.RawMessage `json:"attachments"`
}

type messageMetadata struct {
	ContentReferences []contentReference  `json:"content_references"`
	SearchGroups      []searchResultGroup `json:"search_result_groups"`
	Citations         []citationRef       `json:"citations"`
}

type contentReference struct {
	SafeURLs []string       `json:"safe_urls"`
	Items    []contentEntry `json:"items"`
	Type     string         `json:"type"`
}

type contentEntry struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Snippet     string `json:"snippet"`
	Attribution string `json:"attribution"`
}

type searchResultGroup struct {
	Domain  string        `json:"domain"`
	Entries []searchEntry `json:"entries"`
}

type searchEntry struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Snippet     string `json:"snippet"`
	Attribution string `json:"attribution"`
}

type citationRef struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Attribution string `json:"attribution"`
}

type messageAuthor struct {
	Role string `json:"role"`
	Name string `json:"name"`
}

type messageContent struct {
	ContentType string            `json:"content_type"`
	Parts       []json.RawMessage `json:"parts"`
	Text        string            `json:"text"`
}

type exportMessage struct {
	Role       string
	CreateTime float64
	UpdateTime float64
	Text       string
	References []referenceLink
}

type exportConversation struct {
	ID         string
	Title      string
	CreateTime float64
	UpdateTime float64
	Messages   []exportMessage
}
