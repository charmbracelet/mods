package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	openai "github.com/sashabaranov/go-openai"
)

type MCPServerFileJSON struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

func configuredMCPServers() (resolved map[string]MCPServerConfig, source string, err error) {
	// If MCPServersFrom is set, use that instead of MCPServers
	if config.MCPServersFrom == "" {
		return config.MCPServers, "mods.yml", nil
	}

	jsonFile, err := os.Open(config.MCPServersFrom)
	if err != nil {
		return nil, "", fmt.Errorf("error opening mcp-servers-from file %s: %v", config.MCPServersFrom, err)
	}
	defer jsonFile.Close()

	var mcpServerFile MCPServerFileJSON
	err = json.NewDecoder(jsonFile).Decode(&mcpServerFile)
	if err != nil {
		return nil, "", fmt.Errorf("error decoding mcp-servers-from file %s: %v", config.MCPServersFrom, err)
	}

	return mcpServerFile.MCPServers, config.MCPServersFrom, nil
}

func enabledMCPServers() ([]string, error) {
	configuredServers, source, err := configuredMCPServers()
	if err != nil {
		return nil, fmt.Errorf("error getting configured MCP servers: %v", err)
	}

	// Validate that all servers in UseMCPServers exist in MCPServers
	for _, name := range config.UseMCPServers {
		if _, exists := configuredServers[name]; !exists {
			return nil, fmt.Errorf("server '%s' specified in --mcp-servers not found in %s", name, source)
		}
	}

	for _, name := range config.DefaultMCPServers {
		if _, exists := configuredServers[name]; !exists {
			return nil, fmt.Errorf("server '%s' specified in --default-mcp-servers not found in %s", name, source)
		}
	}

	if config.UseAllMCPServers {
		var allMCPServers []string
		for name := range configuredServers {
			allMCPServers = append(allMCPServers, name)
		}
		return allMCPServers, nil
	}

	allMCPServers := append(config.DefaultMCPServers, config.UseMCPServers...)

	return allMCPServers, nil
}

// todo this reads the json file multiple times
func listMCPServers() error {
	configuredMCPServers, _, err := configuredMCPServers()
	if err != nil {
		return fmt.Errorf("error listing MCP servers: %v", err)
	}

	if len(configuredMCPServers) == 0 {
		fmt.Println(stdoutStyles().Timeago.Render("No MCP servers configured"))
		return nil
	}

	enabledMCPServers, err := enabledMCPServers()
	if err != nil {
		return fmt.Errorf("error listing MCP servers: %v", err)
	}

	if len(configuredMCPServers) > 0 {
		for name := range configuredMCPServers {
			s := stdoutStyles().InlineCode.Render(name)
			if slices.Contains(enabledMCPServers, name) {
				s = s + stdoutStyles().Timeago.Render(" (enabled)")
			}

			fmt.Println(s)
		}
	}

	return nil
}

func listMCPTools() error {
	configuredMCPServers, _, err := configuredMCPServers()
	if err != nil {
		return fmt.Errorf("error listing MCP tools: %v", err)
	}

	if len(configuredMCPServers) == 0 {
		fmt.Println(stdoutStyles().Timeago.Render("No MCP servers configured"))
		return nil
	}

	enabledMCPServers, err := enabledMCPServers()
	if err != nil {
		return fmt.Errorf("error listing MCP tools: %v", err)
	}

	for _, name := range enabledMCPServers {
		mcpClient, err := client.NewStdioMCPClient(configuredMCPServers[name].Command, nil, configuredMCPServers[name].Args...)
		if err != nil {
			fmt.Printf("Error connecting to %s: %v\n", name, err)
			continue
		}
		_, err = mcpClient.Initialize(context.Background(), mcp.InitializeRequest{})
		if err != nil {
			return fmt.Errorf("error initializing %s: %v", name, err)
		}
		defer mcpClient.Close()

		tools, err := mcpClient.ListTools(context.Background(), mcp.ListToolsRequest{})
		if err != nil {
			return fmt.Errorf("error listing tools for %s: %v", name, err)
		}

		for _, tool := range tools.Tools {
			s := stdoutStyles().InlineCode.Render(tool.Name)
			if tool.Description != "" {
				s = s + " - " + tool.Description
			}
			fmt.Println(s)
		}
	}

	return nil
}

// executeMCPTool executes a tool call using the appropriate MCP server
func executeMCPTool(toolName string, arguments map[string]interface{}) (string, error) {
	// Parse the tool name to get server and tool
	parts := strings.SplitN(toolName, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid tool name format: %s", toolName)
	}

	serverName := parts[0]
	toolName = parts[1]

	// Get the server configuration
	configuredServers, _, err := configuredMCPServers()
	if err != nil {
		return "", fmt.Errorf("error getting MCP server configuration: %v", err)
	}

	serverConfig, exists := configuredServers[serverName]
	if !exists {
		return "", fmt.Errorf("MCP server not found: %s", serverName)
	}

	// Connect to the server
	mcpClient, err := client.NewStdioMCPClient(serverConfig.Command, nil, serverConfig.Args...)
	if err != nil {
		return "", fmt.Errorf("error connecting to MCP server %s: %v", serverName, err)
	}
	defer mcpClient.Close()

	// Initialize the client
	_, err = mcpClient.Initialize(context.Background(), mcp.InitializeRequest{})
	if err != nil {
		return "", fmt.Errorf("error initializing MCP server %s: %v", serverName, err)
	}

	// Call the tool
	result, err := mcpClient.CallTool(context.Background(), mcp.CallToolRequest{
		Params: struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Name:      toolName,
			Arguments: arguments,
		},
	})

	if err != nil {
		return "", fmt.Errorf("error calling tool %s on server %s: %v", toolName, serverName, err)
	}

	// Process the result
	var output strings.Builder
	for _, content := range result.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			output.WriteString(textContent.Text)
		} else {
			// Handle other content types if needed
			output.WriteString("[Non-text content]")
		}
	}

	if result.IsError {
		return output.String(), fmt.Errorf("tool execution error: %s", output.String())
	}

	return output.String(), nil
}

// handleToolCalls processes tool calls from OpenAI API responses
func handleToolCalls(toolCalls []openai.ToolCall) ([]openai.ChatCompletionMessage, error) {
	var messages []openai.ChatCompletionMessage

	for _, toolCall := range toolCalls {
		if toolCall.Function.Name == "" || toolCall.Function.Arguments == "" {
			continue
		}

		// Parse arguments
		var arguments map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &arguments); err != nil {
			return nil, fmt.Errorf("error parsing tool arguments: %v", err)
		}

		// Execute the tool
		result, err := executeMCPTool(toolCall.Function.Name, arguments)
		if err != nil {
			// If there's an error, include it in the tool result
			result = fmt.Sprintf("Error: %v", err)
		}

		fmt.Println("++++toolCall", toolCall.Function.Name, toolCall.Function.Arguments)
		fmt.Println("++++result", result)
		// Add tool call and result to messages
		messages = append(messages, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			ToolCallID: toolCall.ID,
			Content:    result,
		})
	}

	return messages, nil
}
