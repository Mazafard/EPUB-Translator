package epub

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
)

type Parser struct {
	logger  *logrus.Logger
	tempDir string
}

func NewParser(logger *logrus.Logger, tempDir string) *Parser {
	return &Parser{
		logger:  logger,
		tempDir: tempDir,
	}
}

func (p *Parser) Extract(epubPath string) (*EPUB, error) {
	p.logger.Debugf("Extracting EPUB: %s", epubPath)

	epubID := generateID()
	extractDir := filepath.Join(p.tempDir, epubID)

	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create extraction directory: %w", err)
	}

	epub := &EPUB{
		ID:        epubID,
		FilePath:  epubPath,
		TempDir:   extractDir,
		CreatedAt: time.Now(),
	}

	if err := p.extractZip(epubPath, extractDir); err != nil {
		return nil, fmt.Errorf("failed to extract ZIP: %w", err)
	}

	if err := p.parseContainer(epub); err != nil {
		return nil, fmt.Errorf("failed to parse container: %w", err)
	}

	if err := p.parsePackage(epub); err != nil {
		return nil, fmt.Errorf("failed to parse package: %w", err)
	}

	if err := p.extractChapters(epub); err != nil {
		return nil, fmt.Errorf("failed to extract chapters: %w", err)
	}

	epub.ProcessedAt = time.Now()
	p.logger.Debugf("Successfully extracted EPUB with %d chapters", len(epub.Chapters))

	return epub, nil
}

func (p *Parser) extractZip(src, dest string) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if err := p.extractFile(file, dest); err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) extractFile(file *zip.File, dest string) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	path := filepath.Join(dest, file.Name)

	if file.FileInfo().IsDir() {
		return os.MkdirAll(path, file.FileInfo().Mode())
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.FileInfo().Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, rc)
	return err
}

func (p *Parser) parseContainer(epub *EPUB) error {
	containerPath := filepath.Join(epub.TempDir, "META-INF", "container.xml")

	data, err := os.ReadFile(containerPath)
	if err != nil {
		return fmt.Errorf("failed to read container.xml: %w", err)
	}

	if err := xml.Unmarshal(data, &epub.Container); err != nil {
		return fmt.Errorf("failed to parse container.xml: %w", err)
	}

	if len(epub.Container.Rootfiles) == 0 {
		return fmt.Errorf("no rootfiles found in container.xml")
	}

	return nil
}

func (p *Parser) parsePackage(epub *EPUB) error {
	packagePath := filepath.Join(epub.TempDir, epub.Container.Rootfiles[0].FullPath)
	epub.Package.OriginalPath = epub.Container.Rootfiles[0].FullPath

	data, err := os.ReadFile(packagePath)
	if err != nil {
		return fmt.Errorf("failed to read package file: %w", err)
	}

	if err := xml.Unmarshal(data, &epub.Package); err != nil {
		return fmt.Errorf("failed to parse package file: %w", err)
	}

	return nil
}

func (p *Parser) extractChapters(epub *EPUB) error {
	packageDir := filepath.Dir(filepath.Join(epub.TempDir, epub.Package.OriginalPath))

	itemMap := make(map[string]Item)
	for _, item := range epub.Package.Manifest.Items {
		itemMap[item.ID] = item
	}

	for order, itemRef := range epub.Package.Spine.ItemRefs {
		item, exists := itemMap[itemRef.IDRef]
		if !exists {
			p.logger.Warnf("Item not found in manifest: %s", itemRef.IDRef)
			continue
		}

		if !isTextContent(item.MediaType) {
			continue
		}

		chapterPath := filepath.Join(packageDir, item.Href)
		content, err := p.extractChapterContent(chapterPath)
		if err != nil {
			p.logger.Warnf("Failed to extract chapter content from %s: %v", chapterPath, err)
			continue
		}

		chapter := Chapter{
			ID:           fmt.Sprintf("%s_%d", epub.ID, order),
			Title:        p.extractTitle(content),
			FilePath:     chapterPath,
			RelativePath: item.Href,
			Content:      content,
			Order:        order,
			WordCount:    countWords(content),
			IsTranslated: false,
		}

		epub.Chapters = append(epub.Chapters, chapter)
	}

	return nil
}

func (p *Parser) extractChapterContent(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(data)))
	if err != nil {
		return "", err
	}

	body := doc.Find("body")
	if body.Length() == 0 {
		return string(data), nil
	}

	html, err := body.Html()
	if err != nil {
		return "", err
	}

	return html, nil
}

func (p *Parser) extractTitle(content string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return "Untitled"
	}

	title := doc.Find("h1, h2, h3, title").First().Text()
	if title == "" {
		title = "Untitled"
	}

	return strings.TrimSpace(title)
}

func (p *Parser) Validate(epub *EPUB) error {
	if epub == nil {
		return fmt.Errorf("epub is nil")
	}

	if epub.ID == "" {
		return fmt.Errorf("epub ID is empty")
	}

	if epub.TempDir == "" {
		return fmt.Errorf("temp directory is empty")
	}

	if len(epub.Container.Rootfiles) == 0 {
		return fmt.Errorf("no rootfiles found")
	}

	if len(epub.Package.Manifest.Items) == 0 {
		return fmt.Errorf("no manifest items found")
	}

	if len(epub.Package.Spine.ItemRefs) == 0 {
		return fmt.Errorf("no spine items found")
	}

	if len(epub.Chapters) == 0 {
		return fmt.Errorf("no chapters extracted")
	}

	return nil
}

func isTextContent(mediaType string) bool {
	return strings.Contains(mediaType, "html") || strings.Contains(mediaType, "xhtml")
}

func countWords(text string) int {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(text))
	if err != nil {
		return 0
	}

	plainText := doc.Text()
	words := strings.Fields(plainText)
	return len(words)
}

func generateID() string {
	return fmt.Sprintf("epub_%d", time.Now().Unix())
}
