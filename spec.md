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

### Continue

You can

### Conversation branchinb

## List conversations

## Show a previous conversation

## Delete a conversation
