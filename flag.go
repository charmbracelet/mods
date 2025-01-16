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
		case 2: //nolint:mnd
			flag = "-" + ps[len(ps)-1]
		case 3: //nolint:mnd
			flag = "--" + ps[len(ps)-1]
		}
	case strings.HasPrefix(s, "unknown flag:"):
		reason = "Flag %s is missing."
		flag = strings.TrimPrefix(s, "unknown flag: ")
	case strings.HasPrefix(s, "unknown shorthand flag:"):
		reason = "Short flag %s is missing."
		re := regexp.MustCompile(`unknown shorthand flag: '.*' in (-\w)`)
		parts := re.FindStringSubmatch(s)
		if len(parts) > 1 {
			flag = parts[1]
		}
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

type durationFlag time.Duration

func (d *durationFlag) Set(s string) error {
	v, err := duration.Parse(s)
	*d = durationFlag(v)
	//nolint: wrapcheck
	return err
}

func (d *durationFlag) String() string {
	return time.Duration(*d).String()
}

func (*durationFlag) Type() string {
	return "duration"
}
