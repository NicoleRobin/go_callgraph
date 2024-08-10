//go:build cgo
// +build cgo

package dot

import (
	"fmt"
	"github.com/goccy/go-graphviz"
	"log"
	"os"
	"path/filepath"
)

func runDotToImage(outFileName string, format string, dot []byte) (string, error) {
	g := graphviz.New()
	graph, err := graphviz.ParseBytes(dot)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := graph.Close(); err != nil {
			log.Fatal(err)
		}
		err := g.Close()
		if err != nil {
			return
		}
	}()
	var img string
	if outFileName == "" {
		img = filepath.Join(os.TempDir(), fmt.Sprintf("go-callvis_export.%s", format))
	} else {
		img = fmt.Sprintf("%s.%s", outFileName, format)
	}
	if err := g.RenderFilename(graph, graphviz.Format(format), img); err != nil {
		return "", err
	}
	return img, nil
}
