package main

type Model struct {
	Name     string
	MaxChars int      `yaml:"max-input-chars"`
	Aliases  []string `yaml:"aliases"`
	API      string   `yaml:"api"`
	Fallback string   `yaml:"fallback"`
}
