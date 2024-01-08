package main

import (
	"regexp"
	"strings"
	"time"

	"github.com/caarlos0/duration"
)

func newFlagParseError(err error) flagParseError {
	var reason, flag string
	s := err.Error()
	switch {
	case strings.HasPrefix(s, "flag needs an argument:"):
		reason = "Flag %s needs an argument."
		ps := strings.Split(s, "-")
		switch len(ps) {
		case 2: //nolint:gomnd
			flag = "-" + ps[len(ps)-1]
		case 3: //nolint:gomnd
			flag = "--" + ps[len(ps)-1]
		}
	case strings.HasPrefix(s, "unknown flag:"):
		reason = "Flag %s is missing."
		flag = strings.TrimPrefix(s, "unknown flag: ")
	case strings.HasPrefix(s, "invalid argument"):
		reason = "Flag %s have an invalid argument."
		re := regexp.MustCompile(`invalid argument ".*" for "(.*)" flag: .*`)
		parts := re.FindStringSubmatch(s)
		if len(parts) > 1 {
			flag = parts[1]
		}
	default:
		reason = s
	}
	return flagParseError{
		err:    err,
		reason: reason,
		flag:   flag,
	}
}

type flagParseError struct {
	err    error
	reason string
	flag   string
}

func (f flagParseError) Error() string {
	return f.err.Error()
}

func (f flagParseError) ReasonFormat() string {
	return f.reason
}

func (f flagParseError) Flag() string {
	return f.flag
}

func newDurationFlag(val time.Duration, p *time.Duration) *durationFlag {
	*p = val
	return (*durationFlag)(p)
}
