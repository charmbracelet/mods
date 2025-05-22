package google

import "github.com/charmbracelet/mods/internal/proto"

func fromProtoMessages(input []proto.Message) []Content {
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
