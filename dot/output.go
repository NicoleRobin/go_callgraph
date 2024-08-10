package dot

import (
	"bytes"
	"context"
	"fmt"
	"go/build"
	"path/filepath"
	"strings"

	"github.com/nicolerobin/log"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

func isSynthetic(edge *callgraph.Edge) bool {
	return edge.Caller.Func.Pkg == nil || edge.Callee.Func.Synthetic != ""
}

func inStd(node *callgraph.Node) bool {
	pkg, _ := build.Import(node.Func.Pkg.Pkg.Path(), "", 0)
	return pkg.Goroot
}

func GenerateDot(
	ctx context.Context,
	prog *ssa.Program,
	cg *callgraph.Graph,
	groupBy []string,
	baseModule string,
) ([]byte, error) {
	var groupType, groupPkg bool
	for _, g := range groupBy {
		switch g {
		case "pkg":
			groupPkg = true
		case "type":
			groupType = true
		}
	}

	cluster := NewDotCluster("focus")
	cluster.Attrs = DotAttrs{
		"bgcolor":   "white",
		"label":     "",
		"labelloc":  "t",
		"labeljust": "c",
		"fontsize":  "18",
	}

	var (
		nodes []*DotNode
		edges []*DotEdge
	)

	nodeMap := make(map[string]*DotNode)
	edgeMap := make(map[string]*DotEdge)

	cg.DeleteSyntheticNodes()

	count := 0
	err := callgraph.GraphVisitEdges(cg, func(edge *callgraph.Edge) error {
		count++
		caller := edge.Caller
		callee := edge.Callee

		// omit synthetic calls
		if isSynthetic(edge) {
			log.Info("synthetic edge, edge.Caller.Func.Pkg:%+v, edge.Callee.Func.Synthetic:%s, skip", caller.Func.Pkg, callee.Func.Synthetic)
			return nil
		}
		if inStd(caller) || inStd(callee) {
			log.Debug("std package, caller:%+v, callee:%+v, skip", caller, callee)
			return nil
		}
		if !strings.HasPrefix(caller.Func.Pkg.String(), baseModule) ||
			!strings.HasPrefix(callee.Func.Pkg.String(), baseModule) {
			return nil
		}

		posCaller := prog.Fset.Position(caller.Func.Pos())
		posCallee := prog.Fset.Position(callee.Func.Pos())
		posEdge := prog.Fset.Position(edge.Pos())
		//fileCaller := fmt.Sprintf("%s:%d", posCaller.Filename, posCaller.Line)
		filenameCaller := filepath.Base(posCaller.Filename)
		log.Info("call node: %s -> %s (%s -> %s) %v\n", caller.Func.Pkg, callee.Func.Pkg, caller, callee, filenameCaller)

		var sprintNode = func(node *callgraph.Node, isCaller bool) *DotNode {
			// only once
			key := node.Func.String()
			nodeTooltip := ""

			fileCaller := fmt.Sprintf("%s:%d", filepath.Base(posCaller.Filename), posCaller.Line)
			fileCallee := fmt.Sprintf("%s:%d", filepath.Base(posCallee.Filename), posCallee.Line)

			if isCaller {
				nodeTooltip = fmt.Sprintf("%s | defined in %s", node.Func.String(), fileCaller)
			} else {
				nodeTooltip = fmt.Sprintf("%s | defined in %s", node.Func.String(), fileCallee)
			}

			if n, ok := nodeMap[key]; ok {
				return n
			}

			attrs := make(DotAttrs)
			// node label
			label := node.Func.RelString(node.Func.Pkg.Pkg)

			// func signature
			sign := node.Func.Signature
			if node.Func.Parent() != nil {
				sign = node.Func.Parent().Signature
			}

			// omit type from label
			if groupType && sign.Recv() != nil {
				parts := strings.Split(label, ".")
				label = parts[len(parts)-1]
			}

			pkg, _ := build.Import(node.Func.Pkg.Pkg.Path(), "", 0)
			if pkg.Goroot {
				attrs["fillcolor"] = "#adedad"
			} else {
				attrs["fillcolor"] = "moccasin"
			}

			// include pkg name
			if !groupPkg {
				label = fmt.Sprintf("%s\n%s", node.Func.Pkg.Pkg.Name(), label)
			}

			attrs["label"] = label

			// func styles
			if node.Func.Parent() != nil {
				attrs["style"] = "dotted,filled"
			} else if node.Func.Object() != nil && node.Func.Object().Exported() {
				attrs["penwidth"] = "1.5"
			} else {
				attrs["penwidth"] = "0.5"
			}

			c := cluster

			// group by pkg
			if groupPkg {
				label := node.Func.Pkg.Pkg.Name()
				if pkg.Goroot {
					label = node.Func.Pkg.Pkg.Path()
				}
				key := node.Func.Pkg.Pkg.Path()
				if _, ok := c.Clusters[key]; !ok {
					c.Clusters[key] = &DotCluster{
						ID:       key,
						Clusters: make(map[string]*DotCluster),
						Attrs: DotAttrs{
							"penwidth":  "0.8",
							"fontsize":  "16",
							"label":     label,
							"style":     "filled",
							"fillcolor": "lightyellow",
							"URL":       fmt.Sprintf("/?f=%s", key),
							"fontname":  "Tahoma bold",
							"tooltip":   fmt.Sprintf("package: %s", key),
							"rank":      "sink",
						},
					}
					if pkg.Goroot {
						c.Clusters[key].Attrs["fillcolor"] = "#E0FFE1"
					}
				}
				c = c.Clusters[key]
			}

			// group by type
			if groupType && sign.Recv() != nil {
				label := strings.Split(node.Func.RelString(node.Func.Pkg.Pkg), ".")[0]
				key := sign.Recv().Type().String()
				if _, ok := c.Clusters[key]; !ok {
					c.Clusters[key] = &DotCluster{
						ID:       key,
						Clusters: make(map[string]*DotCluster),
						Attrs: DotAttrs{
							"penwidth":  "0.5",
							"fontsize":  "15",
							"fontcolor": "#222222",
							"label":     label,
							"labelloc":  "b",
							"style":     "rounded,filled",
							"fillcolor": "wheat2",
							"tooltip":   fmt.Sprintf("type: %s", key),
						},
					}
					if pkg.Goroot {
						c.Clusters[key].Attrs["fillcolor"] = "#c2e3c2"
					}
				}
				c = c.Clusters[key]
			}

			attrs["tooltip"] = nodeTooltip

			n := &DotNode{
				ID:    node.Func.String(),
				Attrs: attrs,
			}

			if c != nil {
				c.Nodes = append(c.Nodes, n)
			} else {
				nodes = append(nodes, n)
			}

			nodeMap[key] = n
			return n
		}
		callerNode := sprintNode(edge.Caller, true)
		calleeNode := sprintNode(edge.Callee, false)

		// edges
		attrs := make(DotAttrs)

		// dynamic call
		if edge.Site != nil && edge.Site.Common().StaticCallee() == nil {
			attrs["style"] = "dashed"
		}

		// go & defer calls
		switch edge.Site.(type) {
		case *ssa.Go:
			attrs["arrowhead"] = "normalnoneodot"
		case *ssa.Defer:
			attrs["arrowhead"] = "normalnoneodiamond"
		}

		// use position in file where callee is called as tooltip for the edge
		fileEdge := fmt.Sprintf(
			"at %s:%d: calling [%s]",
			filepath.Base(posEdge.Filename),
			posEdge.Line,
			edge.Callee.Func.String(),
		)

		// omit duplicate calls, except for tooltip enhancements
		key := fmt.Sprintf("%s = %s => %s", caller.Func, edge.Description(), callee.Func)
		if _, ok := edgeMap[key]; !ok {
			attrs["tooltip"] = fileEdge
			e := &DotEdge{
				From:  callerNode,
				To:    calleeNode,
				Attrs: attrs,
			}
			edgeMap[key] = e
		} else {
			// make sure, tooltip is created correctly
			if _, okk := edgeMap[key].Attrs["tooltip"]; !okk {
				edgeMap[key].Attrs["tooltip"] = fileEdge
			} else {
				edgeMap[key].Attrs["tooltip"] = fmt.Sprintf(
					"%s\n%s",
					edgeMap[key].Attrs["tooltip"],
					fileEdge,
				)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// get edges form edgeMap
	for _, e := range edgeMap {
		e.From.Attrs["tooltip"] = fmt.Sprintf(
			"%s\n%s",
			e.From.Attrs["tooltip"],
			e.Attrs["tooltip"],
		)
		edges = append(edges, e)
	}

	log.Info("%d/%d edges", len(edges), count)

	title := ""
	dot := &DotGraph{
		Title:   title,
		Minlen:  Minlen,
		Cluster: cluster,
		Nodes:   nodes,
		Edges:   edges,
		Options: map[string]string{
			"minlen":    fmt.Sprint(Minlen),
			"nodesep":   fmt.Sprint(Nodesep),
			"nodeshape": fmt.Sprint(Nodeshape),
			"nodestyle": fmt.Sprint(Nodestyle),
			"rankdir":   fmt.Sprint(Rankdir),
		},
	}

	var buf bytes.Buffer
	if err := dot.WriteDot(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
