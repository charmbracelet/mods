# Mods

<p>
    <img src="https://github.com/charmbracelet/mods/assets/25087/5442bf46-b908-47af-bf4e-60f7c38951c4" width="630" alt="Mods product art and type treatment"/>
    <br>
    <a href="https://github.com/charmbracelet/mods/releases"><img src="https://img.shields.io/github/release/charmbracelet/mods.svg" alt="Latest Release"></a>
    <a href="https://github.com/charmbracelet/mods/actions"><img src="https://github.com/charmbracelet/mods/workflows/build/badge.svg" alt="Build Status"></a>
</p>

AI for the command line, built for pipelines.

<p><img src="https://vhs.charm.sh/vhs-5Uyj0U6Hlqi1LVIIRyYKM5.gif" width="900" alt="a GIF of mods running"></p>

Large Language Models (LLM) based AI is useful to ingest command output and
format results in Markdown, JSON, and other text based formats. Mods is a
tool to add a sprinkle of AI in your command line and make your pipelines
artificially intelligent.

It works great with LLMs running locally through [LocalAI]. You can also use
[OpenAI], [Cohere], [Groq], or [Azure OpenAI].

[LocalAI]: https://github.com/go-skynet/LocalAI
[OpenAI]: https://platform.openai.com/account/api-keys
[Cohere]: https://dashboard.cohere.com/api-keys
[Groq]: https://console.groq.com/keys
[Azure OpenAI]: https://azure.microsoft.com/en-us/products/cognitive-services/openai-service

### Installation

Use a package manager:

```bash
# macOS or Linux
brew install charmbracelet/tap/mods

# Windows (with Winget)
winget install charmbracelet.mods

# Arch Linux (btw)
yay -S mods

# Nix
nix-shell -p mods
```

<details>
<summary>Debian/Ubuntu</summary>

```bash
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://repo.charm.sh/apt/gpg.key | sudo gpg --dearmor -o /etc/apt/keyrings/charm.gpg
echo "deb [signed-by=/etc/apt/keyrings/charm.gpg] https://repo.charm.sh/apt/ * *" | sudo tee /etc/apt/sources.list.d/charm.list
sudo apt update && sudo apt install mods
```

</details>

<details>
<summary>Fedora/RHEL</summary>

```bash
echo '[charm]
name=Charm
baseurl=https://repo.charm.sh/yum/
enabled=1
gpgcheck=1
gpgkey=https://repo.charm.sh/yum/gpg.key' | sudo tee /etc/yum.repos.d/charm.repo
sudo yum install mods
```

</details>

Or, download it:

- [Packages][releases] are available in Debian and RPM formats
- [Binaries][releases] are available for Linux, macOS, and Windows

[releases]: https://github.com/charmbracelet/mods/releases

Or, just install it with `go`:

```sh
go install github.com/charmbracelet/mods@latest
```

<details>
<summary>Shell Completions</summary>

All the packages and archives come with pre-generated completion files for Bash,
ZSH, Fish, and PowerShell.

If you built it from source, you can generate them with:

```bash
mods completion bash -h
mods completion zsh -h
mods completion fish -h
mods completion powershell -h
```

If you use a package (like Homebrew, Debs, etc), the completions should be set
up automatically, given your shell is configured properly.

</details>

## What Can It Do?

Mods works by reading standard in and prefacing it with a prompt supplied in
the `mods` arguments. It sends the input text to an LLM and prints out the
result, optionally asking the LLM to format the response as Markdown. This
gives you a way to "question" the output of a command. Mods will also work on
standard in or an argument supplied prompt individually.

Be sure to check out the [examples](examples.md) and a list of all the
[features](features.md).

Mods works with OpenAI compatible endpoints. By default, Mods is configured to
support OpenAI's official API and a LocalAI installation running on port 8080.
You can configure additional endpoints in your settings file by running
`mods --settings`.

## Saved Conversations

Conversations are saved locally by default. Each conversation has a SHA-1
identifier and a title (like `git`!).

<p>
  <img src="https://vhs.charm.sh/vhs-6MMscpZwgzohYYMfTrHErF.gif" width="900" alt="a GIF listing and showing saved conversations.">
</p>

Check the [`./features.md`](./features.md) for more details.

## Usage

- `-m`, `--model`: Specify Large Language Model to use.
- `-f`, `--format`: Ask the LLM to format the response in a given format.
- `--format-as`: Specify the format for the output (used with `--format`).
- `-P`, `--prompt` Include the prompt from the arguments and stdin, truncate stdin to specified number of lines.
- `-p`, `--prompt-args`: Include the prompt from the arguments in the response.
- `-q`, `--quiet`: Only output errors to standard err.
- `-r`, `--raw`: Print raw response without syntax highlighting.
- `--settings`: Open settings.
- `-x`, `--http-proxy`: Use HTTP proxy to connect to the API endpoints.
- `--max-retries`: Maximum number of retries.
- `--max-tokens`: Specify maximum tokens with which to respond.
- `--no-limit`: Do not limit the response tokens.
- `--role`: Specify the role to use (See [custom roles](#custom-roles)).
- `--word-wrap`: Wrap output at width (defaults to 80)
- `--reset-settings`: Restore settings to default.

#### Conversations

- `-t`, `--title`: Set the title for the conversation.
- `-l`, `--list`: List saved conversations.
- `-c`, `--continue`: Continue from last response or specific title or SHA-1.
- `-C`, `--continue-last`: Continue the last conversation.
- `-s`, `--show`: Show saved conversation for the given title or SHA-1.
- `-S`, `--show-last`: Show previous conversation.
- `--delete-older-than=<duration>`: Deletes conversations older than given duration (`10d`, `1mo`).
- `--delete`: Deletes the saved conversations for the given titles or SHA-1s.
- `--no-cache`: Do not save conversations.

#### Advanced

- `--fanciness`: Level of fanciness.
- `--temp`: Sampling temperature.
- `--topp`: Top P value.
- `--topk`: Top K value.

## Custom Roles

Roles allow you to set system prompts. Here is an example of a `shell` role:

```yaml
roles:
  shell:
    - you are a shell expert
    - you do not explain anything
    - you simply output one liners to solve the problems you're asked
    - you do not provide any explanation whatsoever, ONLY the command
```

Then, use the custom role in `mods`:

```sh
mods --role shell list files in the current directory
```

## Setup

### Open AI

Mods uses GPT-4 by default. It will fall back to GPT-3.5 Turbo.

Set the `OPENAI_API_KEY` environment variable. If you don't have one yet, you
can grab it the [OpenAI website](https://platform.openai.com/account/api-keys).

Alternatively, set the [`AZURE_OPENAI_KEY`] environment variable to use Azure
OpenAI. Grab a key from [Azure](https://azure.microsoft.com/en-us/products/cognitive-services/openai-service).

### Cohere

Cohere provides enterprise optimized models.

Set the `COHERE_API_KEY` environment variable. If you don't have one yet, you can
get it from the [Cohere dashboard](https://dashboard.cohere.com/api-keys).

### Local AI

Local AI allows you to run models locally. Mods works with the GPT4ALL-J model
as setup in [this tutorial](https://github.com/go-skynet/LocalAI#example-use-gpt4all-j-model).

### Groq

Groq provides models powered by their LPU inference engine.

Set the `GROQ_API_KEY` environment variable. If you don't have one yet, you can
get it from the [Groq console](https://console.groq.com/keys).

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
