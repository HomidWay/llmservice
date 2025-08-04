package deepseek

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/TitanLombard/llmservice"
	"github.com/TitanLombard/llmservice/internal/helpers"
	"github.com/TitanLombard/logger"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

type MCPConnection struct {
	client *client.Client

	resources string
}

func WithSSEMCP(sseEndpoint string) (MCPConnection, error) {

	sse, err := client.NewSSEMCPClient(sseEndpoint)

	return MCPConnection{client: sse}, err
}

type DeepSeekAIServiceWithMCP struct {
	apikey         string
	ctx            context.Context
	log            logger.Logger
	mcpConnections []MCPConnection
}

func NewDeepSeekServiceWithMCP(apikey string, ctx context.Context, log logger.Logger, mcpConnections ...MCPConnection) (*DeepSeekAIServiceWithMCP, error) {

	if log == nil {
		log = logger.Default(logger.VerbosityInfo, nil)
	}

	var initiatedConnections []MCPConnection

	for _, connection := range mcpConnections {
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

		initiatedConnections = append(initiatedConnections, connection)
	}

	return &DeepSeekAIServiceWithMCP{apikey: apikey, ctx: ctx, log: log, mcpConnections: initiatedConnections}, nil
}

func (ds *DeepSeekAIServiceWithMCP) ServiceTokenLimit() int {
	return 64000
}

func (ds *DeepSeekAIServiceWithMCP) SendMessage(
	messages []llmservice.RequestMessage,
	options ...llmservice.Option,
) (chan string, error) {

	const requestURL string = baseURL + "/chat/completions"

	returnChan := make(chan string)
	dsOptions := DeepSeekOptions{NewDeepSeekChatModel(), nil, nil, nil, nil, nil, nil, nil, nil}

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

	ds.debugPrint(dsOptions)

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

	requestMessages := make([]networkRequestMessage, len(messages))

	for i := range messages {

		messageContent := messages[i].Content()

		if messages[i].Role() == llmservice.SenderRoleSystem && mcpResources != "" {
			messageContent += fmt.Sprintf("\n\n Available MCP resources: %s", mcpResources)
		}

		requestMessages[i] = networkRequestMessage{
			Role:    string(messages[i].Role()),
			Content: messageContent,
		}
	}

	var responseFormat *networkResponseFormat
	if dsOptions.responseFormat != nil {
		responseFormat = &networkResponseFormat{string(*dsOptions.responseFormat)}
	}

	request := networkRequest{
		Model:          string(dsOptions.model.Model()),
		Messages:       requestMessages,
		Streamed:       dsOptions.streamed,
		ResponseFormat: responseFormat,
		MaxTokens:      dsOptions.maxTokens,
		Temperature:    dsOptions.temperature,
		TopP:           dsOptions.topP,
		Logprobs:       dsOptions.logprobs,
		Tools:          dsOptions.tools,
		ToolChoice:     dsOptions.toolChoice,
	}

	go func(initialRequest networkRequest, returnChan chan string) {

		defer close(returnChan)
		request := initialRequest

		for {

			requestBody, err := json.Marshal(request)
			ds.log.Debugf("%s", string(requestBody))

			if err != nil {
				ds.log.Errorf("Error marshalling request: %s", err)
				returnChan <- err.Error()
				break
			}

			ds.log.Debugf("DeepSeek request created")

			req, err := http.NewRequestWithContext(ds.ctx, "POST", requestURL, bytes.NewBuffer(requestBody))
			if err != nil {
				ds.log.Errorf("Error creating HTTP request: %s", err)
				returnChan <- err.Error()
				break
			}

			authHeadervalue := "Bearer " + ds.apikey

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", authHeadervalue)

			client := &http.Client{}

			ds.log.Debugf("DeepSeek request sent")
			resp, err := client.Do(req)
			if err != nil {
				ds.log.Errorf("Error sending HTTP request: %s", err)
				returnChan <- err.Error()
				break
			}

			ds.log.Debugf("DeepSeek responce recieved")

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

				ds.log.Errorf("Error sending HTTP request: %s", dsErr)
				returnChan <- dsErr.Error()
				break
			}

			ds.log.Debugf("DeepSeek response Status: %s\n", resp.Status)

			toolCalls := ds.handleResponse(resp.Body, returnChan)

			if len(toolCalls) == 0 {
				break
			}

			toolCallResults := ds.handleToolCall(toolCalls)

			for _, tool := range toolCalls {
				fmt.Println(tool.Function.Name, tool.Function.Arguments)
			}

			mcpResultMessage, _ := NewMessage(
				llmservice.SenderRoleUser,
				toolCallResults,
			)

			toolResultMessage := networkRequestMessage{
				Role:    string(mcpResultMessage.role),
				Content: mcpResultMessage.content,
			}

			request.Messages = append(request.Messages, toolResultMessage)
		}

	}(request, returnChan)

	return returnChan, nil
}

func (ds *DeepSeekAIServiceWithMCP) handleResponse(readCloser io.ReadCloser, outputChan chan<- string) []networkResponseToolCall {
	scanner := bufio.NewScanner(readCloser)

	toolCallMap := map[int]*networkResponseToolCall{}

	for scanner.Scan() {

		var chunk networkResponse

		data := strings.TrimPrefix(scanner.Text(), "data: ")

		if data == "[DONE]" {
			break
		}

		decoder := json.NewDecoder(strings.NewReader(data))

		err := decoder.Decode(&chunk)
		if err != nil && err != io.EOF {
			ds.log.Errorf("DeepSeek response chunk decode error: %s\n", err.Error())
			ds.log.Debugf("DeepSeek response chunk data: %s\n", data)
			continue
		}

		for _, choice := range chunk.Choices {
			if choice.Delta != nil {
				outputChan <- choice.Delta.Content

				if choice.Delta.FinishReason != nil {
					fmt.Printf("Finish Reason: %s\n", *choice.Delta.FinishReason)
				}

				if choice.Delta.ToolCalls != nil {
					for _, toolCall := range *choice.Delta.ToolCalls {

						var tool *networkResponseToolCall
						var ok bool

						if tool, ok = toolCallMap[toolCall.Index]; !ok {
							tool = &networkResponseToolCall{}
							toolCallMap[toolCall.Index] = tool
						}

						tool.Id = toolCall.Id
						tool.Index = toolCall.Index
						tool.Type = toolCall.Type
						tool.Function.Name += toolCall.Function.Name
						tool.Function.Arguments += toolCall.Function.Arguments
					}
				}
			}

			if choice.Message != nil {

				outputChan <- choice.Message.Content

				if choice.Message.ToolCalls != nil {
					for _, toolCall := range *choice.Delta.ToolCalls {
						var tool *networkResponseToolCall
						var ok bool

						if tool, ok = toolCallMap[toolCall.Index]; !ok {
							tool = &networkResponseToolCall{}
							toolCallMap[toolCall.Index] = tool
						}

						tool.Id = toolCall.Id
						tool.Index = toolCall.Index
						tool.Type = toolCall.Type
						tool.Function.Name += toolCall.Function.Name
						tool.Function.Arguments += toolCall.Function.Arguments
					}
				}
			}
		}
	}

	toolCalls := make([]networkResponseToolCall, len(toolCallMap))
	for i, toolCall := range toolCallMap {
		toolCalls[i] = *toolCall
	}

	return toolCalls
}

func (c *DeepSeekAIServiceWithMCP) handleToolCall(toolCalls []networkResponseToolCall) string {

	result := ""

	for _, toolCall := range toolCalls {
		if toolCall.Function.Name == ResourceCallTool().Function.Name {

			type arguments struct {
				ID  int    `json:"mcp_id"`
				URI string `json:"uri"`
			}
			var args arguments

			err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			if err != nil {
				result += "Failed to unmarshal arguments for resource call tool.\n"
				continue
			}

			var conn MCPConnection

			if len(c.mcpConnections) > args.ID {
				conn = c.mcpConnections[args.ID]
			} else {
				result += fmt.Sprintf("Failed to find MCP instance with ID %d\n", args.ID)
				continue
			}

			mcpResult, err := conn.client.ReadResource(c.ctx, mcp.ReadResourceRequest{
				Params: mcp.ReadResourceParams{
					URI: args.URI,
				},
			})

			if err != nil {
				result += fmt.Sprintf("Failed to fetch resource at URI %s\n", args.URI)
				continue
			}

			content := ""

			for _, item := range mcpResult.Contents {
				switch v := item.(type) {
				case mcp.TextResourceContents:
					content += v.Text
				case mcp.BlobResourceContents:
					content += v.Blob
				default:
					content += fmt.Sprintf("Unknown resource type: %T\n", item)
				}
			}

			result += fmt.Sprintf("Content of resource at URI %s:\n%s\n", args.URI, content)

			continue
		}

		for _, mcpConn := range c.mcpConnections {

			arguments := mcp.CallToolParams{
				Name:      toolCall.Function.Name,
				Arguments: toolCall.Function.Arguments,
			}

			request := mcp.CallToolRequest{
				Params: arguments,
			}

			mcpToolCallResult, err := mcpConn.client.CallTool(context.Background(), request)
			if err != nil {
				continue
			}

			for _, content := range mcpToolCallResult.Content {
				switch v := content.(type) {
				case mcp.TextContent:
					result += fmt.Sprintf("Content of tool call id: %v:\n%s\n", toolCall.Id, v.Text)
				default:
					continue
				}
			}
		}
	}

	return result
}

func (ds DeepSeekAIServiceWithMCP) debugPrint(dsOptions DeepSeekOptions) {
	ds.log.Debugf("Options Details:")
	ds.log.Debugf("- Model: %v", dsOptions.model)
	if dsOptions.responseFormat != nil {
		ds.log.Debugf("- ResponseFormat: %v", *dsOptions.responseFormat)
	} else {
		ds.log.Debugf("- ResponseFormat: nil")
	}
	ds.log.Debugf("- Streamed: %v", dsOptions.streamed)
	ds.log.Debugf("- MaxTokens: %v", dsOptions.maxTokens)
	ds.log.Debugf("- Temperature: %v", dsOptions.temperature)
	ds.log.Debugf("- TopP: %v", dsOptions.topP)
	ds.log.Debugf("- Logprobs: %v", dsOptions.logprobs)
	if dsOptions.tools != nil {
		toolsJson, _ := json.Marshal(dsOptions.tools)
		ds.log.Debugf("- Tools: %s", string(toolsJson))
	} else {
		ds.log.Debugf("- Tools: nil")
	}
	ds.log.Debugf("- ToolChoice: %v", dsOptions.toolChoice)
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
