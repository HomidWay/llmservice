package deepseek

import (
	"errors"
	"strings"

	"github.com/TitanLombard/llmservice"
	"github.com/pkoukk/tiktoken-go"
)

const (
	SenderRoleTool llmservice.SenderRole = "tool"
)

var (
	ErrEmptyContent = errors.New("message content cannot be empty")
	ErrInvalidRole  = errors.New("invalid message role")
)

// RequestMessage represents a domain concept of a message in an AI conversation
type RequestMessage struct {
	role      string
	content   string
	toolCalls []llmservice.MessageToolCall
}

// NewRequestMessage creates a new validated message with domain invariants
func NewMessage(role string, content string, toolCalls []llmservice.MessageToolCall) *RequestMessage {
	return &RequestMessage{
		role:      role,
		content:   strings.TrimSpace(content),
		toolCalls: toolCalls,
	}
}

// Role returns the message role (immutable)
func (m RequestMessage) Role() string {
	return m.role
}

// Content returns the message content (immutable)
func (m RequestMessage) Content() string {
	return m.content
}

func (m RequestMessage) ToolCalls() []llmservice.MessageToolCall {
	return m.toolCalls
}

// TokenCount estimates the token count for this message
func (m RequestMessage) TokenCount() int {
	tokenizer, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return 0
	}
	return len(tokenizer.Encode(m.Content(), nil, nil))
}

func (m RequestMessage) SetContent(newContent string) *RequestMessage {
	return NewMessage(m.role, newContent, m.toolCalls)
}
