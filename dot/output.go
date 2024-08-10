package dot

import (
	"bytes"
	"context"
	"fmt"
	"go/build"
	"go/token"
	"text/template"

	"github.com/nicolerobin/log"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

type Edge struct {
	Caller *ssa.Function
	Callee *ssa.Function

	edge     *callgraph.Edge
	fset     *token.FileSet
	position token.Position // initialized lazily
}

func (e *Edge) pos() *token.Position {
	if e.position.Offset == -1 {
		e.position = e.fset.Position(e.edge.Pos()) // called lazily
	}
	return &e.position
}

func (e *Edge) Filename() string { return e.pos().Filename }
func (e *Edge) Column() int      { return e.pos().Column }
func (e *Edge) Line() int        { return e.pos().Line }
func (e *Edge) Offset() int      { return e.pos().Offset }

func (e *Edge) Dynamic() string {
	if e.edge.Site != nil && e.edge.Site.Common().StaticCallee() == nil {
		return "dynamic"
	}
	return "static"
}

func (e *Edge) Description() string { return e.edge.Description() }

func inStd(node *callgraph.Node) bool {
	pkg, _ := build.Import(node.Func.Pkg.Pkg.Path(), "", 0)
	return pkg.Goroot
}

func GenerateDot(
	ctx context.Context,
	prog *ssa.Program,
	callGraph *callgraph.Graph,
	baseModule string,
) ([]byte, error) {
	log.Info("baseModule:%s", baseModule)
	callGraph.DeleteSyntheticNodes()

	format := `  {{printf "%q" .Caller}} -> {{printf "%q" .Callee}}`
	funcMap := template.FuncMap{
		"posn": func(f *ssa.Function) token.Position {
			return f.Prog.Fset.Position(f.Pos())
		},
	}
	tmpl, err := template.New("-format").Funcs(funcMap).Parse(format)
	if err != nil {
		return nil, fmt.Errorf("invalid -format template: %v", err)
	}

	before := "digraph callgraph {\n\trankdir=LR;\n"
	after := "}\n"
	var buf bytes.Buffer
	data := Edge{fset: prog.Fset}

	buf.WriteString(before)
	if err := callgraph.GraphVisitEdges(callGraph, func(edge *callgraph.Edge) error {
		/*
			if edge.Caller.Func.Pkg == nil {
				log.Debug("edge.Caller.Func.Pkg is nil, edge:%+v, skip", edge)
				return nil
			}

		*/

		/*
			if inStd(edge.Caller) || inStd(edge.Callee) {
				log.Debug("caller or callee in std, edge:%+v, skip", edge)
				return nil
			}

		*/

		/*
			if (edge.Caller.Func.Pkg != nil && !strings.HasPrefix(edge.Caller.Func.Pkg.String(), baseModule)) &&
				(edge.Callee.Func.Pkg != nil && !strings.HasPrefix(edge.Callee.Func.Pkg.String(), baseModule)) {
				log.Debug("caller and callee are both not directly deps, edge:%+v, skip", edge)
				return nil
			}
		*/

		data.position.Offset = -1
		data.edge = edge
		data.Caller = edge.Caller.Func
		data.Callee = edge.Callee.Func

		var tmpBuf bytes.Buffer
		if err := tmpl.Execute(&tmpBuf, &data); err != nil {
			return err
		}
		buf.Write(tmpBuf.Bytes())
		if tmpBufLen := tmpBuf.Len(); tmpBufLen == 0 || buf.Bytes()[tmpBufLen-1] != '\n' {
			buf.WriteString("\n")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	buf.WriteString(after)
	return buf.Bytes(), nil
}
