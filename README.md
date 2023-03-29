# Mods

GPT-4 for the command line.

## Usage

`mods` will read from standard in and use the content as the prompt for GPT-4. If you provide an optional `-p "PREFIX"` flag, PREFIX will get appended to the content before sending the prompt to GPT-4.

## Requirements

OpenAI GPT-4 access. Set the `OPENAI_TOKEN` environment variable which you can get [from here](https://platform.openai.com/account/api-keys).

## Examples

### Write new sections for a README

```bash
cat README.md | mods -p "write a new section to this README that talks about a user sharing feature. Format the output as markdown"
```

### Editorialize your video files

```bash
ls | mods -p "summarize each of these titles, group them by decade and format the result as markdown" | glow

```

### Let GPT-4 choose something for you to watch

```bash
ls | mods -p "I'm bored and want to watch something action packed, pick 5 shows from this list" | gum choose | xargs vlc

```
