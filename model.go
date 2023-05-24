package main

// Model represents the LLM model used in the API call.
type Model struct {
	Name     string
	MaxChars int      `yaml:"max-input-chars"`
	Aliases  []string `yaml:"aliases"`
	API      string   `yaml:"api"`
	Fallback string   `yaml:"fallback"`
}
