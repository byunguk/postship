package publish

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/byunguk/postship/internal/config"
	"github.com/byunguk/postship/internal/gitutil"
	"github.com/byunguk/postship/internal/r2"
)

type Options struct {
	DryRun bool
	NoPush bool
}

var markdownImage = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)

func Run(articleDir string, opts Options) error {
	cfg, _, err := config.Load()
	if err != nil {
		return err
	}
	if err := gitutil.EnsureRepo(cfg.RepoPath); err != nil {
		return fmt.Errorf("configured repo_path is not a Git repository: %w", err)
	}

	articleDir, err = filepath.Abs(articleDir)
	if err != nil {
		return err
	}
	mdPath, err := findMarkdown(articleDir)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(mdPath)
	if err != nil {
		return err
	}

	slug := filepath.Base(articleDir)
	if slug == "." || slug == string(filepath.Separator) || slug == "" {
		slug = strings.TrimSuffix(filepath.Base(mdPath), filepath.Ext(mdPath))
	}

	matches := markdownImage.FindAllStringSubmatch(string(content), -1)
	if len(matches) == 0 {
		fmt.Println("No local Markdown images found; continuing with article only.")
	}

	result := string(content)
	client := r2.New(cfg.R2)
	seen := map[string]string{}

	for _, m := range matches {
		ref := strings.TrimSpace(m[2])
		if isRemote(ref) || strings.HasPrefix(ref, "data:") {
			continue
		}
		cleanRef := strings.Fields(ref)[0]
		imgPath := cleanRef
		if !filepath.IsAbs(imgPath) {
			imgPath = filepath.Join(filepath.Dir(mdPath), filepath.FromSlash(cleanRef))
		}
		if _, err := os.Stat(imgPath); err != nil {
			return fmt.Errorf("image referenced by Markdown not found: %s", cleanRef)
		}
		if url, ok := seen[imgPath]; ok {
			result = strings.ReplaceAll(result, cleanRef, url)
			continue
		}
		key := filepath.ToSlash(filepath.Join("articles", slug, filepath.Base(imgPath)))
		var url string
		if opts.DryRun {
			url = strings.TrimRight(cfg.R2.PublicURL, "/") + "/" + key
			fmt.Printf("[dry-run] upload %s -> r2://%s/%s\n", imgPath, cfg.R2.Bucket, key)
		} else {
			fmt.Printf("Uploading %s...\n", filepath.Base(imgPath))
			url, err = client.Upload(context.Background(), imgPath, key)
			if err != nil {
				return fmt.Errorf("upload %s: %w", imgPath, err)
			}
		}
		seen[imgPath] = url
		result = strings.ReplaceAll(result, cleanRef, url)
	}

	destDir := filepath.Join(cfg.RepoPath, filepath.FromSlash(cfg.ContentDir))
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	dest := filepath.Join(destDir, slug+".md")
	fmt.Printf("Article -> %s\n", dest)
	if opts.DryRun {
		fmt.Printf("[dry-run] would write Markdown, commit, and %s\n", map[bool]string{true: "not push", false: "push"}[opts.NoPush])
		return nil
	}
	if err := os.WriteFile(dest, []byte(result), 0644); err != nil {
		return err
	}

	rel, _ := filepath.Rel(cfg.RepoPath, dest)
	if _, err := gitutil.Run(cfg.RepoPath, "add", filepath.ToSlash(rel)); err != nil {
		return err
	}
	status, err := gitutil.Run(cfg.RepoPath, "status", "--porcelain")
	if err != nil {
		return err
	}
	if status == "" {
		fmt.Println("Nothing changed; no commit created.")
		return nil
	}
	msg := fmt.Sprintf("Publish %s (%s)", slug, time.Now().Format("2006-01-02"))
	if _, err := gitutil.Run(cfg.RepoPath, "commit", "-m", msg); err != nil {
		return err
	}
	fmt.Println("Committed:", msg)
	if !opts.NoPush {
		if _, err := gitutil.Run(cfg.RepoPath, "push"); err != nil {
			return err
		}
		fmt.Println("Pushed to GitHub. Cloudflare Pages should deploy from the GitHub update.")
	}
	return nil
}

func findMarkdown(dir string) (string, error) {
	for _, name := range []string{"article.md", "index.md"} {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no Markdown file found in %s (expected article.md or index.md)", dir)
}

func isRemote(s string) bool {
	s = strings.ToLower(s)
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
