package deepseek

type ResourceCall struct {
	McpIndex int    `json:"mcp_index"`
	URI      string `json:"uri"`
}

func ResourceCallTool() DeepSeekToolDefinition {
	return DeepSeekToolDefinition{
		Type: "function",
		Function: DeepSeekToolFunction{
			Name:        "uri_resourse_call",
			Description: "Calls resource at given URI and returns contents of this call's result",
			Parameters: DeepSeekToolParameters{
				Type: "object",
				Properties: map[string]DeepSeekToolProperty{
					"mcp_id": {Type: "integer", Description: "The ID of the MCP instance to use for this call"},
					"uri":    {Type: "string", Description: "The URI to call the resource"},
				},
				Required: []string{"uri", "mcp_id"},
			},
		},
	}
}
