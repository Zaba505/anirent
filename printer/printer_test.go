package printer

import (
	"testing"

	pb "github.com/Zaba505/anirent/proto"

	"github.com/stretchr/testify/assert"
)

func TestForPlex(t *testing.T) {
	testCases := []struct {
		Name         string
		SearchResult *pb.SearchResult
		Expected     string
	}{
		{
			Name: "Episode",
			SearchResult: &pb.SearchResult{
				Name:       "Tonikaku Kawaii",
				Resolution: pb.Resolution_P_1080,
				Format:     pb.Format_MKV,
				Details: &pb.SearchResult_Episode{
					Episode: &pb.Episode{
						Season: 1,
						Number: 8,
					},
				},
			},
			Expected: "Tonikaku Kawaii - s01e08 (1080p).mkv",
		},
	}

	p := ForPlex()
	for _, testCase := range testCases {
		t.Run(testCase.Name, func(subT *testing.T) {
			s, err := p.Print(testCase.SearchResult)
			if !assert.Nil(subT, err) {
				return
			}

			assert.Equal(subT, testCase.Expected, s)
		})
	}
}
