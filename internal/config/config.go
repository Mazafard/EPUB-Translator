package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Duration is a custom type that handles JSON marshaling/unmarshaling
type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dur
	return nil
}

type Config struct {
	Server struct {
		Port         int      `json:"port"`
		ReadTimeout  Duration `json:"read_timeout"`
		WriteTimeout Duration `json:"write_timeout"`
	} `json:"server"`

	OpenAI struct {
		APIKey      string  `json:"api_key"`
		Model       string  `json:"model"`
		MaxTokens   int     `json:"max_tokens"`
		Temperature float32 `json:"temperature"`
	} `json:"openai"`

	Translation struct {
		BatchSize      int      `json:"batch_size"`
		MaxRetries     int      `json:"max_retries"`
		RetryDelay     Duration `json:"retry_delay"`
		SupportedLangs []string `json:"supported_languages"`
	} `json:"translation"`

	App struct {
		TempDir   string `json:"temp_dir"`
		OutputDir string `json:"output_dir"`
	} `json:"app"`
}

func New() *Config {
	return &Config{
		Server: struct {
			Port         int      `json:"port"`
			ReadTimeout  Duration `json:"read_timeout"`
			WriteTimeout Duration `json:"write_timeout"`
		}{
			Port:         8080,
			ReadTimeout:  Duration{30 * time.Second},
			WriteTimeout: Duration{30 * time.Second},
		},
		OpenAI: struct {
			APIKey      string  `json:"api_key"`
			Model       string  `json:"model"`
			MaxTokens   int     `json:"max_tokens"`
			Temperature float32 `json:"temperature"`
		}{
			Model:       "gpt-4o",
			MaxTokens:   2048,
			Temperature: 0.4,
		},
		Translation: struct {
			BatchSize      int      `json:"batch_size"`
			MaxRetries     int      `json:"max_retries"`
			RetryDelay     Duration `json:"retry_delay"`
			SupportedLangs []string `json:"supported_languages"`
		}{
			BatchSize:  10,
			MaxRetries: 3,
			RetryDelay: Duration{2 * time.Second},
			SupportedLangs: []string{
				"en", "es", "fr", "de", "it", "pt", "ru", "ja", "ko", "zh",
				"ar", "fa", "he", "hi", "tr", "pl", "nl", "sv", "da", "no",
			},
		},
		App: struct {
			TempDir   string `json:"temp_dir"`
			OutputDir string `json:"output_dir"`
		}{
			TempDir:   "tmp",
			OutputDir: "output",
		},
	}
}

func (c *Config) LoadFromFile(filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, c)
}

func (c *Config) SaveToFile(filepath string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath, data, 0644)
}

func (c *Config) LoadFromEnv() {
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		c.OpenAI.APIKey = apiKey
	}
	if model := os.Getenv("OPENAI_MODEL"); model != "" {
		c.OpenAI.Model = model
	}
	if port := os.Getenv("PORT"); port != "" {
		if p := parseInt(port); p > 0 {
			c.Server.Port = p
		}
	}
	if tempDir := os.Getenv("TEMP_DIR"); tempDir != "" {
		c.App.TempDir = tempDir
	}
	if outputDir := os.Getenv("OUTPUT_DIR"); outputDir != "" {
		c.App.OutputDir = outputDir
	}
}

func parseInt(s string) int {
	var result int
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0
		}
		result = result*10 + int(ch-'0')
	}
	return result
}

// Load loads configuration with the following priority:
// 1. Command line flags (handled in main.go)
// 2. Environment variables
// 3. Configuration file (config.json)
// 4. Default values
func Load(configPath string) (*Config, error) {
	cfg := New()

	// Check if config file exists, if not try to create from example
	if err := ensureConfigFile(configPath); err != nil {
		return nil, fmt.Errorf("failed to ensure config file: %w", err)
	}

	// Load from config file
	if err := cfg.LoadFromFile(configPath); err != nil {
		return nil, fmt.Errorf("failed to load config from file: %w", err)
	}

	// Override with environment variables
	cfg.LoadFromEnv()

	// Validate and prompt for missing OpenAI API key
	if cfg.OpenAI.APIKey == "" || cfg.OpenAI.APIKey == "your-openai-api-key-here" {
		apiKey, err := promptForAPIKey()
		if err != nil {
			return nil, fmt.Errorf("failed to get OpenAI API key: %w", err)
		}
		cfg.OpenAI.APIKey = apiKey

		// Save the updated config back to file
		if err := cfg.SaveToFile(configPath); err != nil {
			fmt.Printf("Warning: Failed to save API key to config file: %v\n", err)
		} else {
			fmt.Printf("✅ OpenAI API key saved to %s\n", configPath)
		}
	}

	return cfg, nil
}

// ensureConfigFile checks if config.json exists, if not creates it from config.example.json
func ensureConfigFile(configPath string) error {
	// Check if config file already exists
	if _, err := os.Stat(configPath); err == nil {
		return nil // File exists, nothing to do
	}

	// Try to find config.example.json in the same directory
	configDir := filepath.Dir(configPath)
	examplePath := filepath.Join(configDir, "config.example.json")

	// If we're running from a different directory, try to find the example file
	// relative to the executable
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		if execPath, execErr := os.Executable(); execErr == nil {
			execDir := filepath.Dir(execPath)
			examplePath = filepath.Join(execDir, "config.example.json")
		}
	}

	// Check if example file exists
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		// Create a basic config file with defaults
		fmt.Printf("⚠️  No config.example.json found, creating basic config.json...\n")
		cfg := New()
		return cfg.SaveToFile(configPath)
	}

	// Copy example file to config file
	fmt.Printf("📋 Creating config.json from config.example.json...\n")
	return copyFile(examplePath, configPath)
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = sourceFile.Close() }()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = destFile.Close() }()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return destFile.Sync()
}

// promptForAPIKey prompts the user to enter their OpenAI API key
func promptForAPIKey() (string, error) {
	fmt.Println("\n🔑 OpenAI API Key Required")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("To use the EPUB Translator, you need an OpenAI API key.")
	fmt.Println("Get one at: https://platform.openai.com/api-keys")
	fmt.Println("")

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("Please enter your OpenAI API key: ")
		apiKey, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		apiKey = strings.TrimSpace(apiKey)
		if apiKey == "" {
			fmt.Println("❌ API key cannot be empty. Please try again.")
			continue
		}

		if !strings.HasPrefix(apiKey, "sk-") {
			fmt.Println("⚠️  Warning: OpenAI API keys typically start with 'sk-'")
			fmt.Print("Continue anyway? (y/N): ")
			confirm, err := reader.ReadString('\n')
			if err != nil {
				return "", err
			}
			confirm = strings.TrimSpace(strings.ToLower(confirm))
			if confirm != "y" && confirm != "yes" {
				continue
			}
		}

		return apiKey, nil
	}
}

// GetConfigPath returns the path to the config file
// It looks for config.json in the same directory as the executable
func GetConfigPath() string {
	// Try to get the executable directory
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		return filepath.Join(execDir, "config.json")
	}

	// Fallback to current working directory
	if pwd, err := os.Getwd(); err == nil {
		return filepath.Join(pwd, "config.json")
	}

	// Final fallback
	return "config.json"
}
