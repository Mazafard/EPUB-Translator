package epub

import (
	"encoding/xml"
	"time"
)

type EPUB struct {
	ID          string    `json:"id"`
	FilePath    string    `json:"file_path"`
	TempDir     string    `json:"temp_dir"`
	Container   Container `json:"container"`
	Package     Package   `json:"package"`
	Chapters    []Chapter `json:"chapters"`
	CreatedAt   time.Time `json:"created_at"`
	ProcessedAt time.Time `json:"processed_at,omitempty"`
}

type Container struct {
	XMLName   xml.Name `xml:"container"`
	Version   string   `xml:"version,attr"`
	Rootfiles []struct {
		FullPath  string `xml:"full-path,attr"`
		MediaType string `xml:"media-type,attr"`
	} `xml:"rootfiles>rootfile"`
}

type Package struct {
	XMLName      xml.Name `xml:"package"`
	Version      string   `xml:"version,attr"`
	UniqueID     string   `xml:"unique-identifier,attr"`
	Metadata     Metadata `xml:"metadata"`
	Manifest     Manifest `xml:"manifest"`
	Spine        Spine    `xml:"spine"`
	Guide        Guide    `xml:"guide"`
	OriginalPath string   `json:"original_path"`
}

type Metadata struct {
	XMLName     xml.Name `xml:"metadata"`
	Title       string   `xml:"title"`
	Language    string   `xml:"language"`
	Identifier  string   `xml:"identifier"`
	Creator     string   `xml:"creator"`
	Publisher   string   `xml:"publisher"`
	Date        string   `xml:"date"`
	Description string   `xml:"description"`
	Subject     string   `xml:"subject"`
	Rights      string   `xml:"rights"`
}

type Manifest struct {
	XMLName xml.Name `xml:"manifest"`
	Items   []Item   `xml:"item"`
}

type Item struct {
	ID        string `xml:"id,attr"`
	Href      string `xml:"href,attr"`
	MediaType string `xml:"media-type,attr"`
}

type Spine struct {
	XMLName  xml.Name  `xml:"spine"`
	TOC      string    `xml:"toc,attr"`
	ItemRefs []ItemRef `xml:"itemref"`
}

type ItemRef struct {
	IDRef  string `xml:"idref,attr"`
	Linear string `xml:"linear,attr"`
}

type Guide struct {
	XMLName    xml.Name    `xml:"guide"`
	References []Reference `xml:"reference"`
}

type Reference struct {
	Type  string `xml:"type,attr"`
	Title string `xml:"title,attr"`
	Href  string `xml:"href,attr"`
}

type Chapter struct {
	ID                    string            `json:"id"`
	Title                 string            `json:"title"`
	FilePath              string            `json:"file_path"`
	RelativePath          string            `json:"relative_path"`
	Content               string            `json:"content"`
	TranslatedContent     string            `json:"translated_content,omitempty"`
	Order                 int               `json:"order"`
	WordCount             int               `json:"word_count"`
	IsTranslated          bool              `json:"is_translated"`
	AvailableTranslations map[string]bool   `json:"available_translations"` // lang -> exists
	TranslationPaths      map[string]string `json:"translation_paths"`      // lang -> file path
}

type TranslationProgress struct {
	ID                string    `json:"id"`
	SourceLanguage    string    `json:"source_language"`
	TargetLanguage    string    `json:"target_language"`
	TotalChapters     int       `json:"total_chapters"`
	CompletedChapters int       `json:"completed_chapters"`
	CurrentChapter    string    `json:"current_chapter"`
	Status            string    `json:"status"`
	StartedAt         time.Time `json:"started_at"`
	CompletedAt       time.Time `json:"completed_at,omitempty"`
	ErrorMessage      string    `json:"error_message,omitempty"`
}

type EPUBProcessor interface {
	Extract(filepath string) (*EPUB, error)
	Validate(epub *EPUB) error
	CreateTranslated(epub *EPUB, targetLang string) (string, error)
}

type LanguageDetector interface {
	DetectLanguage(text string) (string, error)
}

type Translator interface {
	TranslateText(text, sourceLang, targetLang string) (string, error)
	TranslateHTML(html, sourceLang, targetLang string) (string, error)
}
