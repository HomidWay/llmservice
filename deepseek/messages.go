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

type DeepSeekMessage struct {
	ContentString          string              `json:"content,omitempty"`
	RoleString             string              `json:"role"`
	ReasoningContentString *string             `json:"reasoning_content,omitempty"`
	ToolCallsArr           *[]DeepSeekToolCall `json:"tool_calls,omitempty"`
	ToolCallIDString       *string             `json:"tool_call_id,omitempty"`
	StopReasonString       *string
}

// NewRequestMessage creates a new validated message with domain invariants
func NewMessage(role string, content string, toolCalls []llmservice.MessageToolCall) *DeepSeekMessage {

	var tc []DeepSeekToolCall

	if len(toolCalls) > 0 {

		tc = make([]DeepSeekToolCall, len(toolCalls))

		for i, toolCall := range toolCalls {

			response := DeepSeekToolCall{}

			response.Type = "function"
			response.IDString = toolCall.ID()
			response.IndexVal = toolCall.Index()

			response.Function = networkResponseFunction{
				Name:      response.ToolName(),
				Arguments: response.Args(),
			}

			tc[i] = response
		}
	}

	return &DeepSeekMessage{
		ContentString: strings.TrimSpace(content),
		RoleString:    role,

		ToolCallsArr: &tc,
	}
}

func NewToolCallResponse(content string, toolCallID string) *DeepSeekMessage {
	return &DeepSeekMessage{
		ContentString:    content,
		RoleString:       string(SenderRoleTool),
		ToolCallIDString: &toolCallID,
	}
}

func (msg DeepSeekMessage) Role() string {
	return msg.RoleString
}

func (msg DeepSeekMessage) MessageContent() string {
	return msg.ContentString
}

func (msg DeepSeekMessage) ReasoningContent() *string {
	return msg.ReasoningContentString
}

func (msg DeepSeekMessage) ToolCalls() []llmservice.MessageToolCall {

	if msg.ToolCallsArr == nil {
		return nil
	}

	toolCalls := make([]llmservice.MessageToolCall, len(*msg.ToolCallsArr))

	for i, toolCall := range *msg.ToolCallsArr {
		toolCalls[i] = toolCall
	}

	return toolCalls
}

func (msg DeepSeekMessage) ToolCallID() string {
	if msg.ToolCallIDString == nil {
		return ""
	}

	return *msg.ToolCallIDString
}

func (msg DeepSeekMessage) StopReason() string {
	if msg.StopReasonString == nil {
		return "none"
	}

	return *msg.StopReasonString
}

func (msg DeepSeekMessage) TokenCount() int {
	tokenizer, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return 0
	}
	return len(tokenizer.Encode(msg.MessageContent(), nil, nil))
}

func (m DeepSeekMessage) SetContent(newContent string) *DeepSeekMessage {
	msg := m

	msg.ContentString = newContent

	return &msg
}
