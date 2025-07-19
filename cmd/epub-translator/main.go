package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"epub-translator/internal/config"
	"epub-translator/internal/server"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	version = "1.0.0"
	logger  *logrus.Logger
)

func init() {
	logger = logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		logger.Fatal(err)
	}
}

var rootCmd = &cobra.Command{
	Use:   "epub-translator",
	Short: "A web-based EPUB translation tool using OpenAI",
	Long:  `EPUB Translator is a CLI application that launches a web server to translate EPUB files using OpenAI's language models with real-time preview capabilities.`,
	Run:   runServer,
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the web server",
	Run:   runServer,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("EPUB Translator v%s\n", version)
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
	Long:  `Manage application configuration including viewing current settings and setting up API keys.`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		showConfig(cmd)
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration file",
	Run: func(cmd *cobra.Command, args []string) {
		initConfig(cmd)
	},
}

func init() {
	rootCmd.PersistentFlags().IntP("port", "p", 8080, "Port to run the web server on")
	rootCmd.PersistentFlags().StringP("openai-key", "k", "", "OpenAI API key")
	rootCmd.PersistentFlags().StringP("output-dir", "o", "output", "Output directory for translated EPUB files")
	rootCmd.PersistentFlags().StringP("temp-dir", "t", "tmp", "Temporary directory for processing files")
	rootCmd.PersistentFlags().StringP("config", "c", "", "Configuration file path (default: config.json beside executable)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose logging")

	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configCmd)

	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configInitCmd)
}

func runServer(cmd *cobra.Command, _ []string) {
	// Load configuration
	cfg, err := loadConfig(cmd)
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup logging based on config and flags
	setupLogging(cmd)

	// Validate configuration
	if cfg.OpenAI.APIKey == "" {
		logger.Fatal("OpenAI API key is required but not found in configuration")
	}

	// Create necessary directories
	if err := os.MkdirAll(cfg.App.TempDir, 0755); err != nil {
		logger.Fatalf("Failed to create temp directory: %v", err)
	}

	if err := os.MkdirAll(cfg.App.OutputDir, 0755); err != nil {
		logger.Fatalf("Failed to create output directory: %v", err)
	}

	// Initialize server
	srv := server.New(cfg, logger)
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      srv.Handler(),
		ReadTimeout:  cfg.Server.ReadTimeout.Duration,
		WriteTimeout: cfg.Server.WriteTimeout.Duration,
	}

	// Start server in goroutine
	go func() {
		logger.Infof("🚀 Starting EPUB Translator server")
		logger.Infof("📡 Server running on port %d", cfg.Server.Port)
		logger.Infof("🌐 Access the application at http://localhost:%d", cfg.Server.Port)
		logger.Infof("📁 Temp directory: %s", cfg.App.TempDir)
		logger.Infof("📤 Output directory: %s", cfg.App.OutputDir)

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("🛑 Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Fatalf("Server forced to shutdown: %v", err)
	}

	logger.Info("✅ Server exited gracefully")
}

func loadConfig(cmd *cobra.Command) (*config.Config, error) {
	// Get config path from flag or use default
	configPath, _ := cmd.Flags().GetString("config")
	if configPath == "" {
		configPath = config.GetConfigPath()
	}

	logger.Debugf("Loading configuration from: %s", configPath)

	// Load configuration using the new system
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	// Override with command line flags
	if port, _ := cmd.Flags().GetInt("port"); port != 8080 {
		cfg.Server.Port = port
		logger.Debugf("Port overridden by flag: %d", port)
	}

	if apiKey, _ := cmd.Flags().GetString("openai-key"); apiKey != "" {
		cfg.OpenAI.APIKey = apiKey
		logger.Debug("OpenAI API key overridden by flag")
	}

	if outputDir, _ := cmd.Flags().GetString("output-dir"); outputDir != "output" {
		cfg.App.OutputDir = outputDir
		logger.Debugf("Output directory overridden by flag: %s", outputDir)
	}

	if tempDir, _ := cmd.Flags().GetString("temp-dir"); tempDir != "tmp" {
		cfg.App.TempDir = tempDir
		logger.Debugf("Temp directory overridden by flag: %s", tempDir)
	}

	return cfg, nil
}

func setupLogging(cmd *cobra.Command) {
	verbose, _ := cmd.Flags().GetBool("verbose")
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}
}

func showConfig(cmd *cobra.Command) {
	configPath, _ := cmd.Flags().GetString("config")
	if configPath == "" {
		configPath = config.GetConfigPath()
	}

	fmt.Printf("📋 EPUB Translator Configuration\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Configuration file: %s\n\n", configPath)

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("❌ Configuration file does not exist\n")
		fmt.Printf("💡 Run 'epub-translator config init' to create one\n")
		return
	}

	// Load and display configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("❌ Failed to load configuration: %v\n", err)
		return
	}

	fmt.Printf("Server Settings:\n")
	fmt.Printf("  Port: %d\n", cfg.Server.Port)
	fmt.Printf("  Read Timeout: %s\n", cfg.Server.ReadTimeout)
	fmt.Printf("  Write Timeout: %s\n", cfg.Server.WriteTimeout)
	fmt.Printf("\n")

	fmt.Printf("OpenAI Settings:\n")
	if cfg.OpenAI.APIKey != "" {
		maskedKey := cfg.OpenAI.APIKey[:6] + "..." + cfg.OpenAI.APIKey[len(cfg.OpenAI.APIKey)-4:]
		fmt.Printf("  API Key: %s\n", maskedKey)
	} else {
		fmt.Printf("  API Key: ❌ Not set\n")
	}
	fmt.Printf("  Model: %s\n", cfg.OpenAI.Model)
	fmt.Printf("  Max Tokens: %d\n", cfg.OpenAI.MaxTokens)
	fmt.Printf("  Temperature: %.1f\n", cfg.OpenAI.Temperature)
	fmt.Printf("\n")

	fmt.Printf("Translation Settings:\n")
	fmt.Printf("  Batch Size: %d\n", cfg.Translation.BatchSize)
	fmt.Printf("  Max Retries: %d\n", cfg.Translation.MaxRetries)
	fmt.Printf("  Retry Delay: %s\n", cfg.Translation.RetryDelay)
	fmt.Printf("  Supported Languages: %d languages\n", len(cfg.Translation.SupportedLangs))
	fmt.Printf("\n")

	fmt.Printf("Application Settings:\n")
	fmt.Printf("  Temp Directory: %s\n", cfg.App.TempDir)
	fmt.Printf("  Output Directory: %s\n", cfg.App.OutputDir)
}

func initConfig(cmd *cobra.Command) {
	configPath, _ := cmd.Flags().GetString("config")
	if configPath == "" {
		configPath = config.GetConfigPath()
	}

	fmt.Printf("🔧 Initializing EPUB Translator Configuration\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Configuration file: %s\n\n", configPath)

	// Check if config file already exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("⚠️  Configuration file already exists\n")
		return
	}

	// Load configuration (this will create the file and prompt for API key)
	_, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("❌ Failed to initialize configuration: %v\n", err)
		return
	}

	fmt.Printf("\n✅ Configuration initialized successfully!\n")
	fmt.Printf("💡 You can now run 'epub-translator' to start the server\n")
	fmt.Printf("📋 Use 'epub-translator config show' to view your configuration\n")
}
