package printer

import (
	"fmt"
	"strconv"
	"strings"
	"text/template"

	pb "github.com/Zaba505/anirent/proto"
)

var (
	funcs = map[string]any{
		"getEpisode":    getEpisode,
		"toLower":       strings.ToLower,
		"fmtResolution": fmtResolution,
		"padInt":        padInt,
	}

	plexTmpl = "{{.Name}} - {{with $x := getEpisode .Details}}s{{padInt .Season}}e{{padInt .Number}}{{end}} ({{fmtResolution .Resolution}}).{{toLower .Format.String}}"
)

type Printer interface {
	Print(*pb.SearchResult) (string, error)
}

type plex struct {
	tmpl *template.Template
}

func ForPlex() Printer {
	return plex{
		tmpl: template.Must(template.New("plex").Funcs(funcs).Parse(plexTmpl)),
	}
}

func (p plex) Print(result *pb.SearchResult) (string, error) {
	var b strings.Builder
	err := p.tmpl.Execute(&b, result)
	return b.String(), err
}

func getEpisode(v any) any {
	switch x := v.(type) {
	case *pb.SearchResult_Episode:
		return x.Episode
	}
	return nil
}

func fmtResolution(v any) any {
	res, ok := v.(pb.Resolution)
	if !ok {
		panic("fmtResolution can only be used with a proto.Resolution")
	}

	switch res {
	case pb.Resolution_P_360:
		return "360p"
	case pb.Resolution_P_480:
		return "480p"
	case pb.Resolution_P_720:
		return "720p"
	case pb.Resolution_P_1080:
		return "1080p"
	case pb.Resolution_P_2160:
		return "2160p"
	case pb.Resolution_K_4:
		return "4K"
	default:
		return v
	}
}

func padInt(v any) any {
	switch n := v.(type) {
	case int64:
		if n < 10 {
			return fmt.Sprintf("0%d", n)
		}
		return strconv.Itoa(int(n))
	default:
		return v
	}
}
