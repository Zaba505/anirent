package parser

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	pb "github.com/Zaba505/anirent/proto"
)

// Resolution represents the desired video resolution e.g. 720p, 1080p, 4k...
type Resolution string

const (
	RESOLUTION_360P  Resolution = "360p"
	RESOLUTION_480P  Resolution = "480p"
	RESOLUTION_720P  Resolution = "720p"
	RESOLUTION_1080P Resolution = "1080p"
	RESOLUTION_2160P Resolution = "2160p"
	RESOLUTION_4K    Resolution = "4k"
)

var validResolutions = map[string]Resolution{
	string(RESOLUTION_360P):  RESOLUTION_360P,
	string(RESOLUTION_480P):  RESOLUTION_480P,
	string(RESOLUTION_720P):  RESOLUTION_720P,
	string(RESOLUTION_1080P): RESOLUTION_1080P,
	string(RESOLUTION_2160P): RESOLUTION_2160P,
	string(RESOLUTION_4K):    RESOLUTION_4K,
}

var flagResToProtoRes = map[Resolution]pb.Resolution{
	RESOLUTION_360P:  pb.Resolution_P_360,
	RESOLUTION_480P:  pb.Resolution_P_480,
	RESOLUTION_720P:  pb.Resolution_P_720,
	RESOLUTION_1080P: pb.Resolution_P_1080,
	RESOLUTION_2160P: pb.Resolution_P_2160,
	RESOLUTION_4K:    pb.Resolution_K_4,
}

func (r Resolution) String() string {
	return string(r)
}

func (r *Resolution) Set(s string) error {
	res, ok := validResolutions[s]
	if !ok {
		return fmt.Errorf("subsplease: unsupported resolution - %s", s)
	}

	*r = res
	return nil
}

func (r Resolution) Type() string {
	return "resolution"
}

func SprintResolution(res pb.Resolution) Resolution {
	for k, v := range flagResToProtoRes {
		if v == res {
			return k
		}
	}

	return ""
}

var formatStringToProto = map[string]pb.Format{
	"mkv": pb.Format_MKV,
}

func Parse(src string) (*pb.SearchResult, error) {
	s := scan(src)
	p := parser{s: s}
	p.pk.tok = -1

	return p.parse()
}

type parser struct {
	s *scanner

	pk item

	result *pb.SearchResult
	err    error
}

func (p *parser) next() item {
	if p.pk.tok > -1 {
		i := p.pk
		p.pk.tok = -1
		return i
	}

	return p.s.nextItem()
}

func (p *parser) peek() item {
	p.pk = p.s.nextItem()
	return p.pk
}

func (p *parser) expect(tok token, context string) item {
	i := p.next()
	if i.tok != tok {
		p.unexpected(i, context)
	}
	return i
}

func (p *parser) errorf(format string, args ...interface{}) {
	format = fmt.Sprintf("parser: %s", format)
	panic(fmt.Errorf(format, args...))
}

func (p *parser) unexpected(i item, context string) {
	p.errorf("unexpected %s in %s", i, context)
}

func (p *parser) recover(err *error) {
	e := recover()
	if e != nil {
		if _, ok := e.(runtime.Error); ok {
			panic(e)
		}
		*err = e.(error)
	}
}

func (p *parser) parse() (result *pb.SearchResult, err error) {
	defer p.recover(&err)

	result = p.parseResult()
	return
}

func (p *parser) parseResult() *pb.SearchResult {
	p.result = new(pb.SearchResult)

	// SubsPlease label
	p.expect(lbrack, "starting SubsPlease label")
	p.expect(ident, "SubsPlease")
	p.expect(rbrack, "ending SubsPlease label")

	// Anime name
	p.parseName()

	// Is it an episode or season?
	switch i := p.next(); i.tok {
	case hyphen:
		p.parseEpisode()
	case lparen:
		p.parseSeason()
	default:
		p.unexpected(i, "expected '-' or '(' after anime name")
	}

	// Resolution label
	p.parseResolution()

	// Unique label
	p.expect(lbrack, "starting unique label")
	p.expect(ident, "unique label")
	p.expect(rbrack, "ending unique label")

	// Video format file extension
	switch p.result.Details.(type) {
	case *pb.SearchResult_Episode:
		p.parseFileExt()
	}

	return p.result
}

func (p *parser) parseName() {
	var ss []string

	i := p.peek()
	for {
		if i.tok != ident {
			break
		}

		ss = append(ss, i.val)
		i = p.peek()
	}

	p.result.Name = strings.Join(ss, " ")
}

func (p *parser) parseEpisode() {
	n := p.parseInt("episode number")

	p.result.Details = &pb.SearchResult_Episode{
		Episode: &pb.Episode{
			Season: 1,
			Number: int64(n),
		},
	}
}

func (p *parser) parseSeason() {
	start := p.parseInt("first episode of season")
	p.expect(hyphen, "season episode range")
	end := p.parseInt("last episode of season")
	p.expect(rparen, "ending season episode range")

	episodes := make([]*pb.Episode, 0, end-start)
	for j := start; j < end+1; j++ {
		episodes = append(episodes, &pb.Episode{
			Season: 1,
			Number: int64(j),
		})
	}

	p.result.Details = &pb.SearchResult_Season{
		Season: &pb.CompleteSeason{
			Number:   1,
			Episodes: episodes,
		},
	}
}

func (p *parser) parseInt(context string) int {
	i := p.expect(ident, context)
	n, err := strconv.Atoi(i.val)
	if err != nil {
		p.errorf("invalid integer - %s", err.Error())
	}
	return n
}

func (p *parser) parseResolution() {
	p.expect(lparen, "starting resolution label")
	i := p.expect(ident, "resolution")
	res, ok := flagResToProtoRes[Resolution(i.val)]
	if !ok {
		p.errorf("unknown resolution - %s", i.val)
	}

	p.result.Resolution = res
	p.expect(rparen, "ending resolution label")
}

func (p *parser) parseFileExt() {
	p.expect(dot, "video format file extension")
	i := p.expect(ident, "video format file extension")
	format, ok := formatStringToProto[i.val]
	if !ok {
		p.errorf("unknown video file format - %s", i.val)
	}
	p.result.Format = format
}
