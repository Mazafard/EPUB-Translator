package translation

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"epub-translator/internal/epub"

	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
)

// WebSocketBroadcaster interface for broadcasting messages
type WebSocketBroadcaster interface {
	BroadcastMessage(msgType interface{}, data interface{})
	BroadcastLog(level, message, module string)
}

type Service struct {
	openai     *OpenAIClient
	logger     *logrus.Logger
	batchSize  int
	progress   map[string]*epub.TranslationProgress
	progressMu sync.RWMutex
	wsHub      WebSocketBroadcaster
}

func NewService(openai *OpenAIClient, logger *logrus.Logger, batchSize int, wsHub WebSocketBroadcaster) *Service {
	return &Service{
		openai:    openai,
		logger:    logger,
		batchSize: batchSize,
		progress:  make(map[string]*epub.TranslationProgress),
		wsHub:     wsHub,
	}
}

func (s *Service) DetectLanguage(epubContent *epub.EPUB) (string, error) {
	if len(epubContent.Chapters) == 0 {
		return "", fmt.Errorf("no chapters found for language detection")
	}

	var textSamples []string
	maxSamples := 3

	for i, chapter := range epubContent.Chapters {
		if i >= maxSamples {
			break
		}

		plainText := s.extractPlainText(chapter.Content)
		if len(plainText) > 500 {
			plainText = plainText[:500]
		}

		if len(strings.TrimSpace(plainText)) > 50 {
			textSamples = append(textSamples, plainText)
		}
	}

	if len(textSamples) == 0 {
		return "", fmt.Errorf("no suitable text samples found for language detection")
	}

	combinedText := strings.Join(textSamples, "\n\n")
	
	detectedLang, err := s.openai.DetectLanguage(combinedText)
	if err != nil {
		return "", fmt.Errorf("failed to detect language: %w", err)
	}

	s.logger.Infof("Detected source language: %s", detectedLang)
	return detectedLang, nil
}

func (s *Service) StartTranslation(epubContent *epub.EPUB, sourceLang, targetLang string) error {
	progressID := epubContent.ID

	progress := &epub.TranslationProgress{
		ID:               progressID,
		SourceLanguage:   sourceLang,
		TargetLanguage:   targetLang,
		TotalChapters:    len(epubContent.Chapters),
		CompletedChapters: 0,
		Status:           "in_progress",
		StartedAt:        time.Now(),
	}

	s.setProgress(progressID, progress)

	go func() {
		if err := s.translateChapters(epubContent, sourceLang, targetLang, progressID); err != nil {
			s.logger.Errorf("Translation failed: %v", err)
			progress.Status = "failed"
			progress.ErrorMessage = err.Error()
			progress.CompletedAt = time.Now()
			s.setProgress(progressID, progress)
		} else {
			s.logger.Infof("Translation completed successfully")
			progress.Status = "completed"
			progress.CompletedAt = time.Now()
			s.setProgress(progressID, progress)
		}
	}()

	return nil
}

func (s *Service) translateChapters(epubContent *epub.EPUB, sourceLang, targetLang, progressID string) error {
	for i := range epubContent.Chapters {
		chapter := &epubContent.Chapters[i]
		
		progress := s.getProgress(progressID)
		if progress != nil {
			progress.CurrentChapter = chapter.Title
			s.setProgress(progressID, progress)
		}

		s.logger.Debugf("Translating chapter %d/%d: %s", i+1, len(epubContent.Chapters), chapter.Title)

		translatedContent, err := s.translateChapterContent(chapter.Content, sourceLang, targetLang)
		if err != nil {
			return fmt.Errorf("failed to translate chapter %s: %w", chapter.Title, err)
		}

		chapter.TranslatedContent = translatedContent
		chapter.IsTranslated = true

		if progress != nil {
			progress.CompletedChapters++
			s.setProgress(progressID, progress)
		}

		s.logger.Debugf("Completed chapter %d/%d", i+1, len(epubContent.Chapters))
	}

	return nil
}

func (s *Service) translateChapterContent(htmlContent, sourceLang, targetLang string) (string, error) {
	if strings.TrimSpace(htmlContent) == "" {
		return htmlContent, nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	var translationErr error

	doc.Find("p, h1, h2, h3, h4, h5, h6, div, span, li, td, th").Each(func(i int, selection *goquery.Selection) {
		if translationErr != nil {
			return
		}

		text := strings.TrimSpace(selection.Text())
		if text == "" {
			return
		}

		translatedText, err := s.openai.TranslateText(text, sourceLang, targetLang)
		if err != nil {
			translationErr = fmt.Errorf("failed to translate text segment: %w", err)
			return
		}

		selection.SetText(translatedText)
	})

	if translationErr != nil {
		return "", translationErr
	}

	result, err := doc.Find("body").Html()
	if err != nil {
		html, htmlErr := doc.Html()
		if htmlErr != nil {
			return "", fmt.Errorf("failed to extract HTML: %w", htmlErr)
		}
		return html, nil
	}

	return result, nil
}

func (s *Service) extractPlainText(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}

	return doc.Text()
}

func (s *Service) GetProgress(progressID string) *epub.TranslationProgress {
	return s.getProgress(progressID)
}

func (s *Service) getProgress(progressID string) *epub.TranslationProgress {
	s.progressMu.RLock()
	defer s.progressMu.RUnlock()
	
	if progress, exists := s.progress[progressID]; exists {
		progressCopy := *progress
		return &progressCopy
	}
	
	return nil
}

func (s *Service) setProgress(progressID string, progress *epub.TranslationProgress) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	
	s.progress[progressID] = progress
	
	// Broadcast progress update via WebSocket if hub is available
	if s.wsHub != nil {
		progressPercent := float64(0)
		if progress.TotalChapters > 0 {
			progressPercent = (float64(progress.CompletedChapters) / float64(progress.TotalChapters)) * 100
		}
		
		progressMsg := map[string]interface{}{
			"epub_id":            progress.ID,
			"total_chapters":     progress.TotalChapters,
			"completed_chapters": progress.CompletedChapters,
			"current_chapter":    progress.CurrentChapter,
			"progress_percent":   progressPercent,
			"status":             progress.Status,
		}
		
		s.wsHub.BroadcastMessage("translation_progress", progressMsg)
		
		// Broadcast status change logs
		switch progress.Status {
		case "in_progress":
			if progress.CurrentChapter != "" {
				s.wsHub.BroadcastLog("info", fmt.Sprintf("Translating chapter: %s (%d/%d)", 
					progress.CurrentChapter, progress.CompletedChapters+1, progress.TotalChapters), "translation")
			}
		case "completed":
			s.wsHub.BroadcastLog("info", "Full translation completed successfully!", "translation")
			s.wsHub.BroadcastMessage("translation_complete", progressMsg)
		case "failed":
			s.wsHub.BroadcastLog("error", fmt.Sprintf("Translation failed: %s", progress.ErrorMessage), "translation")
			s.wsHub.BroadcastMessage("translation_error", map[string]interface{}{
				"epub_id": progress.ID,
				"error":   progress.ErrorMessage,
			})
		}
	}
}

func (s *Service) ClearProgress(progressID string) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	
	delete(s.progress, progressID)
}

func (s *Service) TranslateText(text, sourceLang, targetLang string) (string, error) {
	return s.openai.TranslateText(text, sourceLang, targetLang)
}

func (s *Service) IsRTLLanguage(lang string) bool {
	rtlLanguages := map[string]bool{
		"ar": true, // Arabic
		"he": true, // Hebrew
		"fa": true, // Persian/Farsi
		"ur": true, // Urdu
		"yi": true, // Yiddish
		"ku": true, // Kurdish (some dialects)
		"sd": true, // Sindhi
		"ug": true, // Uyghur
	}

	return rtlLanguages[lang]
}