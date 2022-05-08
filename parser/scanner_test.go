package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScan(t *testing.T) {
	testCases := []struct {
		Name        string
		TorrentName string
		Items       []item
	}{
		{
			Name:        "Valid Episode Name",
			TorrentName: "[SubsPlease] Tonikaku Kawaii - 08 (1080p) [37FBE4D6].mkv",
			Items: []item{
				{tok: lbrack, val: "["},
				{tok: ident, val: "SubsPlease"},
				{tok: rbrack, val: "]"},
				{tok: ident, val: "Tonikaku"},
				{tok: ident, val: "Kawaii"},
				{tok: hyphen, val: "-"},
				{tok: ident, val: "08"},
				{tok: lparen, val: "("},
				{tok: ident, val: "1080p"},
				{tok: rparen, val: ")"},
				{tok: lbrack, val: "["},
				{tok: ident, val: "37FBE4D6"},
				{tok: rbrack, val: "]"},
				{tok: dot, val: "."},
				{tok: ident, val: "mkv"},
				{tok: eof},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(subT *testing.T) {
			s := scan(testCase.TorrentName)

			items := make([]item, 0, len(testCase.Items))
			for {
				item := s.nextItem()
				items = append(items, item)
				subT.Log(item)

				if item.tok == eof {
					break
				}
			}

			assert.Equal(subT, len(testCase.Items), len(items))
		})
	}
}
