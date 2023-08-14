# Mods spec

## Regular usage

By default:

- all messages go to `STDERR`
- all prompts are saved with the first line of the prompt as the title
- glamour is used by default

### Basic

The most basic usage is:

```bash
mods 'first 2 primes'
```

### Pipe from

You can also pipe to it, in which case `STDIN` will not be a TTY:

```bash
echo 'as json' | mods 'first 2 primes'
```

In this case, `mods` should read `STDIN` and append it to the prompt.

### Pipe to

You may also pipe the output to another program, in which case `STDOUT` will not
be a TTY:

```bash
echo 'as json' | mods 'first 2 primes' | jq .
```

In this case, the "Generating" animation will go to `STDERR`, but the response
will be streamed to `STDOUT`.

### Custom save title

You can set a custom save message:

```bash
mods --save='title' 'first 2 primes'
```

### Continue latest

You can continue the latest conversation and save it with a new title using
`--continue=title`:

```bash
mods 'first 2 primes'
mods --continue='primes as json' 'format as json'
```

### Untitled continue latest

```bash
mods 'first 2 primes'
mods --continue-last 'format as json'
```

### Continue from specific conversation, save with a new title

```bash
mods --save='naturals' 'first 5 natural numbers'
mods --continue='naturals' --save='naturals.json' 'format as json'
```

### Conversation branching

You can use the `--continue` and `--save` to branch out conversations, for
instance:

```bash
mods --save='naturals' 'first 5 natural numbers'
mods --continue='naturals' --save='naturals.json' 'format as json'
mods --continue='naturals' --save='naturals.yaml' 'format as yaml'
```

With this you'll end up with 3 conversations: `naturals`, `naturals.json`, and
`naturals.yaml`.

## List conversations

You can list your previous conversations with:

```bash
mods --list
# or
mods -l
```

## Show a previous conversation

You can also show a previous conversation by ID or title, e.g.:

```bash
mods --show='naturals'
mods --show='a2e2'
```

For titles, the match should be exact.
For IDs, only the first 4 chars are needed. If it matches multiple
conversations, you can add more chars until it matches a single one again.

## Delete a conversation

You can also delete conversations by title or ID, same as `--show`, different
flag:

```bash
mods --delete='naturals'
mods --delete='a2e2'
```

Keep in mind that these operations are not reversible.
