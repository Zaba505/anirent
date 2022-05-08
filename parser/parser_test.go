package parser

import (
	"testing"

	pb "github.com/Zaba505/anirent/proto"

	"github.com/stretchr/testify/assert"
)

func TestValidFormats(t *testing.T) {
	testCases := []struct {
		Name           string
		TorrentName    string
		ExpectedResult *pb.SearchResult
	}{
		{
			Name:        "Valid Episode Format",
			TorrentName: "[SubsPlease] Tonikaku Kawaii - 08 (1080p) [37FBE4D6].mkv",
			ExpectedResult: &pb.SearchResult{
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
		},
		{
			Name:        "Valid Season Format",
			TorrentName: "[SubsPlease] Tonikaku Kawaii (01-03) (1080p) [Batch]",
			ExpectedResult: &pb.SearchResult{
				Name:       "Tonikaku Kawaii",
				Resolution: pb.Resolution_P_1080,
				Format:     pb.Format_MKV,
				Details: &pb.SearchResult_Season{
					Season: &pb.CompleteSeason{
						Number: 1,
						Episodes: []*pb.Episode{
							&pb.Episode{
								Season: 1,
								Number: 1,
							},
							&pb.Episode{
								Season: 1,
								Number: 2,
							},
							&pb.Episode{
								Season: 1,
								Number: 3,
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(subT *testing.T) {
			resp, err := Parse(testCase.TorrentName)
			if !assert.Nil(subT, err) {
				return
			}

			assert.Equal(subT, testCase.ExpectedResult, resp)
		})
	}
}
