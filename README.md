# 📚 EPUB Translator

A powerful Go-based CLI application that launches a web server to translate EPUB files using OpenAI's language models with real-time preview capabilities.

## ✨ Features

- **EPUB Processing**: Extract and parse EPUB files with full structure preservation
- **AI-Powered Translation**: Uses OpenAI GPT models for high-quality translations
- **Language Detection**: Automatic source language detection
- **Real-time Preview**: Web interface to preview books before and after translation
- **Progress Tracking**: Real-time translation progress with detailed status updates
- **RTL Language Support**: Proper handling of right-to-left languages (Arabic, Hebrew, Persian)
- **Web Interface**: Clean, responsive web UI for easy file management
- **CLI Interface**: Command-line tool with flexible configuration options

## 🚀 Quick Start

### Prerequisites

- Go 1.21 or higher
- OpenAI API key

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd epub-translator
```

2. Install dependencies:
```bash
go mod download
```

3. Build the application:
```bash
go build -o epub-translator cmd/epub-translator/main.go
```

### Running the Application

1. Set your OpenAI API key:
```bash
export OPENAI_API_KEY="your-openai-api-key"
```

2. Start the server:
```bash
./epub-translator --port 8080
```

3. Open your browser and navigate to `http://localhost:8080`

## 📖 Usage

### Command Line Options

```bash
./epub-translator [command] [flags]

Available Commands:
  server      Start the web server (default)
  version     Print the version number
  help        Help about any command

Flags:
  -p, --port int          Port to run the web server on (default 8080)
  -k, --openai-key string OpenAI API key
  -o, --output-dir string Output directory for translated EPUB files (default "output")
  -t, --temp-dir string   Temporary directory for processing files (default "tmp")
  -v, --verbose           Enable verbose logging
```

### Environment Variables

- `OPENAI_API_KEY`: Your OpenAI API key (required)
- `OPENAI_MODEL`: OpenAI model to use (default: "gpt-3.5-turbo")
- `PORT`: Server port (default: 8080)
- `TEMP_DIR`: Temporary directory (default: "tmp")
- `OUTPUT_DIR`: Output directory (default: "output")

### Web Interface

1. **Upload**: Drag and drop or select an EPUB file
2. **Preview**: Review the book structure and content
3. **Translate**: Select target language and start translation
4. **Download**: Get your translated EPUB file

## 🛠️ Architecture

The application follows clean architecture principles with clear separation of concerns:

```
epub-translator/
├── cmd/epub-translator/     # CLI entry point
├── internal/
│   ├── epub/               # EPUB processing logic
│   ├── translation/        # OpenAI integration
│   ├── server/            # Web server and handlers
│   ├── preview/           # Preview rendering
│   └── config/            # Configuration management
├── web/
│   ├── templates/         # HTML templates
│   └── static/           # CSS and JavaScript
└── pkg/                  # Shared utilities
```

### Key Components

- **EPUB Parser**: Extracts and validates EPUB files
- **Translation Service**: Manages OpenAI API calls with retry logic
- **Web Server**: Gin-based HTTP server with middleware
- **Progress Tracking**: Real-time translation status updates
- **File Builder**: Reconstructs translated EPUB files

## 🌍 Supported Languages

The application supports translation between major world languages including:

- English (en), Spanish (es), French (fr), German (de)
- Italian (it), Portuguese (pt), Russian (ru), Japanese (ja)
- Korean (ko), Chinese (zh), Arabic (ar), Persian (fa)
- Hebrew (he), Hindi (hi), Turkish (tr), Polish (pl)
- Dutch (nl), Swedish (sv), Danish (da), Norwegian (no)
- And many more...

## 🔧 Configuration

Configuration can be managed through:

1. **Command-line flags** (highest priority)
2. **Environment variables**
3. **Configuration file** (JSON format)
4. **Default values** (lowest priority)

### Example Configuration File

```json
{
  "server": {
    "port": 8080,
    "read_timeout": "30s",
    "write_timeout": "30s"
  },
  "openai": {
    "api_key": "your-api-key",
    "model": "gpt-3.5-turbo",
    "max_tokens": 2048,
    "temperature": 0.3
  },
  "translation": {
    "batch_size": 10,
    "max_retries": 3,
    "retry_delay": "2s"
  }
}
```

## 🧪 Testing

Run the test suite:

```bash
go test ./...
```

Run tests with coverage:

```bash
go test -cover ./...
```

## 📝 API Endpoints

- `GET /` - Main upload interface
- `POST /upload` - Upload EPUB file
- `GET /preview/:id` - Preview book content
- `POST /translate` - Start translation
- `GET /status/:id` - Get translation progress
- `GET /download/:id` - Download translated EPUB
- `GET /api/chapters/:id` - Get chapter data
- `DELETE /api/epub/:id` - Delete processed EPUB

## 🔒 Security Considerations

- File size limits (50MB maximum)
- Input validation and sanitization
- Secure file handling
- Rate limiting for API calls
- No storage of sensitive data

## 🚧 Development

### Project Structure

Following Go best practices:
- Clean architecture with dependency injection
- Interface-based design for testability
- Comprehensive error handling
- Structured logging
- Graceful shutdown handling

### Adding New Features

1. Define interfaces in appropriate packages
2. Implement functionality with tests
3. Add integration tests
4. Update documentation

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## 🆘 Troubleshooting

### Common Issues

1. **"OpenAI API key is required"**
   - Set the `OPENAI_API_KEY` environment variable or use `--openai-key` flag

2. **"Failed to extract EPUB"**
   - Ensure the file is a valid EPUB format
   - Check file size (must be under 50MB)

3. **"Translation failed"**
   - Verify OpenAI API key is valid
   - Check internet connection
   - Review server logs for detailed error messages

### Logging

Enable verbose logging for debugging:

```bash
./epub-translator --verbose
```

## 🗺️ Roadmap

- [ ] Support for additional ebook formats (PDF, MOBI)
- [ ] Batch translation of multiple files
- [ ] Translation quality assessment
- [ ] Custom dictionary and terminology management
- [ ] Cloud storage integration
- [ ] Multi-user support with authentication

## 📞 Support

For support and questions:
- Open an issue on GitHub
- Check the troubleshooting section
- Review the documentation

---
## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=Mazafard/EPUB-Translator&type=Date)](https://www.star-history.com/#Mazafard/EPUB-Translator&Date)

Made with ❤️ using Go