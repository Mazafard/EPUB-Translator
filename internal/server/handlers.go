package server

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"epub-translator/internal/epub"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleHome(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"Title":              "EPUB Translator",
		"SupportedLanguages": s.config.Translation.SupportedLangs,
	})
}

func (s *Server) handleUpload(c *gin.Context) {
	file, err := c.FormFile("epub")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	if filepath.Ext(file.Filename) != ".epub" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File must be an EPUB"})
		return
	}

	if file.Size > 50*1024*1024 { // 50MB limit
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large (max 50MB)"})
		return
	}

	tempPath := filepath.Join(s.config.App.TempDir, file.Filename)
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		s.logger.Errorf("Failed to save uploaded file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	epubContent, err := s.epubParser.Extract(tempPath)
	if err != nil {
		s.logger.Errorf("Failed to extract EPUB: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process EPUB file"})
		return
	}

	if err := s.epubParser.Validate(epubContent); err != nil {
		s.logger.Errorf("EPUB validation failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid EPUB file"})
		return
	}

	sourceLang, err := s.translationSvc.DetectLanguage(epubContent)
	if err != nil {
		s.logger.Warnf("Language detection failed: %v", err)
		sourceLang = "unknown"
	}

	epubContent.Package.Metadata.Language = sourceLang
	s.epubStorage[epubContent.ID] = epubContent

	s.logger.Infof("Successfully uploaded and processed EPUB: %s (ID: %s)", file.Filename, epubContent.ID)

	c.JSON(http.StatusOK, gin.H{
		"id":           epubContent.ID,
		"title":        epubContent.Package.Metadata.Title,
		"language":     sourceLang,
		"chapters":     len(epubContent.Chapters),
		"redirect_url": fmt.Sprintf("/preview/%s", epubContent.ID),
	})
}

func (s *Server) handlePreview(c *gin.Context) {
	id := c.Param("id")
	epubContent, exists := s.epubStorage[id]
	if !exists {
		// Try to load from disk if not in memory
		loadedEpub, err := s.epubParser.LoadFromDirectory(id)
		if err != nil {
			s.logger.Debugf("Failed to load EPUB from directory %s: %v", id, err)
			c.HTML(http.StatusNotFound, "error.html", gin.H{
				"Error": "EPUB not found",
			})
			return
		}

		// Store in memory for future requests
		s.epubStorage[id] = loadedEpub
		epubContent = loadedEpub
		s.logger.Debugf("Successfully loaded EPUB %s from disk", id)
	}

	var chapterSummaries []gin.H
	for _, chapter := range epubContent.Chapters {
		chapterSummaries = append(chapterSummaries, gin.H{
			"ID":           chapter.ID,
			"Title":        chapter.Title,
			"WordCount":    chapter.WordCount,
			"IsTranslated": chapter.IsTranslated,
		})
	}

	c.HTML(http.StatusOK, "preview.html", gin.H{
		"Title":              epubContent.Package.Metadata.Title,
		"ID":                 epubContent.ID,
		"Language":           epubContent.Package.Metadata.Language,
		"Chapters":           chapterSummaries,
		"TotalChapters":      len(epubContent.Chapters),
		"SupportedLanguages": s.config.Translation.SupportedLangs,
	})
}

func (s *Server) handleTranslate(c *gin.Context) {
	var request struct {
		ID         string `json:"id" binding:"required"`
		TargetLang string `json:"target_lang" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	epubContent, exists := s.epubStorage[request.ID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "EPUB not found"})
		return
	}

	sourceLang := epubContent.Package.Metadata.Language
	if sourceLang == "" || sourceLang == "unknown" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Source language not detected. Please try uploading the file again."})
		return
	}

	if sourceLang == request.TargetLang {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Source and target languages are the same"})
		return
	}

	if err := s.translationSvc.StartTranslation(epubContent, sourceLang, request.TargetLang); err != nil {
		s.logger.Errorf("Failed to start translation: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start translation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Translation started",
		"status_url":      fmt.Sprintf("/status/%s", request.ID),
		"source_language": sourceLang,
		"target_language": request.TargetLang,
	})
}

func (s *Server) handleStatus(c *gin.Context) {
	id := c.Param("id")

	progress := s.translationSvc.GetProgress(id)
	if progress == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Translation not found"})
		return
	}

	response := gin.H{
		"id":                 progress.ID,
		"status":             progress.Status,
		"source_language":    progress.SourceLanguage,
		"target_language":    progress.TargetLanguage,
		"total_chapters":     progress.TotalChapters,
		"completed_chapters": progress.CompletedChapters,
		"current_chapter":    progress.CurrentChapter,
		"started_at":         progress.StartedAt,
	}

	if progress.Status == "completed" {
		response["completed_at"] = progress.CompletedAt
		response["download_url"] = fmt.Sprintf("/download/%s", id)
	}

	if progress.Status == "failed" {
		response["error_message"] = progress.ErrorMessage
	}

	if progress.TotalChapters > 0 {
		response["progress_percentage"] = (float64(progress.CompletedChapters) / float64(progress.TotalChapters)) * 100
	}

	c.JSON(http.StatusOK, response)
}

func (s *Server) handleDownload(c *gin.Context) {
	id := c.Param("id")

	epubContent, exists := s.epubStorage[id]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "EPUB not found"})
		return
	}

	progress := s.translationSvc.GetProgress(id)
	if progress == nil || progress.Status != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Translation not completed"})
		return
	}

	outputPath, err := s.epubBuilder.CreateTranslated(epubContent, progress.TargetLanguage, s.config.App.OutputDir)
	if err != nil {
		s.logger.Errorf("Failed to create translated EPUB: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create translated file"})
		return
	}

	filename := fmt.Sprintf("%s_%s.epub",
		sanitizeFilename(epubContent.Package.Metadata.Title),
		progress.TargetLanguage)

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/epub+zip")

	c.File(outputPath)
}

func (s *Server) handleDownloadTranslated(c *gin.Context) {
	id := c.Param("id")
	targetLang := c.Param("lang")

	if targetLang == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Target language is required"})
		return
	}

	// Load EPUB content
	epubContent, exists := s.epubStorage[id]
	if !exists {
		// Try to load from disk if not in memory
		loadedEpub, err := s.epubParser.LoadFromDirectory(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "EPUB not found"})
			return
		}
		s.epubStorage[id] = loadedEpub
		epubContent = loadedEpub
	}

	// Build from the translated directory instead of building a new one
	translatedDir := fmt.Sprintf("%s_translated_%s", epubContent.TempDir, targetLang)

	// Check if translated directory exists
	if _, err := os.Stat(translatedDir); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "No translated version found for this language"})
		return
	}

	// Create output filename
	title := epubContent.Package.Metadata.Title
	if title == "" {
		title = fmt.Sprintf("epub_%s", id)
	}
	filename := fmt.Sprintf("%s_%s.epub", sanitizeFilename(title), targetLang)
	outputPath := filepath.Join(s.config.App.OutputDir, filename)

	// Create the EPUB from the translated directory
	if err := s.createEPUBFromDirectory(translatedDir, outputPath); err != nil {
		s.logger.Errorf("Failed to create translated EPUB from directory: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create translated EPUB file"})
		return
	}

	// Set download headers
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/epub+zip")

	// Send the file
	c.File(outputPath)

	// Clean up the created file after sending (optional)
	go func() {
		time.Sleep(30 * time.Second) // Give some time for download to complete
		_ = os.Remove(outputPath)
	}()
}

func (s *Server) handleDownloadProcessed(c *gin.Context) {
	id := c.Param("id")
	targetLang := c.Param("lang")

	if targetLang == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Target language is required for processed download"})
		return
	}

	// Load EPUB content
	epubContent, exists := s.epubStorage[id]
	if !exists {
		loadedEpub, err := s.epubParser.LoadFromDirectory(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "EPUB not found"})
			return
		}
		s.epubStorage[id] = loadedEpub
		epubContent = loadedEpub
	}

	// Use the language-specific translated directory if it exists, otherwise use the original.
	// This allows downloading a partially translated book.
	translatedDir := filepath.Join(s.config.App.TempDir, fmt.Sprintf("%s_translated_%s", id, targetLang))
	sourceDir := filepath.Join(s.config.App.TempDir, id)

	var packageDir string
	if _, err := os.Stat(translatedDir); err == nil {
		s.logger.Infof("Found translated directory for language %s, packaging it.", targetLang)
		packageDir = translatedDir
	} else {
		s.logger.Infof("No translated directory found for language %s, packaging original.", targetLang)
		packageDir = sourceDir
	}

	// Create output filename
	title := epubContent.Package.Metadata.Title
	if title == "" {
		title = fmt.Sprintf("epub_%s", id)
	}
	filename := fmt.Sprintf("%s_processed_%s.epub", sanitizeFilename(title), targetLang)
	outputPath := filepath.Join(s.config.App.OutputDir, filename)

	// Create the EPUB from the chosen directory
	if err := s.createEPUBFromDirectory(packageDir, outputPath); err != nil {
		s.logger.Errorf("Failed to create processed EPUB from directory: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create processed EPUB file"})
		return
	}

	// Set download headers
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/epub+zip")

	// Send the file
	c.File(outputPath)

	// Clean up the created file after sending
	go func() {
		time.Sleep(30 * time.Second)
		_ = os.Remove(outputPath)
	}()
}

func (s *Server) createEPUBFromDirectory(sourceDir, outputPath string) error {
	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create the zip file
	zipFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() { _ = zipFile.Close() }()

	zipWriter := zip.NewWriter(zipFile)
	defer func() { _ = zipWriter.Close() }()

	// Write mimetype file first (uncompressed)
	writer, err := zipWriter.CreateHeader(&zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store, // No compression for mimetype
	})
	if err != nil {
		return fmt.Errorf("failed to create mimetype entry: %w", err)
	}
	if _, err := writer.Write([]byte("application/epub+zip")); err != nil {
		return fmt.Errorf("failed to write mimetype: %w", err)
	}

	// Walk through the source directory and add all files
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Skip mimetype file as we already added it
		if relPath == "mimetype" {
			return nil
		}

		// Open source file
		sourceFile, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open source file %s: %w", path, err)
		}
		defer func() { _ = sourceFile.Close() }()

		// Create entry in zip
		zipEntry, err := zipWriter.Create(relPath)
		if err != nil {
			return fmt.Errorf("failed to create zip entry for %s: %w", relPath, err)
		}

		// Copy file content
		if _, err := io.Copy(zipEntry, sourceFile); err != nil {
			return fmt.Errorf("failed to copy file content for %s: %w", relPath, err)
		}

		return nil
	})
}

func (s *Server) handleGetChapters(c *gin.Context) {
	id := c.Param("id")

	epubContent, exists := s.epubStorage[id]
	if !exists {
		// Try to load from disk if not in memory
		loadedEpub, err := s.epubParser.LoadFromDirectory(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "EPUB not found"})
			return
		}
		s.epubStorage[id] = loadedEpub
		epubContent = loadedEpub
	}

	page := 0
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p >= 0 {
			page = p
		}
	}

	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	start := page * limit
	end := start + limit
	if end > len(epubContent.Chapters) {
		end = len(epubContent.Chapters)
	}
	if start >= len(epubContent.Chapters) {
		start = len(epubContent.Chapters)
		end = start
	}

	chapters := epubContent.Chapters[start:end]

	var chapterData []gin.H
	for _, chapter := range chapters {
		chapterData = append(chapterData, gin.H{
			"id":                 chapter.ID,
			"title":              chapter.Title,
			"content":            truncateText(chapter.Content, 500),
			"translated_content": truncateText(chapter.TranslatedContent, 500),
			"word_count":         chapter.WordCount,
			"is_translated":      chapter.IsTranslated,
			"order":              chapter.Order,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"chapters":     chapterData,
		"total":        len(epubContent.Chapters),
		"page":         page,
		"limit":        limit,
		"has_next":     end < len(epubContent.Chapters),
		"has_previous": page > 0,
	})
}

func (s *Server) handleDeleteEpub(c *gin.Context) {
	id := c.Param("id")

	if _, exists := s.epubStorage[id]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "EPUB not found"})
		return
	}

	delete(s.epubStorage, id)
	s.translationSvc.ClearProgress(id)

	c.JSON(http.StatusOK, gin.H{"message": "EPUB deleted successfully"})
}

func sanitizeFilename(filename string) string {
	// Simple filename sanitization
	result := ""
	for _, r := range filename {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result += string(r)
		} else if r == ' ' {
			result += "_"
		}
	}
	if result == "" {
		result = "translated_book"
	}
	return result
}

func truncateText(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength] + "..."
}

func (s *Server) handleGetChapter(c *gin.Context) {
	epubID := c.Param("epub_id")
	chapterID := c.Param("chapter_id")

	epubContent, exists := s.epubStorage[epubID]
	if !exists {
		// Try to load from disk if not in memory
		loadedEpub, err := s.epubParser.LoadFromDirectory(epubID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "EPUB not found"})
			return
		}
		s.epubStorage[epubID] = loadedEpub
		epubContent = loadedEpub
	}

	// Find the specific chapter
	var targetChapter *epub.Chapter
	for i := range epubContent.Chapters {
		if epubContent.Chapters[i].ID == chapterID {
			targetChapter = &epubContent.Chapters[i]
			break
		}
	}

	if targetChapter == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chapter not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                 targetChapter.ID,
		"title":              targetChapter.Title,
		"content":            targetChapter.Content,
		"translated_content": targetChapter.TranslatedContent,
		"word_count":         targetChapter.WordCount,
		"is_translated":      targetChapter.IsTranslated,
		"order":              targetChapter.Order,
	})
}

func (s *Server) handleTranslatePage(c *gin.Context) {
	var request struct {
		EPUBID     string `json:"epub_id" binding:"required"`
		ChapterID  string `json:"chapter_id" binding:"required"`
		Content    string `json:"content" binding:"required"`
		TargetLang string `json:"target_lang" binding:"required"`
		SourceLang string `json:"source_lang"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	epubContent, exists := s.epubStorage[request.EPUBID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "EPUB not found"})
		return
	}

	sourceLang := request.SourceLang
	if sourceLang == "" {
		sourceLang = epubContent.Package.Metadata.Language
	}

	// Broadcast the start of page translation
	s.wsHub.BroadcastLog("info", fmt.Sprintf("Starting page translation from %s to %s", sourceLang, request.TargetLang), "translation")

	// Perform translation
	go func() {
		translatedText, err := s.translationSvc.TranslateText(request.Content, sourceLang, request.TargetLang)
		if err != nil {
			s.logger.Errorf("Failed to translate page: %v", err)
			s.wsHub.BroadcastLog("error", fmt.Sprintf("Page translation failed: %v", err), "translation")
			return
		}

		// Find the chapter to get its file path
		var targetChapter *epub.Chapter
		for i := range epubContent.Chapters {
			if epubContent.Chapters[i].ID == request.ChapterID {
				targetChapter = &epubContent.Chapters[i]
				break
			}
		}

		if targetChapter != nil {
			// Save the translated content to disk with language-specific styling
			if err := s.epubParser.SaveTranslatedChapter(request.EPUBID, targetChapter.FilePath, translatedText, request.TargetLang); err != nil {
				s.logger.Errorf("Failed to save translated chapter to disk: %v", err)
				s.wsHub.BroadcastLog("error", fmt.Sprintf("Failed to persist translation: %v", err), "translation")
			} else {
				s.logger.Debugf("Successfully saved translated chapter to disk with language %s: %s", request.TargetLang, targetChapter.FilePath)
				s.wsHub.BroadcastLog("info", fmt.Sprintf("Translation saved with %s language support", request.TargetLang), "translation")

				// Update the in-memory chapter data
				targetChapter.TranslatedContent = translatedText
				targetChapter.IsTranslated = true
			}
		} else {
			s.logger.Warnf("Chapter not found for ID: %s", request.ChapterID)
		}

		// Broadcast the result
		pageTranslationMsg := PageTranslationMessage{
			EPUBID:         request.EPUBID,
			ChapterID:      request.ChapterID,
			OriginalText:   request.Content,
			TranslatedText: translatedText,
			SourceLanguage: sourceLang,
			TargetLanguage: request.TargetLang,
		}

		s.wsHub.BroadcastMessage(MessageTypePageTranslation, pageTranslationMsg)
		s.wsHub.BroadcastLog("info", "Page translation completed successfully", "translation")
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "Page translation started",
		"status":  "processing",
	})
}

func (s *Server) handleReader(c *gin.Context) {
	id := c.Param("id")
	epubContent, exists := s.epubStorage[id]
	if !exists {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"Error": "EPUB not found",
		})
		return
	}

	var chapterSummaries []gin.H
	for _, chapter := range epubContent.Chapters {
		chapterSummaries = append(chapterSummaries, gin.H{
			"ID":           chapter.ID,
			"Title":        chapter.Title,
			"WordCount":    chapter.WordCount,
			"IsTranslated": chapter.IsTranslated,
			"Order":        chapter.Order,
		})
	}

	c.HTML(http.StatusOK, "reader.html", gin.H{
		"Title":              epubContent.Package.Metadata.Title,
		"ID":                 epubContent.ID,
		"Language":           epubContent.Package.Metadata.Language,
		"Chapters":           chapterSummaries,
		"TotalChapters":      len(epubContent.Chapters),
		"SupportedLanguages": s.config.Translation.SupportedLangs,
	})
}

// FileInfo represents a file in the temp directory
type FileInfo struct {
	Name              string    `json:"name"`
	Path              string    `json:"path"`
	Size              int64     `json:"size"`
	SizeFormatted     string    `json:"size_formatted"`
	Modified          time.Time `json:"modified"`
	ModifiedFormatted string    `json:"modified_formatted"`
	IsEPUB            bool      `json:"is_epub"`
}

func (s *Server) handlePreviousWork(c *gin.Context) {
	files, err := s.listTempFiles()
	if err != nil {
		s.logger.Errorf("Failed to list temp files: %v", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"Error": "Failed to load previous work",
		})
		return
	}

	c.HTML(http.StatusOK, "previous-work.html", gin.H{
		"Files":      files,
		"TotalFiles": len(files),
	})
}

func (s *Server) listTempFiles() ([]FileInfo, error) {
	tempDir := s.config.App.TempDir

	// Ensure temp directory exists
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		return []FileInfo{}, nil
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read temp directory: %w", err)
	}

	var files []FileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(tempDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			s.logger.Warnf("Failed to get info for file %s: %v", entry.Name(), err)
			continue
		}

		// Skip hidden files and system files
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		fileInfo := FileInfo{
			Name:              entry.Name(),
			Path:              fullPath,
			Size:              info.Size(),
			SizeFormatted:     formatFileSize(info.Size()),
			Modified:          info.ModTime(),
			ModifiedFormatted: formatTime(info.ModTime()),
			IsEPUB:            strings.ToLower(filepath.Ext(entry.Name())) == ".epub",
		}

		files = append(files, fileInfo)
	}

	// Sort files by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].Modified.After(files[j].Modified)
	})

	return files, nil
}

func (s *Server) handleDownloadFile(c *gin.Context) {
	filePath := c.Query("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File path is required"})
		return
	}

	// Security check: ensure the file is within the temp directory
	tempDir := s.config.App.TempDir
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file path"})
		return
	}

	absTempDir, err := filepath.Abs(tempDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server configuration error"})
		return
	}

	if !strings.HasPrefix(absPath, absTempDir) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Serve the file
	filename := filepath.Base(absPath)
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// Set appropriate content type based on file extension
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".epub":
		c.Header("Content-Type", "application/epub+zip")
	default:
		c.Header("Content-Type", "application/octet-stream")
	}

	c.File(absPath)
}

func (s *Server) handleDeleteFile(c *gin.Context) {
	var request struct {
		Path string `json:"path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Security check: ensure the file is within the temp directory
	tempDir := s.config.App.TempDir
	absPath, err := filepath.Abs(request.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file path"})
		return
	}

	absTempDir, err := filepath.Abs(tempDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server configuration error"})
		return
	}

	if !strings.HasPrefix(absPath, absTempDir) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Delete the file
	if err := os.Remove(absPath); err != nil {
		s.logger.Errorf("Failed to delete file %s: %v", absPath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file"})
		return
	}

	s.logger.Infof("Successfully deleted file: %s", absPath)
	c.JSON(http.StatusOK, gin.H{"message": "File deleted successfully"})
}

func (s *Server) handleProcessFile(c *gin.Context) {
	var request struct {
		Path string `json:"path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Security check: ensure the file is within the temp directory
	tempDir := s.config.App.TempDir
	absPath, err := filepath.Abs(request.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file path"})
		return
	}

	absTempDir, err := filepath.Abs(tempDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server configuration error"})
		return
	}

	if !strings.HasPrefix(absPath, absTempDir) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Check if it's an EPUB file
	if strings.ToLower(filepath.Ext(absPath)) != ".epub" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File must be an EPUB"})
		return
	}

	// Extract and process the EPUB
	epubContent, err := s.epubParser.Extract(absPath)
	if err != nil {
		s.logger.Errorf("Failed to extract existing EPUB: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process EPUB file"})
		return
	}

	if err := s.epubParser.Validate(epubContent); err != nil {
		s.logger.Errorf("EPUB validation failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid EPUB file"})
		return
	}

	// Detect language
	sourceLang, err := s.translationSvc.DetectLanguage(epubContent)
	if err != nil {
		s.logger.Warnf("Language detection failed: %v", err)
		sourceLang = "unknown"
	}

	epubContent.Package.Metadata.Language = sourceLang
	s.epubStorage[epubContent.ID] = epubContent

	filename := filepath.Base(absPath)
	s.logger.Infof("Successfully processed existing EPUB: %s (ID: %s)", filename, epubContent.ID)

	c.JSON(http.StatusOK, gin.H{
		"id":           epubContent.ID,
		"title":        epubContent.Package.Metadata.Title,
		"language":     sourceLang,
		"chapters":     len(epubContent.Chapters),
		"redirect_url": fmt.Sprintf("/preview/%s", epubContent.ID),
		"message":      "EPUB processed successfully",
	})
}

// Helper functions
func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "Just now"
	} else if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "Yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	} else {
		return t.Format("Jan 2, 2006")
	}
}
