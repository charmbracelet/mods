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
	openai "github.com/sashabaranov/go-openai"
)

func main() {
	log.SetLevel(log.DebugLevel)
	log.SetReportCaller(true)

	token := os.Getenv("OPENAI_TOKEN")
	if token == "" {
		log.Fatal("Error: OPENAI_TOKEN environment variable is required")
	}

	prefixFlag := flag.String("p", "", "PREFIX flag to prepend to the standard input content.")
	flag.Parse()

	reader := bufio.NewReader(os.Stdin)
	stdinBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		log.Fatalf("Error reading standard input: %v\n", err)
	}

	content := string(stdinBytes)
	if *prefixFlag != "" {
		content = strings.TrimSpace(*prefixFlag) + "\n\n" + content
	}

	client := openai.NewClient(token)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: content,
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return
	}

	fmt.Println(resp.Choices[0].Message.Content)
}
