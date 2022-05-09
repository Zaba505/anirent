package printer

import (
	"testing"

	pb "github.com/Zaba505/anirent/proto"
)

func TestForPlex(t *testing.T) {
	p := ForPlex()

	s, err := p.Print(&pb.SearchResult{
		Name:       "Tonikaku Kawaii",
		Resolution: pb.Resolution_P_1080,
		Format:     pb.Format_MKV,
		Details: &pb.SearchResult_Episode{
			Episode: &pb.Episode{
				Season: 1,
				Number: 8,
			},
		},
	})
	if err != nil {
		t.Error(err)
		return
	}
	if s != "Tonikaku Kawaii - s01e08 (1080p).mkv" {
		t.Log(s)
		t.Fail()
		return
	}
}
