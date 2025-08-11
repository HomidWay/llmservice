package deepseek_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"testing"

	"github.com/TitanLombard/llmservice"
	"github.com/TitanLombard/llmservice/deepseek"
	"github.com/TitanLombard/llmservice/internal/helpers"
	"github.com/TitanLombard/logger"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

var apiKeyFlag = flag.String("apikey", "", "DeepSeekApi key")

const (
	sysMessageContent  = "This is a test for MCP client."
	userMessageContent = "Напиши привет, а после вызови MCP и напиши короткую сводку данных о сотруднике с ID 5560 с его показателями за неделю за неделю, "
)

func TestDebugMCPServerEndpoints(t *testing.T) {

	client, err := client.NewSSEMCPClient("http://192.168.50.80:8770/sse")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	t.Log("Client connected")

	err = client.Start(context.Background())
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	t.Log("Client started")

	initial, err := client.Initialize(context.Background(), mcp.InitializeRequest{})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	t.Logf("%v", initial)

	resourceTemplates, err := client.ListResourceTemplates(context.Background(), mcp.ListResourceTemplatesRequest{})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	t.Logf("%v", resourceTemplates)

	resources, err := client.ListResources(context.Background(), mcp.ListResourcesRequest{})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	t.Logf("%v", resources)
}

func TestSendMessage_streamed(t *testing.T) {

	mcpConnection, err := deepseek.WithSSEMCP("http://192.168.50.80:8770/sse")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	t.Log("Streamed response test started")

	flag.Parse()

	if *apiKeyFlag == "" {
		t.Fatal("Skipping test - no API key provided")
		t.FailNow()
	}

	apikey := *apiKeyFlag

	log := logger.Default(logger.VerbosityDebug, nil)
	ctx := context.Background()
	deepSeek, err := deepseek.NewDeepSeekServiceWithMCP(apikey, ctx, log, mcpConnection)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	sysMessage := deepseek.NewMessage(string(llmservice.SenderRoleSystem), sysMessageContent, nil)
	usrMessage := deepseek.NewMessage(string(llmservice.SenderRoleUser), userMessageContent, nil)

	messages := []llmservice.RequestMessage{
		*sysMessage,
		*usrMessage,
	}

	returnChan, err := llmInterfaceCall(
		deepSeek,
		messages,
		deepseek.WithModel(deepseek.NewDeepSeekChatModel()),
		deepseek.WithStreamed(true),
		deepseek.WithTools([]deepseek.DeepSeekToolDefinition{deepseek.ResourceCallTool()}),
	)

	var invalidOption *helpers.InvalidOptionError
	if errors.As(err, &invalidOption) {
		t.Fatalf("Invalid option type: %s", invalidOption.Error())
		t.FailNow()
	}
	if err != nil {
		t.Errorf("Test failed due to error in deep seek request: %s", err.Error())
		t.FailNow()
	}

	type ToolCall struct {
		ID   string
		Name string
		Args string
	}

	toolCalls := make(map[int]ToolCall, 0)
	fmt.Println("============ RESPONSE ==================")
	for line := range returnChan {
		fmt.Print(line.MessageContent())

		for _, call := range line.ToolCalls() {
			var toolCall ToolCall
			var ok bool

			if toolCall, ok = toolCalls[call.Index()]; !ok {
				toolCall = ToolCall{Name: "", Args: ""}
			}

			if toolCall.ID == "" {
				toolCall.ID = call.ID()
			}

			toolCall.Name += call.ToolName()
			toolCall.Args += call.Args()
			toolCalls[call.Index()] = toolCall
		}
	}
	fmt.Println()
	fmt.Println("Tool Calls:")
	for id, tool := range toolCalls {
		fmt.Printf("(%d: %s) %s: %s\n\n", id, tool.ID, tool.Name, tool.Args)
	}
	fmt.Println("========================================")
	fmt.Println()
}

func llmInterfaceCall(llmservice llmservice.LLMService, messages []llmservice.RequestMessage, args ...llmservice.Option) (chan llmservice.ResponseMessage, error) {
	return llmservice.SendMessage(messages, args...)
}
