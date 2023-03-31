package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/mattn/go-isatty"
	openai "github.com/sashabaranov/go-openai"
)

func printUsage() {
	fmt.Printf("Usage: %s [OPTIONS] [PREFIX TERM]\n", os.Args[0])
	flag.PrintDefaults()
}

func readStdinContent() string {
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		reader := bufio.NewReader(os.Stdin)
		stdinBytes, err := ioutil.ReadAll(reader)
		if err != nil {
			log.Fatal("Error reading standard input: ", err)
		}
		return string(stdinBytes)
	}
	return ""
}

func createClient(apiKey string) *openai.Client {
	if apiKey == "" {
		log.Fatal("Error: OPENAI_API_KEY environment variable is required")
	}
	return openai.NewClient(apiKey)
}

func main() {
	modelVersionFlag := flag.String("m", "gpt-4", "OpenAI model flag.")
	formatFlag := flag.Bool("f", false, "Ask GPT to format the output as Markdown.")
	flag.Usage = printUsage
	flag.Parse()

	client := createClient(os.Getenv("OPENAI_API_KEY"))
	content := readStdinContent()
	prefix := strings.Join(flag.Args(), " ")
	if *formatFlag {
		prefix = fmt.Sprintf("%s Format output as Markdown.", prefix)
	}

	if prefix != "" {
		content = strings.TrimSpace(prefix + "\n\n" + content)
	}

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: *modelVersionFlag,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: content,
				},
			},
		},
	)

	if err != nil {
		log.Fatal("ChatCompletion error: ", err)
	}

	fmt.Println(resp.Choices[0].Message.Content)
}
