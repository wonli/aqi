package aqi

import (
	"strings"
	"time"

	"github.com/fatih/color"
)

var (
	Branch        string
	Revision      string
	BuildDate     string
	CommitVersion string
)

var asciiLogo = `
%s Started on %s

██▀▄─██─▄▄▄─█▄─▄█   Branch   : %s-%s
██─▀─██─██▀─██─██   Commit   : %s
▀▄▄▀▄▄▀───▄▄▀▄▄▄▀   Build at : %s

`

func AsciiLogo(serverName ...string) {
	color.Cyan(asciiLogo,
		strings.TrimSpace(strings.Join(serverName, " ")),
		time.Now().Format("2006-01-02 15:04:05"),
		Branch,
		Revision,
		CommitVersion,
		BuildDate,
	)
}
