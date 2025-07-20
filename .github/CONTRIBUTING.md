# Contributing to EPUB Translator

Thank you for your interest in contributing to EPUB Translator! This document provides guidelines and information for contributors.

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## How to Contribute

### Reporting Bugs

Before creating a bug report, please check the [existing issues](https://github.com/Mazafard/EPUB-Translator/issues) to see if the problem has already been reported.

When creating a bug report, please include:
- A clear and descriptive title
- Steps to reproduce the issue
- Expected behavior vs actual behavior
- Screenshots (if applicable)
- Environment details (OS, browser, version)
- EPUB file details (size, language, format)

### Suggesting Features

We welcome feature suggestions! Please create an issue using the feature request template and include:
- A clear description of the proposed feature
- The use case and who would benefit
- Any implementation ideas you might have

### Translation Issues

If you encounter translation quality issues, please use the translation issue template and provide:
- The source and target languages
- The original and translated text
- Context where the issue occurs
- Suggestions for improvement

### Code Contributions

#### Development Setup

1. **Fork the repository**
   ```bash
   git clone https://github.com/your-username/EPUB-Translator.git
   cd EPUB-Translator
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Set up configuration**
   ```bash
   cp config.example.json config.json
   # Edit config.json with your settings
   ```

4. **Run tests**
   ```bash
   go test ./...
   ```

5. **Start the development server**
   ```bash
   go run cmd/epub-translator/main.go
   ```

#### Making Changes

1. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes**
   - Follow the existing code style
   - Add tests for new functionality
   - Update documentation as needed

3. **Test your changes**
   ```bash
   # Run tests
   go test ./...
   
   # Run linting
   golangci-lint run
   
   # Test manually with different EPUB files
   ```

4. **Commit your changes**
   ```bash
   git add .
   git commit -m "feat: add your feature description"
   ```

5. **Push to your fork**
   ```bash
   git push origin feature/your-feature-name
   ```

6. **Create a Pull Request**

#### Code Style Guidelines

- **Go Code**: Follow standard Go conventions
  - Use `gofmt` for formatting
  - Follow effective Go guidelines
  - Add comments for exported functions
  - Use meaningful variable names

- **JavaScript**: 
  - Use consistent indentation (2 spaces)
  - Use meaningful variable names
  - Add comments for complex logic

- **HTML/CSS**:
  - Use semantic HTML
  - Follow BEM methodology for CSS classes
  - Ensure responsive design

#### Testing

- Write unit tests for new functionality
- Test with various EPUB files and formats
- Test translation accuracy for supported languages
- Verify UI responsiveness across browsers

### Documentation

Help improve our documentation by:
- Fixing typos or unclear explanations
- Adding examples and use cases
- Improving API documentation
- Creating tutorials or guides

## Development Workflow

### Branching Strategy

- `main`: Production-ready code
- `develop`: Integration branch for features
- `feature/*`: New features or enhancements
- `bugfix/*`: Bug fixes
- `hotfix/*`: Critical production fixes

### Commit Message Convention

We use conventional commits:

- `feat:` new features
- `fix:` bug fixes
- `docs:` documentation changes
- `style:` formatting, missing semicolons, etc
- `refactor:` code refactoring
- `test:` adding tests
- `chore:` updating build tasks, package manager configs, etc

Examples:
```
feat: add support for RTL languages
fix: resolve EPUB parsing error for large files
docs: update API documentation
```

### Pull Request Process

1. Ensure your PR has a clear title and description
2. Link to any related issues
3. Ensure all tests pass
4. Update documentation if needed
5. Request review from maintainers

### Review Process

- All submissions require review
- We may suggest changes or improvements
- Once approved, maintainers will merge the PR

## Getting Help

- **Questions**: Create a [Discussion](https://github.com/Mazafard/EPUB-Translator/discussions)
- **Issues**: Create an [Issue](https://github.com/Mazafard/EPUB-Translator/issues)
- **Chat**: Join our community chat (if available)

## Recognition

Contributors will be recognized in:
- The project README
- Release notes
- Contributors page

Thank you for contributing to EPUB Translator!