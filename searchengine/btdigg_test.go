package searchengine

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBTDigg(t *testing.T) {
	env := os.Getenv("ENV")
	if env == "ci" {
		t.Skip("skipping torrent based test while in ci env")
		return
	}

	t.Run("should return multiple pages of results", func(subT *testing.T) {
		resultCh := make(chan *Result)
		errCh := make(chan error, 1)
		go func() {
			defer close(errCh)

			var se BTDigg
			err := se.Search(context.Background(), "[SubsPlease] Tonikaku Kawaii (1080p)", resultCh)
			errCh <- err
		}()

		var results []*Result
		for {
			select {
			case result := <-resultCh:
				if result == nil {
					break
				}
				results = append(results, result)
				continue
			case err := <-errCh:
				if !assert.Nil(subT, err) {
					return
				}
			}
			break
		}

		// 10 results per page and test search should return multiple pages.
		assert.Greater(subT, len(results), 10)
		for _, result := range results {
			assert.NotEqual(subT, "", result.TorrentName)
			assert.NotEqual(subT, "", result.Magnet)
		}
	})
}
