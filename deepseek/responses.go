package deepseek

import "github.com/TitanLombard/llmservice"

// MARK: - Request structs

type networkRequest struct {
	Model          string                  `json:"model"`
	Messages       []networkRequestMessage `json:"messages"`
	Streamed       *bool                   `json:"stream,omitempty"`
	ResponseFormat *networkResponseFormat  `json:"response_format,omitempty"`
	MaxTokens      *int                    `json:"max_tokens,omitempty"`
	Temperature    *float32                `json:"temperature,omitempty"`
	TopP           *float32                `json:"top_p,omitempty"`
	Logprobs       *bool                   `json:"logprobs,omitempty"`
	Tools          *[]ToolDefinition       `json:"tools,omitempty"`
	ToolChoice     *string                 `json:"tool_choice,omitempty"`
}

type networkRequestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type networkResponseFormat struct {
	Type string `json:"type"`
}

type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  ToolParameters `json:"parameters"`
}

type ToolParameters struct {
	Type       string                  `json:"type"`
	Properties map[string]ToolProperty `json:"properties"`
	Required   []string                `json:"required,omitempty"`
}

type ToolProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// MARK: - Response structs

type networkResponse struct {
	ID                string                  `json:"id"`
	Object            string                  `json:"object"`
	Created           int                     `json:"created"`
	Model             string                  `json:"model"`
	Choices           []networkResponseChoice `json:"choices"`
	Usage             *networkResponseUsage   `json:"usage,omitempty"`
	SystemFingerprint string                  `json:"system_fingerprint"`
}

type networkResponseChoice struct {
	Index        int                      `json:"index"`
	Message      *networkResponseMessage  `json:"message,omitempty"`
	Delta        *networkResponseMessage  `json:"delta,omitempty"`
	FinishReason *string                  `json:"finish_reason,omitempty"`
	Logprobs     *networkResponseLogprobs `json:"logprobs"`
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

type networkResponseMessage struct {
	Content          string                     `json:"content"`
	ReasoningContent *string                    `json:"reasoning_content,omitempty"`
	FinishReason     *string                    `json:"finish_reason,omitempty"`
	ToolCalls        *[]networkResponseToolCall `json:"tool_calls,omitempty"`
	Role             llmservice.SenderRole      `json:"role"`
}

type networkResponseToolCall struct {
	Index    int                     `json:"index"`
	Id       string                  `json:"id"`
	Type     string                  `json:"type"`
	Function networkResponseFunction `json:"function"`
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
