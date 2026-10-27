package main

import (
	"fmt"
	"strings"
	"unicode"
)

// Names holds the different spellings of a user-supplied resource name.
//
// For the input "product-item", "product_item" or "ProductItem":
//
//	Pascal = "ProductItem"   (Go type names)
//	Camel  = "productItem"   (unexported Go identifiers)
//	File   = "productitem"   (file names)
//	Route  = "product-items" (URL path segment, pluralized)
type Names struct {
	Pascal string
	Camel  string
	File   string
	Route  string
}

// parseName turns a free-form name into safe identifiers. It rejects anything
// that cannot become a valid Go identifier, so a name can never escape the
// target directory (for example "../../x") or produce uncompilable code.
func parseName(raw string) (Names, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Names{}, fmt.Errorf("name must not be empty")
	}
	for _, r := range raw {
		isASCIILetter := r < unicode.MaxASCII && unicode.IsLetter(r)
		isDigit := r >= '0' && r <= '9'
		if !isASCIILetter && !isDigit && r != '-' && r != '_' && r != ' ' {
			return Names{}, fmt.Errorf("invalid name %q: use only letters, digits, '-' and '_'", raw)
		}
	}
	if first := rune(raw[0]); !(first < unicode.MaxASCII && unicode.IsLetter(first)) {
		return Names{}, fmt.Errorf("invalid name %q: it must start with a letter", raw)
	}
	words := splitWords(raw)
	if len(words) == 0 {
		return Names{}, fmt.Errorf("invalid name %q", raw)
	}
	if len(raw) > 64 {
		return Names{}, fmt.Errorf("name is too long (max 64 characters)")
	}

	var pascal strings.Builder
	for _, w := range words {
		pascal.WriteString(strings.ToUpper(w[:1]) + strings.ToLower(w[1:]))
	}
	p := pascal.String()

	lower := make([]string, len(words))
	for i, w := range words {
		lower[i] = strings.ToLower(w)
	}
	plural := append([]string(nil), lower...)
	plural[len(plural)-1] = pluralize(plural[len(plural)-1])

	return Names{
		Pascal: p,
		Camel:  strings.ToLower(p[:1]) + p[1:],
		File:   strings.Join(lower, ""),
		Route:  strings.Join(plural, "-"),
	}, nil
}

// splitWords splits on separators and on camelCase boundaries:
// "ProductItem" and "product-item" both give [Product Item]; "APIKey" gives [API Key].
func splitWords(s string) []string {
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			words = append(words, string(cur))
			cur = nil
		}
	}
	runes := []rune(s)
	for i, r := range runes {
		if r == '-' || r == '_' || r == ' ' {
			flush()
			continue
		}
		if unicode.IsUpper(r) && len(cur) > 0 {
			prev := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
				flush()
			}
		}
		cur = append(cur, r)
	}
	flush()
	return words
}

// pluralize applies the common English rules. It is only used for URL paths.
func pluralize(w string) string {
	switch {
	case strings.HasSuffix(w, "y") && len(w) > 1 && !strings.ContainsRune("aeiou", rune(w[len(w)-2])):
		return w[:len(w)-1] + "ies"
	case strings.HasSuffix(w, "s"), strings.HasSuffix(w, "x"), strings.HasSuffix(w, "z"),
		strings.HasSuffix(w, "ch"), strings.HasSuffix(w, "sh"):
		return w + "es"
	}
	return w + "s"
}
