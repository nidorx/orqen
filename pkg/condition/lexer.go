package condition

import (
	"strings"
	"unicode"
)

type lexer struct {
	input   string
	pos     int
	readPos int
	ch      byte
}

func newLexer(input string) *lexer {
	l := &lexer{input: input}
	l.readChar()
	return l
}

func (l *lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPos]
	}
	l.pos = l.readPos
	l.readPos++
}

func (l *lexer) peekChar() byte {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

func (l *lexer) skipWhitespace() {
	for unicode.IsSpace(rune(l.ch)) {
		l.readChar()
	}
}

func (l *lexer) tokenize() []Token {
	var tokens []Token

	for {
		l.skipWhitespace()

		var tok Token

		switch l.ch {
		case 0:
			tok = Token{TokenEOF, ""}
		case '(':
			tok = Token{TokenLParen, "("}
			l.readChar()
		case ')':
			tok = Token{TokenRParen, ")"}
			l.readChar()
		case ',':
			tok = Token{TokenComma, ","}
			l.readChar()
		case '\'':
			tok = l.readString()
		case '=':
			if l.peekChar() == '=' {
				l.readChar()
				tok = Token{TokenEQ, "=="}
				l.readChar()
			} else {
				tok = Token{TokenEQ, "="}
				l.readChar()
			}
		case '!':
			if l.peekChar() == '=' {
				l.readChar()
				tok = Token{TokenNE, "!="}
				l.readChar()
			} else {
				tok = Token{TokenIllegal, "!"}
				l.readChar()
			}
		case '>':
			if l.peekChar() == '=' {
				l.readChar()
				tok = Token{TokenGTE, ">="}
				l.readChar()
			} else {
				tok = Token{TokenGT, ">"}
				l.readChar()
			}
		case '<':
			if l.peekChar() == '=' {
				l.readChar()
				tok = Token{TokenLTE, "<="}
				l.readChar()
			} else {
				tok = Token{TokenLT, "<"}
				l.readChar()
			}
		default:
			if isLetter(l.ch) {
				tok = l.readIdentifier()
				// Check if it's a keyword
				tok.Type = keywordType(tok.Literal)
				if tok.Type == "" {
					tok.Type = TokenIdent
				}
			} else if isDigit(l.ch) {
				tok = l.readNumber()
			} else {
				tok = Token{TokenIllegal, string(l.ch)}
				l.readChar()
			}
		}

		tokens = append(tokens, tok)

		if tok.Type == TokenEOF {
			break
		}
	}

	return tokens
}

func (l *lexer) readString() Token {
	l.readChar() // skip opening quote
	var sb strings.Builder
	for l.ch != 0 && l.ch != '\'' {
		sb.WriteByte(l.ch)
		l.readChar()
	}
	if l.ch == '\'' {
		l.readChar() // skip closing quote
	}
	return Token{TokenString, sb.String()}
}

func (l *lexer) readNumber() Token {
	start := l.pos
	for isDigit(l.ch) || l.ch == '.' {
		l.readChar()
	}
	return Token{TokenNumber, l.input[start:l.pos]}
}

func (l *lexer) readIdentifier() Token {
	start := l.pos
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	return Token{TokenIdent, l.input[start:l.pos]}
}

func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

// keywordType returns the token type for a keyword, or empty string if not a keyword.
func keywordType(lit string) TokenType {
	switch strings.ToUpper(lit) {
	case "AND":
		return TokenAND
	case "OR":
		return TokenOR
	case "NOT":
		return TokenNOT
	case "LIKE":
		return TokenLIKE
	case "IN":
		return TokenIN
	case "EXISTS":
		return TokenEXISTS
	case "CONTAINS":
		return TokenCONTAINS
	case "PREFIX":
		return TokenPREFIX
	case "SUFFIX":
		return TokenSUFFIX
	case "BETWEEN":
		return TokenBETWEEN
	case "IS":
		return TokenIS
	case "NULL":
		return TokenNULL
	case "ANY_OF":
		return TokenANYOF
	case "ALL_OF":
		return TokenALLOF
	case "HAS_LENGTH":
		return TokenHASLEN
	default:
		return ""
	}
}
