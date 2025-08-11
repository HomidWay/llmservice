package deepseek

import (
	"github.com/TitanLombard/llmservice"
	"github.com/pkoukk/tiktoken-go"
)

// MARK: - Request structs

type DeepSeekCompletionCall struct {
	Model          string                    `json:"model"`
	Messages       []deepSeekRequestMessage  `json:"messages"`
	Streamed       *bool                     `json:"stream,omitempty"`
	ResponseFormat *DeepSeekResponseFormat   `json:"response_format,omitempty"`
	MaxTokens      *int                      `json:"max_tokens,omitempty"`
	Temperature    *float32                  `json:"temperature,omitempty"`
	TopP           *float32                  `json:"top_p,omitempty"`
	Logprobs       *bool                     `json:"logprobs,omitempty"`
	Tools          *[]DeepSeekToolDefinition `json:"tools,omitempty"`
	ToolChoice     *string                   `json:"tool_choice,omitempty"`
}

type deepSeekRequestMessage struct {
	ContentString          string                      `json:"content,omitempty"`
	ReasoningContentString *string                     `json:"reasoning_content,omitempty"`
	ToolCallsArr           *[]DeepSeekToolCallResponse `json:"tool_calls,omitempty"`
	RoleString             string                      `json:"role"`
}

func (msg deepSeekRequestMessage) Role() string {
	return msg.RoleString
}

func (msg deepSeekRequestMessage) MessageContent() string {
	return msg.ContentString
}

func (msg deepSeekRequestMessage) ReasoningContent() *string {
	return msg.ReasoningContentString
}

func (msg deepSeekRequestMessage) ToolCalls() []llmservice.MessageToolCall {

	if msg.ToolCallsArr == nil {
		return nil
	}

	toolCalls := make([]llmservice.MessageToolCall, len(*msg.ToolCallsArr))

	for i, toolCall := range *msg.ToolCallsArr {
		toolCalls[i] = toolCall
	}

	return toolCalls
}

func (msg deepSeekRequestMessage) TokenCount() int {
	tokenizer, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return 0
	}
	return len(tokenizer.Encode(msg.MessageContent(), nil, nil))
}

type DeepSeekResponseFormat struct {
	Type string `json:"type"`
}

type DeepSeekToolDefinition struct {
	Type     string               `json:"type"`
	Function DeepSeekToolFunction `json:"function"`
}

type DeepSeekToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  DeepSeekToolParameters `json:"parameters"`
}

type DeepSeekToolParameters struct {
	Type       string                          `json:"type"`
	Properties map[string]DeepSeekToolProperty `json:"properties"`
	Required   []string                        `json:"required,omitempty"`
}

type DeepSeekToolProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// MARK: - Response structs

type networkResponse struct {
	ID                string                `json:"id"`
	Object            string                `json:"object"`
	Created           int                   `json:"created"`
	Model             string                `json:"model"`
	Choices           []deepSeekChoice      `json:"choices"`
	Usage             *networkResponseUsage `json:"usage,omitempty"`
	SystemFingerprint string                `json:"system_fingerprint"`
}

type deepSeekChoice struct {
	Index        int                     `json:"index"`
	Message      *deepSeekRequestMessage `json:"message,omitempty"`
	Delta        *deepSeekRequestMessage `json:"delta,omitempty"`
	FinishReason *string                 `json:"finish_reason"`
	Logprobes    networkResponseLogprobs `json:"logprobes,omitempty"`
}

type networkResponseLogprobs struct {
	Content networkResponseLogprobsContent `json:"content"`
}

type networkResponseLogprobsContent struct {
	networkResponseLogprobe
	TopLogprobs []networkResponseLogprobe `json:"top_logprobs"`
}

type networkResponseLogprobe struct {
	Token string  `json:"token"`
	Probs float32 `json:"logprob"`
	Bytes []int   `json:"bytes"`
}

type DeepSeekToolCallResponse struct {
	IndexVal int                     `json:"index"`
	IDString string                  `json:"id"`
	Type     string                  `json:"type"`
	Function networkResponseFunction `json:"function"`
}

// Args implements llmservice.MessageToolCall.
func (d DeepSeekToolCallResponse) Args() string {
	return d.Function.Arguments
}

// ToolName implements llmservice.MessageToolCall.
func (d DeepSeekToolCallResponse) ToolName() string {
	return d.Function.Name
}

// ID implements llmservice.MessageToolCall.
func (d DeepSeekToolCallResponse) ID() string {
	return d.IDString
}

func (d DeepSeekToolCallResponse) Index() int {
	return d.IndexVal
}

// ToolCall implements llmservice.MessageToolCall.
func (d DeepSeekToolCallResponse) ToolCall() map[string]interface{} {

	toolCalls := make(map[string]interface{})

	toolCalls[d.Function.Name] = d.Function.Arguments
	return toolCalls
}

type networkResponseFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type networkResponseUsage struct {
	CompletionTokens        int                         `json:"completion_tokens"`
	PromptTokens            int                         `json:"prompt_tokens"`
	PromptCacheHitTokens    int                         `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens   int                         `json:"prompt_cache_miss_tokens"`
	TotalTokens             int                         `json:"total_tokens"`
	CompletionTokensDetails netowrkResponseUsageDetails `json:"completion_tokens_details"`
}

type netowrkResponseUsageDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

type networkResponseError struct {
	Error networkResponseErrorObject `json:"error"`
}

type networkResponseErrorObject struct {
	Message   string `json:"message"`
	Type      string `json:"type"`
	Parameter string `json:"param"`
	Code      int    `json:"code"`
}
