package main

import (
	"math/rand"
)

var examples = map[string]string{
	"Write new sections for a readme": `cat README.md | mods "write a new section to this README documenting a pdf sharing feature"`,
	"Editorialze your video files":    `ls ~/vids | mods -f "summarize each of these titles, group them by decade" | glow`,
	"Let GPT pick something to watch": `ls ~/vids | mods "Pick 5 action packed shows from the 80s from this list" | gum choose | xargs vlc`,
}

func randomExample() (string, string) {
	keys := make([]string, 0, len(examples))
	for k := range examples {
		keys = append(keys, k)
	}
	desc := keys[rand.Intn(len(keys))]
	return desc, examples[desc]
}
