# Mods!

<p>
    <img src="https://github.com/charmbracelet/mods/assets/25087/5442bf46-b908-47af-bf4e-60f7c38951c4" width="630" alt="Mods product art and type treatment"/>
    <br>
    <a href="https://github.com/charmbracelet/mods/releases"><img src="https://img.shields.io/github/release/charmbracelet/mods.svg" alt="Latest Release"></a>
    <a href="https://github.com/charmbracelet/mods/actions"><img src="https://github.com/charmbracelet/mods/workflows/build/badge.svg" alt="Build Status"></a>
</p>

AI for the command line, built for pipelines.

<p><img src="https://vhs.charm.sh/vhs-5Uyj0U6Hlqi1LVIIRyYKM5.gif" width="900" alt="a GIF of mods running"></p>

LLM based AI is really good at interpreting the output of commands and
returning the results in CLI friendly text formats like Markdown. Mods is a
simple tool that makes it super easy to use AI on the command line and in your
pipelines.
Mods works with [OpenAI](https://platform.openai.com/account/api-keys),
[Groq](https://console.groq.com/keys),
[Azure OpenAI](https://azure.microsoft.com/en-us/products/cognitive-services/openai-service),
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

Be sure to check out the [examples](examples.md) and a list of all the
[features](features.md).

## Installation

Mods works with OpenAI compatible endpoints. By default, Mods is configured to
support OpenAI's official API and a LocalAI installation running on port 8080.
You can configure additional endpoints in your settings file by running
`mods --settings`.

### OpenAI

Mods uses GPT-4 by default and will fall back to GPT-3.5 Turbo if it's not
available. Set the `OPENAI_API_KEY` environment variable to a valid OpenAI key,
which you can get [from here](https://platform.openai.com/account/api-keys).

Mods can also use the [Azure OpenAI](https://azure.microsoft.com/en-us/products/cognitive-services/openai-service)
service. Set the `AZURE_OPENAI_KEY` environment variable and configure your
Azure endpoint with `mods --settings`.

### LocalAI

LocalAI allows you to run a multitude of models locally. Mods works with the
GPT4ALL-J model as setup in [this tutorial](https://github.com/go-skynet/LocalAI#example-use-gpt4all-j-model).
You can define more LocalAI models and endpoints with `mods --settings`.

### Groq

Groq provides some models powered by their LPU inference engine.
Mods will work with both their models (`mixtral-8x7b-32768` and
`llama2-70b-4096`).
Set the `GROQ_API_KEY` environment variable to a valid key, which you can get
[from here](https://console.groq.com/keys).

### Install Mods

```bash
# macOS or Linux
brew install charmbracelet/tap/mods

# Windows (with Winget)
winget install charmbracelet.mods

# Windows (with Scoop)
scoop bucket add charm https://github.com/charmbracelet/scoop-bucket.git
scoop install mods

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

- [Packages][releases] are available in Debian and RPM formats
- [Binaries][releases] are available for Linux, macOS, and Windows

[releases]: https://github.com/charmbracelet/mods/releases

Or, just install it with `go`:

```sh
go install github.com/charmbracelet/mods@latest
```

## Saving conversations

Conversations save automatically. They are identified by their latest prompt.
Similar to Git, conversations have a SHA-1 identifier and a title. Conversations
can be updated, maintaining their SHA-1 identifier but changing their title.

Check the [features document](./features.md) for more details.

<p><img src="https://vhs.charm.sh/vhs-6MMscpZwgzohYYMfTrHErF.gif" width="900" alt="a GIF listing and showing saved conversations."></p>

## Settings

`--settings`

Mods lets you tune your query with a variety of settings. You can configure
Mods with `mods --settings` or pass the settings as environment variables
and flags.

#### Dirs

`--dirs`

Prints the local directories used by Mods to store its data.
Useful if you want to back your conversations up, for example.

#### Model

`-m`, `--model`, `MODS_MODEL`

Mods uses `gpt-4` with OpenAI by default, but you can specify any model as long
as your account has access to it or you have installed locally with LocalAI.

You can add new models to the settings with `mods --settings`.
You can also specify a model and an API endpoint with `-m` and `-a`
to use models not in the settings file.

#### Ask Model

`-M` `--ask-model`

Ask which model to use with an interactive prompt.

#### Title

`-t`, `--title`

Set a custom save title for the conversation.

#### Continue last

`-C`, `--continue-last`

Continues the previous conversation.

#### Continue

`-c`, `--continue`

Continue from the last response or a given title or SHA1.

#### List

`-l`, `--list`

Lists all saved conversations.

#### Show last

`-S`, `--show-last`

Show the previous conversation.

#### Show

`-s`, `--show`

Show the saved conversation the given title or SHA1.

#### Delete

`--delete`

Deletes the saved conversation with the given title or SHA1.

`--delete-older-than=duration`

Delete conversations older than the given duration (e.g. `10d`, `3w`, `1mo`,
`1y`).

If the terminal is interactive, it'll first list the conversations to be deleted
and then will ask for confirmation.

If the terminal is not interactive, or if `--quiet` is provided, it'll delete
the conversations without any confirmation.

#### Format

`-f`, `--format`, `MODS_FORMAT`

Ask the LLM to format the response in a given format.
You can edit the text passed to the LLM with `mods --settings` then changing the
`format-text` value.
You'll likely want to use this in with `--format-as`.

#### Format As

`--format-as`, `MODS_FORMAT_AS`

When `--format` is on, instructs the LLM about which format you want the output
to be.
This can be customized with `mods --settings`.

#### Role

`--role`, `MODS_ROLE`

You can have customized roles in your settings file, which will be fed to the
LLM as system messages in order to change its behavior.
The `--role` flag allows you to change which of these custom roles to use.

#### Raw

`-r`, `--raw`, `MODS_RAW`

Print the raw response without syntax highlighting, even when connect to a TTY.

#### Max Tokens

`--max-tokens`, `MODS_MAX_TOKENS`

Max tokens tells the LLM to respond in less than this number of tokens. LLMs
are better at longer responses so values larger than 256 tend to work best.

#### Temperature

`--temp`, `MODS_TEMP`

Sampling temperature is a number between 0.0 and 2.0 and determines how
confident the model is in its choices. Higher values make the output more
random and lower values make it more deterministic.

#### Stop

`--stop`, `MODS_STOP`

Up to 4 sequences where the API will stop generating further tokens.

#### Top P

`--topp`, `MODS_TOPP`

Top P is an alternative to sampling temperature. It's a number between 0.0 and
2.0 with smaller numbers narrowing the domain from which the model will create
its response.

#### No Limit

`--no-limit`, `MODS_NO_LIMIT`

By default, Mods attempts to size the input to the maximum size the allowed by
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

Only output errors to standard err. Hides the spinner and success messages
that would go to standard err.

#### Reset Settings

`--reset-settings`

Backup your old settings file and reset everything to the defaults.

#### No Cache

`--no-cache`, `MODS_NO_CACHE`

Disables conversation saving.

#### Wrap Words

`--word-wrap`, `MODS_WORD_WRAP`

Wrap formatted output at specific width (default is 80)

#### HTTP Proxy

`-x`, `--http-proxy`, `MODS_HTTP_PROXY`

Use the HTTP proxy to the connect the API endpoints.

## Defining custom roles

Roles allow you to set system prompts, for instance, you might want a `shell`
role that instructs the LLM to output shell commands instead of explanations.

In mods, roles are defined in the settings file[^settings]:

```yaml
roles:
  "shell":
    - you are a shell expert
    - you do not explain anything
    - you simply output one liners to solve the problems you're asked
    - you do not provide any explanation whatsoever, ONLY the command
```

You can then use said role as:

```sh
mods --role shell list files in the current directory
```

You can verify that the system prompts were set by running `mods -S`.

Roles can also be loaded from URIs, e.g.:

```yaml
roles:
  "create_summary":
    - https://raw.githubusercontent.com/danielmiessler/fabric/main/patterns/create_summary/system.md
  "create_summary_2":
    - file:///fabric/patterns/create_summary/system.md
```

[^settings]: You can edit it by running `mods --settings`.

## Using within Vim/neovim

You can use mods as an assistant inside Vim.
Here are some examples:

1. `:'<,'>w !mods explain this`
1. `:.!mods -f write a copyright footer for mycompany, 2024`
1. `:'<,'>.!mods improve this code`

You can also add user commands for common actions, for example:

```vim
command! -range -nargs=0 ModsExplain :'<,'>w !mods explain this, be very succint
command! -range -nargs=* ModsRefactor :'<,'>!mods refactor this to improve its readability
command! -range -nargs=+ Mods :'<,'>w !mods <q-args>
```

This allows you to visual select some test, and run `:ModsExplain`,
`:ModsRefactor`, and `:Mods your prompt`.

## Whatcha Think?

We’d love to hear your thoughts on this project. Feel free to drop us a note.

- [Twitter](https://twitter.com/charmcli)
- [The Fediverse](https://mastodon.social/@charmcli)
- [Discord](https://charm.sh/chat)

## License

[MIT](https://github.com/charmbracelet/mods/raw/main/LICENSE)

---

Part of [Charm](https://charm.sh).

<a href="https://charm.sh/"><img alt="The Charm logo" width="400" src="https://stuff.charm.sh/charm-badge.jpg" /></a>

<!--prettier-ignore-->
Charm热爱开源 • Charm loves open source
