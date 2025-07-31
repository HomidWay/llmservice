package deepseek

func ResourceCallTool() ToolDefinition {
	return ToolDefinition{
		Type: "function",
		Function: ToolFunction{
			Name:        "uri_resource_call",
			Description: "Calls resource at given URI and returns contents of this call's result",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"mcp_id": {Type: "integer", Description: "The ID of the MCP instance to use for this call"},
					"uri":    {Type: "string", Description: "The URI to call the resource"},
				},
				Required: []string{"uri", "mcp_id"},
			},
		},
	}
}
