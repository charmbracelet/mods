package google

import "github.com/charmbracelet/mods/internal/proto"

func fromProtoMessages(input []proto.Message) []Content {
	// Check for images and error if present (not supported yet)
	for _, msg := range input {
		if len(msg.Images) > 0 {
			panic("image input is not supported for Google API yet - use OpenAI API for vision capabilities")
		}
	}

	result := make([]Content, 0, len(input))
	for _, in := range input {
		switch in.Role {
		case proto.RoleSystem, proto.RoleUser:
			result = append(result, Content{
				Role:  proto.RoleUser,
				Parts: []Part{{Text: in.Content}},
			})
		}
	}
	return result
}
