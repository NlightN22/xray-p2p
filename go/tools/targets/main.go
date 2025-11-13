package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/buildtarget"
)

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "matrix":
		runMatrix(args)
	case "assets":
		runAssets(args)
	case "list":
		runList(args)
	case "build":
		runBuild(args)
	case "deps":
		runDeps(args)
	default:
		usage()
		os.Exit(2)
	}
}

func runMatrix(args []string) {
	if len(args) > 0 {
		usage()
		os.Exit(2)
	}

	targets := buildtarget.ReleaseTargets()
	include := make([]map[string]string, 0, len(targets))
	for _, t := range targets {
		include = append(include, map[string]string{
			"goos":    t.GOOS,
			"goarch":  t.GOARCH,
			"archive": string(t.Archive),
			"ext":     t.BinaryExt,
		})
	}

	payload := map[string]any{"include": include}
	data, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("encode matrix: %v", err)
	}
	fmt.Println(string(data))
}

func runAssets(args []string) {
	fs := flag.NewFlagSet("assets", flag.ExitOnError)
	name := fs.String("name", "xp2p", "release name prefix")
	version := fs.String("version", "", "semantic version (without leading v)")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("parse flags: %v", err)
	}
	if strings.TrimSpace(*version) == "" {
		log.Fatal("version is required")
	}

	targets := buildtarget.ReleaseTargets()
	for _, t := range targets {
		fmt.Printf("%s\t%s\t%s\n", t.Identifier(), t.ArchiveName(*name, *version), t.LatestArchiveName(*name))
	}

	msiRelease := fmt.Sprintf("%s-%s-windows-amd64.msi", *name, *version)
	msiLatest := fmt.Sprintf("%s-latest-windows-amd64.msi", *name)
	fmt.Printf("windows-amd64-msi\t%s\t%s\n", msiRelease, msiLatest)
}

func runList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	scope := fs.String("scope", "release", "target scope (release|all)")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("parse flags: %v", err)
	}

	var targets []buildtarget.Target
	switch strings.ToLower(strings.TrimSpace(*scope)) {
	case "release":
		targets = buildtarget.ReleaseTargets()
	case "all":
		targets = buildtarget.All()
	default:
		log.Fatalf("unknown scope %q (expected release|all)", *scope)
	}

	for _, t := range targets {
		fmt.Println(t.Identifier())
	}
}

func runBuild(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	targetID := fs.String("target", "", "target identifier (GOOS-GOARCH)")
	baseDir := fs.String("base", "build", "base directory for build outputs")
	outDir := fs.String("out-dir", "", "explicit output directory (overrides --base)")
	binaryName := fs.String("binary", "xp2p", "binary name without extension")
	pkg := fs.String("pkg", "./go/cmd/xp2p", "Go package to build")
	ldflags := fs.String("ldflags", "", "ldflags passed to go build")
	trimpath := fs.Bool("trimpath", true, "enable -trimpath during build")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("parse flags: %v", err)
	}
	if strings.TrimSpace(*targetID) == "" {
		log.Fatal("--target is required")
	}

	goos, goarch, err := splitIdentifier(*targetID)
	if err != nil {
		log.Fatalf("parse target: %v", err)
	}

	target, ok := buildtarget.Lookup(goos, goarch)
	if !ok {
		log.Fatalf("unknown target %s", *targetID)
	}

	dir := strings.TrimSpace(*outDir)
	if dir == "" {
		dir = target.OutputDir(*baseDir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	output := filepath.Join(dir, target.BinaryName(*binaryName))

	argsList := []string{"build"}
	if *trimpath {
		argsList = append(argsList, "-trimpath")
	}
	if v := strings.TrimSpace(*ldflags); v != "" {
		argsList = append(argsList, "-ldflags", v)
	}
	argsList = append(argsList, "-o", output, *pkg)

	cmd := exec.Command("go", argsList...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = buildEnv(target)

	if err := cmd.Run(); err != nil {
		log.Fatalf("go build failed: %v", err)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: targets <matrix|assets|list|build|deps> [flags]\n")
}

func buildEnv(t buildtarget.Target) []string {
	env := os.Environ()
	env = append(env, "GOOS="+t.GOOS, "GOARCH="+t.GOARCH)
	for k, v := range t.GoEnv {
		env = append(env, k+"="+v)
	}
	return env
}

func splitIdentifier(id string) (string, string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", "", errors.New("empty identifier")
	}
	parts := strings.SplitN(id, "-", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid identifier %q", id)
	}
	return parts[0], parts[1], nil
}

func runDeps(args []string) {
	fs := flag.NewFlagSet("deps", flag.ExitOnError)
	targetID := fs.String("target", "", "target identifier (GOOS-GOARCH)")
	destDir := fs.String("dest", "", "directory where dependencies will be staged (overrides --out-dir/--base)")
	outDir := fs.String("out-dir", "", "explicit output directory")
	baseDir := fs.String("base", "build", "base directory for build outputs")
	repoRoot := fs.String("repo", ".", "repository root for resolving dependencies")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("parse flags: %v", err)
	}

	if strings.TrimSpace(*targetID) == "" {
		log.Fatal("--target is required")
	}

	goos, goarch, err := splitIdentifier(*targetID)
	if err != nil {
		log.Fatalf("parse target: %v", err)
	}

	target, ok := buildtarget.Lookup(goos, goarch)
	if !ok {
		log.Fatalf("unknown target %s", *targetID)
	}
	if len(target.Dependencies) == 0 {
		return
	}

	dir := strings.TrimSpace(*destDir)
	if dir == "" {
		dir = strings.TrimSpace(*outDir)
	}
	if dir == "" {
		dir = target.OutputDir(strings.TrimSpace(*baseDir))
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatalf("create dest dir: %v", err)
	}

	for _, dep := range target.Dependencies {
		if err := stageDependency(*repoRoot, dir, dep); err != nil {
			log.Fatalf("prepare dependency %s: %v", dep.Source, err)
		}
	}
}

func stageDependency(repoRoot, destDir string, dep buildtarget.Dependency) error {
	src := filepath.Join(repoRoot, filepath.FromSlash(dep.Source))
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) && dep.Optional {
			log.Printf("warning: optional dependency %s not found, skipping", dep.Source)
			return nil
		}
		return fmt.Errorf("stat source: %w", err)
	}

	destName := dep.Destination
	if strings.TrimSpace(destName) == "" {
		destName = filepath.Base(src)
	}
	dst := filepath.Join(destDir, destName)
	if err := copyFile(src, dst); err != nil {
		return fmt.Errorf("copy %s to %s: %w", src, dst, err)
	}
	return nil
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	info, err := source.Stat()
	if err != nil {
		return err
	}

	dest, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer dest.Close()

	if _, err := io.Copy(dest, source); err != nil {
		return err
	}
	return nil
}
