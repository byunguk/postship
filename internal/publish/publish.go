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
var htmlImage = regexp.MustCompile(`(?i)<img\b[^>]*\bsrc\s*=\s*["']([^"']+)["'][^>]*>`)
var frontmatterSlug = regexp.MustCompile(`(?m)^slug\s*:\s*["\']?([^"\'\r\n#]+)["\']?\s*(?:#.*)?$`)
var frontmatterLanguage = regexp.MustCompile(`(?m)^(?:lang|language)\s*:\s*["\']?([^"\'\r\n#]+)["\']?\s*(?:#.*)?$`)
var invalidSlugChars = regexp.MustCompile(`[^a-z0-9-]+`)
var repeatedHyphens = regexp.MustCompile(`-+`)
var validLanguage = regexp.MustCompile(`^[a-z]{2,3}(?:-[a-z0-9]{2,8})*$`)

func Run(articleDir string, opts Options) error {
	cfg, _, err := config.Load()
	if err != nil {
		return err
	}
	if err := gitutil.EnsureRepo(cfg.RepoPath); err != nil {
		return fmt.Errorf("configured repo_path is not a Git repository: %w", err)
	}
	if !opts.DryRun {
		if err := syncTargetRepo(cfg.RepoPath); err != nil {
			return err
		}
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

	slug, slugSource, err := resolveSlug(string(content), articleDir, mdPath)
	if err != nil {
		return err
	}
	fmt.Printf("Using slug: %s (%s)\n", slug, slugSource)
	language, err := languageFromFrontmatter(string(content))
	if err != nil {
		return err
	}
	if language != "en" && slugSource != "frontmatter" {
		return fmt.Errorf("translated articles must declare the same explicit English slug as the English article")
	}
	fmt.Printf("Using language: %s\n", language)

	refs := imageReferences(string(content))
	if len(refs) == 0 {
		fmt.Println("No local images found; continuing with article only.")
	}

	result := string(content)
	client := r2.New(cfg.R2)
	seen := map[string]string{}

	for _, ref := range refs {
		if isRemote(ref) || strings.HasPrefix(ref, "data:") {
			continue
		}
		cleanRef := ref
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
		keyParts := []string{"articles", slug}
		if language != "en" {
			keyParts = append(keyParts, language)
		}
		keyParts = append(keyParts, filepath.Base(imgPath))
		key := filepath.ToSlash(filepath.Join(keyParts...))
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
	if language != "en" {
		destDir = filepath.Join(destDir, language)
	}
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
	if language != "en" {
		msg = fmt.Sprintf("Publish %s [%s] (%s)", slug, language, time.Now().Format("2006-01-02"))
	}
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

func syncTargetRepo(repo string) error {
	status, err := gitutil.Run(repo, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("check target repository before pull: %w", err)
	}
	if status != "" {
		return fmt.Errorf("target repository has uncommitted changes; commit or stash them before publishing")
	}
	fmt.Println("Syncing target repository (git pull --ff-only)...")
	if _, err := gitutil.Run(repo, "pull", "--ff-only"); err != nil {
		return fmt.Errorf("sync target repository before publishing: %w", err)
	}
	return nil
}

func imageReferences(content string) []string {
	refs := make([]string, 0)
	for _, match := range markdownImage.FindAllStringSubmatch(content, -1) {
		ref := strings.TrimSpace(match[2])
		if fields := strings.Fields(ref); len(fields) > 0 {
			refs = append(refs, fields[0])
		}
	}
	for _, match := range htmlImage.FindAllStringSubmatch(content, -1) {
		refs = append(refs, strings.TrimSpace(match[1]))
	}
	return refs
}

func resolveSlug(content, articleDir, mdPath string) (string, string, error) {
	if slug := slugFromFrontmatter(content); slug != "" {
		normalized := normalizeSlug(slug)
		if normalized == "" {
			return "", "", fmt.Errorf("frontmatter slug %q does not contain any URL-safe characters", slug)
		}
		return normalized, "frontmatter", nil
	}

	fallback := filepath.Base(articleDir)
	if fallback == "." || fallback == string(filepath.Separator) || fallback == "" {
		fallback = strings.TrimSuffix(filepath.Base(mdPath), filepath.Ext(mdPath))
	}
	normalized := normalizeSlug(fallback)
	if normalized == "" {
		return "", "", fmt.Errorf("cannot derive a valid slug from article directory %q", articleDir)
	}
	return normalized, "article directory", nil
}

func slugFromFrontmatter(content string) string {
	return frontmatterValue(content, frontmatterSlug)
}

func languageFromFrontmatter(content string) (string, error) {
	language := strings.ToLower(strings.TrimSpace(frontmatterValue(content, frontmatterLanguage)))
	if language == "" {
		return "en", nil
	}
	language = strings.ReplaceAll(language, "_", "-")
	if !validLanguage.MatchString(language) {
		return "", fmt.Errorf("invalid article language %q; use a BCP 47 code such as en, ko, or ko-kr", language)
	}
	return language, nil
}

func frontmatterValue(content string, pattern *regexp.Regexp) string {
	content = strings.TrimPrefix(content, "\ufeff")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return ""
	}

	frontmatter := strings.Join(lines[1:end], "\n")
	m := pattern.FindStringSubmatch(frontmatter)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func normalizeSlug(slug string) string {
	slug = strings.ToLower(strings.TrimSpace(slug))
	slug = strings.ReplaceAll(slug, "_", "-")
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = invalidSlugChars.ReplaceAllString(slug, "-")
	slug = repeatedHyphens.ReplaceAllString(slug, "-")
	return strings.Trim(slug, "-")
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
