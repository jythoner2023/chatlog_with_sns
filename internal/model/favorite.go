package model

import (
	"encoding/xml"
	"fmt"
	"html"
	"strings"
	"time"
)

var favoriteTypeNames = map[int]string{
	1:  "文本",
	2:  "图片",
	4:  "视频",
	5:  "文章",
	6:  "位置",
	8:  "文件",
	14: "聊天记录",
	16: "视频",
	18: "笔记",
	19: "名片",
	20: "视频号",
}

var favoriteTypeFilters = map[string]int{
	"text":     1,
	"image":    2,
	"video":    4,
	"article":  5,
	"location": 6,
	"file":     8,
	"chat":     14,
	"note":     18,
	"card":     19,
	"finder":   20,
}

type FavoriteItem struct {
	ID                int64  `json:"id"`
	TypeCode          int    `json:"type_code"`
	Type              string `json:"type"`
	UpdateTime        int64  `json:"update_time"`
	UpdateTimeStr     string `json:"update_time_str"`
	Summary           string `json:"summary"`
	Title             string `json:"title,omitempty"`
	Description       string `json:"description,omitempty"`
	Link              string `json:"link,omitempty"`
	FromUser          string `json:"from_user,omitempty"`
	FromDisplayName   string `json:"from,omitempty"`
	SourceChat        string `json:"source_chat,omitempty"`
	SourceChatDisplay string `json:"source_chat_name,omitempty"`
	XMLContent        string `json:"xml_content,omitempty"`
}

type FavoriteContent struct {
	Summary     string
	Title       string
	Description string
	Link        string
}

type favoriteXML struct {
	XMLName    xml.Name           `xml:"favitem"`
	TypeAttr   int                `xml:"type,attr"`
	Desc       string             `xml:"desc"`
	Title      string             `xml:"title"`
	Link       string             `xml:"link"`
	WebURLItem favoriteWebURLItem `xml:"weburlitem"`
	FinderFeed favoriteFinderFeed `xml:"finderFeed"`
	LocItem    favoriteLocItem    `xml:"locitem"`
	DataList   favoriteDataList   `xml:"datalist"`
}

type favoriteWebURLItem struct {
	PageTitle string `xml:"pagetitle"`
	PageDesc  string `xml:"pagedesc"`
	Link      string `xml:"link"`
}

type favoriteFinderFeed struct {
	Nickname string `xml:"nickname"`
	Desc     string `xml:"desc"`
}

type favoriteLocItem struct {
	Label   string `xml:"label"`
	POIName string `xml:"poiname"`
}

type favoriteDataList struct {
	Items []favoriteDataItem `xml:"dataitem"`
}

type favoriteDataItem struct {
	DataTitle string `xml:"datatitle"`
	DataDesc  string `xml:"datadesc"`
	DataSrc   string `xml:"datasrcname"`
	Label     string `xml:"label"`
	POIName   string `xml:"poiname"`
}

func FavoriteTypeName(typeCode int) string {
	if name, ok := favoriteTypeNames[typeCode]; ok {
		return name
	}
	return fmt.Sprintf("type=%d", typeCode)
}

func FavoriteTypeCode(filter string) (int, bool) {
	code, ok := favoriteTypeFilters[strings.ToLower(strings.TrimSpace(filter))]
	return code, ok
}

func ParseFavoriteContent(xmlContent string, favType int) (*FavoriteContent, error) {
	content := &FavoriteContent{Summary: defaultFavoriteSummary(favType)}
	if strings.TrimSpace(xmlContent) == "" {
		return content, nil
	}

	var doc favoriteXML
	if err := xml.Unmarshal([]byte(xmlContent), &doc); err != nil {
		return content, err
	}

	firstItem := favoriteDataItem{}
	if len(doc.DataList.Items) > 0 {
		firstItem = doc.DataList.Items[0]
	}

	title := firstNonEmpty(doc.WebURLItem.PageTitle, doc.Title, firstItem.DataTitle)
	desc := firstNonEmpty(doc.Desc, doc.WebURLItem.PageDesc, firstItem.DataDesc)
	link := firstNonEmpty(doc.Link, doc.WebURLItem.Link)

	content.Title = cleanFavoriteText(title)
	content.Description = cleanFavoriteText(desc)
	content.Link = cleanFavoriteText(link)

	switch favType {
	case 1:
		content.Summary = content.Description
	case 2:
		content.Summary = "[图片收藏]"
	case 4, 16:
		content.Summary = fallbackSummary(defaultFavoriteSummary(favType), content.Title, content.Description)
	case 5:
		content.Summary = joinFavoriteSummary(content.Title, content.Description, " - ", defaultFavoriteSummary(favType))
	case 6:
		content.Title = cleanFavoriteText(firstNonEmpty(doc.LocItem.POIName, firstItem.POIName))
		content.Description = cleanFavoriteText(firstNonEmpty(doc.LocItem.Label, firstItem.Label))
		content.Summary = joinFavoriteSummary(content.Title, content.Description, " - ", defaultFavoriteSummary(favType))
	case 8:
		content.Summary = fallbackSummary(defaultFavoriteSummary(favType), content.Title, content.Description)
	case 14:
		content.Title = cleanFavoriteText(firstNonEmpty(doc.Title, "聊天记录"))
		if len(doc.DataList.Items) > 0 {
			content.Description = fmt.Sprintf("%d 条消息", len(doc.DataList.Items))
		}
		content.Summary = joinFavoriteSummary(content.Title, content.Description, "（", defaultFavoriteSummary(favType))
	case 18:
		content.Title = cleanFavoriteText(firstNonEmpty(doc.Title, firstItem.DataTitle, "笔记"))
		if content.Description == "" {
			content.Description = cleanFavoriteText(firstItem.DataDesc)
		}
		content.Summary = joinFavoriteSummary(content.Title, content.Description, " - ", defaultFavoriteSummary(favType))
	case 19:
		content.Summary = fallbackSummary(defaultFavoriteSummary(favType), content.Description, content.Title)
	case 20:
		content.Title = cleanFavoriteText(doc.FinderFeed.Nickname)
		content.Description = cleanFavoriteText(doc.FinderFeed.Desc)
		content.Summary = joinFavoriteSummary(content.Title, content.Description, " ", defaultFavoriteSummary(favType))
	default:
		content.Summary = fallbackSummary(defaultFavoriteSummary(favType), content.Description, content.Title)
	}

	if content.Summary == "" {
		content.Summary = defaultFavoriteSummary(favType)
	}
	return content, nil
}

func (f *FavoriteItem) PlainText() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("[%s] [%s] %s", f.UpdateTimeStr, f.Type, f.Summary))
	if from := f.FromName(); from != "" {
		b.WriteString("\n来自: ")
		b.WriteString(from)
	}
	if chat := f.SourceChatName(); chat != "" {
		b.WriteString("\n聊天: ")
		b.WriteString(chat)
	}
	if f.Link != "" {
		b.WriteString("\n链接: ")
		b.WriteString(f.Link)
	}

	return b.String()
}

func (f *FavoriteItem) FromName() string {
	if f.FromDisplayName != "" {
		return f.FromDisplayName
	}
	return f.FromUser
}

func (f *FavoriteItem) SourceChatName() string {
	if f.SourceChatDisplay != "" {
		return f.SourceChatDisplay
	}
	return f.SourceChat
}

func BuildFavoriteItem(id int64, typeCode int, updateTime int64, xmlContent, fromUser, sourceChat string, parsed *FavoriteContent) *FavoriteItem {
	item := &FavoriteItem{
		ID:            id,
		TypeCode:      typeCode,
		Type:          FavoriteTypeName(typeCode),
		UpdateTime:    updateTime,
		UpdateTimeStr: time.Unix(updateTime, 0).Format("2006-01-02 15:04:05"),
		FromUser:      fromUser,
		SourceChat:    sourceChat,
		XMLContent:    xmlContent,
	}
	if parsed != nil {
		item.Summary = parsed.Summary
		item.Title = parsed.Title
		item.Description = parsed.Description
		item.Link = parsed.Link
	}
	if item.Summary == "" {
		item.Summary = defaultFavoriteSummary(typeCode)
	}
	return item
}

func defaultFavoriteSummary(favType int) string {
	switch favType {
	case 2:
		return "[图片收藏]"
	case 4, 16:
		return "[视频收藏]"
	case 5:
		return "[文章收藏]"
	case 6:
		return "[位置收藏]"
	case 8:
		return "[文件收藏]"
	case 14:
		return "[聊天记录收藏]"
	case 18:
		return "[笔记收藏]"
	case 19:
		return "[名片收藏]"
	case 20:
		return "[视频号收藏]"
	default:
		return "[收藏]"
	}
}

func cleanFavoriteText(s string) string {
	return strings.TrimSpace(html.UnescapeString(s))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if clean := cleanFavoriteText(value); clean != "" {
			return clean
		}
	}
	return ""
}

func fallbackSummary(defaultValue string, values ...string) string {
	for _, value := range values {
		if value = cleanFavoriteText(value); value != "" {
			return value
		}
	}
	return defaultValue
}

func joinFavoriteSummary(title, desc, sep, defaultValue string) string {
	title = cleanFavoriteText(title)
	desc = cleanFavoriteText(desc)
	switch {
	case title != "" && desc != "":
		if sep == "（" {
			return title + sep + desc + "）"
		}
		return title + sep + desc
	case title != "":
		return title
	case desc != "":
		return desc
	default:
		return defaultValue
	}
}
