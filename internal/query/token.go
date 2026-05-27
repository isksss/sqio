package query

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Tokens returns lowercase SQL tokens after comments and literal contents have
// been scrubbed. It is intentionally conservative, but avoids matching keywords
// hidden inside strings, comments, or identifier substrings.
func Tokens(sql string) []string {
	text := AnalysisText(sql)
	tokens := []string{}
	for i := 0; i < len(text); {
		r, width := utf8.DecodeRuneInString(text[i:])
		if isTokenRune(r) {
			start := i
			i += width
			for i < len(text) {
				next, nextWidth := utf8.DecodeRuneInString(text[i:])
				if !isTokenRune(next) {
					break
				}
				i += nextWidth
			}
			tokens = append(tokens, strings.ToLower(text[start:i]))
			continue
		}
		if strings.ContainsRune("(),.*=<>+-/", r) {
			tokens = append(tokens, string(r))
		}
		i += width
	}
	return tokens
}

func isTokenRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func HasToken(tokens []string, token string) bool {
	token = strings.ToLower(token)
	for _, got := range tokens {
		if got == token {
			return true
		}
	}
	return false
}

func HasTokenSequence(tokens []string, sequence ...string) bool {
	if len(sequence) == 0 || len(tokens) < len(sequence) {
		return false
	}
	for i := 0; i <= len(tokens)-len(sequence); i++ {
		matched := true
		for j, want := range sequence {
			if tokens[i+j] != strings.ToLower(want) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
