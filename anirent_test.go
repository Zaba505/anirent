package anirent

import (
	"context"
	"io"
	"testing"

	pb "github.com/Zaba505/anirent/proto"
	"github.com/Zaba505/anirent/searchengine"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var validSearchResult = searchengine.Result{
	TorrentName: "[SubsPlease] Tonikaku Kawaii - 08 (1080p) [37FBE4D6].mkv",
	Magnet:      "",
}

func dialAnirent() (pb.AnirentClient, error) {
	cc, err := grpc.Dial("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return pb.NewAnirentClient(cc), nil
}

func TestAnirentService(t *testing.T) {
	l, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}

	zap.ReplaceGlobals(l)

	t.Run("should successfully stream search results back", func(subT *testing.T) {
		mockSearchEngine := func(ctx context.Context, query string, resultCh chan<- *searchengine.Result) error {
			defer close(resultCh)

			resultCh <- &validSearchResult
			resultCh <- &validSearchResult
			resultCh <- &validSearchResult

			return nil
		}

		s, err := NewService(SearchEngine(searchengine.InterfaceFunc(mockSearchEngine)))
		if !assert.Nil(subT, err) {
			return
		}

		errCh := make(chan error, 1)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			defer close(errCh)
			err := s.Serve(ctx)
			errCh <- err
		}()

		client, err := dialAnirent()
		if !assert.Nil(subT, err) {
			return
		}

		stream, err := client.Search(ctx, &pb.SearchRequest{
			AnimeName:   "Test",
			Resolutions: []pb.Resolution{pb.Resolution_K_4},
		})
		if !assert.Nil(subT, err) {
			return
		}

		results := make([]*pb.SearchResult, 0, 10)
		for {
			result, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if !assert.Nil(subT, err) {
				return
			}

			results = append(results, result)
		}
		cancel()

		assert.Equal(subT, 3, len(results))
	})
}
