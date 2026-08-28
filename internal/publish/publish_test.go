package publish

import "testing"

func TestSlugFromFrontmatter(t *testing.T) {
	content := `---
title: "Know This Before Buying a Tesla"
slug: "tesla-buying-guide"
description: "test"
---

# Article
`
	if got := slugFromFrontmatter(content); got != "tesla-buying-guide" {
		t.Fatalf("slugFromFrontmatter() = %q, want %q", got, "tesla-buying-guide")
	}
}

func TestResolveSlugPrefersFrontmatter(t *testing.T) {
	content := `---
slug: custom-public-url
---
`
	got, source, err := resolveSlug(content, "/tmp/some-folder-name", "/tmp/some-folder-name/article.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "custom-public-url" {
		t.Fatalf("resolveSlug() = %q, want %q", got, "custom-public-url")
	}
	if source != "frontmatter" {
		t.Fatalf("source = %q, want frontmatter", source)
	}
}

func TestResolveSlugFallsBackToDirectory(t *testing.T) {
	content := `---
title: No explicit slug
---
`
	got, source, err := resolveSlug(content, "/tmp/My Article Folder", "/tmp/My Article Folder/article.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "my-article-folder" {
		t.Fatalf("resolveSlug() = %q, want %q", got, "my-article-folder")
	}
	if source != "article directory" {
		t.Fatalf("source = %q, want article directory", source)
	}
}

func TestNormalizeSlug(t *testing.T) {
	if got := normalizeSlug("  My_Custom Slug!!  "); got != "my-custom-slug" {
		t.Fatalf("normalizeSlug() = %q, want %q", got, "my-custom-slug")
	}
}

func TestImageReferencesIncludesMarkdownAndHTML(t *testing.T) {
	content := `![Markdown image](images/markdown.png)
<figure><img class="article-image" src="images/html.jpg" alt="HTML image" /></figure>
<IMG SRC='images/uppercase.webp'>`

	got := imageReferences(content)
	want := []string{"images/markdown.png", "images/html.jpg", "images/uppercase.webp"}
	if len(got) != len(want) {
		t.Fatalf("imageReferences() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("imageReferences()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
