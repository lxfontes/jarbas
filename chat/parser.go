package chat

import (
	"errors"
	"strings"
	"unicode/utf8"
)

const (
	marker = byte('\x05')
)

func SplitMarker(s string) (string, string) {
	parts := strings.SplitN(s, string(marker), 2)
	return parts[0], parts[1]
}

func HasMarker(s string) bool {
	return strings.Contains(s, string(marker))
}

// ScanQuotedWords parses string key value pairs as "this"="that"
// Guarantees:
// - Only one separator (=) in word
// - All quotes are balanced
func ScanQuotedWords(data []byte, atEOF bool) (int, []byte, error) {
	// Skip leading spaces.
	start := 0
	for width := 0; start < len(data); start += width {
		var r rune
		r, width = utf8.DecodeRune(data[start:])
		if !isSpace(r) {
			break
		}
	}

	token := []byte{}

	seenSeparator := false
	quotes := []rune{}
	for width, i := 0, start; i < len(data); i += width {
		var r rune
		r, width = utf8.DecodeRune(data[i:])

		if r == rune(marker) {
			return 0, nil, errors.New("contains split marker")
		}

		if isQuote(r) {
			// closing quotes
			if len(quotes) > 0 {
				lastQuote := quotes[0]

				if lastQuote == r {
					quotes = quotes[1:]
					continue
				}
			}

			// starting quotes, add to back
			quotes = append([]rune{r}, quotes...)
			continue
		}

		insideQuotes := len(quotes) > 0

		if insideQuotes {
			// we are inside quotation marks
			token = append(token, byte(r))
			continue
		}

		if isSeparator(r) {
			if seenSeparator {
				// error, tryint to do this: this==aaaa or this=a=aaa
				return 0, nil, errors.New("double separator")
			}
			seenSeparator = true
			token = append(token, marker)
			continue
		}

		if isSpace(r) {
			return i + width, token, nil
		}

		token = append(token, byte(r))
	}

	// If we're at EOF, we have a final, non-empty, non-terminated word. Return it.
	if atEOF && len(data) > start {
		// we did not close all nested quotations
		if len(quotes) > 0 {
			return 0, nil, errors.New("double separator")
		}
		return len(data), token, nil
	}

	// Request more data.
	return start, nil, nil
}

func isBackslash(r rune) bool {
	return r == '\\'
}

func isSeparator(r rune) bool {
	return r == '='
}

func isQuote(r rune) bool {
	return r == '\'' || r == '"'
}

func isSpace(r rune) bool {
	if r <= '\u00FF' {
		// Obvious ASCII ones: \t through \r plus space. Plus two Latin-1 oddballs.
		switch r {
		case ' ', '\t', '\n', '\v', '\f', '\r':
			return true
		case '\u0085', '\u00A0':
			return true
		}
		return false
	}
	// High-valued ones.
	if '\u2000' <= r && r <= '\u200a' {
		return true
	}
	switch r {
	case '\u1680', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000':
		return true
	}
	return false
}
