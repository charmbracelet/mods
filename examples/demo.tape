Output mods.gif

Set Height 900
Set Width 1600
Set Framerate 24

Hide
  Type `export OPENAI_API_KEY=$(pass OPENAI_API_KEY)`
  Enter
  Ctrl+L
  Sleep 500ms
Show

Sleep 500ms

Type@25ms `curl -s `

Sleep 500ms

# Simulate pasting link.
Hide
  Type@5ms `https://api.github.com/orgs/charmbracelet/repos`
Show

Sleep 500ms

Type@25ms ` | mods -f "rate this github org and summarize each repository"`

Enter

Sleep 30s
