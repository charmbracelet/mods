# Mods Examples

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

`mods "write a new section to this readme for a feature that sends you a free rabbit if you hit r" < README.md | glow`

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
