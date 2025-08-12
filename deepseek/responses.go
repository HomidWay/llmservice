package deepseek

// MARK: - Request structs

type DeepSeekCompletionCall struct {
	Model          string                    `json:"model"`
	Messages       []DeepSeekMessage         `json:"messages"`
	Streamed       *bool                     `json:"stream,omitempty"`
	ResponseFormat *DeepSeekResponseFormat   `json:"response_format,omitempty"`
	MaxTokens      *int                      `json:"max_tokens,omitempty"`
	Temperature    *float32                  `json:"temperature,omitempty"`
	TopP           *float32                  `json:"top_p,omitempty"`
	Logprobs       *bool                     `json:"logprobs,omitempty"`
	Tools          *[]DeepSeekToolDefinition `json:"tools,omitempty"`
	ToolChoice     *string                   `json:"tool_choice,omitempty"`
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
	Message      *DeepSeekMessage        `json:"message,omitempty"`
	Delta        *DeepSeekMessage        `json:"delta,omitempty"`
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

type DeepSeekToolCall struct {
	IndexVal int                     `json:"index"`
	IDString string                  `json:"id"`
	Type     string                  `json:"type"`
	Function networkResponseFunction `json:"function"`
}

// Args implements llmservice.MessageToolCall.
func (d DeepSeekToolCall) Args() string {
	return d.Function.Arguments
}

// ToolName implements llmservice.MessageToolCall.
func (d DeepSeekToolCall) ToolName() string {
	return d.Function.Name
}

// ID implements llmservice.MessageToolCall.
func (d DeepSeekToolCall) ID() string {
	return d.IDString
}

func (d DeepSeekToolCall) Index() int {
	return d.IndexVal
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
