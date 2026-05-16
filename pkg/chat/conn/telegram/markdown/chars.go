package markdown

import (
	"reflect"
	"unsafe"
)

// SpecialRune define custom rune object.
type SpecialRune rune

// Rune from SpecialRune.
func (sr SpecialRune) Rune() rune {
	return rune(sr)
}

// SpecialChar define custom byte object.
type SpecialChar byte

// Byte from SpecialChar.
func (sc SpecialChar) Byte() byte {
	return byte(sc)
}

// Escaped return SpecialChar as escaped byte char.
func (sc SpecialChar) Escaped() []byte {
	return append([]byte{SlashChar.Byte()}, sc.Byte())
}

// SpecialTag define Markdown formatting characters.
type SpecialTag []SpecialChar

// Bytes from SpecialTags.
func (st SpecialTag) Bytes() []byte {
	header := *(*reflect.SliceHeader)(unsafe.Pointer(&st))
	header.Len *= int(unsafe.Sizeof(SpecialChar(0)))
	header.Cap *= int(unsafe.Sizeof(SpecialChar(0)))
	return *(*[]byte)(unsafe.Pointer(&header))
}

// define characters.
const (
	UnderscoreChar   SpecialChar = '_'
	AsteriskChar     SpecialChar = '*'
	OpenBracketChar  SpecialChar = '['
	CloseBracketChar SpecialChar = ']'
	OpenParenChar    SpecialChar = '('
	CloseParenChar   SpecialChar = ')'
	OpenBraceChar    SpecialChar = '{'
	CloseBraceChar   SpecialChar = '}'
	HashChar         SpecialChar = '#'
	PlusChar         SpecialChar = '+'
	MinusChar        SpecialChar = '-'
	EqualChar        SpecialChar = '='
	DotChar          SpecialChar = '.'
	TildeChar        SpecialChar = '~'
	PipeChar         SpecialChar = '|'
	ExclamationChar  SpecialChar = '!'
	GreaterThanChar  SpecialChar = '>'
	LessThanChar     SpecialChar = '<'
	BackqouteChar    SpecialChar = '`'
	SpaceChar        SpecialChar = ' '
	NewLineChar      SpecialChar = '\n'
	SlashChar        SpecialChar = '\\'
	TabChar          SpecialChar = '\t'
)

// define symbols.
const (
	CircleSymbol   SpecialRune = '•'
	TriangleSymbol SpecialRune = '⁃'
	SquareSymbol   SpecialRune = '‣'
)

// define Telegram MarkdownV2 formatting tags.
// Spec: https://core.telegram.org/bots/api#markdownv2-style
// *bold text* (single asterisk)
// _italic text_ (single underscore)
// __underline__ (double underscore)
// ~strikethrough~ (single tilde)
// ||spoiler|| (double pipe)
// `inline code` (single backtick)
// ```\npre-formatted code\n``` (triple backtick for blocks)
var (
	BoldTg          SpecialTag = []SpecialChar{AsteriskChar}
	StrikethroughTg SpecialTag = []SpecialChar{TildeChar}
	UnderlineTg     SpecialTag = []SpecialChar{UnderscoreChar, UnderscoreChar}
	HiddenTg        SpecialTag = []SpecialChar{PipeChar, PipeChar}
	ItalicsTg       SpecialTag = []SpecialChar{UnderscoreChar}
	CodeTg          SpecialTag = []SpecialChar{BackqouteChar, BackqouteChar, BackqouteChar}
	SpanTg          SpecialTag = []SpecialChar{BackqouteChar}
)

// define escape map.
// Telegram MarkdownV2 spec:
// - In all other places: _, *, [, ], (, ), ~, `, >, #, +, -, =, |, {, }, ., ! must be escaped
// - Inside pre/code: all ` and \ must be escaped
// - Inside link URL: ) and \ must be escaped
// - \ itself must always be escaped as \\
var escape = map[byte][]byte{
	SlashChar.Byte():        SlashChar.Escaped(),
	UnderscoreChar.Byte():   UnderscoreChar.Escaped(),
	AsteriskChar.Byte():     AsteriskChar.Escaped(),
	OpenBracketChar.Byte():  OpenBracketChar.Escaped(),
	CloseBracketChar.Byte(): CloseBracketChar.Escaped(),
	OpenParenChar.Byte():    OpenParenChar.Escaped(),
	CloseParenChar.Byte():   CloseParenChar.Escaped(),
	OpenBraceChar.Byte():    OpenBraceChar.Escaped(),
	CloseBraceChar.Byte():   CloseBraceChar.Escaped(),
	HashChar.Byte():         HashChar.Escaped(),
	PlusChar.Byte():         PlusChar.Escaped(),
	MinusChar.Byte():        MinusChar.Escaped(),
	EqualChar.Byte():        EqualChar.Escaped(),
	DotChar.Byte():          DotChar.Escaped(),
	ExclamationChar.Byte():  ExclamationChar.Escaped(),
	GreaterThanChar.Byte():  GreaterThanChar.Escaped(),
	LessThanChar.Byte():     LessThanChar.Escaped(),
	TildeChar.Byte():        TildeChar.Escaped(),
	PipeChar.Byte():         PipeChar.Escaped(),
	BackqouteChar.Byte():    BackqouteChar.Escaped(),
}

// escapeCode is the escape map for content inside code/pre blocks.
// Only ` and \ need to be escaped inside code.
var escapeCode = map[byte][]byte{
	SlashChar.Byte():     SlashChar.Escaped(),
	BackqouteChar.Byte(): BackqouteChar.Escaped(),
}

// escapeURL is the escape map for content inside link URLs.
// Only ) and \ need to be escaped inside URLs.
var escapeURL = map[byte][]byte{
	SlashChar.Byte():      SlashChar.Escaped(),
	CloseParenChar.Byte(): CloseParenChar.Escaped(),
}
