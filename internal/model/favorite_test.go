package model

import "testing"

func TestParseFavoriteContentArticle(t *testing.T) {
	xmlContent := `<favitem type="5"><weburlitem><pagetitle>示例文章</pagetitle><pagedesc>摘要内容</pagedesc><link>https://example.com</link></weburlitem></favitem>`

	content, err := ParseFavoriteContent(xmlContent, 5)
	if err != nil {
		t.Fatalf("ParseFavoriteContent() error = %v", err)
	}
	if content.Summary != "示例文章 - 摘要内容" {
		t.Fatalf("unexpected summary: %q", content.Summary)
	}
	if content.Link != "https://example.com" {
		t.Fatalf("unexpected link: %q", content.Link)
	}
}
