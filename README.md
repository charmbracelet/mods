# Mods!

<p>
    <img src="https://github.com/charmbracelet/mods/assets/25087/5442bf46-b908-47af-bf4e-60f7c38951c4" width="630" alt="Mods product art and type treatment"/>
    <br>
    <a href="https://github.com/charmbracelet/mods/releases"><img src="https://img.shields.io/github/release/charmbracelet/mods.svg" alt="Latest Release"></a>
    <a href="https://github.com/charmbracelet/mods/actions"><img src="https://github.com/charmbracelet/mods/workflows/build/badge.svg" alt="Build Status"></a>
</p>

AI for the command line, built for pipelines.

<p><img src="https://github.com/charmbracelet/mods/assets/25087/347300c6-b382-462d-9f80-8520a27e14bb" width="900" alt="a GIF of mods running"></p>

LLM based AI is really good at interpreting the output of commands and
returning the results in CLI friendly text formats like Markdown. Mods is a
simple tool that makes it super easy to use AI on the command line and in your
pipelines. Mods works with [OpenAI](https://platform.openai.com/account/api-keys)
and [LocalAI](https://github.com/go-skynet/LocalAI)

To get started, [install Mods](#installation) and check out some of the
examples below. Since Mods has built-in Markdown formatting, you may also want
to grab [Glow](https://github.com/charmbracelet/glow) to give the output some
_pizzazz_.

## What Can It Do?

Mods works by reading standard in and prefacing it with a prompt supplied in
the `mods` arguments. It sends the input text to an LLM and prints out the
result, optionally asking the LLM to format the response as Markdown. This
gives you a way to "question" the output of a command. Mods will also work on
standard in or an argument supplied prompt individually.

For example you can:

### Improve Your Code

Piping source code to Mods and giving it an instruction on what to do with it
gives you a lot of options for refactoring, enhancing or debugging code.

`mods -f "what are your thoughts on improving this code?" < main.go | glow`

<p><img src="https://github.com/charmbracelet/mods/assets/25087/738fe969-1c9f-4849-af8a-cde38156ce92" width="900" alt="a GIF of mods offering code refactoring suggestions"></p>

### Come Up With Product Features

Mods can also come up with entirely new features based on source code (or a
README file).

`mods -f "come up with 10 new features for this tool." < main.go | glow`

<p><img src="https://github.com/charmbracelet/mods/assets/25087/025de860-798a-4ab2-b1cf-a0b32dbdbe4d" width="900" alt="a GIF of mods suggesting feature improvements"></p>

### Help Write Docs

Mods can quickly give you a first draft for new documentation.

`mods "write a new section to this readme for a feature that sends you a free rabbit if you hit r" | glow`

<p><img src="https://github.com/charmbracelet/mods/assets/25087/c26a17a9-c772-40cc-b3f1-9189ac682730" width="900" alt="a GIF of mods contributing to a product README"></p>

### Organize Your Videos

The file system can be an amazing source of input for Mods. If you have music
or video files, Mods can parse the output of `ls` and offer really good
editorialization of your content.

`ls ~/vids | mods -f "organize these by decade and summarize each" | glow`

<p><img src="https://github.com/charmbracelet/mods/assets/25087/8204d06a-8cf1-401d-802f-2b94345dec5d" width="900" alt="a GIF of mods oraganizing and summarizing video from a shell ls statement"></p>

### Make Recommendations

Mods is really good at generating recommendations based on what you have as
well, both for similar content but also content in an entirely different media
(like getting music recommendations based on movies you have).

`ls ~/vids | mods -f "recommend me 10 shows based on these, make them obscure" | glow`

`ls ~/vids | mods -f "recommend me 10 albums based on these shows, do not include any soundtrack music or music from the show" | glow`

<p><img src="https://github.com/charmbracelet/mods/assets/25087/48159b19-5cae-413b-9677-dce8c6dfb6b8" width="900" alt="a GIF of mods generating television show recommendations based on a file listing from a directory of videos"></p>

### Read Your Fortune

It's easy to let your downloads folder grow into a chaotic never-ending pit of
files, but with Mods you can use that to your advantage!

`ls ~/Downloads | mods -f "tell my fortune based on these files" | glow`

<p><img src="https://github.com/charmbracelet/mods/assets/25087/da2206a8-799f-4c92-b75e-bac66c56ea88" width="900" alt="a GIF of mods generating a fortune from the contents of a downloads directory"></p>

### Understand APIs

Mods can parse and understand the output of an API call with `curl` and convert
it to something human readable.

`curl "https://api.open-meteo.com/v1/forecast?latitude=29.00&longitude=-90.00&current_weather=true&hourly=temperature_2m,relativehumidity_2m,windspeed_10m" 2>/dev/null | mods -f "summarize this weather data for a human." | glow`

<p><img src="https://github.com/charmbracelet/mods/assets/25087/3af13876-46a3-4bab-986e-50d9f54d2921" width="900" alt="a GIF of mods summarizing the weather from JSON API output"></p>

### Read The Comments (so you don't have to)

Just like with APIs, Mods can read through raw HTML and summarize the contents.

`curl "https://news.ycombinator.com/item?id=30048332" 2>/dev/null | mods -f "what are the authors of these comments saying?" | glow`

<p><img src="https://github.com/charmbracelet/mods/assets/25087/e4d94ef8-43aa-45ea-9be5-fe13e53d5203" width="900" alt="a GIF of mods summarizing the comments on hacker news"></p>

## Installation

Mods works with OpenAI compatible endpoints. By default, Mods is configured to
support OpenAI's official API and a LocalAI installation running on port 8080.
You can configure additional endpoints in your settings file by running `mods -s`.

### OpenAI

Mods uses GPT-4 by default and will fallback to GPT-3.5 Turbo if it's not
available. Set the `OPENAI_API_KEY` environment variable to a valid OpenAI key,
which you can get [from here](https://platform.openai.com/account/api-keys).

### LocalAI

LocalAI allows you to run a multitude of models locally. Mods works with the
GPT4ALL-J model as setup in [this tutorial](https://github.com/go-skynet/LocalAI#example-use-gpt4all-j-model).
You can define more LocalAI models and endpoints with `mods -s`.

### Install Mods

```bash
# macOS or Linux
brew install charmbracelet/tap/mods

# Arch Linux (btw)
yay -S mods

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
```

Or, download it:

* [Packages][releases] are available in Debian and RPM formats
* [Binaries][releases] are available for Linux, macOS, and Windows

[releases]: https://github.com/charmbracelet/mods/releases

Or, just install it with `go`:

```sh
go install github.com/charmbracelet/mods@latest
```

## Settings

Mods lets you tune your query with a variety of settings. You can configure
Mods with `mods -s` or pass the settings as environment variables and flags.

#### Model

`-m`, `--model`, `MODS_MODEL`

Mods uses `gpt-4` with OpenAI by default but you can specify any model as long
as your account has access to it or you have installed locally with LocalAI.

You can add new models to the settings with `mods -s`. You can also specify a
model and an API endpoint with `-m` and `-a` to use models not in the settings
file.

#### Format As Markdown

`-f`, `--format`, `MODS_FORMAT`

LLMs are very good at generating their response in Markdown format. They
can even organize their content naturally with headers, bullet lists... Use
this option to append the phrase "Format the response as Markdown." to the
prompt.

#### Max Tokens

`--max-tokens`, `MODS_MAX_TOKENS`

Max tokens tells the LLM to respond in less than this number of tokens. LLMs
are better at longer responses so values larger than 256 tend to work best.

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
setting this but also risk getting a max token exceeded error from the OpenAI API.

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

## Whatcha Think?

We’d love to hear your thoughts on this project. Feel free to drop us a note.

* [Twitter](https://twitter.com/charmcli)
* [The Fediverse](https://mastodon.social/@charmcli)
* [Discord](https://charm.sh/chat)

## License

[MIT](https://github.com/charmbracelet/mods/raw/main/LICENSE)

***

Part of [Charm](https://charm.sh).

<a href="https://charm.sh/"><img alt="The Charm logo" width="400" src="https://stuff.charm.sh/charm-badge.jpg" /></a>

Charm热爱开源 • Charm loves open source
