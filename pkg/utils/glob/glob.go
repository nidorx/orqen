package glob

import (
	"regexp"
	"strings"
	"time"

	"github.com/nidorx/orqen/pkg/utils/tinylfu"
)

var cache = tinylfu.NewSyncCacheT[*regexp.Regexp](500, 100000, 24*time.Hour)

func IsGlob(pattern string) bool {
	return strings.ContainsAny(pattern, "*{}[]?")
}

func Cached(pattern string) *regexp.Regexp {
	reg, _ := cache.GetOrSet(pattern, func() (*regexp.Regexp, error) {
		return Glob(pattern), nil
	})
	return reg
}

// Glob converts a glob pattern to a compiled regexp.
//
// Supported glob features:
//   - *  matches any sequence of non-separator characters
//   - ** matches any sequence of characters, including separators (recursive)
//   - ?  matches any single non-separator character
//   - [abc] matches any character in the set
//   - {a,b} matches any of the alternatives
//   - . and other regex metacharacters are escaped automatically
//
// Path separators: both "/" and "\" are treated as separators.
// Matching is case-insensitive.
func Glob(pattern string) *regexp.Regexp {
	// Normalize backslashes to forward slashes
	pattern = strings.ReplaceAll(pattern, "\\", "/")

	var sb strings.Builder
	sb.WriteString("(?i)^") // case-insensitive, anchored at start

	i := 0
	n := len(pattern)

	for i < n {
		ch := pattern[i]

		switch ch {
		case '*':
			// Check for **
			if i+1 < n && pattern[i+1] == '*' {
				// Consume both stars
				i += 2
				// Consume trailing separator if present
				if i < n && pattern[i] == '/' {
					i++
				}
				sb.WriteString(".*")
			} else {
				i++
				sb.WriteString("[^/]*")
			}

		case '?':
			i++
			sb.WriteString("[^/]")

		case '[':
			// Find closing bracket
			j := i + 1
			// Handle negation [!...]
			if j < n && pattern[j] == '!' {
				j++
			}
			// First char after [ or [! can be ]
			if j < n && pattern[j] == ']' {
				j++
			}
			// Find the closing ]
			for j < n && pattern[j] != ']' {
				if pattern[j] == '\\' {
					j++ // skip escaped char
				}
				j++
			}
			if j < n {
				// Found closing bracket
				bracket := pattern[i : j+1]
				// Convert [!...] to [^...]
				if strings.HasPrefix(bracket, "[!") {
					bracket = "[^" + bracket[2:]
				}
				sb.WriteString(bracket)
				i = j + 1
			} else {
				// No closing bracket, treat [ as literal
				sb.WriteString("\\[")
				i++
			}

		case '{':
			// Find matching closing brace (handle nesting)
			depth := 1
			j := i + 1
			for j < n && depth > 0 {
				switch pattern[j] {
				case '{':
					depth++
				case '}':
					depth--
				}
				if depth > 0 {
					j++
				}
			}
			if j < n && depth == 0 {
				// Extract alternatives
				content := pattern[i+1 : j]
				sb.WriteString("(?:")
				sb.WriteString(convertAlternatives(content))
				sb.WriteString(")")
				i = j + 1
			} else {
				// No matching brace, treat { as literal
				sb.WriteString("\\{")
				i++
			}

		case '.', '+', '(', ')', '^', '$', '|', '\\':
			// Escape regex metacharacters
			sb.WriteString("\\")
			sb.WriteByte(ch)
			i++

		default:
			sb.WriteByte(ch)
			i++
		}
	}

	sb.WriteString("$")

	return regexp.MustCompile(sb.String())
}

// convertAlternatives converts comma-separated alternatives inside { }
// Each alternative is itself processed as a glob pattern.
func convertAlternatives(content string) string {
	var result []string
	var current strings.Builder
	depth := 0

	for i := 0; i < len(content); i++ {
		ch := content[i]
		switch ch {
		case '{':
			depth++
			current.WriteByte(ch)
		case '}':
			depth--
			current.WriteByte(ch)
		case ',':
			if depth == 0 {
				result = append(result, current.String())
				current.Reset()
			} else {
				current.WriteByte(ch)
			}
		default:
			current.WriteByte(ch)
		}
	}
	result = append(result, current.String())

	var parts []string
	for _, alt := range result {
		// Recursively convert each alternative (strip outer * if needed)
		parts = append(parts, convertAlternative(alt))
	}

	return strings.Join(parts, "|")
}

// convertAlternative converts a single alternative to its regex form.
func convertAlternative(alt string) string {
	var sb strings.Builder

	i := 0
	n := len(alt)

	for i < n {
		ch := alt[i]

		switch ch {
		case '*':
			if i+1 < n && alt[i+1] == '*' {
				i += 2
				if i < n && alt[i] == '/' {
					i++
				}
				sb.WriteString(".*")
			} else {
				i++
				sb.WriteString("[^/]*")
			}

		case '?':
			i++
			sb.WriteString("[^/]")

		case '[':
			j := i + 1
			if j < n && alt[j] == '!' {
				j++
			}
			if j < n && alt[j] == ']' {
				j++
			}
			for j < n && alt[j] != ']' {
				if alt[j] == '\\' {
					j++
				}
				j++
			}
			if j < n {
				bracket := alt[i : j+1]
				if strings.HasPrefix(bracket, "[!") {
					bracket = "[^" + bracket[2:]
				}
				sb.WriteString(bracket)
				i = j + 1
			} else {
				sb.WriteString("\\[")
				i++
			}

		case '{':
			depth := 1
			j := i + 1
			for j < n && depth > 0 {
				switch alt[j] {
				case '{':
					depth++
				case '}':
					depth--
				}
				if depth > 0 {
					j++
				}
			}
			if j < n && depth == 0 {
				content := alt[i+1 : j]
				sb.WriteString("(?:")
				sb.WriteString(convertAlternatives(content))
				sb.WriteString(")")
				i = j + 1
			} else {
				sb.WriteString("\\{")
				i++
			}

		case '.', '+', '(', ')', '^', '$', '|', '\\':
			sb.WriteString("\\")
			sb.WriteByte(ch)
			i++

		default:
			sb.WriteByte(ch)
			i++
		}
	}

	return sb.String()
}
