# Mods

<p>
    <img src="https://stuff.charm.sh/mods/mods-header.webp" width="548" alt="Mods product art and type treatment"/>
    <br>
    <a href="https://github.com/charmbracelet/mods/releases"><img src="https://img.shields.io/github/release/charmbracelet/mods.svg" alt="Latest Release"></a>
    <a href="https://github.com/charmbracelet/mods/actions"><img src="https://github.com/charmbracelet/mods/workflows/build/badge.svg" alt="Build Status"></a>
</p>

AI-powered terminal workflows and pipelines.

<img width="600px" src="https://vhs.charm.sh/vhs-2IIUgygv7SwdadyjxKDK86.gif" alt="Made with VHS">

GPT models are fantastic at interpreting output and returning results in command line friendly text formats, such as markdown.

`mods` is a tool that makes it easy for you to use GPT models in your existing pipelines.

## Usage

Mods reads `stdin` and prefixes a prompt given as arguments.
This way you can ask questions about the outputs of any command.
Since the answer will output to `stdout` you can make mods a part of any pipeline.

For example, you can ask `mods` to improve your code:

```bash
mods -f "refactor this code" < main.go | glow
```

Or, come up with new product features:

```bash
`mods -f "come up with 10 new features for this tool." < main.go | glow`
```

Even draft up documentation:

```bash
mods "write a new section to this readme for a feature that sends you a free rabbit if you hit 'r'" | glow
```

Organize your file system:

```bash
ls ~/vids | mods -f "organize these by decade and supply a single sentence summary of each" | glow
```

Get recommendations on what to watch:

```bash
ls ~/vids | mods -f "recommend me 10 shows based on these, make them obscure" | glow
ls ~/vids | mods -f "recommend me 10 albums based on these shows, do not include any soundtrack music or music from the show" | glow
```

Predict your fortune based on your chaotic downloads folder:

```bash
ls ~/Downloads | mods -f "tell my fortune based on these files" | glow
```

Quickly understand APIs:

```bash
curl "https://api.open-meteo.com/v1/forecast?latitude=29.00&longitude=-90.00&current_weather=true&hourly=temperature_2m,relativehumidity_2m,windspeed_10m" 2>/dev/null | mods -f "summarize this weather data for a human." | glow
```

Read the comments (so you don't have to):

```bash
curl "https://news.ycombinator.com/item?id=30048332" 2>/dev/null | mods -f "what are the authors of these comments saying?" | glow
```

## Installation

Install `mods` with your preferred package manager.
`mods` has built-in markdown output which pairs great with `Glow`

```bash
# macOS or Linux
brew install mods

# Arch Linux (btw)
pacman -S mods

# Nix
nix-env -iA nixpkgs.mods
```

## Configuration

`mods` requires you to have an `OPENAI_API_KEY` set.

Grab a key from [Open AI](https://platform.openai.com/account/api-keys).


## License

[MIT](https://github.com/charmbracelet/mods/raw/main/LICENSE)

***

Part of [Charm](https://charm.sh).

<a href="https://charm.sh/"><img alt="The Charm logo" width="400" src="https://stuff.charm.sh/charm-badge.jpg" /></a>

Charm热爱开源 • Charm loves open source
