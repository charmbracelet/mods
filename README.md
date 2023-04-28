# Mods

GPT for the command line, built for pipelines.

[demo gif]

GPT models are really good at interpreting the output of commands and returning
their results in CLI friendly text formats like Markdown. Mods is a simple tool
that makes it super easy to use GPT models on the command line and in your
pipelines.

To get started, [install Mods]() and check out some of the examples below.
Since Mods has built-in Markdown formatting, you may also want to grab
[Glow](https://github.com/charmbracelet/glow) to give the output some _pizzazz_.

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

`mods -f "refactor this code" < main.go | glow`

### Come Up With Product Features

Mods can also come up with entirely new features based on source code (or a
README file).

`mods -f "come up with 10 new features for this tool." < main.go | glow`

### Help Write Docs

Mods can quickly give you a first draft for new documentation.

`mods "write a new section to this readme for a feature that sends you a free rabbit if you hit 'r'" | glow`

### Organize Your Videos

The file system can be an amazing source of input for Mods. If you have music
or video files, Mods can parse the output of `ls` and offer really good
editorialization of your content.

`ls ~/vids | mods -f "organize these by decade and supply a single sentence summary of each" | glow`

### Make Recommendations

Mods is really good at generating recommendations based on what you have as
well, both for similar content but also content in an entirely different media
(like getting music recommendations based on movies you have).

`ls ~/vids | mods -f "recommend me 10 shows based on these, make them obscure" | glow`

`ls ~/vids | mods -f "recommend me 10 albums based on these shows, do not include any soundtrack music or music from the show" | glow`

### Read Your Fortune

It's easy to let your downloads folder grow into a chaotic never-ending pit of
files, but with Mods you can use that to your advantage!

`ls ~/Downloads | mods -f "tell my fortune based on these files" | glow`

### Understand APIs

Mods can parse and understand the output of an API call with `curl` and convert
it to something human readable.

`curl "https://api.open-meteo.com/v1/forecast?latitude=29.00&longitude=-90.00&current_weather=true&hourly=temperature_2m,relativehumidity_2m,windspeed_10m" 2>/dev/null | mods -f "summarize this weather data for a human." | glow`

### Read The Comments (so you don't have to)

Just like with APIs, Mods can read through raw HTML and summarize the contents.

`curl "https://news.ycombinator.com/item?id=30048332" 2>/dev/null | mods -f "what are the authors of these comments saying?" | glow`

## Installation

Mods currently works with OpenAI's models, so you'll need to set the
`OPENAI_API_KEY` environment variable to a valid OpenAI key, which you can get
[from here](https://platform.openai.com/account/api-keys).

## License

[MIT](https://github.com/charmbracelet/mods/raw/main/LICENSE)

***

Part of [Charm](https://charm.sh).

<a href="https://charm.sh/"><img alt="The Charm logo" width="400" src="https://stuff.charm.sh/charm-badge.jpg" /></a>

Charm热爱开源 • Charm loves open source
