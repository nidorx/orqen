// Package cli provides terminal interaction utilities with AI-styled message output.
// It supports internationalization (i18n) and simulates a remote AI agent "thinking"
// by streaming messages word-by-word with small delays.
//
// All messages are prefixed with the [•_•] indicator to give the impression of
// a charismatic AI assistant conversing with the user.
//
// Locale detection is cross-platform:
//   - Linux/macOS: reads LANG / LC_ALL environment variables
//   - Windows: reads env vars first, then falls back to the Windows registry
//     for true OS-level locale detection
//
// Usage:
//
//	var msgs = cli.Messages{
//		"pt-BR": {"welcome": "Oi! Eu sou o Orqen, bem-vindo!"},
//		"en":    {"welcome": "Hi! I'm Orqen, welcome aboard!"},
//	}
//
//	cli.Printf(msgs, "welcome")
package cli

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"golang.org/x/text/language"
)

// DefaultWordDelay is the default delay between each word when streaming a message.
const DefaultWordDelay = 40 * time.Millisecond

// Prefix is the visual indicator prepended to every message, giving the
// impression of a charismatic AI assistant.
const Prefix = "[•_•] "

// Messages maps a locale string (e.g., "en", "pt-BR") to a map of message keys
// and their corresponding text values.
//
// Example:
//
//	var msgs = cli.Messages{
//		"pt-BR": {
//			"welcome": "Oi! Eu sou o Orqen, bem-vindo!",
//		},
//		"en": {
//			"welcome": "Hi! I'm Orqen, welcome aboard!",
//		},
//	}
type Messages map[string]map[string]string

// DetectLocale determines the user's preferred language using a multi-tier approach:
//
//  1. Read LANG and LC_ALL environment variables (works on all platforms where set)
//  2. On Windows, if env vars are empty, query the Windows registry for the system locale
//  3. Parse the result with golang.org/x/text/language for normalization
//  4. If the detected locale has entries in the provided messages map, return it
//  5. Attempt fuzzy matching (e.g., "pt" matches "pt-BR", "en-US" matches "en")
//  6. Fallback to "en" if nothing matches
func DetectLocale(msgs Messages) string {
	// Tier 1: environment variables
	raw := os.Getenv("LANG")
	if raw == "" {
		raw = os.Getenv("LC_ALL")
	}

	// Tier 2: Windows registry (if still empty)
	if raw == "" && isWindows() {
		raw = detectLocaleFromWindowsRegistry()
	}

	if raw == "" {
		return "en"
	}

	// Normalize: try parsing with golang.org/x/text/language
	lang, err := language.Parse(raw)
	if err != nil {
		return "en"
	}

	// Try exact base-region match (e.g., "pt-BR")
	base, _ := lang.Base()
	region, confidence := lang.Region()
	if confidence >= language.Low {
		candidate := strings.ToLower(base.String()) + "-" + region.String()
		if _, ok := msgs[candidate]; ok {
			return candidate
		}
	}

	// Try partial match: check if any key in msgs starts with the base language
	baseStr := strings.ToLower(base.String())
	for key := range msgs {
		if strings.HasPrefix(strings.ToLower(key), baseStr) {
			return key
		}
	}

	// Fallback
	return "en"
}

// Sprintf formats a message using the provided args and the auto-detected locale,
// returning the result as a string (without streaming or prefix). This is useful
// for contexts where streaming output is not appropriate (e.g., log messages,
// string composition).
//
// If the key is not found for the detected locale, it falls back to "en".
// If the key is also not found in "en", it returns "<unknown message key: %s>".
func Sprintf(messages Messages, key string, args ...any) string {
	locale := DetectLocale(messages)

	msgMap, ok := messages[locale]
	if !ok {
		msgMap = messages["en"]
	}

	text, ok := msgMap[key]
	if !ok {
		return "<unknown message key: " + key + ">"
	}

	if len(args) > 0 {
		return fmt.Sprintf(text, args...)
	}
	return text
}

// Printf formats a message using the provided args, looks it up in the messages
// map using the auto-detected locale, and streams it to stdout word-by-word
// to simulate an AI "thinking" / typing effect.
//
// The output is prefixed with [•_•] and the function blocks until the full
// message has been displayed.
//
// If the key is not found for the detected locale, it falls back to "en".
// If the key is also not found in "en", it prints "[•_•] <unknown message key>".
//
// Example:
//
//	var msgs = cli.Messages{
//		"pt-BR": {"greeting": "Olá, %s!"},
//		"en":    {"greeting": "Hello, %s!"},
//	}
//	cli.Printf(msgs, "greeting", "World")
//	// Output: [•_•] Olá, World!   (streamed word-by-word)
func Printf(messages Messages, key string, args ...any) {
	locale := DetectLocale(messages)

	// Fallback to English if locale key not found
	msgMap, ok := messages[locale]
	if !ok {
		msgMap = messages["en"]
		locale = "en"
	}

	text, ok := msgMap[key]
	if !ok {
		fmt.Printf("%s<unknown message key: %s>\n", Prefix, key)
		return
	}

	// Apply format arguments if provided
	if len(args) > 0 {
		text = fmt.Sprintf(text, args...)
	}

	// Stream word-by-word
	stream(text)
}

// stream prints the given text word-by-word with a variable delay between each word,
// simulating a remote AI agent typing/thinking. It prepends the [•_•] prefix.
// The function blocks until the entire text has been printed.
//
// Delay behavior:
//   - Normal words: random delay between 60ms and 200ms
//   - After punctuation (. ! ? ...): longer pause of 300-600ms
//   - Every ~8-12 words: no delay at all (simulating a chunk of text arriving at once)
func stream(text string) {
	lines := strings.Split(text, "\n")
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	fmt.Printf("\r%s", Prefix)

	for li, line := range lines {
		if li > 0 {
			fmt.Println()
		}

		words := strings.Fields(line)
		wordsSinceChunk := rng.Intn(5) + 8

		for i, word := range words {
			wordsSinceChunk--
			if wordsSinceChunk <= 0 {
				wordsSinceChunk = rng.Intn(5) + 8
			} else {
				prevWord := ""
				if i > 0 {
					prevWord = words[i-1]
				}
				if endsSentence(prevWord) {
					time.Sleep(time.Duration(rng.Intn(300)+300) * time.Millisecond)
				} else {
					time.Sleep(time.Duration(rng.Intn(140)+60) * time.Millisecond)
				}
			}

			if i == 0 {
				fmt.Print(word)
			} else {
				fmt.Print(" " + word)
			}
		}
	}
	time.Sleep(time.Duration(rng.Intn(300)+300) * time.Millisecond)
	fmt.Println()
}

// endsSentence returns true if the word ends with sentence-ending punctuation.
func endsSentence(word string) bool {
	if len(word) == 0 {
		return false
	}
	last := word[len(word)-1]
	return last == '.' || last == '!' || last == '?'
}
