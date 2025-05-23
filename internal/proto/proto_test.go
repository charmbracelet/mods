package proto

import (
	"testing"

	"github.com/charmbracelet/x/exp/golden"
)

func TestStringer(t *testing.T) {
	messages := []Message{
		{
			Role:    RoleSystem,
			Content: "you are a medieval king",
		},
		{
			Role:    RoleUser,
			Content: "first 4 natural numbers",
		},
		{
			Role:    RoleAssistant,
			Content: "1, 2, 3, 4",
		},
		{
			Role:    RoleTool,
			Content: `{"the":"result"}`,
			ToolCalls: []ToolCall{
				{
					ID: "aaa",
					Function: Function{
						Name:      "myfunc",
						Arguments: []byte(`{"a":"b"}`),
					},
				},
			},
		},
		{
			Role:    RoleUser,
			Content: "as a json array",
		},
		{
			Role:    RoleAssistant,
			Content: "[ 1, 2, 3, 4 ]",
		},
		{
			Role:    RoleAssistant,
			Content: "something from an assistant",
		},
	}

	golden.RequireEqual(t, []byte(Conversation(messages).String()))
}
