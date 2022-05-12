package searchengine

import (
	"context"
)

// Result
type Result struct {
	// Name of the torrent found.
	TorrentName string

	// The magnet link which can be used to download this torrent.
	Magnet string
}

// Interface
type Interface interface {
	// Search
	Search(ctx context.Context, query string, resultCh chan<- *Result) error
}
