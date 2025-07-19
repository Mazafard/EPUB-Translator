package translation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
)

type OpenAIClient struct {
	client      *openai.Client
	logger      *logrus.Logger
	model       string
	maxTokens   int
	temperature float32
	maxRetries  int
	retryDelay  time.Duration
}

func NewOpenAIClient(apiKey, model string, maxTokens int, temperature float32, maxRetries int, retryDelay time.Duration, logger *logrus.Logger) *OpenAIClient {
	return &OpenAIClient{
		client:      openai.NewClient(apiKey),
		logger:      logger,
		model:       model,
		maxTokens:   maxTokens,
		temperature: temperature,
		maxRetries:  maxRetries,
		retryDelay:  retryDelay,
	}
}

func (c *OpenAIClient) DetectLanguage(text string) (string, error) {
	prompt := fmt.Sprintf(`Detect the language of the following text. Respond with only the ISO 639-1 language code (e.g., "en", "es", "fr", "de").

Text: %s`, text)

	response, err := c.makeRequest(prompt)
	if err != nil {
		return "", fmt.Errorf("failed to detect language: %w", err)
	}

	lang := strings.TrimSpace(strings.ToLower(response))
	if len(lang) > 3 {
		lang = lang[:2]
	}

	c.logger.Debugf("Detected language: %s", lang)
	return lang, nil
}

func (c *OpenAIClient) TranslateText(text, sourceLang, targetLang string) (string, error) {
	if text == "" {
		return "", nil
	}

	sourceLanguage := getLanguageName(sourceLang)
	targetLanguage := getLanguageName(targetLang)

	prompt := fmt.Sprintf(`Translate the following text from %s to %s. Maintain the original tone, style, and formatting as much as possible. Return only the translated text without any additional comments or explanations.

Text: %s`, sourceLanguage, targetLanguage, text)

	response, err := c.makeRequest(prompt)
	if err != nil {
		return "", fmt.Errorf("failed to translate text: %w", err)
	}

	return strings.TrimSpace(response), nil
}

func (c *OpenAIClient) TranslateHTML(htmlContent, sourceLang, targetLang string) (string, error) {
	if htmlContent == "" {
		return "", nil
	}

	sourceLanguage := getLanguageName(sourceLang)
	targetLanguage := getLanguageName(targetLang)

	prompt := fmt.Sprintf(`Translate the following HTML content from %s to %s. 

IMPORTANT INSTRUCTIONS:
1. Preserve ALL HTML tags, attributes, and structure exactly as they are
2. Only translate the text content between HTML tags
3. Do NOT translate HTML tag names, attributes, or values
4. Maintain the original formatting, spacing, and line breaks
5. Keep any CSS classes, IDs, and other attributes unchanged
6. Return only the translated HTML without any additional comments

HTML content:
%s`, sourceLanguage, targetLanguage, htmlContent)

	response, err := c.makeRequest(prompt)
	if err != nil {
		return "", fmt.Errorf("failed to translate HTML: %w", err)
	}

	return strings.TrimSpace(response), nil
}

func (c *OpenAIClient) makeRequest(prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			c.logger.Debugf("Retrying OpenAI request (attempt %d/%d)", attempt+1, c.maxRetries+1)
			time.Sleep(c.retryDelay)
		}

		resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:       c.model,
			MaxTokens:   c.maxTokens,
			Temperature: c.temperature,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		})

		if err != nil {
			lastErr = err
			c.logger.Warnf("OpenAI request failed (attempt %d): %v", attempt+1, err)
			continue
		}

		if len(resp.Choices) == 0 {
			lastErr = fmt.Errorf("no response choices returned")
			continue
		}

		return resp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("max retries exceeded, last error: %w", lastErr)
}

func getLanguageName(code string) string {
	languages := map[string]string{
		"en": "English",
		"es": "Spanish",
		"fr": "French",
		"de": "German",
		"it": "Italian",
		"pt": "Portuguese",
		"ru": "Russian",
		"ja": "Japanese",
		"ko": "Korean",
		"zh": "Chinese",
		"ar": "Arabic",
		"fa": "Persian",
		"he": "Hebrew",
		"hi": "Hindi",
		"tr": "Turkish",
		"pl": "Polish",
		"nl": "Dutch",
		"sv": "Swedish",
		"da": "Danish",
		"no": "Norwegian",
		"fi": "Finnish",
		"cs": "Czech",
		"sk": "Slovak",
		"hu": "Hungarian",
		"ro": "Romanian",
		"bg": "Bulgarian",
		"hr": "Croatian",
		"sl": "Slovenian",
		"et": "Estonian",
		"lv": "Latvian",
		"lt": "Lithuanian",
		"el": "Greek",
		"th": "Thai",
		"vi": "Vietnamese",
		"id": "Indonesian",
		"ms": "Malay",
		"tl": "Filipino",
		"uk": "Ukrainian",
		"be": "Belarusian",
		"ka": "Georgian",
		"hy": "Armenian",
		"az": "Azerbaijani",
		"kk": "Kazakh",
		"ky": "Kyrgyz",
		"uz": "Uzbek",
		"tj": "Tajik",
		"mn": "Mongolian",
	}

	if name, exists := languages[code]; exists {
		return name
	}

	return code
}