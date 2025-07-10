package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"golang.org/x/sync/errgroup"
)

func enabledMCPs() iter.Seq2[string, MCPServerConfig] {
	return func(yield func(string, MCPServerConfig) bool) {
		names := slices.Collect(maps.Keys(config.MCPServers))
		slices.Sort(names)
		for _, name := range names {
			if !isMCPEnabled(name) {
				continue
			}
			if !yield(name, config.MCPServers[name]) {
				return
			}
		}
	}
}

func isMCPEnabled(name string) bool {
	return !slices.Contains(config.MCPDisable, "*") &&
		!slices.Contains(config.MCPDisable, name)
}

func mcpList() {
	for name := range config.MCPServers {
		s := name
		if isMCPEnabled(name) {
			s += stdoutStyles().Timeago.Render(" (enabled)")
		}
		fmt.Println(s)
	}
}

func mcpListTools(ctx context.Context) error {
	servers, err := mcpTools(ctx)
	if err != nil {
		return err
	}
	for sname, tools := range servers {
		for _, tool := range tools {
			fmt.Print(stdoutStyles().Timeago.Render(sname + " > "))
			fmt.Println(tool.Name)
		}
	}
	return nil
}

func mcpTools(ctx context.Context) (map[string][]mcp.Tool, error) {
	var mu sync.Mutex
	var wg errgroup.Group
	result := map[string][]mcp.Tool{}
	for sname, server := range enabledMCPs() {
		wg.Go(func() error {
			serverTools, err := mcpToolsFor(ctx, sname, server)
			if errors.Is(err, context.DeadlineExceeded) {
				return modsError{
					err:    fmt.Errorf("timeout while listing tools for %q - make sure the configuration is correct. If your server requires a docker container, make sure it's running", sname),
					reason: "Could not list tools",
				}
			}
			if err != nil {
				return modsError{
					err:    err,
					reason: "Could not list tools",
				}
			}
			mu.Lock()
			result[sname] = append(result[sname], serverTools...)
			mu.Unlock()
			return nil
		})
	}
	if err := wg.Wait(); err != nil {
		return nil, err //nolint:wrapcheck
	}
	return result, nil
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
	defer cli.Close() //nolint:errcheck
	if _, err := cli.Initialize(ctx, mcp.InitializeRequest{}); err != nil {
		return nil, fmt.Errorf("could not setup %s: %w", name, err)
	}
	tools, err := cli.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("could not setup %s: %w", name, err)
	}
	return tools.Tools, nil
}

func toolCall(ctx context.Context, name string, data []byte) (string, error) {
	sname, tool, ok := strings.Cut(name, "_")
	if !ok {
		return "", fmt.Errorf("mcp: invalid tool name: %q", name)
	}
	server, ok := config.MCPServers[sname]
	if !ok {
		return "", fmt.Errorf("mcp: invalid server name: %q", sname)
	}
	if !isMCPEnabled(sname) {
		return "", fmt.Errorf("mcp: server is disabled: %q", sname)
	}
	client, err := client.NewStdioMCPClient(
		server.Command,
		append(os.Environ(), server.Env...),
		server.Args...,
	)
	if err != nil {
		return "", fmt.Errorf("mcp: %w", err)
	}
	defer client.Close() //nolint:errcheck

	// Initialize the client
	if _, err = client.Initialize(ctx, mcp.InitializeRequest{}); err != nil {
		return "", fmt.Errorf("mcp: %w", err)
	}

	var args map[string]any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &args); err != nil {
			return "", fmt.Errorf("mcp: %w: %s", err, string(data))
		}
	}

	request := mcp.CallToolRequest{}
	request.Params.Name = tool
	request.Params.Arguments = args
	result, err := client.CallTool(context.Background(), request)
	if err != nil {
		return "", fmt.Errorf("mcp: %w", err)
	}

	var sb strings.Builder
	for _, content := range result.Content {
		switch content := content.(type) {
		case mcp.TextContent:
			sb.WriteString(content.Text)
		default:
			sb.WriteString("[Non-text content]")
		}
	}

	if result.IsError {
		return "", errors.New(sb.String())
	}
	return sb.String(), nil
}
