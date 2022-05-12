package searchengine

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/debug"
	"go.uber.org/zap"
)

type globalZapDebugger struct{}

func (globalZapDebugger) Init() error { return nil }

func (globalZapDebugger) Event(e *debug.Event) {
	zap.L().Debug(
		"colly debug event",
		zap.String("type", e.Type),
		zap.Uint32("request_id", e.RequestID),
		zap.Uint32("collector_id", e.CollectorID),
		zap.Any("values", e.Values),
	)
}

// BTDigg
type BTDigg struct{}

// make sure BTDigg abides by searchengine interface.
var _ Interface = &BTDigg{}

// Search scrapes the BTDigg website to obtain search results.
func (se *BTDigg) Search(ctx context.Context, query string, resultCh chan<- *Result) error {
	defer close(resultCh)

	c := colly.NewCollector(
		colly.Async(true),
		colly.UserAgent("searchengine/btdigg"),
		colly.AllowedDomains("btdig.com"),
		colly.Debugger(globalZapDebugger{}),
	)
	c.Limit(&colly.LimitRule{
		Parallelism: 4,
	})

	c.OnError(func(resp *colly.Response, err error) {
		zap.L().Error("unexpected error from colly", zap.Error(err))
	})

	c.OnHTML("div[class='one_result']", func(el *colly.HTMLElement) {
		name := el.ChildText("div[class='torrent_name']")
		magnet := el.ChildAttr("div[class='torrent_magnet'] a", "href")

		zap.L().Debug("found search result", zap.String("torrent_name", name), zap.String("magnet", magnet))
		resultCh <- &Result{
			TorrentName: name,
			Magnet:      magnet,
		}
	})

	c.OnHTML("a", func(el *colly.HTMLElement) {
		route := el.Attr("href")
		if !strings.HasPrefix(route, "/search") {
			return
		}
		if !strings.Contains(route, "p=") {
			return
		}

		uri := "https://" + path.Join("btdig.com", route)
		visited, err := c.HasVisited(uri)
		if err != nil {
			zap.L().Error("unexpected error when checking if uri has already been visited", zap.String("uri", uri))
			return
		}
		if visited {
			zap.L().Info("already visited scraped uri before", zap.String("uri", uri))
			return
		}

		fmt.Println("visiting", uri)
		zap.L().Info("visiting scraped link", zap.String("uri", uri))
		err = c.Visit(uri)
		if err != nil {
			zap.L().Error("unexpected error when visiting scraped link", zap.String("uri", uri))
			return
		}
	})

	uri := fmt.Sprintf("https://btdig.com/search?q=%s&p=0&order=0", query)
	uri = strings.ReplaceAll(uri, " ", "+")
	err := c.Visit(uri)
	if err != nil {
		zap.L().Error("unexpected error when visiting first search page", zap.Error(err))
		return err
	}

	zap.L().Debug("waiting for colly collector to finish")
	c.Wait()
	zap.L().Debug("colly collector finished")
	return nil
}
