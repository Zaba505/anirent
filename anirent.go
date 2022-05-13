package anirent

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"sync"
	"time"

	"github.com/Zaba505/anirent/event"
	"github.com/Zaba505/anirent/parser"
	pb "github.com/Zaba505/anirent/proto"
	"github.com/Zaba505/anirent/searchengine"

	"github.com/anacrolix/dht/v2"
	"github.com/anacrolix/log"
	"github.com/anacrolix/torrent"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Service
type Service struct {
	pb.UnimplementedAnirentServer

	doneCh chan struct{}
	rander io.Reader
	ls     net.Listener

	// search config
	se searchengine.Interface

	// download config
	tcOnce  sync.Once
	tcErr   error
	tc      *torrent.Client
	dataDir string
	dq      chan downloadRequest // download queue
	dOnce   sync.Once
	bus     *event.Bus[*pb.Event]
}

// A AnirentOption sets an option on a Anirent service.
type AnirentOption func(*Service)

// WithListener allows a custom net.Listener to be provided.
//
// The default listener will listen on ":8080".
//
func WithListener(ls net.Listener) AnirentOption {
	return func(s *Service) {
		s.ls = ls
	}
}

// SearchEngine sets the search engine to use for querying for torrents.
//
// The default search engine is BTDigg.
//
func SearchEngine(se searchengine.Interface) AnirentOption {
	return func(s *Service) {
		s.se = se
	}
}

// TorrentDir is the directory where torrents are downloaded to.
func TorrentDir(dir string) AnirentOption {
	return func(s *Service) {
		s.dataDir = dir
	}
}

// NewService
func NewService(opts ...AnirentOption) (*Service, error) {
	s := &Service{
		se:      &searchengine.BTDigg{},
		doneCh:  make(chan struct{}, 1),
		rander:  rand.Reader,
		dataDir: os.TempDir(),
		dq:      make(chan downloadRequest, 1),
		bus:     event.NewBus[*pb.Event](),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// Serve handles the initialization of the underlying gRPC server and registering
// the Anirent Server service with it. It then begins serving requests. The
// provided context can be used to gracefully shutdown the server.
//
func (s *Service) Serve(ctx context.Context) error {
	var err error
	if s.ls == nil {
		s.ls, err = net.Listen("tcp", ":8080")
	}
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	pb.RegisterAnirentServer(grpcServer, s)

	errCh := make(chan error, 1)
	go func(ls net.Listener) {
		defer close(errCh)

		addr := ls.Addr()
		zap.L().Info("anirent started", zap.String("network", addr.Network()), zap.String("addr", addr.String()))

		err := grpcServer.Serve(ls)
		errCh <- err
	}(s.ls)

	select {
	case <-ctx.Done():
		close(s.doneCh)
		grpcServer.GracefulStop()
		<-errCh
		return nil
	case err := <-errCh:
		close(s.doneCh)
		return err
	}
}

// Search
func (s *Service) Search(req *pb.SearchRequest, stream pb.Anirent_SearchServer) error {
	resultChs := make([]<-chan *searchengine.Result, 0, len(req.Resolutions))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	g, gctx := errgroup.WithContext(ctx)
	for _, resolution := range req.Resolutions {
		resolution := resolution
		resultCh := make(chan *searchengine.Result, 10)
		resultChs = append(resultChs, resultCh)

		g.Go(func() error {
			zap.L().Debug("searching", zap.String("anime", req.AnimeName), zap.String("resolution", resolution.String()))
			return s.search(gctx, req.AnimeName, resolution, resultCh)
		})
	}

	resultCh := merge(gctx.Done(), resultChs...)
	g.Go(func() error {
		for {
			select {
			case <-gctx.Done():
				zap.L().Warn("context was cancelled before search results were completely processed")
				return gctx.Err()
			case scrapeRes := <-resultCh:
				if scrapeRes == nil {
					return nil
				}
				zap.L().Debug(
					"received search engine result",
					zap.String("torrent", scrapeRes.TorrentName),
					zap.String("magnet", scrapeRes.Magnet),
				)

				torrentName := scrapeRes.TorrentName

				zap.L().Debug("parsing scraping result", zap.String("torrent_name", torrentName))

				searchResult, err := parser.Parse(torrentName)
				if err != nil {
					zap.L().Error("unexpected error when parsing torrent name", zap.String("torrent_name", torrentName), zap.Error(err))
					continue
				}
				searchResult.Magnet = scrapeRes.Magnet

				var typ string
				switch searchResult.Details.(type) {
				case *pb.SearchResult_Episode:
					typ = "episode"
				case *pb.SearchResult_Season:
					typ = "season"
				}

				zap.L().Debug(
					"parsed torrent result",
					zap.String("name", searchResult.Name),
					zap.String("resolution", parser.SprintResolution(searchResult.Resolution).String()),
					zap.String("type", typ),
				)

				err = stream.Send(searchResult)
				if err != nil {
					zap.L().Error("unexpected error when sending search result", zap.Error(err))
				}
			}
		}
	})

	zap.L().Debug("waiting for search to complete")
	return g.Wait()
}

func (s *Service) search(ctx context.Context, name string, resolution pb.Resolution, resultCh chan<- *searchengine.Result) error {
	res := parser.SprintResolution(resolution)
	query := fmt.Sprintf("[SubsPlease] %s (%s)", name, res)
	return s.se.Search(ctx, query, resultCh)
}

func merge[T any](done <-chan struct{}, channels ...<-chan T) <-chan T {
	var wg sync.WaitGroup

	wg.Add(len(channels))
	fanIn := make(chan T)
	multiplex := func(c <-chan T) {
		defer wg.Done()
		for i := range c {
			select {
			case <-done:
				return
			case fanIn <- i:
			}
		}
	}
	for _, c := range channels {
		if c == nil {
			break
		}
		go multiplex(c)
	}
	go func() {
		wg.Wait()
		close(fanIn)
	}()
	return fanIn
}

type downloadRequest struct {
	subscriptionId string
	result         *pb.SearchResult
}

// Download
func (s *Service) Download(ctx context.Context, req *pb.DownloadRequest) (*pb.DownloadResponse, error) {
	s.tcOnce.Do(s.startTorrentClient)
	if s.tcErr != nil {
		return nil, s.tcErr
	}
	s.dOnce.Do(s.startDownloader)

	id := uuid.Must(uuid.NewRandomFromReader(s.rander)).String()
	result := downloadRequest{
		subscriptionId: id,
		result:         req.Result,
	}

	select {
	case <-ctx.Done():
		zap.L().Error("context cancelled before download request could be submitted")
		return nil, ctx.Err()
	case s.dq <- result:
		zap.L().Info("successfully submitted download request", zap.String("id", id), zap.String("magnet", req.Result.Magnet))
	}

	s.bus.NewStream(id)
	subscription := &pb.Subscription{Id: id}
	return &pb.DownloadResponse{Subscription: subscription}, nil
}

// Subscribe
func (s *Service) Subscribe(req *pb.Subscription, stream pb.Anirent_SubscribeServer) error {
	errCh := make(chan error, 1)

	unsubscribe, err := s.bus.Subscribe(req.Id, func(event *pb.Event) {
		err := stream.Send(event)
		if err != nil {
			zap.L().Error("unexpected error when sending event", zap.Error(err))
			errCh <- err
			close(errCh)
			return
		}

		switch event.Payload.(type) {
		case *pb.Event_Completed:
			close(errCh)
		case *pb.Event_Failure:
			close(errCh)
		}
	})
	if err != nil {
		zap.L().Error("unexpected error when subscribing to event bus", zap.Error(err))
		return err
	}
	defer unsubscribe()

	err = <-errCh
	return err
}

func (s *Service) startTorrentClient() {
	tcfg := torrent.NewDefaultClientConfig()
	tcfg.ConfigureAnacrolixDhtServer = func(cfg *dht.ServerConfig) {
		cfg.Logger = log.Default.FilterLevel(log.Error)
	}
	tcfg.NoUpload = true
	tcfg.HTTPUserAgent = "anirent"
	tcfg.Logger = log.Default.FilterLevel(log.Error)
	tcfg.DataDir = s.dataDir

	c, err := torrent.NewClient(tcfg)
	if err != nil {
		s.tcErr = err
		return
	}
	s.tc = c
}

func (s *Service) startDownloader() {
	go func() {
		for {
			select {
			case <-s.doneCh:
				return
			case dr := <-s.dq:
				go s.processDownloadRequest(dr)
			}
		}
	}()
}

func (s *Service) processDownloadRequest(dr downloadRequest) {
	subId := dr.subscriptionId

	result := dr.result
	zap.L().Info("starting download", zap.String("magnet", result.Magnet))

	t, err := s.tc.AddMagnet(result.Magnet)
	if err != nil {
		zap.L().Error(
			"unexpected error when adding magnet to torrent client",
			zap.String("magnet", result.Magnet),
			zap.Error(err),
		)
		return
	}

	select {
	case <-s.doneCh:
		// TODO: cleanup
		zap.L().Warn("service shutdown before torrent download could start", zap.String("magnet", result.Magnet))
		return
	case <-t.GotInfo():
	}
	files := t.Files()
	fileName := files[0].DisplayPath()
	addr := path.Join("/dns/localhost/tcp/20/file", s.dataDir, fileName)
	totalBytes := int64(t.Info().TotalLength())
	s.publishStarted(subId, result.Magnet, totalBytes, addr)

	t.DisallowDataUpload()
	t.DownloadAll()
	defer t.Drop()

	downloadedBytes := int64(0)
	for {
		select {
		case <-s.doneCh:
			// TODO: cleanup
			zap.L().Warn("service shutdown before torrent download could complete", zap.String("magnet", result.Magnet))
			return
		case <-time.After(1 * time.Second):
		}

		stats := t.Stats()
		bytesRead := stats.BytesReadData.Int64()

		zap.L().Info(
			"stats",
			zap.Int("active_peers", stats.ActivePeers),
			zap.Int("total_peers", stats.TotalPeers),
			zap.Int64("bytes_read", bytesRead),
		)

		if downloadedBytes == bytesRead {
			continue
		}
		downloadedBytes = bytesRead

		s.publishProgress(subId, result.Magnet, downloadedBytes, totalBytes, addr)

		if downloadedBytes >= totalBytes {
			break
		}
	}

	s.publishDone(subId, result.Magnet, totalBytes, addr)
}

func (s *Service) publishStarted(subId, magnet string, total int64, addr string) {
	s.bus.Publish(subId, &pb.Event{
		Id:             uuid.Must(uuid.NewRandomFromReader(s.rander)).String(),
		SubscriptionId: subId,
		Payload: &pb.Event_Started{
			Started: &pb.DownloadStarted{
				Magnet:     magnet,
				TotalBytes: total,
				MultiAddr:  addr,
			},
		},
	})
}

func (s *Service) publishProgress(subId, magnet string, downloaded, total int64, addr string) {
	s.bus.Publish(subId, &pb.Event{
		Id:             uuid.Must(uuid.NewRandomFromReader(s.rander)).String(),
		SubscriptionId: subId,
		Payload: &pb.Event_Progress{
			Progress: &pb.DownloadProgress{
				Magnet:          magnet,
				DownloadedBytes: downloaded,
				TotalBytes:      total,
				MultiAddr:       addr,
			},
		},
	})
}

func (s *Service) publishDone(subId, magnet string, total int64, multiAddr string) {
	s.bus.Publish(subId, &pb.Event{
		Id:             uuid.Must(uuid.NewRandomFromReader(s.rander)).String(),
		SubscriptionId: subId,
		Payload: &pb.Event_Completed{
			Completed: &pb.DownloadComplete{
				Magnet:     magnet,
				TotalBytes: total,
				MultiAddr:  multiAddr,
			},
		},
	})
}
