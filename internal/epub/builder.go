package epub

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

type Builder struct {
	logger *logrus.Logger
}

func NewBuilder(logger *logrus.Logger) *Builder {
	return &Builder{
		logger: logger,
	}
}

func (b *Builder) CreateTranslated(epub *EPUB, targetLang string, outputDir string) (string, error) {
	b.logger.Debugf("Creating translated EPUB for language: %s", targetLang)

	if err := b.updateChapterFiles(epub); err != nil {
		return "", fmt.Errorf("failed to update chapter files: %w", err)
	}

	if err := b.updateMetadata(epub, targetLang); err != nil {
		return "", fmt.Errorf("failed to update metadata: %w", err)
	}

	outputFileName := fmt.Sprintf("%s_%s.epub", epub.ID, targetLang)
	outputPath := filepath.Join(outputDir, outputFileName)

	if err := b.createZip(epub.TempDir, outputPath); err != nil {
		return "", fmt.Errorf("failed to create ZIP: %w", err)
	}

	b.logger.Infof("Created translated EPUB: %s", outputPath)
	return outputPath, nil
}

func (b *Builder) updateChapterFiles(epub *EPUB) error {
	packageDir := filepath.Dir(filepath.Join(epub.TempDir, epub.Package.OriginalPath))

	for _, chapter := range epub.Chapters {
		if !chapter.IsTranslated {
			continue
		}

		chapterPath := filepath.Join(packageDir, chapter.RelativePath)

		originalContent, err := os.ReadFile(chapterPath)
		if err != nil {
			return fmt.Errorf("failed to read original chapter file %s: %w", chapterPath, err)
		}

		updatedContent := b.replaceBodyContent(string(originalContent), chapter.TranslatedContent)

		if err := os.WriteFile(chapterPath, []byte(updatedContent), 0644); err != nil {
			return fmt.Errorf("failed to write translated chapter file %s: %w", chapterPath, err)
		}

		b.logger.Debugf("Updated chapter file: %s", chapterPath)
	}

	return nil
}

func (b *Builder) replaceBodyContent(originalHTML, translatedBody string) string {
	bodyStart := strings.Index(originalHTML, "<body")
	bodyEnd := strings.Index(originalHTML, "</body>")

	if bodyStart == -1 || bodyEnd == -1 {
		return translatedBody
	}

	bodyOpenEnd := strings.Index(originalHTML[bodyStart:], ">")
	if bodyOpenEnd == -1 {
		return translatedBody
	}

	bodyOpenEnd += bodyStart + 1

	before := originalHTML[:bodyOpenEnd]
	after := originalHTML[bodyEnd:]

	return before + translatedBody + after
}

func (b *Builder) updateMetadata(epub *EPUB, targetLang string) error {
	epub.Package.Metadata.Language = targetLang

	if epub.Package.Metadata.Title != "" {
		epub.Package.Metadata.Title += fmt.Sprintf(" (%s)", strings.ToUpper(targetLang))
	}

	packagePath := filepath.Join(epub.TempDir, epub.Package.OriginalPath)

	packageContent, err := b.generatePackageXML(&epub.Package)
	if err != nil {
		return fmt.Errorf("failed to generate package XML: %w", err)
	}

	if err := os.WriteFile(packagePath, []byte(packageContent), 0644); err != nil {
		return fmt.Errorf("failed to write package file: %w", err)
	}

	return nil
}

func (b *Builder) generatePackageXML(pkg *Package) (string, error) {
	var builder strings.Builder

	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf(`<package version="%s" unique-identifier="%s" xmlns="http://www.idpf.org/2007/opf">`,
		pkg.Version, pkg.UniqueID))
	builder.WriteString("\n")

	builder.WriteString("  <metadata xmlns:dc=\"http://purl.org/dc/elements/1.1/\">\n")
	if pkg.Metadata.Title != "" {
		builder.WriteString(fmt.Sprintf("    <dc:title>%s</dc:title>\n", escapeXML(pkg.Metadata.Title)))
	}
	if pkg.Metadata.Language != "" {
		builder.WriteString(fmt.Sprintf("    <dc:language>%s</dc:language>\n", pkg.Metadata.Language))
	}
	if pkg.Metadata.Identifier != "" {
		builder.WriteString(fmt.Sprintf("    <dc:identifier id=\"%s\">%s</dc:identifier>\n",
			pkg.UniqueID, escapeXML(pkg.Metadata.Identifier)))
	}
	if pkg.Metadata.Creator != "" {
		builder.WriteString(fmt.Sprintf("    <dc:creator>%s</dc:creator>\n", escapeXML(pkg.Metadata.Creator)))
	}
	if pkg.Metadata.Publisher != "" {
		builder.WriteString(fmt.Sprintf("    <dc:publisher>%s</dc:publisher>\n", escapeXML(pkg.Metadata.Publisher)))
	}
	if pkg.Metadata.Date != "" {
		builder.WriteString(fmt.Sprintf("    <dc:date>%s</dc:date>\n", pkg.Metadata.Date))
	}
	builder.WriteString("  </metadata>\n")

	builder.WriteString("  <manifest>\n")
	for _, item := range pkg.Manifest.Items {
		builder.WriteString(fmt.Sprintf("    <item id=\"%s\" href=\"%s\" media-type=\"%s\"/>\n",
			item.ID, item.Href, item.MediaType))
	}
	builder.WriteString("  </manifest>\n")

	builder.WriteString(fmt.Sprintf("  <spine toc=\"%s\">\n", pkg.Spine.TOC))
	for _, itemRef := range pkg.Spine.ItemRefs {
		linear := ""
		if itemRef.Linear != "" {
			linear = fmt.Sprintf(` linear="%s"`, itemRef.Linear)
		}
		builder.WriteString(fmt.Sprintf("    <itemref idref=\"%s\"%s/>\n", itemRef.IDRef, linear))
	}
	builder.WriteString("  </spine>\n")

	if len(pkg.Guide.References) > 0 {
		builder.WriteString("  <guide>\n")
		for _, ref := range pkg.Guide.References {
			builder.WriteString(fmt.Sprintf("    <reference type=\"%s\" title=\"%s\" href=\"%s\"/>\n",
				ref.Type, escapeXML(ref.Title), ref.Href))
		}
		builder.WriteString("  </guide>\n")
	}

	builder.WriteString("</package>")

	return builder.String(), nil
}

func (b *Builder) createZip(sourceDir, outputPath string) error {
	zipFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() { _ = zipFile.Close() }()

	zipWriter := zip.NewWriter(zipFile)
	defer func() { _ = zipWriter.Close() }()

	if err := b.writeMimetypeFile(zipWriter); err != nil {
		return err
	}

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		if relPath == "mimetype" {
			return nil
		}

		return b.addFileToZip(zipWriter, path, relPath)
	})
}

func (b *Builder) writeMimetypeFile(zipWriter *zip.Writer) error {
	writer, err := zipWriter.CreateHeader(&zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store,
	})
	if err != nil {
		return err
	}

	_, err = writer.Write([]byte("application/epub+zip"))
	return err
}

func (b *Builder) addFileToZip(zipWriter *zip.Writer, filePath, relPath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	writer, err := zipWriter.Create(relPath)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	return err
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
