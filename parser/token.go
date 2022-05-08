package parser

type token int

const (
	eof    token = iota
	lparen       // (
	rparen       // )
	lbrack       // [
	rbrack       // ]
	ident        // e.g. SubsPlease, Tonikaku Kawaii, 1080p, etc.
	hyphen       // -
	dot          // .
)

var tokens = map[token]string{
	eof:    "eof",
	lparen: "(",
	rparen: ")",
	lbrack: "[",
	rbrack: "]",
	ident:  "IDENT",
	hyphen: "-",
	dot:    ".",
}

func (t token) String() string {
	return tokens[t]
}
