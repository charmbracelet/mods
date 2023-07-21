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

Be sure to check out the [examples](examples.md).

## Installation

Mods works with OpenAI compatible endpoints. By default, Mods is configured to
support OpenAI's official API and a LocalAI installation running on port 8080.
You can configure additional endpoints in your settings file by running `mods -s`.

### OpenAI

Mods uses GPT-4 by default and will fallback to GPT-3.5 Turbo if it's not
available. Set the `OPENAI_API_KEY` environment variable to a valid OpenAI key,
which you can get [from here](https://platform.openai.com/account/api-keys).

Mods can also use the [Azure OpenAI](https://azure.microsoft.com/en-us/products/cognitive-services/openai-service)
service. Set the `AZURE_OPENAI_KEY` environment variable and configure your
Azure endpoint with `mods -s`.

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

#### Save

`--save`

Saves the conversation with the given name. Continue the conversation back with
`-c` and pass the name.


#### Continue

`-c`, `--continue`

Continue from the last response or a given name.

Additional instances of `-c` will continue the conversation. Once a prompt is
sent without `-c`.

Providing a name will automatically save the conversation with that name if it
doesn't already exist.

#### List

`-l`, `--list`

Lists all saved conversations.

#### Delete

`--delete`

Deletes the saved conversation with the given name.

#### Format As Markdown

`-f`, `--format`, `MODS_FORMAT`

Ask the LLM to format the response as markdown. You can edit the text passed to
the LLM with `mods -s` then changing the `format-text` value.

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

#### Reset Settings

`--reset-settings`

Backup your old settings file and reset everything to the defaults.

#### No Cache

`--no-cache`, `MODS_NO_CACHE`

Disable saving and reading the most recent prompt/response.


#### HTTP Proxy
`-x`, `--http-proxy`, `MODS_HTTP_PROXY`

Use the HTTP proxy to the connect the API endpoints.

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
