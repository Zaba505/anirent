package cmd

import (
	"bytes"
	"context"
	"io"

	"github.com/Zaba505/anirent"
	"github.com/Zaba505/anirent/parser"
	pb "github.com/Zaba505/anirent/proto"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

var flagResToProtoRes = map[parser.Resolution]pb.Resolution{
	parser.RESOLUTION_360P:  pb.Resolution_P_360,
	parser.RESOLUTION_480P:  pb.Resolution_P_480,
	parser.RESOLUTION_720P:  pb.Resolution_P_720,
	parser.RESOLUTION_1080P: pb.Resolution_P_1080,
	parser.RESOLUTION_2160P: pb.Resolution_P_2160,
	parser.RESOLUTION_4K:    pb.Resolution_K_4,
}

var subspleaseCmd = &cobra.Command{
	Use:   "subsplease [ANIME]",
	Short: "Search for subbed anime",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		var lvl zapcore.Level
		lvlStr := cmd.Flags().Lookup("log-level").Value.String()
		err := lvl.UnmarshalText([]byte(lvlStr))
		if err != nil {
			panic(err)
		}

		l, err := zap.NewDevelopment(zap.IncreaseLevel(lvl))
		if err != nil {
			panic(err)
		}

		zap.ReplaceGlobals(l)
	},
	Run: func(cmd *cobra.Command, args []string) {
		res := cmd.Flags().Lookup("resolution").Value.String()
		resolution := flagResToProtoRes[parser.Resolution(res)]

		s, err := anirent.NewService()
		if err != nil {
			zap.L().Error("unexpected error when creating anirent service", zap.Error(err))
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
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

		stream, err := client.Search(ctx, &pb.SearchRequest{
			AnimeName:   args[0],
			Resolutions: []pb.Resolution{resolution},
		})
		if err != nil {
			zap.L().Error("unexpected error when sending search request", zap.Error(err))
			return
		}

		results := make([]*pb.SearchResult, 0, 10)
		for {
			result, err := stream.Recv()
			if err == io.EOF {
				zap.L().Info("no more search results")
				break
			}
			if err != nil {
				zap.L().Error("unexpected error when receiving search results", zap.Error(err))
				return
			}

			var typ string
			switch result.Details.(type) {
			case *pb.SearchResult_Episode:
				typ = "episode"
			case *pb.SearchResult_Season:
				typ = "season"
			}
			zap.L().Info(
				"received search result",
				zap.String("name", result.Name),
				zap.String("resolution", parser.SprintResolution(result.Resolution).String()),
				zap.String("type", typ),
				zap.String("magnet", result.Magnet),
			)

			results = append(results, result)
		}

		err = writeSearchResults(cmd.OutOrStdout(), results)
		if err != nil {
			zap.L().Error("unexpected error when writing search results", zap.Error(err))
		}
	},
}

func writeSearchResults(w io.Writer, results []*pb.SearchResult) error {
	if len(results) == 0 {
		_, err := w.Write([]byte("[]"))
		return err
	}

	var buf bytes.Buffer
	buf.WriteRune('[')
	for i, result := range results {
		b, err := protojson.Marshal(result)
		if err != nil {
			return err
		}

		buf.Write(b)
		if i != len(results)-1 {
			buf.WriteRune(',')
		}
	}
	buf.WriteRune(']')

	_, err := buf.WriteTo(w)
	return err
}

func init() {
	rootCmd.AddCommand(subspleaseCmd)

	res := parser.RESOLUTION_1080P
	subspleaseCmd.Flags().VarP(&res, "resolution", "r", "Specify desired resolution")
}
