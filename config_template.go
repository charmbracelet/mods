package main

const configTemplate = `# {{ index .Help "model" }}
default-model: gpt-4
# {{ index .Help "format-text" }}
format-text: Format the response as markdown without enclosing backticks.
# {{ index .Help "format" }}
format: false
# {{ index .Help "raw" }}
raw: false
# {{ index .Help "quiet" }}
quiet: false
# {{ index .Help "temp" }}
temp: 1.0
# {{ index .Help "topp" }}
topp: 1.0
# {{ index .Help "no-limit" }}
no-limit: false
# {{ index .Help "prompt-args" }}
include-prompt-args: false
# {{ index .Help "prompt" }}
include-prompt: 0
# {{ index .Help "max-retries" }}
max-retries: 5
# {{ index .Help "fanciness" }}
fanciness: 10
# {{ index .Help "status-text" }}
status-text: Generating
# {{ index .Help "max-input-chars" }}
max-input-chars: 12250
# {{ index .Help "max-tokens" }}
# max-tokens: 100
# {{ index .Help "apis" }}
apis:
  openai:
    base-url: https://api.openai.com/v1
    api-key: 
    api-key-env: OPENAI_API_KEY
    models:
      gpt-4:
        aliases: ["4"]
        max-input-chars: 24500
        fallback: gpt-3.5-turbo
      gpt-4-32k:
        aliases: ["32k"]
        max-input-chars: 98000
        fallback: gpt-4
      gpt-3.5-turbo:
        aliases: ["35t"]
        max-input-chars: 12250
        fallback: gpt-3.5
      gpt-3.5-turbo-16k:
        aliases: ["35t16k"]
        max-input-chars: 44500
        fallback: gpt-3.5
      gpt-3.5:
        aliases: ["35"]
        max-input-chars: 12250
        fallback:
  localai:
    # LocalAI setup instructions: https://github.com/go-skynet/LocalAI#example-use-gpt4all-j-model
    base-url: http://localhost:8080
    models:
      ggml-gpt4all-j:
        aliases: ["local", "4all"]
        max-input-chars: 12250
        fallback:
  azure:
    # Set to 'azure-ad' to use Active Directory
    # Azure OpenAI setup: https://learn.microsoft.com/en-us/azure/cognitive-services/openai/how-to/create-resource
    base-url: https://YOUR_RESOURCE_NAME.openai.azure.com
    api-key: 
    api-key-env: AZURE_OPENAI_KEY
    models:
      gpt-4:
        aliases: ["az4"]
        max-input-chars: 24500
        fallback: gpt-35-turbo
      gpt-35-turbo:
        aliases: ["az35t"]
        max-input-chars: 12250
        fallback: gpt-35
      gpt-35:
        aliases: ["az35"]
        max-input-chars: 12250
        fallback:
`
