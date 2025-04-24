package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func enabledMCPs() map[string]MCPServerConfig {
	result := map[string]MCPServerConfig{}
	for k, v := range config.MCPServers {
		if !isMCPEnabled(k) {
			continue
		}
		result[k] = v
	}
	return result
}

func isMCPEnabled(name string) bool {
	return !slices.Contains(config.MCPDisable, "*") &&
		!slices.Contains(config.MCPDisable, name)
}

func mcpList() error {
	for name := range config.MCPServers {
		s := name
		if isMCPEnabled(name) {
			s += stdoutStyles().Timeago.Render(" (enabled)")
		}
		fmt.Println(s)
	}
	return nil
}

func mcpListTools() error {
	ctx := context.Background()
	for sname, server := range enabledMCPs() {
		tools, err := mcpToolsFor(ctx, sname, server)
		if err != nil {
			return err
		}
		for _, tool := range tools {
			fmt.Print(stdoutStyles().Timeago.Render(sname + " > "))
			fmt.Println(tool.Name)
		}
	}
	return nil
}

func mcpTools(ctx context.Context) ([]anthropic.ToolUnionParam, error) {
	var tools []anthropic.ToolUnionParam
	for sname, server := range enabledMCPs() {
		serverTools, err := mcpToolsFor(ctx, sname, server)
		if err != nil {
			return nil, err
		}
		for _, tool := range serverTools {
			tools = append(tools, anthropic.ToolUnionParam{
				OfTool: &anthropic.ToolParam{
					InputSchema: anthropic.ToolInputSchemaParam{
						Properties: tool.InputSchema.Properties,
					},
					Name:        fmt.Sprintf("%s_%s", sname, tool.Name),
					Description: anthropic.String(tool.Description),
				},
			})
		}
	}
	return tools, nil
}

func mcpToolsFor(ctx context.Context, name string, server MCPServerConfig) ([]mcp.Tool, error) {
	cli, err := client.NewStdioMCPClient(
		server.Command,
		append(os.Environ(), server.Env...),
		server.Args...,
	)
	if err != nil {
		return nil, fmt.Errorf("could not setup %s: %w", name, err)
	}
	defer cli.Close()
	if _, err := cli.Initialize(ctx, mcp.InitializeRequest{}); err != nil {
		return nil, fmt.Errorf("could not setup %s: %w", name, err)
	}
	tools, err := cli.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("could not setup %s: %w", name, err)
	}
	return tools.Tools, nil
}

func toolCall(name string, input []byte) (string, error) {
	sname, tool, ok := strings.Cut(name, "_")
	if !ok {
		return "", fmt.Errorf("mcp: invalid tool name: %q", name)
	}
	server, ok := enabledMCPs()[sname]
	if !ok {
		return "", fmt.Errorf("mcp: invalid server name: %q", sname)
	}
	client, err := client.NewStdioMCPClient(
		server.Command,
		append(os.Environ(), server.Env...),
		server.Args...,
	)
	if err != nil {
		return "", fmt.Errorf("mcp: %w", err)
	}
	defer client.Close()

	// Initialize the client
	if _, err = client.Initialize(context.Background(), mcp.InitializeRequest{}); err != nil {
		return "", fmt.Errorf("mcp: %w", err)
	}

	var args map[string]any
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("mcp: %w", err)
	}

	result, err := client.CallTool(context.Background(), mcp.CallToolRequest{
		Params: struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Name:      tool,
			Arguments: args,
		},
	})
	if err != nil {
		return "", fmt.Errorf("mcp: %w", err)
	}

	var sb strings.Builder
	for _, content := range result.Content {
		switch content := content.(type) {
		case mcp.TextContent:
			sb.WriteString(content.Text)
		default:
			// TODO: can we make this better somehow?
			sb.WriteString("[Non-text content]")
		}
	}

	if result.IsError {
		return sb.String(), errors.New("mcp: tool failed")
	}
	return sb.String(), nil
}
