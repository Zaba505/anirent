package anirent

import (
	"context"
	"fmt"
	"strings"

	"github.com/Zaba505/anirent/parser"
	pb "github.com/Zaba505/anirent/proto"

	"github.com/gocolly/colly/v2"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type scrapeResult struct {
	TorrentName string
	Magnet      string
}

func scrape(ctx context.Context, req *pb.SearchRequest, resultCh chan<- scrapeResult) error {
	defer close(resultCh)

	g, _ := errgroup.WithContext(ctx)
	for _, resolution := range req.Resolutions {
		res := parser.SprintResolution(resolution) // TODO: map resolution to correct string orientation

		g.Go(func() error {
			uri := fmt.Sprintf("https://btdig.com/search?q=[SubsPlease] %s (%s)&p=0&order=0", req.AnimeName, res)
			uri = strings.ReplaceAll(uri, " ", "+")

			c := colly.NewCollector(
				colly.UserAgent("anirent-scraper"),
			)
			c.Limit(&colly.LimitRule{
				Parallelism: 1,
			})

			c.OnHTML("div[class='one_result']", func(el *colly.HTMLElement) {
				name := el.ChildText("div[class='torrent_name']")
				magnet := el.ChildAttr("div[class='torrent_magnet'] a", "href")

				zap.L().Debug("found search result", zap.String("torrent_name", name), zap.String("magnet", magnet))
				resultCh <- scrapeResult{
					TorrentName: name,
					Magnet:      magnet,
				}
			})

			return c.Visit(uri)
		})
	}

	return g.Wait()
}
