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

func usage() {
	fmt.Printf("Usage: %s [OPTIONS] [PREFIX TERM]\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	modelVersionFlag := flag.String("m", "gpt-4", "OpenAI model flag.")
	formatFlag := flag.Bool("f", false, "Ask GPT to format the output as Markdown.")
	flag.Usage = usage
	flag.Parse()

	token := os.Getenv("OPENAI_API_KEY")
	if token == "" {
		log.Fatal("Error: OPENAI_API_KEY environment variable is required")
	}

	content := ""
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		reader := bufio.NewReader(os.Stdin)
		stdinBytes, err := ioutil.ReadAll(reader)
		if err != nil {
			log.Fatal("Error reading standard input: ", err)
		}
		content = string(stdinBytes)
	}

	prefix := strings.Join(flag.Args(), " ")
	if *formatFlag {
		prefix = fmt.Sprintf("%s Format output as Markdown.", prefix)
	}

	if prefix != "" {
		content = strings.TrimSpace(prefix + "\n\n" + content)
	}

	client := openai.NewClient(token)
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
