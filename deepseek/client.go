package deepseek

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/HomidWay/llmservice"
	"github.com/HomidWay/llmservice/internal/helpers"
	"github.com/mark3labs/mcp-go/mcp"
)

const baseURL = "https://api.deepseek.com/"

type DeepSeekAIService struct {
	apikey         string
	ctx            context.Context
	mcpConnections []MCPConnection
}

func NewDeepSeekService(apikey string, ctx context.Context) *DeepSeekAIService {
	return &DeepSeekAIService{apikey: apikey, ctx: ctx}
}

func NewDeepSeekServiceWithMCP(apikey string, ctx context.Context, mcpConnections ...MCPConnection) (*DeepSeekAIService, error) {

	var initiatedConnections []MCPConnection

	for i, connection := range mcpConnections {
		err := connection.client.Start(ctx)
		if err != nil {
			return nil, err
		}

		_, err = connection.client.Initialize(ctx, mcp.InitializeRequest{})
		if err != nil {
			return nil, err
		}

		connection.resources, err = extractResources(connection)
		if err != nil {
			return nil, err
		}

		connection.tools, err = extractTools(connection, i)
		if err != nil {
			return nil, err
		}

		initiatedConnections = append(initiatedConnections, connection)
	}

	return &DeepSeekAIService{apikey: apikey, ctx: ctx, mcpConnections: initiatedConnections}, nil
}

func (ds *DeepSeekAIService) ServiceTokenLimit() int {
	return 64000
}

func (ds *DeepSeekAIService) SendMessage(
	messages []llmservice.LLMMessage,
	options ...llmservice.Option,
) (chan llmservice.LLMMessage, error) {

	const requestURL string = baseURL + "/chat/completions"

	returnChan := make(chan llmservice.LLMMessage)
	dsOptions := DeepSeekOptions{NewDeepSeekV4FlashModel(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil}

	for _, option := range options {
		dsOption, ok := option.(DeepSeekOption)
		if ok && isValidOptionType(dsOption) {
			if err := dsOption.Apply(&dsOptions); err != nil {
				return returnChan, err
			}
		} else {
			return returnChan, helpers.NewInvalidOptionError((*DeepSeekOption)(nil), option)
		}
	}

	if len(messages) == 0 {
		return returnChan, deepSeekNoMessagesError{}
	}

	mcpResources := ""

	for i, conn := range ds.mcpConnections {

		if i > 0 {
			mcpResources += "\n"
		}

		mcpResources += fmt.Sprintf("MCP[%d] Resources: %s", i, conn.resources)
	}

	requestMessages := make([]DeepSeekMessage, len(messages))

	for i := range messages {

		switch messages[i].Role() {
		case string(llmservice.SenderRoleSystem):
			messageContent := messages[i].MessageContent()

			if messages[i].Role() == string(llmservice.SenderRoleSystem) && mcpResources != "" {
				messageContent += fmt.Sprintf("\n\n Available MCP resources: %s", mcpResources)
			}

			requestMessages[i] = *NewMessage(messages[i].Role(), messageContent, messages[i].ToolCalls())
		case string(SenderRoleTool):
			requestMessages[i] = *NewToolCallResponse(messages[i].MessageContent(), messages[i].ToolCallID())
		default:
			requestMessages[i] = *NewMessage(messages[i].Role(), messages[i].MessageContent(), messages[i].ToolCalls())
		}
	}

	var responseFormat *DeepSeekResponseFormat
	if dsOptions.responseFormat != nil {
		responseFormat = &DeepSeekResponseFormat{string(*dsOptions.responseFormat)}
	}

	// Build MCP tool definitions
	var allTools []DeepSeekToolDefinition
	if dsOptions.tools != nil {
		allTools = append(allTools, *dsOptions.tools...)
	}
	for _, conn := range ds.mcpConnections {
		allTools = append(allTools, conn.tools...)
	}

	var toolsPtr *[]DeepSeekToolDefinition
	if len(allTools) > 0 {
		toolsPtr = &allTools
	}

	request := DeepSeekCompletionCall{
		Model:           string(dsOptions.model.Model()),
		Messages:        requestMessages,
		Streamed:        dsOptions.streamed,
		ResponseFormat:  responseFormat,
		MaxTokens:       dsOptions.maxTokens,
		Temperature:     dsOptions.temperature,
		TopP:            dsOptions.topP,
		Logprobs:        dsOptions.logprobs,
		Tools:           toolsPtr,
		ToolChoice:      dsOptions.toolChoice,
		Thinking:        dsOptions.thinking,
		ReasoningEffort: dsOptions.reasoningEffort,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return returnChan, fmt.Errorf("error marshalling request: %s", err)
	}

	req, err := http.NewRequestWithContext(ds.ctx, "POST", requestURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return returnChan, fmt.Errorf("Error creating HTTP request: %s", err)
	}

	authHeadervalue := "Bearer " + ds.apikey

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeadervalue)

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return returnChan, fmt.Errorf("Error sending HTTP request: %s", err)
	}

	if resp.StatusCode != http.StatusOK {
		var dsErr deepSeekRequestError

		dsErr.Code = resp.StatusCode
		dsErr.Message = fmt.Sprintf("DeepSeek returned error: %s", resp.Status)

		var responseErr networkResponseError
		rawBody, _ := io.ReadAll(resp.Body)
		err := json.Unmarshal(rawBody, &responseErr)
		if err == nil {
			dsErr.ErrorBody = &responseErr
		}

		return returnChan, fmt.Errorf("Error sending HTTP request: %s", dsErr)
	}

	go func(readCloser io.ReadCloser, outputChan chan<- llmservice.LLMMessage) {
		ds.handleResponse(readCloser, outputChan)
	}(resp.Body, returnChan)

	return returnChan, nil
}

func (ds *DeepSeekAIService) HandleToolCall(toolCalls []llmservice.MessageToolCall) ([]llmservice.LLMMessage, error) {

	messages := make([]llmservice.LLMMessage, len(toolCalls))

	for i, toolCall := range toolCalls {
		// Route MCP tool calls (prefixed with mcp_N_)
		if name, mcpIndex, ok := parseMCPToolName(toolCall.ToolName()); ok {
			if mcpIndex >= len(ds.mcpConnections) {
				messages[i] = NewToolCallResponse(
					fmt.Sprintf("invalid MCP index %d, only %d connections available", mcpIndex, len(ds.mcpConnections)),
					toolCall.ID(),
				)
				continue
			}

			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Args()), &args); err != nil {
				messages[i] = NewToolCallResponse(
					fmt.Sprintf("invalid MCP tool arguments: %s", err.Error()),
					toolCall.ID(),
				)
				continue
			}

			result, err := ds.mcpConnections[mcpIndex].client.CallTool(ds.ctx, mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      name,
					Arguments: args,
				},
			})
			if err != nil {
				messages[i] = NewToolCallResponse(
					fmt.Sprintf("MCP tool call failed: %s", err.Error()),
					toolCall.ID(),
				)
				continue
			}

			if result.IsError {
				textData := extractTextFromContent(result.Content)
				messages[i] = NewToolCallResponse(
					fmt.Sprintf("MCP tool returned error: %s", textData),
					toolCall.ID(),
				)
				continue
			}

			messages[i] = NewToolCallResponse(
				extractTextFromContent(result.Content),
				toolCall.ID(),
			)
			continue
		}

		if toolCall.ToolName() == ResourceCallTool().Function.Name {

			var resourceCall ResourceCall
			err := json.Unmarshal([]byte(toolCall.Args()), &resourceCall)
			if err != nil {
				messages[i] = NewToolCallResponse(
					fmt.Sprintf("Incorrect argument json. Expected {mcp_id: <Int>, uri: <String>}; Got: %s, err: %s", toolCall.Args(), err.Error()),
					toolCall.ID(),
				)
				continue
			}

			if len(ds.mcpConnections)-1 < resourceCall.McpIndex {
				return nil, fmt.Errorf("invalid mcp_index. MCP connections count is %d; Got: %d", len(ds.mcpConnections), resourceCall.McpIndex)
			}

			resourceReadReq := mcp.ReadResourceRequest{
				Params: mcp.ReadResourceParams{
					URI: resourceCall.URI,
				},
			}

			result, err := ds.mcpConnections[resourceCall.McpIndex].client.ReadResource(ds.ctx, resourceReadReq)
			if err != nil {
				messages[i] = NewToolCallResponse(
					fmt.Sprintf("MCP Resource read failed with error: %s", err.Error()),
					toolCall.ID(),
				)
				continue
			}

			textData := ""

			for _, chunk := range result.Contents {
				switch v := chunk.(type) {
				case mcp.TextResourceContents:
					textData += v.Text
				case mcp.BlobResourceContents:
					textData += v.Blob
				}
			}

			messages[i] = NewToolCallResponse(
				textData,
				toolCall.ID(),
			)

			continue
		} else {
			messages[i] = NewToolCallResponse(fmt.Sprintf("Unsupported tool: %s", toolCall.ToolName()), toolCall.ID())
		}
	}

	return messages, nil
}

func (ds *DeepSeekAIService) handleResponse(readCloser io.ReadCloser, outputChan chan<- llmservice.LLMMessage) error {
	defer close(outputChan)
	scanner := bufio.NewScanner(readCloser)

	for scanner.Scan() {

		bodyText := scanner.Text()

		var chunk networkResponse

		data := strings.TrimPrefix(bodyText, "data: ")

		if data == "[DONE]" {
			break
		}

		decoder := json.NewDecoder(strings.NewReader(data))

		err := decoder.Decode(&chunk)
		if err != nil && err != io.EOF {
			return fmt.Errorf("deepseek response chunk decode error: %s", err.Error())
		}

		for _, choice := range chunk.Choices {

			if choice.Delta == nil && choice.Message == nil {
				return fmt.Errorf("no messages found in the request")
			}

			if choice.Delta != nil {
				delta := choice.Delta

				delta.StopReasonString = choice.FinishReason

				outputChan <- delta
			}

			if choice.Message != nil {
				message := choice.Message
				message.StopReasonString = choice.FinishReason

				outputChan <- message
			}
		}
	}

	return nil
}

func extractResources(mcpConnection MCPConnection) (string, error) {

	stringBuilder := strings.Builder{}

	resources, err := mcpConnection.client.ListResources(context.Background(), mcp.ListResourcesRequest{})
	if err != nil {
		return "", fmt.Errorf("failed to list resources: %s", err.Error())
	}

	resourceTemplates, err := mcpConnection.client.ListResourceTemplates(context.Background(), mcp.ListResourceTemplatesRequest{})
	if err != nil {
		return "", fmt.Errorf("failed to list resource templates: %s", err.Error())
	}

	if len(resources.Resources) > 0 {
		stringBuilder.WriteString("Resources:\n")
		for i, resource := range resources.Resources {
			if i > 0 {
				stringBuilder.WriteString("\n")
			}
			jsonString, err := json.Marshal(resource)
			if err != nil {
				return "", fmt.Errorf("failed to marshal resource: %s", err.Error())
			}

			stringBuilder.WriteString(string(jsonString))
		}
	}

	if len(resourceTemplates.ResourceTemplates) > 0 {
		stringBuilder.WriteString("ResourceTemplates:\n")
		for i, networkResponse := range resourceTemplates.ResourceTemplates {
			if i > 0 {
				stringBuilder.WriteString("\n")
			}
			jsonString, err := json.Marshal(networkResponse)
			if err != nil {
				return "", fmt.Errorf("failed to marshal resource template: %s", err.Error())
			}

			stringBuilder.WriteString(string(jsonString))
		}
	}

	return stringBuilder.String(), nil
}

// extractTools lists tools from an MCP connection and converts them to DeepSeek tool definitions.
// Each tool name is prefixed with "mcp_{index}_" to enable routing back to the correct connection.
func extractTools(mcpConnection MCPConnection, index int) ([]DeepSeekToolDefinition, error) {
	tools, err := mcpConnection.client.ListTools(context.Background(), mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %s", err.Error())
	}

	defs := make([]DeepSeekToolDefinition, len(tools.Tools))
	for i, tool := range tools.Tools {
		props := make(map[string]DeepSeekToolProperty, len(tool.InputSchema.Properties))
		for key, val := range tool.InputSchema.Properties {
			if propMap, ok := val.(map[string]interface{}); ok {
				prop := DeepSeekToolProperty{}
				if t, ok := propMap["type"].(string); ok {
					prop.Type = t
				}
				if d, ok := propMap["description"].(string); ok {
					prop.Description = d
				}
				props[key] = prop
			}
		}

		defs[i] = DeepSeekToolDefinition{
			Type: "function",
			Function: DeepSeekToolFunction{
				Name:        fmt.Sprintf("mcp_%d_%s", index, tool.Name),
				Description: tool.Description,
				Parameters: DeepSeekToolParameters{
					Type:       tool.InputSchema.Type,
					Properties: props,
					Required:   tool.InputSchema.Required,
				},
			},
		}
	}

	return defs, nil
}

// parseMCPToolName extracts the original tool name and connection index from an MCP-prefixed tool name.
// Expected format: mcp_{index}_{toolName}
func parseMCPToolName(fullName string) (toolName string, mcpIndex int, ok bool) {
	const prefix = "mcp_"
	if !strings.HasPrefix(fullName, prefix) {
		return "", 0, false
	}

	rest := fullName[len(prefix):]
	underscoreIdx := strings.IndexByte(rest, '_')
	if underscoreIdx < 0 {
		return "", 0, false
	}

	index, err := strconv.Atoi(rest[:underscoreIdx])
	if err != nil {
		return "", 0, false
	}

	return rest[underscoreIdx+1:], index, true
}

// extractTextFromContent reads text from MCP content items.
func extractTextFromContent(contents []mcp.Content) string {
	var textData string
	for _, content := range contents {
		if textContent, ok := mcp.AsTextContent(content); ok {
			if textData != "" {
				textData += "\n"
			}
			textData += textContent.Text
		}
	}
	return textData
}
