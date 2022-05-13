package anirent

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	pb "github.com/Zaba505/anirent/proto"
	"github.com/Zaba505/anirent/searchengine"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var testErr = errors.New("test")
var validSearchResult = searchengine.Result{
	TorrentName: "[SubsPlease] Tonikaku Kawaii - 08 (1080p) [37FBE4D6].mkv",
	Magnet:      "",
}

func init() {
	l, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}

	zap.ReplaceGlobals(l)
}

func newRandomListener() (net.Listener, error) {
	return net.Listen("tcp", ":0")
}

func dialAnirent(addr net.Addr) (pb.AnirentClient, error) {
	cc, err := grpc.Dial(addr.String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return pb.NewAnirentClient(cc), nil
}

func TestAnirentService_Search(t *testing.T) {
	t.Run("should successfully stream search results back", func(subT *testing.T) {
		mockSearchEngine := func(ctx context.Context, query string, resultCh chan<- *searchengine.Result) error {
			defer close(resultCh)

			resultCh <- &validSearchResult
			resultCh <- &validSearchResult
			resultCh <- &validSearchResult

			return nil
		}

		ls, err := newRandomListener()
		if !assert.Nil(subT, err) {
			return
		}

		s, err := NewService(
			SearchEngine(searchengine.InterfaceFunc(mockSearchEngine)),
			WithListener(ls),
		)
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

		client, err := dialAnirent(ls.Addr())
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

		results := make([]*pb.SearchResult, 0, 3)
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

	t.Run("should just close stream if no results are found", func(subT *testing.T) {
		mockSearchEngine := func(ctx context.Context, query string, resultCh chan<- *searchengine.Result) error {
			defer close(resultCh)
			return nil
		}

		ls, err := newRandomListener()
		if !assert.Nil(subT, err) {
			return
		}

		s, err := NewService(
			SearchEngine(searchengine.InterfaceFunc(mockSearchEngine)),
			WithListener(ls),
		)
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

		client, err := dialAnirent(ls.Addr())
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

		var results []*pb.SearchResult
		for {
			_, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if !assert.Nil(subT, err) {
				return
			}

			assert.Fail(subT, "should not have received any values")
		}
		cancel()

		assert.Equal(subT, 0, len(results))
	})

	t.Run("should successfully stream results before failing", func(subT *testing.T) {
		mockSearchEngine := func(ctx context.Context, query string, resultCh chan<- *searchengine.Result) error {
			defer close(resultCh)

			resultCh <- &validSearchResult
			resultCh <- &validSearchResult
			resultCh <- &validSearchResult

			<-time.After(50 * time.Millisecond)
			return testErr
		}

		ls, err := newRandomListener()
		if !assert.Nil(subT, err) {
			return
		}

		s, err := NewService(
			SearchEngine(searchengine.InterfaceFunc(mockSearchEngine)),
			WithListener(ls),
		)
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

		client, err := dialAnirent(ls.Addr())
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

		results := make([]*pb.SearchResult, 0, 3)
		for {
			result, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if len(results) == 3 {
				assert.Error(subT, err)
				break
			}
			if !assert.Nil(subT, err) {
				subT.Logf("len(results) = %d", len(results))
				return
			}

			results = append(results, result)
		}
		cancel()

		assert.Equal(subT, 3, len(results))
	})
}
