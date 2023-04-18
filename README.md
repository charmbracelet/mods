# Mods

GPT for the command line, built for pipelines.

## Usage

`mods` lets you add GPT to your command line pipelines. It reads content from
standard in and combines it with a "command" then returns the results from GPT.
Adding the `-md` flag asks GPT to format the output as Markdown (useful when
piping to something like [Glow](https://github.com/charmbracelet/glow)). By
default `mods` uses the GPT-4 model, to specify a different model use `-m
gpt-3.5-turbo`.

## Examples

### Write new sections for a README

```bash
cat README.md | mods "write a new section to this README documenting a pdf sharing feature"
```

### Editorialize your video files

```bash
ls | mods -md "summarize each of these titles, group them by decade" | glow

```

### Let GPT choose something for you to watch

```bash
ls | mods "Pick 5 action packed shows from the 80s from this list" | gum choose | xargs vlc

```

## Requirements

You need to set the `OPENAI_API_KEY` environment variable to a valid OpenAI
key, which you can get [from
here](https://platform.openai.com/account/api-keys).

## License

[MIT](https://github.com/charmbracelet/mods/raw/main/LICENSE)

***

Part of [Charm](https://charm.sh).

<a href="https://charm.sh/"><img alt="The Charm logo" width="400" src="https://stuff.charm.sh/charm-badge.jpg" /></a>

Charm热爱开源 • Charm loves open source
