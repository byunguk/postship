# PostShip

PostShip publishes a local Markdown article to a Git-backed static blog while uploading local Markdown images to Cloudflare R2.

Repository: `https://github.com/byunguk/postship`

## Workflow

```text
article.md + local images
       |
       v
   postship
    /    \
   v      v
  R2    Git repo
(images) (Markdown)
           |
           v
         GitHub
           |
           v
   Cloudflare Pages
```

PostShip does not call the Cloudflare Pages API. Your Pages project should already be connected to GitHub; a successful `git push` triggers the build/deploy.

## Install with Homebrew

After `github.com/byunguk/homebrew-tap` is created and the first release has been published:

```bash
brew install byunguk/tap/postship
```

## First-time setup

```bash
postship init
```

You will be asked for:

- Local Git repository path
- Astro content directory (default `src/content/blog`)
- Cloudflare Account ID
- R2 bucket
- R2 public URL, e.g. `https://images.knowthisbefore.com`
- R2 Access Key ID
- R2 Secret Access Key

The v0.1 implementation stores configuration at `~/.config/postship/config.json` with mode `0600`. Moving secrets to macOS Keychain is recommended for a later release.

GitHub credentials are intentionally not managed by PostShip. Configure normal Git/SSH or GitHub CLI authentication first.

## Article format

```text
buying-a-tesla/
├── article.md
└── images/
    ├── cover.webp
    └── charging.webp
```

Example Markdown:

```markdown
---
title: "Know This Before Buying a Tesla"
slug: "know-this-before-buying-a-tesla"
description: "Important things to know before buying a Tesla."
publishDate: 2026-08-28
---

# Know This Before Buying a Tesla

Before buying one, consider these details.

![Home charging](./images/charging.webp)
```

Publish:

```bash
postship publish ./buying-a-tesla
```

PostShip uses the frontmatter `slug` as the canonical publishing slug. If `slug` is omitted, it falls back to the article directory name. The same slug is used for both the published Markdown filename and the R2 object path.

For the example above, PostShip uploads the image to:

```text
articles/know-this-before-buying-a-tesla/charging.webp
```

and rewrites the published Markdown image URL to:

```text
https://images.example.com/articles/know-this-before-buying-a-tesla/charging.webp
```

The final Markdown file is written to your configured content directory as:

```text
src/content/blog/know-this-before-buying-a-tesla.md
```

Then PostShip runs `git add`, `git commit`, and `git push`.

## Useful options

```bash
postship publish ./buying-a-tesla --dry-run
postship publish ./buying-a-tesla --no-push
postship config
postship version
```

## Local development

```bash
go mod tidy
go build -o postship .
./postship version
```

## Homebrew release setup

Create these repositories:

```text
github.com/byunguk/postship
github.com/byunguk/homebrew-tap
```

The `homebrew-tap` repository can initially be empty.

Create a GitHub Personal Access Token that can write to `byunguk/homebrew-tap`, then add it to the **postship** repository as this Actions secret:

```text
HOMEBREW_TAP_GITHUB_TOKEN
```

Create the first release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

GitHub Actions runs GoReleaser. It will:

1. Build macOS Intel/Apple Silicon and Linux binaries.
2. Create a GitHub Release.
3. Calculate checksums.
4. Generate/update `Formula/postship.rb` in `byunguk/homebrew-tap`.

Then users can install:

```bash
brew install byunguk/tap/postship
```

To release an update:

```bash
git tag v0.1.1
git push origin v0.1.1
brew upgrade postship
```

## R2 permissions

Use a dedicated R2 API credential with the narrowest permissions needed to write objects to the blog media bucket. Do not use a broad Cloudflare Global API Key.

## Current v0.1 limitations

- Markdown image parsing intentionally handles standard `![alt](path)` syntax only.
- No image resizing/compression yet.
- No cleanup of orphaned R2 images yet.
- R2 credentials are stored in a `0600` config file rather than Keychain.
- The target Git repository must already exist locally and have a working remote/authentication setup.
