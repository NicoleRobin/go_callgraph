package main

import (
	"context"
	"fmt"

	"github.com/nicolerobin/log"
	"go.uber.org/zap"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/static"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/nicolerobin/go_callgraph/dot"
)

func main() {
	log.SetLevel(log.LevelInfo)

	ctx := context.Background()
	pkgs, err := loadPackages(ctx, "../shorturl")
	if err != nil {
		log.Error("loadPackages() failed, err:%s", err)
		return
	}
	err = printPkgs(ctx, pkgs)
	if err != nil {
		log.Error("printPkgs() failed, error:%+v", err)
	}

	ssaProg, ssaPkgs, callGraph := genGraph(ctx, pkgs)
	if err != nil {
		log.Error("genGraph() failed, err:%s", err)
		return
	}
	err = printSsaPkgs(ctx, ssaPkgs)
	if err != nil {
		log.Error("printSsaPkgs() failed, error:%+v", err)
		return
	}
	err = printSsaProg(ctx, ssaProg)
	if err != nil {
		log.Error("printSsaPkgs() failed, error:%+v", err)
		return
	}
	err = printCallGraph(ctx, callGraph)
	if err != nil {
		log.Error("printCallGraph() failed, err:%+v", err)
		return
	}

	bytesOutput, err := dot.GenerateDot(ctx, ssaProg, callGraph, pkgs[0].String())
	if err != nil {
		log.Error("dot.PrintOutput() failed", zap.Error(err))
		return
	}
	log.Debug("dot.PrintOutput() success, len(bytesOutput):%d, bytesOutput:%s", len(bytesOutput), string(bytesOutput))

	// save to file
	imgName, err := dot.DotToImage(ctx, true, "go_callgraph", "svg", bytesOutput)
	if err != nil {
		log.Error("dot.DotToImage() failed, err:%s", err)
		return
	}
	log.Info("dot.DotToImage() success, imgName:%s", imgName)
}

const (
	loadMode = packages.NeedDeps | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedTypes | packages.NeedTypesSizes | packages.NeedImports | packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles
)

func loadPackages(ctx context.Context, packageDir string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode:       loadMode,
		Tests:      false,
		Dir:        packageDir,
		BuildFlags: nil,
	}

	pkgs, err := packages.Load(cfg)
	if err != nil {
		return nil, err
	}
	if errorCount := packages.PrintErrors(pkgs); errorCount > 0 {
		return nil, fmt.Errorf("pkgs contain errors, errCount:%d", errorCount)
	}

	return pkgs, nil
}

func printPkgs(ctx context.Context, pkgs []*packages.Package) error {
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		_ = pkg
		// log.Info("pkg:%+v", pkg)
	})
	for _, pkg := range pkgs {
		log.Debug("pkg:%+v", pkg)
		for pkgImport := range pkg.Imports {
			log.Debug("\t%s", pkgImport)
		}
	}

	return nil
}

func printSsaPkgs(ctx context.Context, pkgs []*ssa.Package) error {
	for _, pkg := range pkgs {
		log.Debug("pkg:%+v", pkg)
	}

	return nil
}

func printSsaProg(ctx context.Context, prog *ssa.Program) error {
	for f := range ssautil.AllFunctions(prog) {
		log.Debug("func:%+v", f)
	}
	return nil
}

func printCallGraph(ctx context.Context, callGraph *callgraph.Graph) error {
	var count = 0
	err := callgraph.GraphVisitEdges(callGraph, func(edge *callgraph.Edge) error {
		count++
		caller := edge.Caller
		callee := edge.Callee
		log.Debug("call node: %s -> %s (%s -> %s)", caller.Func.Pkg, callee.Func.Pkg, caller, callee)
		return nil
	})
	if err != nil {
		log.Error("callgraph.GraphVisitEdges() failed, err:%s", err)
		return err
	}
	log.Info("callgraph.GraphVisitEdges() success, edge count:%d", count)
	return nil
}

func genGraph(ctx context.Context, pkgs []*packages.Package) (*ssa.Program, []*ssa.Package, *callgraph.Graph) {
	// ssaProg, ssaPkgs := ssautil.Packages(pkgs, 0)
	ssaProg, ssaPkgs := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	ssaProg.Build()

	return ssaProg, ssaPkgs, static.CallGraph(ssaProg)
}
