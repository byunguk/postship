package postship

import (
	"flag"
	"fmt"
	"os"

	"github.com/byunguk/postship/internal/config"
	"github.com/byunguk/postship/internal/publish"
)

var Version = "dev"

func Execute() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "init":
		if err := config.InteractiveInit(); err != nil {
			fatal(err)
		}
	case "publish":
		fs := flag.NewFlagSet("publish", flag.ExitOnError)
		dryRun := fs.Bool("dry-run", false, "validate and show actions without uploading or pushing")
		noPush := fs.Bool("no-push", false, "commit locally but do not git push")
		_ = fs.Parse(os.Args[2:])
		path := "."
		if fs.NArg() > 0 {
			path = fs.Arg(0)
		}
		if err := publish.Run(path, publish.Options{DryRun: *dryRun, NoPush: *noPush}); err != nil {
			fatal(err)
		}
	case "config":
		cfg, path, err := config.Load()
		if err != nil {
			fatal(err)
		}
		fmt.Printf("Config: %s\nRepo: %s\nContent dir: %s\nR2 bucket: %s\nPublic URL: %s\n", path, cfg.RepoPath, cfg.ContentDir, cfg.R2.Bucket, cfg.R2.PublicURL)
	case "version", "--version", "-v":
		fmt.Println("postship", Version)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`PostShip - publish Markdown + images to a static blog

Usage:
  postship init
  postship publish [article-dir] [--dry-run] [--no-push]
  postship config
  postship version

Expected article directory:
  article.md (or index.md)
  images/
    cover.webp
    image-1.webp
`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "postship:", err)
	os.Exit(1)
}
