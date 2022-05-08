package parser

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type scanner struct {
	src        string
	start, cur int
	width      int
}

func scan(s string) *scanner {
	return &scanner{
		src: s,
	}
}

var eofRune rune = -1

func (s *scanner) next() rune {
	if s.cur == len(s.src) {
		return eofRune
	}

	r, w := utf8.DecodeRuneInString(s.src[s.cur:])
	s.cur += w
	s.width = w
	return r
}

func (s *scanner) backup() {
	s.cur -= s.width
	s.width = 0
}

func (s *scanner) acceptUntil(chars string) {
	for {
		r := s.next()
		if r == eofRune {
			break
		}

		if strings.ContainsRune(chars, r) {
			s.backup()
			break
		}
	}
}

type item struct {
	tok token
	val string
}

func (i item) String() string {
	switch {
	case i.tok == eof:
		return "EOF"
	default:
		return fmt.Sprintf("%q", i.val)
	}
}

func (s *scanner) nextItem() item {
	if s.cur == len(s.src) {
		return item{
			tok: eof,
		}
	}

	r := s.next()
	for {
		if r == eofRune {
			return item{
				tok: eof,
			}
		}

		if r == ' ' || r == '\t' {
			s.start = s.cur
			r = s.next()
			continue
		}
		break
	}

	var tok token
	switch r {
	case '(':
		tok = lparen
	case ')':
		tok = rparen
	case '[':
		tok = lbrack
	case ']':
		tok = rbrack
	case '-':
		tok = hyphen
	case '.':
		tok = dot
	default:
		s.acceptUntil("()[]-. \t")
		tok = ident
	}

	item := item{
		tok: tok,
		val: s.src[s.start:s.cur],
	}
	s.start = s.cur
	return item
}
