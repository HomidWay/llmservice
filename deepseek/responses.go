package deepseek

import "github.com/TitanLombard/llmservice"

// MARK: - Request structs

type networkRequest struct {
	Model          string                  `json:"model"`
	Messages       []networkRequestMessage `json:"messages"`
	Streamed       *bool                   `json:"stream"`
	ResponseFormat *networkResponseFormat  `json:"response_format"`
	MaxTokens      *int                    `json:"max_tokens"`
	Temperature    *float32                `json:"temperature"`
	TopP           *float32                `json:"top_p"`
	Logprobs       *bool                   `json:"logprobs"`
}

func newNetworkRequest(model string, messages []networkRequestMessage) *networkRequest {
	return &networkRequest{
		Model:    model,
		Messages: messages,
	}
}

type networkRequestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func newNetworkRequestMessage(role, content string) *networkRequestMessage {
	return &networkRequestMessage{
		Role:    role,
		Content: content,
	}
}

type networkResponseFormat struct {
	Type string `json:"type"`
}

func newNetworkResponseFormat(formatType string) *networkResponseFormat {
	return &networkResponseFormat{
		Type: formatType,
	}
}

// MARK: - Response structs

type networkResponse struct {
	Id                string                   `json:"id"`
	Choices           []netoworkResponseChoice `json:"choices"`
	CreatedAt         string                   `json:"created_at"`
	Model             string                   `json:"model"`
	SystemFingerprint string                   `json:"system_fingerprint"`
	Object            string                   `json:"object"`
	Usage             networkResponseUsage     `json:"usage"`
}

type netoworkResponseChoice struct {
	FinishReason string                  `json:"finish_reason"`
	Index        int                     `json:"index"`
	Message      *networkResponceMessage `json:"message"`
	Delta        *networkResponceDelta   `json:"delta"`
}

type networkResponceMessage struct {
	Content          string                   `json:"content"`
	ReasoningContent string                   `json:"reasoning_content"`
	Toolchanin       networkResponseToolchain `json:"toolchain"`
	Role             llmservice.SenderRole    `json:"role"`
}

type networkResponceDelta struct {
	Content string
}

type networkResponseToolchain struct {
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
	CompletionTokensDetails netowrkResponceUsageDetails `json:"completion_tokens_details"`
}

type netowrkResponceUsageDetails struct {
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
