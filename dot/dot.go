package dot

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

var (
	Minlen    uint    = 2
	Nodesep   float64 = 0.35
	Nodeshape string  = "box"
	Nodestyle string  = "filled,rounded"
	Rankdir   string  = "LR"
)

const tmplCluster = `{{define "cluster" -}}
    {{printf "subgraph %q {" .}}
        {{printf "%s" .Attrs.Lines}}
        {{range .Nodes}}
        {{template "node" .}}
        {{- end}}
        {{range .Clusters}}
        {{template "cluster" .}}
        {{- end}}
    {{println "}" }}
{{- end}}`

const tmplNode = `{{define "edge" -}}
    {{printf "%q -> %q [ %s ]" .From .To .Attrs}}
{{- end}}`

const tmplEdge = `{{define "node" -}}
    {{printf "%q [ %s ]" .ID .Attrs}}
{{- end}}`

const tmplGraph = `digraph go_callgraph {
    label="{{.Title}}";
    labeljust="l";
    fontname="Arial";
    fontsize="14";
    rankdir="{{.Options.rankdir}}";
    bgcolor="lightgray";
    style="solid";
    penwidth="0.5";
    pad="0.0";
    nodesep="{{.Options.nodesep}}";

    node [shape="{{.Options.nodeshape}}" style="{{.Options.nodestyle}}" fillcolor="honeydew" fontname="Verdana" penwidth="1.0" margin="0.05,0.0"];
    edge [minlen="{{.Options.minlen}}"]

    {{template "cluster" .Cluster}}

    {{- range .Edges}}
    {{template "edge" .}}
    {{- end}}
}
`

// DotCluster dot cluster
type DotCluster struct {
	ID       string
	Clusters map[string]*DotCluster
	Nodes    []*DotNode
	Attrs    DotAttrs
}

func NewDotCluster(id string) *DotCluster {
	return &DotCluster{
		ID:       id,
		Clusters: make(map[string]*DotCluster),
		Attrs:    make(DotAttrs),
	}
}

func (c *DotCluster) String() string {
	return fmt.Sprintf("cluster_%s", c.ID)
}

// DotNode dot node
type DotNode struct {
	ID    string
	Attrs DotAttrs
}

func (n *DotNode) String() string {
	return n.ID
}

// DotEdge dot edge
type DotEdge struct {
	From  *DotNode
	To    *DotNode
	Attrs DotAttrs
}

// DotAttrs dot attrs
type DotAttrs map[string]string

func (p DotAttrs) List() []string {
	l := []string{}
	for k, v := range p {
		l = append(l, fmt.Sprintf("%s=%q", k, v))
	}
	return l
}

func (p DotAttrs) String() string {
	return strings.Join(p.List(), " ")
}

func (p DotAttrs) Lines() string {
	return fmt.Sprintf("%s;", strings.Join(p.List(), ";\n"))
}

// DotGraph dot graph
type DotGraph struct {
	Title   string
	Minlen  uint
	Attrs   DotAttrs
	Cluster *DotCluster
	Nodes   []*DotNode
	Edges   []*DotEdge
	Options map[string]string
}

func (g *DotGraph) WriteDot(w io.Writer) error {
	t := template.New("dot")
	for _, s := range []string{tmplCluster, tmplNode, tmplEdge, tmplGraph} {
		if _, err := t.Parse(s); err != nil {
			return err
		}
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, g); err != nil {
		return err
	}
	_, err := buf.WriteTo(w)
	return err
}

// DotToImage dot to image
func DotToImage(ctx context.Context, isGraphvizFlag bool, outFileName string, format string, dot []byte) (string, error) {
	if isGraphvizFlag {
		return runDotToImageCallSystemGraphviz(outFileName, format, dot)
	}

	return runDotToImage(outFileName, format, dot)
}

// location of dot executable for converting from .dot to .svg
// it's usually at: /usr/bin/dot
var dotSystemBinary string

// runDotToImageCallSystemGraphviz generates a SVG using the 'dot' utility, returning the filepath
func runDotToImageCallSystemGraphviz(outFileName string, format string, dot []byte) (string, error) {
	if dotSystemBinary == "" {
		dot, err := exec.LookPath("dot")
		if err != nil {
			log.Fatalln("unable to find program 'dot', please install it or check your PATH")
		}
		dotSystemBinary = dot
	}

	var img string
	if outFileName == "" {
		img = filepath.Join(os.TempDir(), fmt.Sprintf("go_callgraph.%s", format))
	} else {
		img = fmt.Sprintf("%s.%s", outFileName, format)
	}
	cmd := exec.Command(dotSystemBinary, fmt.Sprintf("-T%s", format), "-o", img)
	cmd.Stdin = bytes.NewReader(dot)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("command '%v': %v\n%v", cmd, err, stderr.String())
	}
	return img, nil
}
