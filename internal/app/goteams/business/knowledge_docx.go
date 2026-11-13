// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_storage"

	"golang.org/x/image/webp"
	xhtml "golang.org/x/net/html"
)

const (
	knowledgeDOCXMaxImageCount  = 100
	knowledgeDOCXMaxImageBytes  = 50 * 1024 * 1024
	knowledgeDOCXMaxImageWidth  = 5_730_000
	knowledgeDOCXMaxImageHeight = 7_620_000
)

type knowledgeWordRun struct {
	Text      string
	Bold      bool
	Italic    bool
	Underline bool
	Code      bool
	Break     bool
	Image     *knowledgeWordImage
}

type knowledgeWordImage struct {
	Source         string
	Alt            string
	RelationshipID string
	FileName       string
	ContentType    string
	Data           []byte
	WidthEMU       int64
	HeightEMU      int64
}

type knowledgeWordParagraph struct {
	Style string
	Runs  []knowledgeWordRun
}

type knowledgeWordFormat struct {
	Bold      bool
	Italic    bool
	Underline bool
	Code      bool
}

var knowledgeOrderedListPattern = regexp.MustCompile(`^\s*\d+[.)]\s+`)
var knowledgeMarkdownImagePattern = regexp.MustCompile(`!\[([^\]]*)\]\((?:<([^>]+)>|([^\s)]+))(?:\s+["'][^"']*["'])?\)`)
var knowledgeMarkdownFilePattern = regexp.MustCompile(`^\[([^\]]+)\]\((?:<([^>]+)>|([^\s)]+))\)$`)

func buildKnowledgeDOCX(adminId int, resourceBaseURL, title, content, contentMode string) ([]byte, error) {
	paragraphs := []knowledgeWordParagraph{{
		Style: `Title`,
		Runs:  []knowledgeWordRun{{Text: title}},
	}}
	if contentMode == define.KnowledgeContentModeMarkdown {
		paragraphs = append(paragraphs, knowledgeMarkdownParagraphs(content)...)
	} else {
		paragraphs = append(paragraphs, knowledgeHTMLParagraphs(content)...)
	}
	if len(paragraphs) == 1 {
		paragraphs = append(paragraphs, knowledgeWordParagraph{})
	}
	images := knowledgeResolveDOCXImages(adminId, paragraphs)
	knowledgeMakeDOCXResourceURLsAbsolute(paragraphs, resourceBaseURL)

	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	files := map[string]string{
		`[Content_Types].xml`:          knowledgeDOCXContentTypes,
		`_rels/.rels`:                  knowledgeDOCXRootRelationships,
		`docProps/app.xml`:             knowledgeDOCXAppProperties,
		`docProps/core.xml`:            knowledgeDOCXCoreProperties(title),
		`word/document.xml`:            knowledgeDOCXDocumentXML(paragraphs),
		`word/_rels/document.xml.rels`: knowledgeDOCXDocumentRelationships(images),
		`word/styles.xml`:              knowledgeDOCXStyles,
	}
	for _, name := range []string{
		`[Content_Types].xml`, `_rels/.rels`, `docProps/app.xml`,
		`docProps/core.xml`, `word/document.xml`, `word/_rels/document.xml.rels`, `word/styles.xml`,
	} {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			_ = writer.Close()
			return nil, err
		}
		if _, err = io.WriteString(entry, files[name]); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	for _, imageItem := range images {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: `word/media/` + imageItem.FileName, Method: zip.Deflate})
		if err != nil {
			_ = writer.Close()
			return nil, err
		}
		if _, err = entry.Write(imageItem.Data); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func knowledgeMakeDOCXResourceURLsAbsolute(paragraphs []knowledgeWordParagraph, resourceBaseURL string) {
	baseURL := strings.TrimRight(strings.TrimSpace(resourceBaseURL), `/`)
	if baseURL == `` {
		return
	}
	for paragraphIndex := range paragraphs {
		for runIndex := range paragraphs[paragraphIndex].Runs {
			run := &paragraphs[paragraphIndex].Runs[runIndex]
			if run.Image != nil {
				continue
			}
			run.Text = strings.ReplaceAll(run.Text, `/uploads/`, baseURL+`/uploads/`)
		}
	}
}

func knowledgeHTMLParagraphs(content string) []knowledgeWordParagraph {
	document, err := xhtml.Parse(strings.NewReader(`<body>` + content + `</body>`))
	if err != nil {
		return []knowledgeWordParagraph{{Runs: []knowledgeWordRun{{Text: content}}}}
	}
	body := knowledgeFindHTMLNode(document, `body`)
	if body == nil {
		body = document
	}
	paragraphs := make([]knowledgeWordParagraph, 0)
	for child := body.FirstChild; child != nil; child = child.NextSibling {
		knowledgeAppendHTMLBlock(child, &paragraphs)
	}
	return paragraphs
}

func knowledgeHTMLPlainText(content string) string {
	paragraphs := knowledgeHTMLParagraphs(content)
	lines := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		var line strings.Builder
		for _, run := range paragraph.Runs {
			if run.Break {
				line.WriteByte('\n')
				continue
			}
			line.WriteString(run.Text)
		}
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}

func knowledgeFindHTMLNode(node *xhtml.Node, name string) *xhtml.Node {
	if node.Type == xhtml.ElementNode && node.Data == name {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := knowledgeFindHTMLNode(child, name); found != nil {
			return found
		}
	}
	return nil
}

func knowledgeAppendHTMLBlock(node *xhtml.Node, paragraphs *[]knowledgeWordParagraph) {
	if node.Type == xhtml.TextNode {
		if text := strings.TrimSpace(node.Data); text != `` {
			*paragraphs = append(*paragraphs, knowledgeWordParagraph{Runs: []knowledgeWordRun{{Text: text}}})
		}
		return
	}
	if node.Type != xhtml.ElementNode {
		return
	}
	if name, address, isResource := knowledgeHTMLResource(node); isResource {
		*paragraphs = append(*paragraphs, knowledgeResourceParagraph(name, address))
		return
	}
	switch node.Data {
	case `script`, `style`, `noscript`:
		return
	case `img`:
		paragraph := knowledgeWordParagraph{}
		knowledgeAppendHTMLImageRun(node, &paragraph.Runs)
		*paragraphs = append(*paragraphs, paragraph)
		return
	case `p`, `div`, `blockquote`, `pre`, `li`, `h1`, `h2`, `h3`, `h4`, `h5`, `h6`:
		paragraph := knowledgeWordParagraph{}
		if len(node.Data) == 2 && node.Data[0] == 'h' {
			paragraph.Style = `Heading` + node.Data[1:]
		}
		if node.Data == `li` {
			paragraph.Runs = append(paragraph.Runs, knowledgeWordRun{Text: `• `})
		}
		format := knowledgeWordFormat{Code: node.Data == `pre`}
		knowledgeCollectHTMLRuns(node, &paragraph.Runs, format)
		*paragraphs = append(*paragraphs, paragraph)
		return
	case `table`:
		for row := node.FirstChild; row != nil; row = row.NextSibling {
			knowledgeAppendHTMLTableRows(row, paragraphs)
		}
		return
	case `ul`, `ol`:
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			knowledgeAppendHTMLBlock(child, paragraphs)
		}
		return
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		knowledgeAppendHTMLBlock(child, paragraphs)
	}
}

func knowledgeAppendHTMLTableRows(node *xhtml.Node, paragraphs *[]knowledgeWordParagraph) {
	if node.Type == xhtml.ElementNode && node.Data == `tr` {
		paragraph := knowledgeWordParagraph{}
		for cell := node.FirstChild; cell != nil; cell = cell.NextSibling {
			if cell.Type != xhtml.ElementNode || cell.Data != `td` && cell.Data != `th` {
				continue
			}
			if len(paragraph.Runs) > 0 {
				paragraph.Runs = append(paragraph.Runs, knowledgeWordRun{Text: `\t`})
			}
			knowledgeCollectHTMLRuns(cell, &paragraph.Runs, knowledgeWordFormat{Bold: cell.Data == `th`})
		}
		*paragraphs = append(*paragraphs, paragraph)
		return
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		knowledgeAppendHTMLTableRows(child, paragraphs)
	}
}

func knowledgeCollectHTMLRuns(node *xhtml.Node, runs *[]knowledgeWordRun, format knowledgeWordFormat) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == xhtml.TextNode {
			if child.Data != `` {
				*runs = append(*runs, knowledgeWordRun{
					Text: child.Data, Bold: format.Bold, Italic: format.Italic,
					Underline: format.Underline, Code: format.Code,
				})
			}
			continue
		}
		if child.Type != xhtml.ElementNode || child.Data == `script` || child.Data == `style` {
			continue
		}
		if name, address, isResource := knowledgeHTMLResource(child); isResource {
			resource := knowledgeResourceParagraph(name, address)
			*runs = append(*runs, resource.Runs...)
			continue
		}
		if child.Data == `br` {
			*runs = append(*runs, knowledgeWordRun{Break: true})
			continue
		}
		if child.Data == `img` {
			knowledgeAppendHTMLImageRun(child, runs)
			continue
		}
		if child.Data == `a` {
			before := len(*runs)
			knowledgeCollectHTMLRuns(child, runs, format)
			href := strings.TrimSpace(knowledgeHTMLAttribute(child, `href`))
			if href != `` && !knowledgeRunsContainText((*runs)[before:], href) {
				*runs = append(*runs, knowledgeWordRun{Text: ` (` + href + `)`, Underline: true})
			}
			continue
		}
		next := format
		switch child.Data {
		case `strong`, `b`:
			next.Bold = true
		case `em`, `i`:
			next.Italic = true
		case `u`:
			next.Underline = true
		case `code`, `pre`:
			next.Code = true
		}
		knowledgeCollectHTMLRuns(child, runs, next)
	}
}

func knowledgeRunsContainText(runs []knowledgeWordRun, text string) bool {
	for _, run := range runs {
		if strings.Contains(run.Text, text) {
			return true
		}
	}
	return false
}

func knowledgeAppendHTMLImageRun(node *xhtml.Node, runs *[]knowledgeWordRun) {
	alt := strings.TrimSpace(knowledgeHTMLAttribute(node, `alt`))
	if alt == `` {
		alt = `图片`
	}
	*runs = append(*runs, knowledgeWordRun{
		Text: `[` + alt + `]`, Italic: true,
		Image: &knowledgeWordImage{Source: strings.TrimSpace(knowledgeHTMLAttribute(node, `src`)), Alt: alt},
	})
}

func knowledgeHTMLAttribute(node *xhtml.Node, key string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return attribute.Val
		}
	}
	return ``
}

func knowledgeHTMLHasAttribute(node *xhtml.Node, key string) bool {
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return true
		}
	}
	return false
}

func knowledgeHTMLResource(node *xhtml.Node) (string, string, bool) {
	if knowledgeHTMLHasAttribute(node, `data-editor-file`) {
		name := strings.TrimSpace(knowledgeHTMLAttribute(node, `data-file-name`))
		address := strings.TrimSpace(knowledgeHTMLAttribute(node, `data-file-href`))
		if address == `` {
			if link := knowledgeFindHTMLNode(node, `a`); link != nil {
				address = strings.TrimSpace(knowledgeHTMLAttribute(link, `href`))
			}
		}
		return name, address, true
	}
	if knowledgeHTMLHasAttribute(node, `data-editor-video`) {
		address := strings.TrimSpace(knowledgeHTMLAttribute(node, `data-video-original-url`))
		if address == `` {
			address = strings.TrimSpace(knowledgeHTMLAttribute(node, `data-video-src`))
		}
		return ``, address, true
	}
	return ``, ``, false
}

func knowledgeResourceParagraph(name, address string) knowledgeWordParagraph {
	paragraph := knowledgeWordParagraph{}
	if name != `` {
		paragraph.Runs = append(paragraph.Runs, knowledgeWordRun{Text: name, Bold: true})
	}
	if address != `` {
		if len(paragraph.Runs) > 0 {
			paragraph.Runs = append(paragraph.Runs, knowledgeWordRun{Break: true})
		}
		paragraph.Runs = append(paragraph.Runs, knowledgeWordRun{Text: address, Underline: true})
	}
	return paragraph
}

func knowledgeMarkdownParagraphs(content string) []knowledgeWordParagraph {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	paragraphs := make([]knowledgeWordParagraph, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, `data-editor-video`) {
			videoParagraphs := knowledgeHTMLParagraphs(trimmed)
			if len(videoParagraphs) > 0 {
				paragraphs = append(paragraphs, videoParagraphs...)
				continue
			}
		}
		paragraph := knowledgeWordParagraph{}
		for level := 6; level >= 1; level-- {
			prefix := strings.Repeat(`#`, level) + ` `
			if strings.HasPrefix(trimmed, prefix) {
				paragraph.Style = fmt.Sprintf(`Heading%d`, level)
				trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
				break
			}
		}
		if strings.HasPrefix(trimmed, `- `) || strings.HasPrefix(trimmed, `* `) || strings.HasPrefix(trimmed, `+ `) {
			trimmed = `• ` + strings.TrimSpace(trimmed[2:])
		} else if knowledgeOrderedListPattern.MatchString(trimmed) {
			trimmed = knowledgeOrderedListPattern.ReplaceAllString(trimmed, `• `)
		}
		if fileMatch := knowledgeMarkdownFilePattern.FindStringSubmatch(trimmed); len(fileMatch) > 0 {
			address := fileMatch[2]
			if address == `` {
				address = fileMatch[3]
			}
			paragraph = knowledgeResourceParagraph(fileMatch[1], address)
		} else {
			paragraph.Runs = knowledgeMarkdownRuns(trimmed)
		}
		paragraphs = append(paragraphs, paragraph)
	}
	return paragraphs
}

func knowledgeMarkdownRuns(line string) []knowledgeWordRun {
	matches := knowledgeMarkdownImagePattern.FindAllStringSubmatchIndex(line, -1)
	if len(matches) == 0 {
		return []knowledgeWordRun{{Text: line}}
	}
	runs := make([]knowledgeWordRun, 0, len(matches)*2+1)
	position := 0
	for _, match := range matches {
		if match[0] > position {
			runs = append(runs, knowledgeWordRun{Text: line[position:match[0]]})
		}
		alt := line[match[2]:match[3]]
		source := ``
		if match[4] >= 0 {
			source = line[match[4]:match[5]]
		} else if match[6] >= 0 {
			source = line[match[6]:match[7]]
		}
		if alt == `` {
			alt = `图片`
		}
		runs = append(runs, knowledgeWordRun{
			Text: `[` + alt + `]`, Italic: true,
			Image: &knowledgeWordImage{Source: strings.TrimSpace(source), Alt: alt},
		})
		position = match[1]
	}
	if position < len(line) {
		runs = append(runs, knowledgeWordRun{Text: line[position:]})
	}
	return runs
}

func knowledgeResolveDOCXImages(adminId int, paragraphs []knowledgeWordParagraph) []*knowledgeWordImage {
	resolved := make(map[string]*knowledgeWordImage)
	failed := make(map[string]bool)
	images := make([]*knowledgeWordImage, 0)
	totalBytes := 0
	for paragraphIndex := range paragraphs {
		for runIndex := range paragraphs[paragraphIndex].Runs {
			run := &paragraphs[paragraphIndex].Runs[runIndex]
			if run.Image == nil || run.Image.Source == `` {
				run.Image = nil
				continue
			}
			source := run.Image.Source
			if imageItem, exists := resolved[source]; exists {
				run.Image = imageItem
				continue
			}
			if failed[source] || len(images) >= knowledgeDOCXMaxImageCount {
				knowledgeUseDOCXImageFallback(run)
				continue
			}
			data, extension, contentType, width, height, err := knowledgeLoadDOCXImage(adminId, source)
			if err != nil || totalBytes+len(data) > knowledgeDOCXMaxImageBytes {
				failed[source] = true
				knowledgeUseDOCXImageFallback(run)
				continue
			}
			widthEMU, heightEMU := knowledgeDOCXImageSize(width, height)
			imageItem := &knowledgeWordImage{
				Source: source, Alt: run.Image.Alt, Data: data, ContentType: contentType,
				RelationshipID: fmt.Sprintf(`rId%d`, len(images)+2),
				FileName:       fmt.Sprintf(`image%d.%s`, len(images)+1, extension),
				WidthEMU:       widthEMU, HeightEMU: heightEMU,
			}
			resolved[source] = imageItem
			images = append(images, imageItem)
			totalBytes += len(data)
			run.Image = imageItem
		}
	}
	return images
}

func knowledgeUseDOCXImageFallback(run *knowledgeWordRun) {
	if run.Image == nil {
		return
	}
	source := strings.TrimSpace(run.Image.Source)
	if strings.HasPrefix(source, `http://`) || strings.HasPrefix(source, `https://`) || strings.HasPrefix(source, `/uploads/`) {
		run.Text += ` ` + source
	}
	run.Image = nil
}

func knowledgeLoadDOCXImage(adminId int, source string) ([]byte, string, string, int, int, error) {
	var data []byte
	var err error
	if strings.HasPrefix(source, `data:`) {
		data, err = knowledgeDecodeImageDataURL(source)
	} else {
		_, key, parseErr := lib_storage.ParseStored(source)
		cleanKey := filepath.ToSlash(filepath.Clean(filepath.FromSlash(key)))
		if parseErr != nil || strings.Contains(key, `\`) || cleanKey != key ||
			!lib_storage.M.IsOwned(adminId, lib_storage.BusinessKnowledgeBase, source) {
			return nil, ``, ``, 0, 0, fmt.Errorf(`knowledge image source is not tenant owned`)
		}
		data, err = lib_storage.M.Read(source)
	}
	if err != nil {
		return nil, ``, ``, 0, 0, err
	}
	if len(data) == 0 || len(data) > MaxImageSize {
		return nil, ``, ``, 0, 0, fmt.Errorf(`knowledge image size is invalid`)
	}

	contentType := http.DetectContentType(data)
	extension := ``
	switch contentType {
	case `image/png`:
		extension = `png`
	case `image/jpeg`:
		extension = `jpeg`
	case `image/gif`:
		extension = `gif`
	case `image/webp`:
		decoded, decodeErr := webp.Decode(bytes.NewReader(data))
		if decodeErr != nil {
			return nil, ``, ``, 0, 0, decodeErr
		}
		var converted bytes.Buffer
		if encodeErr := png.Encode(&converted, decoded); encodeErr != nil {
			return nil, ``, ``, 0, 0, encodeErr
		}
		data = converted.Bytes()
		contentType = `image/png`
		extension = `png`
	default:
		return nil, ``, ``, 0, 0, fmt.Errorf(`unsupported knowledge image type`)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, ``, ``, 0, 0, fmt.Errorf(`decode knowledge image dimensions: %w`, err)
	}
	if config.Width <= 0 || config.Height <= 0 {
		return nil, ``, ``, 0, 0, fmt.Errorf(`knowledge image dimensions are invalid`)
	}
	return data, extension, contentType, config.Width, config.Height, nil
}

func knowledgeDecodeImageDataURL(source string) ([]byte, error) {
	comma := strings.IndexByte(source, ',')
	if comma <= len(`data:`) || !strings.Contains(strings.ToLower(source[:comma]), `;base64`) {
		return nil, fmt.Errorf(`invalid image data url`)
	}
	header := strings.ToLower(source[len(`data:`):comma])
	if !strings.HasPrefix(header, `image/`) {
		return nil, fmt.Errorf(`data url is not an image`)
	}
	encoded := source[comma+1:]
	if len(encoded) > MaxImageSize*4/3+4 {
		return nil, fmt.Errorf(`image data url is too large`)
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(encoded)
	}
	return data, err
}

func knowledgeDOCXImageSize(width, height int) (int64, int64) {
	widthEMU := int64(width) * 9525
	heightEMU := int64(height) * 9525
	if widthEMU > knowledgeDOCXMaxImageWidth {
		heightEMU = heightEMU * knowledgeDOCXMaxImageWidth / widthEMU
		widthEMU = knowledgeDOCXMaxImageWidth
	}
	if heightEMU > knowledgeDOCXMaxImageHeight {
		widthEMU = widthEMU * knowledgeDOCXMaxImageHeight / heightEMU
		heightEMU = knowledgeDOCXMaxImageHeight
	}
	return widthEMU, heightEMU
}

func knowledgeDOCXDocumentXML(paragraphs []knowledgeWordParagraph) string {
	var body strings.Builder
	drawingID := 0
	for _, paragraph := range paragraphs {
		body.WriteString(`<w:p>`)
		if paragraph.Style != `` {
			body.WriteString(`<w:pPr><w:pStyle w:val="` + paragraph.Style + `"/></w:pPr>`)
		}
		for _, run := range paragraph.Runs {
			if run.Image != nil {
				drawingID++
				body.WriteString(knowledgeDOCXImageXML(run.Image, drawingID))
				continue
			}
			body.WriteString(`<w:r>`)
			if run.Bold || run.Italic || run.Underline || run.Code {
				body.WriteString(`<w:rPr>`)
				if run.Bold {
					body.WriteString(`<w:b/>`)
				}
				if run.Italic {
					body.WriteString(`<w:i/>`)
				}
				if run.Underline {
					body.WriteString(`<w:u w:val="single"/>`)
				}
				if run.Code {
					body.WriteString(`<w:rFonts w:ascii="Consolas" w:hAnsi="Consolas"/>`)
				}
				body.WriteString(`</w:rPr>`)
			}
			if run.Break {
				body.WriteString(`<w:br/>`)
			} else {
				body.WriteString(`<w:t xml:space="preserve">` + knowledgeEscapeXML(run.Text) + `</w:t>`)
			}
			body.WriteString(`</w:r>`)
		}
		body.WriteString(`</w:p>`)
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" ` +
		`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">` +
		`<w:body>` + body.String() +
		`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/></w:sectPr>` +
		`</w:body></w:document>`
}

func knowledgeDOCXImageXML(imageItem *knowledgeWordImage, drawingID int) string {
	id := fmt.Sprintf(`%d`, drawingID)
	width := fmt.Sprintf(`%d`, imageItem.WidthEMU)
	height := fmt.Sprintf(`%d`, imageItem.HeightEMU)
	alt := knowledgeEscapeXML(imageItem.Alt)
	name := knowledgeEscapeXML(imageItem.FileName)
	return `<w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0">` +
		`<wp:extent cx="` + width + `" cy="` + height + `"/>` +
		`<wp:effectExtent l="0" t="0" r="0" b="0"/>` +
		`<wp:docPr id="` + id + `" name="Picture ` + id + `" descr="` + alt + `"/>` +
		`<wp:cNvGraphicFramePr><a:graphicFrameLocks noChangeAspect="1"/></wp:cNvGraphicFramePr>` +
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">` +
		`<pic:pic><pic:nvPicPr><pic:cNvPr id="0" name="` + name + `"/><pic:cNvPicPr/></pic:nvPicPr>` +
		`<pic:blipFill><a:blip r:embed="` + imageItem.RelationshipID + `"/>` +
		`<a:stretch><a:fillRect/></a:stretch></pic:blipFill>` +
		`<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="` + width + `" cy="` + height + `"/></a:xfrm>` +
		`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic>` +
		`</a:graphicData></a:graphic></wp:inline></w:drawing></w:r>`
}

func knowledgeEscapeXML(value string) string {
	var result bytes.Buffer
	cleaned := strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' || r >= 0x20 && r <= 0xD7FF || r >= 0xE000 && r <= 0xFFFD || r >= 0x10000 && r <= 0x10FFFF {
			return r
		}
		return -1
	}, value)
	_ = xml.EscapeText(&result, []byte(cleaned))
	return result.String()
}

func knowledgeDOCXCoreProperties(title string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" ` +
		`xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>` + knowledgeEscapeXML(title) +
		`</dc:title><dc:creator>GoTeams</dc:creator></cp:coreProperties>`
}

const knowledgeDOCXContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Default Extension="png" ContentType="image/png"/>
<Default Extension="jpeg" ContentType="image/jpeg"/>
<Default Extension="gif" ContentType="image/gif"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>
</Types>`

const knowledgeDOCXRootRelationships = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>`

func knowledgeDOCXDocumentRelationships(images []*knowledgeWordImage) string {
	var relationships strings.Builder
	relationships.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	relationships.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	relationships.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`)
	for _, imageItem := range images {
		relationships.WriteString(`<Relationship Id="` + imageItem.RelationshipID +
			`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/` +
			knowledgeEscapeXML(imageItem.FileName) + `"/>`)
	}
	relationships.WriteString(`</Relationships>`)
	return relationships.String()
}

const knowledgeDOCXAppProperties = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
<Application>GoTeams</Application></Properties>`

const knowledgeDOCXStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:rPr><w:sz w:val="22"/><w:szCs w:val="22"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:after="240"/></w:pPr><w:rPr><w:b/><w:sz w:val="36"/><w:szCs w:val="36"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:rPr><w:b/><w:sz w:val="32"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:basedOn w:val="Normal"/><w:rPr><w:b/><w:sz w:val="28"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:basedOn w:val="Normal"/><w:rPr><w:b/><w:sz w:val="26"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading4"><w:name w:val="heading 4"/><w:basedOn w:val="Normal"/><w:rPr><w:b/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading5"><w:name w:val="heading 5"/><w:basedOn w:val="Normal"/><w:rPr><w:b/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading6"><w:name w:val="heading 6"/><w:basedOn w:val="Normal"/><w:rPr><w:b/></w:rPr></w:style>
</w:styles>`
