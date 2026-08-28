package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	RepoPath   string   `json:"repo_path"`
	ContentDir string   `json:"content_dir"`
	R2         R2Config `json:"r2"`
}

type R2Config struct {
	AccountID       string `json:"account_id"`
	Bucket          string `json:"bucket"`
	PublicURL       string `json:"public_url"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
}

func defaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "postship", "config.json"), nil
}

func Load() (Config, string, error) {
	p, err := defaultPath()
	if err != nil {
		return Config{}, "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return Config{}, p, fmt.Errorf("cannot read config; run 'postship init' first: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, p, err
	}
	return cfg, p, validate(cfg)
}

func InteractiveInit() error {
	reader := bufio.NewReader(os.Stdin)
	ask := func(label, def string) string {
		if def != "" {
			fmt.Printf("%s [%s]: ", label, def)
		} else {
			fmt.Printf("%s: ", label)
		}
		s, _ := reader.ReadString('\n')
		s = strings.TrimSpace(s)
		if s == "" {
			return def
		}
		return s
	}
	fmt.Println("PostShip setup")
	cfg := Config{}
	cfg.RepoPath = ask("Local Git repository path", "./")
	if abs, err := filepath.Abs(cfg.RepoPath); err == nil {
		cfg.RepoPath = abs
	}
	cfg.ContentDir = ask("Astro content directory", "src/content/blog")
	cfg.R2.AccountID = ask("Cloudflare Account ID", "")
	cfg.R2.Bucket = ask("R2 bucket", "")
	cfg.R2.PublicURL = strings.TrimRight(ask("R2 public base URL", ""), "/")
	cfg.R2.AccessKeyID = ask("R2 Access Key ID", "")
	cfg.R2.SecretAccessKey = ask("R2 Secret Access Key", "")
	if err := validate(cfg); err != nil {
		return err
	}
	p, err := defaultPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(p, b, 0600); err != nil {
		return err
	}
	fmt.Println("Saved:", p)
	fmt.Println("Note: v0.1 stores R2 credentials in this 0600 config file. macOS Keychain support is a recommended next step.")
	return nil
}

func validate(c Config) error {
	var missing []string
	if c.RepoPath == "" {
		missing = append(missing, "repo_path")
	}
	if c.ContentDir == "" {
		missing = append(missing, "content_dir")
	}
	if c.R2.AccountID == "" {
		missing = append(missing, "r2.account_id")
	}
	if c.R2.Bucket == "" {
		missing = append(missing, "r2.bucket")
	}
	if c.R2.PublicURL == "" {
		missing = append(missing, "r2.public_url")
	}
	if c.R2.AccessKeyID == "" {
		missing = append(missing, "r2.access_key_id")
	}
	if c.R2.SecretAccessKey == "" {
		missing = append(missing, "r2.secret_access_key")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing config: %s", strings.Join(missing, ", "))
	}
	return nil
}
