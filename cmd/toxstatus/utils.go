package main

import (
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

func stripSpaces(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

func getSimpleDurationFormat(duration time.Duration) string {
	hours := duration.Hours()
	var format string

	if hours >= 24 {
		format = fmt.Sprintf("%.0f day", hours/24)
	} else if hours > 1 {
		format = fmt.Sprintf("%.0f hour", hours)
	} else if hours*60 > 1 {
		format = fmt.Sprintf("%.0f minute", hours*60)
	} else {
		format = fmt.Sprintf("%.0f second", hours*60*60)
	}

	if strings.Split(format, " ")[0] != "1" {
		format += "s"
	}

	return format
}

func getTimeSinceString(stamp int64) string {
	if stamp == 0 {
		return "Never"
	}

	return getSimpleDurationFormat(time.Now().Sub(time.Unix(stamp, 0)))
}

func getTimeString(stamp int64) string {
	return time.Unix(stamp, 0).String()
}

func increment(i int) int {
	return i + 1
}

func getLocString(loc string) string {
	return countries[loc]
}

func mimeTypeByExtension(ext string) string {
	t := mime.TypeByExtension(filepath.Ext(ext))
	if t == "" {
		t = "application/octet-stream"
	}
	return t
}
