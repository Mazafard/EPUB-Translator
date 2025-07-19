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

// LoadFromDirectory loads an EPUB from an already extracted directory
func (p *Parser) LoadFromDirectory(epubID string) (*EPUB, error) {
	extractDir := filepath.Join(p.tempDir, epubID)

	// Check if directory exists
	if _, err := os.Stat(extractDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("EPUB directory not found: %s", extractDir)
	}

	p.logger.Debugf("Loading EPUB from directory: %s", extractDir)

	epub := &EPUB{
		ID:        epubID,
		TempDir:   extractDir,
		CreatedAt: time.Now(),
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
	p.logger.Debugf("Successfully loaded EPUB from directory with %d chapters", len(epub.Chapters))

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
	// Get the relative path from epub temp dir to package dir for URL rewriting
	packageDirRelative := filepath.Dir(epub.Package.OriginalPath)

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

		// Rewrite media URLs in the content
		rewrittenContent := p.rewriteMediaURLs(content, epub.ID, packageDirRelative)

		chapter := Chapter{
			ID:           fmt.Sprintf("%s_%d", epub.ID, order),
			Title:        p.extractTitle(content), // Use original content for title extraction
			FilePath:     chapterPath,
			RelativePath: item.Href,
			Content:      rewrittenContent, // Use rewritten content for display
			Order:        order,
			WordCount:    countWords(content), // Use original content for word count
			IsTranslated: false,
		}

		epub.Chapters = append(epub.Chapters, chapter)
	}

	return nil
}

// rewriteMediaURLs rewrites relative URLs in HTML content to absolute URLs for serving media files
func (p *Parser) rewriteMediaURLs(htmlContent string, epubID string, packageDir string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent // Return original content if parsing fails
	}

	// Rewrite CSS links
	doc.Find("link[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && !strings.HasPrefix(href, "http") && !strings.HasPrefix(href, "/") {
			// Convert relative path to absolute URL
			newHref := fmt.Sprintf("/epub_files/%s/%s/%s", epubID, packageDir, href)
			s.SetAttr("href", newHref)
		}
	})

	// Rewrite image sources
	doc.Find("img[src]").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if exists && !strings.HasPrefix(src, "http") && !strings.HasPrefix(src, "/") {
			// Convert relative path to absolute URL
			newSrc := fmt.Sprintf("/epub_files/%s/%s/%s", epubID, packageDir, src)
			s.SetAttr("src", newSrc)
		}
	})

	// Rewrite other media references (audio, video, etc.)
	doc.Find("audio[src], video[src], source[src]").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if exists && !strings.HasPrefix(src, "http") && !strings.HasPrefix(src, "/") {
			newSrc := fmt.Sprintf("/epub_files/%s/%s/%s", epubID, packageDir, src)
			s.SetAttr("src", newSrc)
		}
	})

	// Get the modified HTML
	body := doc.Find("body")
	if body.Length() == 0 {
		// If no body tag, return the entire document
		html, _ := doc.Html()
		return html
	}

	// Return only the body content
	html, _ := body.Html()
	return html
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

// CreateTranslatedCopy creates a copy of the EPUB directory for storing translations
func (p *Parser) CreateTranslatedCopy(epubID string) (string, error) {
	sourceDir := filepath.Join(p.tempDir, epubID)
	translatedDir := filepath.Join(p.tempDir, epubID+"_translated")

	// Check if source directory exists
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return "", fmt.Errorf("source EPUB directory not found: %s", sourceDir)
	}

	// Check if translated directory already exists
	if _, err := os.Stat(translatedDir); err == nil {
		p.logger.Debugf("Translated directory already exists: %s", translatedDir)
		return translatedDir, nil
	}

	p.logger.Debugf("Creating translated copy: %s -> %s", sourceDir, translatedDir)

	// Copy the entire directory
	if err := p.copyDir(sourceDir, translatedDir); err != nil {
		return "", fmt.Errorf("failed to copy EPUB directory: %w", err)
	}

	p.logger.Debugf("Successfully created translated copy: %s", translatedDir)
	return translatedDir, nil
}

// copyDir recursively copies a directory
func (p *Parser) copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := p.copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := p.copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file
func (p *Parser) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Copy file permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}

// SaveTranslatedChapter saves a translated chapter to the translated EPUB directory
func (p *Parser) SaveTranslatedChapter(epubID, chapterPath, translatedContent string) error {
	translatedDir := filepath.Join(p.tempDir, epubID+"_translated")

	// Ensure translated directory exists
	if _, err := os.Stat(translatedDir); os.IsNotExist(err) {
		if _, err := p.CreateTranslatedCopy(epubID); err != nil {
			return fmt.Errorf("failed to create translated copy: %w", err)
		}
	}

	// Calculate the translated file path
	originalDir := filepath.Join(p.tempDir, epubID)
	relPath, err := filepath.Rel(originalDir, chapterPath)
	if err != nil {
		return fmt.Errorf("failed to calculate relative path: %w", err)
	}

	translatedFilePath := filepath.Join(translatedDir, relPath)

	// Ensure the directory for the file exists
	if err := os.MkdirAll(filepath.Dir(translatedFilePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for translated file: %w", err)
	}

	// Read the original file to get the full HTML structure
	originalContent, err := os.ReadFile(chapterPath)
	if err != nil {
		return fmt.Errorf("failed to read original file: %w", err)
	}

	// Replace the body content with translated content
	translatedFullContent, err := p.replaceBodyContent(string(originalContent), translatedContent)
	if err != nil {
		return fmt.Errorf("failed to replace body content: %w", err)
	}

	// Write the translated content
	if err := os.WriteFile(translatedFilePath, []byte(translatedFullContent), 0644); err != nil {
		return fmt.Errorf("failed to write translated file: %w", err)
	}

	p.logger.Debugf("Successfully saved translated chapter: %s", translatedFilePath)
	return nil
}

// replaceBodyContent replaces the body content in an HTML file while preserving the structure
func (p *Parser) replaceBodyContent(originalHTML, newBodyContent string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(originalHTML))
	if err != nil {
		return "", err
	}

	// Find the body element and replace its content
	body := doc.Find("body")
	if body.Length() > 0 {
		body.SetHtml(newBodyContent)
	} else {
		// If no body tag, wrap the new content in the original structure
		return newBodyContent, nil
	}

	// Return the complete HTML document
	html, err := doc.Html()
	if err != nil {
		return "", err
	}

	return html, nil
}
