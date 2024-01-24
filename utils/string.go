package utils

import "strings"

func NormStringOpt(opt string) string {
	return strings.TrimSpace(strings.ToLower(opt))
}