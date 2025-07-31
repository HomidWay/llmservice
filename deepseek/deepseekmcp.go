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

func NewDeepSeekServiceWithMCP(apikey string, ctx context.Context, log logger.Logger, mcpConnections ...MCPConnection) *DeepSeekAIServiceWithMCP {

	if log == nil {
		log = logger.Default(logger.VerbosityInfo, nil)
	}

	for _, connection := range mcpConnections {
		err := connection.client.Start(ctx)
		if err != nil {
			continue
		}

		_, err = connection.client.Initialize(ctx, mcp.InitializeRequest{})
		if err != nil {
			continue
		}
	}

	return &DeepSeekAIServiceWithMCP{apikey: apikey, ctx: ctx, log: log, mcpConnections: mcpConnections}

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

	resources, err := extractResources(ds.mcpConnections)
	if err != nil {
		return returnChan, fmt.Errorf("failed to extract resources from the request: %w", err)
	}

	requestMessages := make([]networkRequestMessage, len(messages))

	for i := range messages {

		messageContent := messages[i].Content()

		if messages[i].Role() == llmservice.SenderRoleSystem {
			messageContent += fmt.Sprintf("\n\n Available MCP resources: %s", resources)
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

	requestBody, err := json.Marshal(request)

	ds.log.Debugf("%s", requestBody)

	if err != nil {
		return returnChan, err
	}

	ds.log.Debugf("DeepSeek request created")

	req, err := http.NewRequestWithContext(ds.ctx, "POST", requestURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return returnChan, err
	}

	authHeadervalue := "Bearer " + ds.apikey

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeadervalue)

	client := &http.Client{}

	ds.log.Debugf("DeepSeek request sent")
	resp, err := client.Do(req)
	if err != nil {
		return returnChan, err
	}

	go func(messages []llmservice.RequestMessage, resp *http.Response, returnChan chan string) {

		defer close(returnChan)
		defer resp.Body.Close()

		var body io.ReadCloser = resp.Body

		for {
			toolCalls := ds.handleResponse(body, returnChan)

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

			reqMessage := networkRequestMessage{
				Role:    string(mcpResultMessage.role),
				Content: mcpResultMessage.content,
			}

			followupRequest := request
			followupRequest.Messages = append(followupRequest.Messages, reqMessage)

			requestBody, err := json.Marshal(followupRequest)

			ds.log.Debugf("%s", requestBody)
			if err != nil {
				break
			}

			ds.log.Debugf("DeepSeek request created")

			req, err := http.NewRequestWithContext(ds.ctx, "POST", requestURL, bytes.NewBuffer(requestBody))
			if err != nil {
				break
			}

			authHeadervalue := "Bearer " + ds.apikey

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", authHeadervalue)

			client := &http.Client{}

			ds.log.Debugf("DeepSeek request sent")
			resp, err := client.Do(req)
			if err != nil {
				break
			}

			body = resp.Body
		}

	}(messages, resp, returnChan)

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

		return returnChan, dsErr
	}

	ds.log.Debugf("DeepSeek response Status: %s\n", resp.Status)
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
				result += fmt.Sprintf("Failed to find MCP instance with ID %s\n", args.ID)
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
		}
	}

	return result
}

func extractResources(mcpConnections []MCPConnection) (string, error) {

	stringBuilder := strings.Builder{}

	for i, connection := range mcpConnections {

		if i > 0 {
			stringBuilder.WriteString("\n")
		}

		stringBuilder.WriteString(fmt.Sprintf("MCP ID: %d\n", i))

		resource, err := connection.client.ListResourceTemplates(context.Background(), mcp.ListResourceTemplatesRequest{})
		if err != nil {
			continue
		}

		for j, resource := range resource.ResourceTemplates {

			if j > 0 {
				stringBuilder.WriteString("\n")
			}

			jsonString, err := json.Marshal(resource)
			if err != nil {
				continue
			}

			stringBuilder.WriteString(string(jsonString))
		}
	}

	return stringBuilder.String(), nil
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
