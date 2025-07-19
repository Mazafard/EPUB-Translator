package server

import (
	"path/filepath"

	"epub-translator/internal/config"
	"epub-translator/internal/epub"
	"epub-translator/internal/translation"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Server struct {
	config         *config.Config
	logger         *logrus.Logger
	epubParser     *epub.Parser
	epubBuilder    *epub.Builder
	translationSvc *translation.Service
	epubStorage    map[string]*epub.EPUB
	router         *gin.Engine
	wsHub          *Hub
}

func New(cfg *config.Config, logger *logrus.Logger) *Server {
	gin.SetMode(gin.ReleaseMode)

	epubParser := epub.NewParser(logger, cfg.App.TempDir)
	epubBuilder := epub.NewBuilder(logger)

	openaiClient := translation.NewOpenAIClient(
		cfg.OpenAI.APIKey,
		cfg.OpenAI.Model,
		cfg.OpenAI.MaxTokens,
		cfg.OpenAI.Temperature,
		cfg.Translation.MaxRetries,
		cfg.Translation.RetryDelay.Duration,
		logger,
	)

	// Create WebSocket hub
	wsHub := NewHub(logger)
	go wsHub.Run()

	// Set WebSocket broadcaster on OpenAI client for LLM logging
	openaiClient.SetWebSocketBroadcaster(wsHub)

	translationSvc := translation.NewService(openaiClient, logger, cfg.Translation.BatchSize, wsHub)

	s := &Server{
		config:         cfg,
		logger:         logger,
		epubParser:     epubParser,
		epubBuilder:    epubBuilder,
		translationSvc: translationSvc,
		epubStorage:    make(map[string]*epub.EPUB),
		wsHub:          wsHub,
	}

	s.setupRoutes()
	return s
}

func (s *Server) Handler() *gin.Engine {
	return s.router
}

func (s *Server) setupRoutes() {
	s.router = gin.New()

	s.router.Use(s.loggingMiddleware())
	s.router.Use(s.corsMiddleware())
	s.router.Use(gin.Recovery())

	s.router.Static("/static", "web/static")

	// Convert temp directory to absolute path for static file serving
	absTempDir, err := filepath.Abs(s.config.App.TempDir)
	if err != nil {
		s.logger.Errorf("Failed to get absolute path for temp directory: %v", err)
		absTempDir = s.config.App.TempDir
	}
	s.router.Static("/epub_files", absTempDir)

	s.router.LoadHTMLGlob("web/templates/*")

	s.router.GET("/", s.handleHome)
	s.router.POST("/upload", s.handleUpload)
	s.router.GET("/preview/:id", s.handlePreview)
	s.router.POST("/translate", s.handleTranslate)
	s.router.GET("/status/:id", s.handleStatus)
	s.router.GET("/download/:id", s.handleDownload)
	s.router.GET("/api/chapters/:id", s.handleGetChapters)
	s.router.DELETE("/api/epub/:id", s.handleDeleteEpub)

	// WebSocket endpoint
	s.router.GET("/ws", s.HandleWebSocket)

	// Enhanced preview endpoints
	s.router.GET("/api/chapter/:epub_id/:chapter_id", s.handleGetChapter)
	s.router.POST("/api/translate-page", s.handleTranslatePage)
	s.router.GET("/reader/:id", s.handleReader)

	// Previous work endpoints
	s.router.GET("/previous-work", s.handlePreviousWork)
	s.router.GET("/api/download-file", s.handleDownloadFile)
	s.router.POST("/api/delete-file", s.handleDeleteFile)
	s.router.POST("/api/process-file", s.handleProcessFile)

	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "websocket_clients": s.wsHub.GetClientCount()})
	})
}

func (s *Server) loggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		s.logger.WithFields(logrus.Fields{
			"status":     param.StatusCode,
			"method":     param.Method,
			"path":       param.Path,
			"ip":         param.ClientIP,
			"user_agent": param.Request.UserAgent(),
			"latency":    param.Latency,
		}).Info("HTTP Request")
		return ""
	})
}

func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
