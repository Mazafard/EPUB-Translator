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

		//// Rewrite media URLs in the content
		rewrittenContent := p.rewriteMediaURLs(content, epub.ID, packageDirRelative)

		// Check for translated content in the _translated folder
		translatedContent := ""
		isTranslated := false

		// Check if there's a translated version of this chapter
		translatedDir := filepath.Join(p.tempDir, epub.ID+"_translated")
		translatedChapterPath := filepath.Join(translatedDir, filepath.Dir(epub.Package.OriginalPath), item.Href)

		if _, err := os.Stat(translatedChapterPath); err == nil {
			// Translated file exists, load it
			if translatedFileContent, err := p.extractChapterContent(translatedChapterPath); err == nil {
				translatedContent = p.rewriteMediaURLs(translatedFileContent, epub.ID, packageDirRelative)
				isTranslated = true
				p.logger.Debugf("Loaded translated content for chapter: %s", translatedChapterPath)
			} else {
				p.logger.Warnf("Failed to load translated content from %s: %v", translatedChapterPath, err)
			}
		}

		chapter := Chapter{
			ID:                fmt.Sprintf("%s_%d", epub.ID, order),
			Title:             p.extractTitle(content), // Use original content for title extraction
			FilePath:          chapterPath,
			RelativePath:      item.Href,
			Content:           rewrittenContent, // Use rewritten content for display
			TranslatedContent: translatedContent,
			Order:             order,
			WordCount:         countWords(content), // Use original content for word count
			IsTranslated:      isTranslated,
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
	//doc.Find("link[href]").Each(func(i int, s *goquery.Selection) {
	//	href, exists := s.Attr("href")
	//	if exists && !strings.HasPrefix(href, "http") && !strings.HasPrefix(href, "/") {
	//		// Convert relative path to absolute URL
	//		newHref := fmt.Sprintf("/epub_files/%s/%s/%s", epubID, packageDir, href)
	//		s.SetAttr("href", newHref)
	//	}
	//})

	// Rewrite image sources
	//doc.Find("img[src]").Each(func(i int, s *goquery.Selection) {
	//	src, exists := s.Attr("src")
	//	if exists && !strings.HasPrefix(src, "http") && !strings.HasPrefix(src, "/") {
	//		// Convert relative path to absolute URL
	//		newSrc := fmt.Sprintf("/epub_files/%s/%s/%s", epubID, packageDir, src)
	//		s.SetAttr("src", newSrc)
	//	}
	//})

	// Rewrite other media references (audio, video, etc.)
	//doc.Find("audio[src], video[src], source[src]").Each(func(i int, s *goquery.Selection) {
	//	src, exists := s.Attr("src")
	//	if exists && !strings.HasPrefix(src, "http") && !strings.HasPrefix(src, "/") {
	//		newSrc := fmt.Sprintf("/epub_files/%s/%s/%s", epubID, packageDir, src)
	//		s.SetAttr("src", newSrc)
	//	}
	//})

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

// SaveTranslatedChapter saves a translated chapter to the language-specific translated EPUB directory
func (p *Parser) SaveTranslatedChapter(epubID, chapterPath, translatedContent, targetLang string) error {
	translatedDir := filepath.Join(p.tempDir, fmt.Sprintf("%s_translated_%s", epubID, targetLang))

	// Ensure translated directory exists
	if _, err := os.Stat(translatedDir); os.IsNotExist(err) {
		if _, err := p.CreateTranslatedCopyWithLanguage(epubID, targetLang); err != nil {
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

	// Replace the body content with translated content and apply language-specific styling
	translatedFullContent, err := p.replaceBodyContentWithLanguageSupport(string(originalContent), translatedContent, targetLang)
	if err != nil {
		return fmt.Errorf("failed to replace body content: %w", err)
	}

	// Write the translated content
	if err := os.WriteFile(translatedFilePath, []byte(translatedFullContent), 0644); err != nil {
		return fmt.Errorf("failed to write translated file: %w", err)
	}

	p.logger.Debugf("Successfully saved translated chapter for language %s: %s", targetLang, translatedFilePath)
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

// CreateTranslatedCopyWithLanguage creates a language-specific copy of the EPUB directory for storing translations
func (p *Parser) CreateTranslatedCopyWithLanguage(epubID, targetLang string) (string, error) {
	sourceDir := filepath.Join(p.tempDir, epubID)
	translatedDir := filepath.Join(p.tempDir, fmt.Sprintf("%s_translated_%s", epubID, targetLang))

	// Check if source directory exists
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return "", fmt.Errorf("source EPUB directory not found: %s", sourceDir)
	}

	// Check if translated directory already exists
	if _, err := os.Stat(translatedDir); err == nil {
		p.logger.Debugf("Translated directory for language %s already exists: %s", targetLang, translatedDir)
		return translatedDir, nil
	}

	p.logger.Debugf("Creating translated copy for language %s: %s -> %s", targetLang, sourceDir, translatedDir)

	// Copy the entire directory
	if err := p.copyDir(sourceDir, translatedDir); err != nil {
		return "", fmt.Errorf("failed to copy EPUB directory: %w", err)
	}

	// Inject language-specific CSS into existing CSS files
	if err := p.injectLanguageCSS(translatedDir, targetLang); err != nil {
		p.logger.Warnf("Failed to inject language-specific CSS: %v", err)
	}

	p.logger.Debugf("Successfully created translated copy for language %s: %s", targetLang, translatedDir)
	return translatedDir, nil
}

// replaceBodyContentWithLanguageSupport replaces body content with language-specific styling
func (p *Parser) replaceBodyContentWithLanguageSupport(originalHTML, newBodyContent, targetLang string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(originalHTML))
	if err != nil {
		return "", err
	}

	// Check if target language is RTL
	isRTL := p.isRTLLanguage(targetLang)

	// Find or create the head element to add language-specific CSS
	head := doc.Find("head")
	if head.Length() == 0 {
		// If no head tag, create one
		doc.Find("html").PrependHtml("<head></head>")
		head = doc.Find("head")
	}

	// Add inline CSS for immediate RTL support if needed
	if isRTL {

		rtlCSS := fmt.Sprintf(`<style type="text/css">
body {
    direction: rtl;
    text-align: right;
    line-height: 1.8;
}

p, div, span, h1, h2, h3, h4, h5, h6, li, td, th {
    direction: rtl;
    text-align: right;
}

table {
    direction: rtl;
}

th, td {
    text-align: right;
}

.rtl-content {
    direction: rtl;
    text-align: right;
}
</style>`)
		head.AppendHtml(rtlCSS)
	}

	// Set html element attributes
	html := doc.Find("html")
	if html.Length() > 0 {
		html.SetAttr("lang", targetLang)
		if isRTL {
			html.SetAttr("dir", "rtl")
		} else {
			html.SetAttr("dir", "ltr")
		}
	}

	// Find the body element and replace its content
	body := doc.Find("body")
	if body.Length() > 0 {
		if isRTL {
			// Add RTL attributes to body
			body.SetAttr("dir", "rtl")
			body.SetAttr("lang", targetLang)
			body.AddClass("rtl-content")
		} else {
			// Ensure LTR for non-RTL languages
			body.SetAttr("dir", "ltr")
			body.SetAttr("lang", targetLang)
			body.RemoveClass("rtl-content")
		}
		body.SetHtml(newBodyContent)
	} else {
		// If no body tag, wrap the new content with language attributes
		if isRTL {
			return fmt.Sprintf(`<div dir="rtl" lang="%s" class="rtl-content">%s</div>`, targetLang, newBodyContent), nil
		}
		return fmt.Sprintf(`<div dir="ltr" lang="%s">%s</div>`, targetLang, newBodyContent), nil
	}

	// Return the complete HTML document
	htmlContent, err := doc.Html()
	if err != nil {
		return "", err
	}

	return htmlContent, nil
}

// isRTLLanguage checks if a language code represents a right-to-left language
func (p *Parser) isRTLLanguage(languageCode string) bool {
	rtlLanguages := map[string]bool{
		"ar": true, // Arabic
		"fa": true, // Persian/Farsi
		"he": true, // Hebrew
		"ur": true, // Urdu
		"yi": true, // Yiddish
		"ji": true, // Yiddish (alternative code)
		"iw": true, // Hebrew (alternative code)
		"ku": true, // Kurdish
		"ps": true, // Pashto
		"sd": true, // Sindhi
	}
	return rtlLanguages[strings.ToLower(languageCode)]
}

// injectLanguageCSS finds existing CSS files in the EPUB and injects language-specific styles
func (p *Parser) injectLanguageCSS(translatedDir, targetLang string) error {
	// Find all CSS files in the EPUB directory
	cssFiles, err := p.findCSSFiles(translatedDir)
	if err != nil {
		return fmt.Errorf("failed to find CSS files: %w", err)
	}

	if len(cssFiles) == 0 {
		p.logger.Warnf("No CSS files found in EPUB, creating default stylesheet")
		return p.createDefaultStylesheet(translatedDir, targetLang)
	}

	// Generate language-specific CSS content
	languageCSS := p.generateLanguageCSS(targetLang)

	// Inject language CSS into existing CSS files
	for _, cssFile := range cssFiles {
		if err := p.injectCSSIntoFile(cssFile, languageCSS, targetLang); err != nil {
			p.logger.Warnf("Failed to inject CSS into %s: %v", cssFile, err)
			continue
		}
		p.logger.Debugf("Successfully injected %s language styles into: %s", targetLang, cssFile)
	}

	return nil
}

// findCSSFiles recursively finds all CSS files in the EPUB directory
func (p *Parser) findCSSFiles(dir string) ([]string, error) {
	var cssFiles []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-CSS files
		if info.IsDir() {
			return nil
		}

		// Check for CSS files
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".css" {
			cssFiles = append(cssFiles, path)
		}

		return nil
	})

	return cssFiles, err
}

// generateLanguageCSS creates the CSS content for a specific language
func (p *Parser) generateLanguageCSS(targetLang string) string {
	if p.isRTLLanguage(targetLang) {
		// RTL CSS content
		cssContent := `
/* RTL Language Support - Auto-injected for ` + targetLang + ` */
body {
    direction: rtl !important;
    text-align: right !important;
}

p, div, span, h1, h2, h3, h4, h5, h6, li, td, th, blockquote {
    direction: rtl !important;
    text-align: right !important;
}

/* Table support */
table {
    direction: rtl !important;
}

th, td {
    text-align: right !important;
}

/* List support */
ul, ol {
    direction: rtl !important;
    text-align: right !important;
}

li {
    text-align: right !important;
}

/* Quote and emphasis support */
blockquote {
    border-right: 4px solid #ccc !important;
    border-left: none !important;
    padding-right: 1em !important;
    padding-left: 0 !important;
    margin-right: 0 !important;
    margin-left: 1em !important;
}

/* Text alignment utilities */
.text-left { text-align: right !important; }
.text-right { text-align: left !important; }
.text-center { text-align: center !important; }

/* Float adjustments */
.float-left { float: right !important; }
.float-right { float: left !important; }

/* Margin and padding adjustments */
.margin-left { margin-right: inherit !important; margin-left: 0 !important; }
.margin-right { margin-left: inherit !important; margin-right: 0 !important; }
.padding-left { padding-right: inherit !important; padding-left: 0 !important; }
.padding-right { padding-left: inherit !important; padding-right: 0 !important; }
`

		// Add language-specific font families
		switch targetLang {
		case "fa":
			cssContent += `
/* Persian/Farsi specific fonts */
body, p, div, span, h1, h2, h3, h4, h5, h6 {
    font-family: "Vazirmatn", "Noto Sans", "Iranian Sans", "B Nazanin", "Tahoma", Arial, sans-serif !important;
}
`
		case "ar":
			cssContent += `
/* Arabic specific fonts */
body, p, div, span, h1, h2, h3, h4, h5, h6 {
    font-family: "Noto Sans Arabic", "Arabic UI Text", "Tahoma", Arial, sans-serif !important;
}
`
		case "he":
			cssContent += `
/* Hebrew specific fonts */
body, p, div, span, h1, h2, h3, h4, h5, h6 {
    font-family: "Noto Sans Hebrew", "Hebrew UI Text", "David", "Tahoma", Arial, sans-serif !important;
}
`
		}

		return cssContent
	}

	// LTR CSS content (minimal, mainly for consistency)
	return `
/* LTR Language Support - Auto-injected for ` + targetLang + ` */
body {
    direction: ltr !important;
    text-align: left !important;
    unicode-bidi: embed !important;
}

p, div, span, h1, h2, h3, h4, h5, h6, li, td, th, blockquote {
    direction: ltr !important;
    text-align: left !important;
    unicode-bidi: embed !important;
}

table {
    direction: ltr !important;
}

th, td {
    text-align: left !important;
}

ul, ol {
    direction: ltr !important;
    text-align: left !important;
}

li {
    text-align: left !important;
}

blockquote {
    border-left: 4px solid #ccc !important;
    border-right: none !important;
    padding-left: 1em !important;
    padding-right: 0 !important;
    margin-left: 0 !important;
    margin-right: 1em !important;
}
`
}

// injectCSSIntoFile injects CSS content into an existing CSS file
func (p *Parser) injectCSSIntoFile(cssFilePath, languageCSS, targetLang string) error {
	// Read existing CSS content
	existingContent, err := os.ReadFile(cssFilePath)
	if err != nil {
		return fmt.Errorf("failed to read CSS file: %w", err)
	}

	existingCSS := string(existingContent)

	// Check if language-specific CSS is already injected
	marker := fmt.Sprintf("/* RTL Language Support - Auto-injected for %s */", targetLang)
	if strings.Contains(existingCSS, marker) {
		p.logger.Debugf("Language CSS already exists in %s, skipping injection", cssFilePath)
		return nil
	}

	// Remove any previous language injections
	existingCSS = p.removePreviousLanguageInjections(existingCSS)

	// Append the new language CSS
	newContent := existingCSS + "\n" + languageCSS

	// Write the updated CSS file
	if err := os.WriteFile(cssFilePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write updated CSS file: %w", err)
	}

	return nil
}

// removePreviousLanguageInjections removes any previously injected language CSS
func (p *Parser) removePreviousLanguageInjections(cssContent string) string {
	// Pattern to match previously injected language CSS blocks
	startMarker := "/* RTL Language Support - Auto-injected for"
	endMarker := "/* End RTL Language Support */"

	for {
		startIdx := strings.Index(cssContent, startMarker)
		if startIdx == -1 {
			break
		}

		// Find the end of this injection block or the start of the next one
		remaining := cssContent[startIdx:]
		endIdx := strings.Index(remaining, endMarker)

		var nextStartIdx int
		if nextStart := strings.Index(remaining[1:], startMarker); nextStart != -1 {
			nextStartIdx = nextStart + 1
		} else {
			nextStartIdx = -1
		}

		var cutEnd int
		if endIdx != -1 && (nextStartIdx == -1 || endIdx < nextStartIdx) {
			// Found proper end marker
			cutEnd = startIdx + endIdx + len(endMarker)
		} else if nextStartIdx != -1 {
			// No end marker, but found next start marker
			cutEnd = startIdx + nextStartIdx
		} else {
			// No end marker and no next start, remove to end of file
			cutEnd = len(cssContent)
		}

		// Remove the injection block
		cssContent = cssContent[:startIdx] + cssContent[cutEnd:]
	}

	return cssContent
}

// createDefaultStylesheet creates a default CSS file if none exist
func (p *Parser) createDefaultStylesheet(translatedDir, targetLang string) error {
	// Create a styles directory
	stylesDir := filepath.Join(translatedDir, "styles")
	if err := os.MkdirAll(stylesDir, 0755); err != nil {
		return fmt.Errorf("failed to create styles directory: %w", err)
	}

	// Create default CSS file
	cssFilePath := filepath.Join(stylesDir, "default.css")
	languageCSS := p.generateLanguageCSS(targetLang)

	defaultCSS := `/* Default EPUB Stylesheet */
body {
    font-family: serif;
    line-height: 1.6;
    margin: 0;
    padding: 1em;
}

h1, h2, h3, h4, h5, h6 {
    font-weight: bold;
    margin-top: 1em;
    margin-bottom: 0.5em;
}

p {
    margin-bottom: 1em;
}
` + languageCSS

	if err := os.WriteFile(cssFilePath, []byte(defaultCSS), 0644); err != nil {
		return fmt.Errorf("failed to write default CSS file: %w", err)
	}

	p.logger.Debugf("Created default stylesheet with %s language support: %s", targetLang, cssFilePath)
	return nil
}
