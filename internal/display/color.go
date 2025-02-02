// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package display

import "fmt"

type color uint

func (c color) Code() string {
	switch c {
	case reset:
		return "\033[0m"
	case red:
		return "\033[31m"
	case green:
		return "\033[32m"
	case yellow:
		return "\033[33m"
	case blue:
		return "\033[34m"
	case magenta:
		return "\033[35m"
	case cyan:
		return "\033[36m"
	case gray:
		return "\033[37m"
	case white:
		return "\033[97m"
	default:
		return ""
	}
}

const (
	reset color = iota
	red
	green
	yellow
	blue
	magenta
	cyan
	gray
	white
)

type colorerFunc = func(string, color) string

var colorer colorerFunc = colorerOn //nolint:revive

func colorerOn(s string, c color) string {
	return fmt.Sprintf("%s%s%s", c.Code(), s, reset.Code())
}

func colorerOff(s string, _ color) string {
	return s
}

func EnableColor() {
	colorer = colorerOn
}

func DisableColor() {
	colorer = colorerOff
}
