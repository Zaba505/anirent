package cmd

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Zaba505/anirent"
	"github.com/Zaba505/anirent/printer"
	pb "github.com/Zaba505/anirent/proto"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

var downloadCmd = &cobra.Command{
	Use:   "download [SEARCH_RESULT]",
	Short: "Download torrent from search result",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		r := getSearchResultSrc(args[0])
		result, err := readSearchResult(r)
		if err != nil {
			zap.L().Error("unexpected error when reading search result", zap.Error(err))
			return
		}

		s, err := anirent.NewService()
		if err != nil {
			zap.L().Error("unexpected error when creating anirent service", zap.Error(err))
			return
		}

		ctx, cancel := context.WithCancel(cmd.Context())
		go func() {
			defer cancel()

			err := s.Serve(ctx)
			zap.L().Error("error from anirent service", zap.Error(err))
		}()

		cc, err := grpc.Dial("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			zap.L().Error("unexpected error when dialing to anirent service", zap.Error(err))
			return
		}

		client := pb.NewAnirentClient(cc)
		resp, err := client.Download(ctx, &pb.DownloadRequest{
			Result: result,
		})
		if err != nil {
			zap.L().Error("unexpected error when sending downloading request", zap.Error(err))
			return
		}

		zap.L().Info("download submitted and subscribing to events", zap.String("subscription_id", resp.Subscription.Id))

		stream, err := client.Subscribe(ctx, resp.Subscription)
		if err != nil {
			zap.L().Error("unexpected error when subscribing for events", zap.Error(err))
			return
		}

		bar := newProgressBar()
		for {
			ev, err := stream.Recv()
			if err == io.EOF {
				zap.L().Info("no more events")
				return
			}
			if err != nil {
				zap.L().Error("unexpected error when receiving event", zap.Error(err))
				return
			}

			zap.L().Info("received event", zap.String("id", ev.Id), zap.String("subscription_id", ev.SubscriptionId))
			switch x := ev.Payload.(type) {
			case *pb.Event_Started:
				started := x.Started

				zap.L().Info(
					"download started",
					zap.String("magnet", started.Magnet),
					zap.Int64("total", started.TotalBytes),
				)

				bar.ChangeMax64(started.TotalBytes)
				bar.Describe(started.MultiAddr)
			case *pb.Event_Progress:
				progress := x.Progress

				zap.L().Info(
					"download progress",
					zap.String("magnet", progress.Magnet),
					zap.Int64("downloaded", progress.DownloadedBytes),
					zap.Int64("total", progress.TotalBytes),
				)
				bar.ChangeMax64(progress.TotalBytes)
				bar.Set64(progress.DownloadedBytes)
				bar.Describe(progress.MultiAddr)
			case *pb.Event_Completed:
				done := x.Completed

				zap.L().Info(
					"download completed",
					zap.String("magnet", done.Magnet),
					zap.Int64("total", done.TotalBytes),
					zap.String("multi_addr", done.MultiAddr),
				)
				bar.Close()

				dir, err := cmd.Flags().GetString("dir")
				if err != nil {
					panic(err)
				}

				err = moveDownload(dir, done.MultiAddr, result, printer.ForPlex())
				if err != nil {
					zap.L().Error("unexpected error when moving download content", zap.Error(err))
				}
				return
			case *pb.Event_Failure:
				failure := x.Failure

				zap.L().Error("downloaded failed", zap.String("error", failure.Error))
				bar.Close()
				return
			}
		}
	},
}

func getSearchResultSrc(s string) io.Reader {
	if s == "-" {
		return os.Stdin
	}

	return strings.NewReader(s)
}

func readSearchResult(r io.Reader) (*pb.SearchResult, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var result pb.SearchResult
	err = protojson.Unmarshal(b, &result)
	return &result, err
}

func newProgressBar() *progressbar.ProgressBar {
	return progressbar.NewOptions64(
		1,
		progressbar.OptionSetDescription("downloaded"),
		progressbar.OptionSetWriter(os.Stdout),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(10),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Printf("\n")
		}),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
	)
}

func moveDownload(outDir, downloadAddr string, result *pb.SearchResult, p printer.Printer) error {
	absDir, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}

	downloadPath := getFilePath(downloadAddr)
	outPath, err := p.Print(result)
	if err != nil {
		return err
	}

	fullOutPath := filepath.Join(absDir, outPath)

	zap.L().Debug(
		"renaming download",
		zap.String("old", downloadPath),
		zap.String("new", fullOutPath),
	)
	return move(downloadPath, fullOutPath)
}

func getFilePath(downloadAddr string) string {
	ss := strings.Split(downloadAddr, "/")
	i := 0
	for j, s := range ss {
		if s == "file" {
			i = j
			break
		}
	}
	ss = ss[i+1:]
	return filepath.Join("/", filepath.Join(ss...))
}

func move(oldPath, newPath string) error {
	oldFile, err := os.Open(oldPath)
	if err != nil {
		return err
	}

	newFile, err := os.Create(newPath)
	if err != nil {
		return err
	}
	defer newFile.Close()

	_, err = io.Copy(newFile, oldFile)
	if err != nil {
		return err
	}
	oldFile.Close()

	return os.Remove(oldPath)
}

func init() {
	rootCmd.AddCommand(downloadCmd)

	downloadCmd.Flags().StringP("dir", "d", ".", "Specify directory to move torrent to after downloading.")
	downloadCmd.Flags().BoolP("plex", "p", true, "Save content with a Plex friendly name.")
}
