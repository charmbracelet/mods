package main

// Model represents the LLM model used in the API call.
type Model struct {
	Name     string
	API      string
	MaxChars int      `yaml:"max-input-chars"`
	Aliases  []string `yaml:"aliases"`
	Fallback string   `yaml:"fallback"`
}

// API represents an API endpoint and its models.
type API struct {
	BaseURL string           `yaml:"base-url"`
	Models  map[string]Model `yaml:"models"`
}
