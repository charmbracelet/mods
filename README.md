# Mods!

<p>
    <img src="https://github.com/charmbracelet/mods/assets/25087/0b8872df-7e43-44a7-a2ce-231acd0d2baa" width="630" alt="Mods product art and type treatment"/>
    <br>
    <a href="https://github.com/charmbracelet/mods/releases"><img src="https://img.shields.io/github/release/charmbracelet/mods.svg" alt="Latest Release"></a>
    <a href="https://github.com/charmbracelet/mods/actions"><img src="https://github.com/charmbracelet/mods/workflows/build/badge.svg" alt="Build Status"></a>
</p>

GPT for the command line, built for pipelines.

<p><img src="https://github.com/charmbracelet/mods/assets/25087/ffaad9a3-1105-429f-a076-dae47ef01a07" width="900" alt="a GIF of mods running"></p>

GPT models are really good at interpreting the output of commands and returning
their results in CLI friendly text formats like Markdown. Mods is a simple tool
that makes it super easy to use GPT models on the command line and in your
pipelines.

To get started, [install Mods](#installation) and check out some of the
examples below. Since Mods has built-in Markdown formatting, you may also want
to grab [Glow](https://github.com/charmbracelet/glow) to give the output some
_pizzazz_.

## What Can It Do?

Mods works by reading standard in and prefacing it with a prompt supplied in
the `mods` arguments. It sends the input text to a GPT model and prints out the
result, optionally asking GPT to format the response as Markdown. This gives
you a way to "question" the output of a command. Mods will also work on
standard in or an argument supplied prompt individually.

For example you can:

### Improve Your Code

Piping source code to Mods and giving it an instruction on what to do with it
gives you a lot of options for refactoring, enhancing or debugging code.

`mods -f "what are your thoughts on improving this code?" < main.go | glow`

<p><img src="https://github.com/charmbracelet/mods/assets/25087/e4802dbe-2fb5-444e-a240-831a9865801f" width="900" alt="a GIF of mods offering code refactoring suggestions"></p>

### Come Up With Product Features

Mods can also come up with entirely new features based on source code (or a
README file).

`mods -f "come up with 10 new features for this tool." < main.go | glow`

<p><img src="https://github.com/charmbracelet/mods/assets/25087/667be64b-459e-4a5d-9e2c-e5e8697e5fea" width="900" alt="a GIF of mods suggesting feature improvements"></p>

### Help Write Docs

Mods can quickly give you a first draft for new documentation.

`mods "write a new section to this readme for a feature that sends you a free rabbit if you hit r" | glow`

<p><img src="https://github.com/charmbracelet/mods/assets/25087/c653187b-a071-4599-82d1-f7023f954b15" width="900" alt="a GIF of mods contributing to a product README"></p>

### Organize Your Videos

The file system can be an amazing source of input for Mods. If you have music
or video files, Mods can parse the output of `ls` and offer really good
editorialization of your content.

`ls ~/vids | mods -f "organize these by decade and summarize each" | glow`

<p><img src="https://github.com/charmbracelet/mods/assets/25087/0319568c-c87c-4f9d-980c-847ac95797f6" width="900" alt="a GIF of mods oraganizing and summarizing video from a shell ls statement"></p>

### Make Recommendations

Mods is really good at generating recommendations based on what you have as
well, both for similar content but also content in an entirely different media
(like getting music recommendations based on movies you have).

`ls ~/vids | mods -f "recommend me 10 shows based on these, make them obscure" | glow`

`ls ~/vids | mods -f "recommend me 10 albums based on these shows, do not include any soundtrack music or music from the show" | glow`

<p><img src="https://github.com/charmbracelet/mods/assets/25087/300da607-2de7-42b2-9f5b-3de579ab0587" width="900" alt="a GIF of mods generating television show recommendations based on a file listing from a directory of videos"></p>

### Read Your Fortune

It's easy to let your downloads folder grow into a chaotic never-ending pit of
files, but with Mods you can use that to your advantage!

`ls ~/Downloads | mods -f "tell my fortune based on these files" | glow`

<p><img src="https://github.com/charmbracelet/mods/assets/25087/23c5bc39-c4ba-43b9-b61e-e6b56da5a4d8" width="900" alt="a GIF of mods generating a fortune from the contents of a downloads directory"></p>

### Understand APIs

Mods can parse and understand the output of an API call with `curl` and convert
it to something human readable.

`curl "https://api.open-meteo.com/v1/forecast?latitude=29.00&longitude=-90.00&current_weather=true&hourly=temperature_2m,relativehumidity_2m,windspeed_10m" 2>/dev/null | mods -f "summarize this weather data for a human." | glow`

<p><img src="https://github.com/charmbracelet/mods/assets/25087/7f21bf79-af25-4800-9702-98b984f4f539" width="900" alt="a GIF of mods summarizing the weather from JSON API output"></p>

### Read The Comments (so you don't have to)

Just like with APIs, Mods can read through raw HTML and summarize the contents.

`curl "https://news.ycombinator.com/item?id=30048332" 2>/dev/null | mods -f "what are the authors of these comments saying?" | glow`

<p><img src="https://github.com/charmbracelet/mods/assets/25087/563676a9-3355-4102-ab6b-9cfdfc537d8a" width="900" alt="a GIF of mods summarizing the comments on hacker news"></p>

## Installation

Mods currently works with OpenAI's models, so you'll need to set the
`OPENAI_API_KEY` environment variable to a valid OpenAI key, which you can get
[from here](https://platform.openai.com/account/api-keys).

Then install Mods with your package manager:

```
# macOS or Linux
brew install mods

# macOS (via MacPorts)
sudo port install mods

# Arch Linux (btw)
pacman -S mods

# Nix
nix-env -iA nixpkgs.mods

# Debian/Ubuntu
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://repo.charm.sh/apt/gpg.key | sudo gpg --dearmor -o /etc/apt/keyrings/charm.gpg
echo "deb [signed-by=/etc/apt/keyrings/charm.gpg] https://repo.charm.sh/apt/ * *" | sudo tee /etc/apt/sources.list.d/charm.list
sudo apt update && sudo apt install mods

# Fedora/RHEL
echo '[charm]
name=Charm
baseurl=https://repo.charm.sh/yum/
enabled=1
gpgcheck=1
gpgkey=https://repo.charm.sh/yum/gpg.key' | sudo tee /etc/yum.repos.d/charm.repo
sudo yum install mods

# Void Linux
sudo xbps-install mods

# Windows
scoop install mods

```

Or, download it:

* [Packages][releases] are available in Debian and RPM formats
* [Binaries][releases] are available for Linux, macOS, and Windows

Or, just install it with `go`:

```sh
go install github.com/charmbracelet/mods@latest
```

## Settings

Mods lets you tune your query with a variety of settings that you can use with
flags or environment variables.

#### Model

`-m`, `--model`, `MODS_MODEL`

Mods uses `gpt-4` by default but you can specify any OpenAI model as long as
your account has access to it. Mods also plans to eventually support local
models.

#### Format As Markdown

`-f`, `--format`, `MODS_FORMAT`

GPT models are very good at generating their response in Markdown format. They
can even organize their content naturally with headers, bullet lists... Use
this option to append the phrase "Format the response as Markdown." to the
prompt.

#### Max Tokens

`--max-tokens`, `MODS_MAX_TOKENS`

Max tokens tells GPT to respond in less than this number of tokens. GPT is
better at longer responses so values larger than 256 tend to work best.

#### Temperature

`--temp`, `MODS_TEMP`

Sampling temperature is a number between 0.0 and 2.0 and determines how
confident the model is in its choices. Higher values make the output more
random and lower values make it more deterministic.

#### TopP

`--topp`, `MODS_TOPP`

Top P is an alternative to sampling temperature. It's a number between 0.0 and
2.0 with smaller numbers narrowing the domain from which the model will create
its response.

#### No Limit

`--no-limit`, `MODS_NO_LIMIT`

By default Mods attempts to size the input to the maximum size the allowed by
the model. You can potentially squeeze a few more tokens into the input by
setting this but also risk getting a max token exceeded error from the GPT API.

#### Include Prompt

`-P`, `--prompt`, `MODS_INCLUDE_PROMPT`

Include prompt will preface the response with the entire prompt, both standard
in and the prompt supplied by the arguments.

#### Include Prompt Args

`-p`, `--prompt-args`, `MODS_INCLUDE_PROMPT_ARGS`

Include prompt args will include _only_ the prompt supplied by the arguments.
This can be useful if your standard in content is long and you just a want a
summary before the response.

#### Max Retries

`--max-retries`, `MODS_MAX_RETRIES`

The maximum number of retries to failed API calls. The retries happen with an
exponential backoff.

#### Fanciness

`--fanciness`, `MODS_FANCINESS`

Your desired level of fanciness.

#### Quiet

`-q`, `--quiet`, `MODS_QUIET`

Output nothing to standard err.

## License

[MIT](https://github.com/charmbracelet/mods/raw/main/LICENSE)

***

Part of [Charm](https://charm.sh).

<a href="https://charm.sh/"><img alt="The Charm logo" width="400" src="https://stuff.charm.sh/charm-badge.jpg" /></a>

Charm热爱开源 • Charm loves open source
