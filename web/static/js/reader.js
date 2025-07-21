// Reader page functionality with WebSocket support
document.addEventListener('DOMContentLoaded', function() {
    // DOM elements
    const chapterItems = document.querySelectorAll('.chapter-item');
    const currentChapterTitle = document.getElementById('current-chapter-title');
    const chapterContent = document.getElementById('chapter-content');
    const chapterControls = document.getElementById('chapter-controls');
    const translatePageBtn = document.getElementById('translate-page-btn');
    const toggleViewBtn = document.getElementById('toggle-view-btn');
    const sideBySideBtn = document.getElementById('side-by-side-btn');
    const targetLanguageSelect = document.getElementById('target-language');
    const translationStatus = document.getElementById('translation-status');
    const translationModal = document.getElementById('translation-modal');
    const translationProgressText = document.getElementById('translation-progress-text');
    const sourceLangDisplay = document.getElementById('source-lang-display');
    const targetLangDisplay = document.getElementById('target-lang-display');
    
    // Logs panel elements
    const toggleLogsBtn = document.getElementById('toggle-logs');
    const logsPanel = document.getElementById('logs-panel');
    const logsContainer = document.getElementById('logs-container');
    const clearLogsBtn = document.getElementById('clear-logs');
    const wsIndicator = document.getElementById('ws-indicator');
    const wsStatus = document.getElementById('ws-status');
    
    // State
    let currentChapter = null;
    let currentChapterData = null;
    let showingTranslated = false;
    let currentMode = 'original'; // 'original', 'translated', 'side-by-side'
    let websocket = null;
    let isTranslating = false;
    
    // Fix image and media paths in HTML content for display
    function fixImagePaths(htmlContent) {
        if (!htmlContent) return htmlContent;
        
        const tempDiv = document.createElement('div');
        tempDiv.innerHTML = htmlContent;
        
        // Fix image sources
        const images = tempDiv.querySelectorAll('img[src]');
        images.forEach(img => {
            const src = img.getAttribute('src');
            // Only fix relative paths, not absolute URLs or already fixed paths
            if (src && !src.startsWith('http') && !src.startsWith('/')) {
                const newSrc = `/epub_files/${epubId}/OEBPS/${src}`;
                img.setAttribute('src', newSrc);
            }
        });
        
        // Fix CSS links
        const links = tempDiv.querySelectorAll('link[href]');
        links.forEach(link => {
            const href = link.getAttribute('href');
            if (href && !href.startsWith('http') && !href.startsWith('/')) {
                const newHref = `/epub_files/${epubId}/OEBPS/${href}`;
                link.setAttribute('href', newHref);
            }
        });
        
        // Fix audio/video sources
        const mediaSources = tempDiv.querySelectorAll('audio[src], video[src], source[src]');
        mediaSources.forEach(media => {
            const src = media.getAttribute('src');
            if (src && !src.startsWith('http') && !src.startsWith('/')) {
                const newSrc = `/epub_files/${epubId}/OEBPS/${src}`;
                media.setAttribute('src', newSrc);
            }
        });
        
        return tempDiv.innerHTML;
    }
    
    // Initialize WebSocket connection
    initWebSocket();
    
    // Event listeners
    chapterItems.forEach(item => {
        item.addEventListener('click', () => loadChapter(item.dataset.chapterId));
    });
    
    translatePageBtn.addEventListener('click', translateCurrentPage);
    toggleViewBtn.addEventListener('click', () => setViewMode('translated'));
    sideBySideBtn.addEventListener('click', () => setViewMode('side-by-side'));
    toggleLogsBtn.addEventListener('click', toggleLogsPanel);
    clearLogsBtn.addEventListener('click', clearLogs);
    
    targetLanguageSelect.addEventListener('change', function() {
        updateTranslateButtonState();
    });
    
    function initWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        try {
            websocket = new WebSocket(wsUrl);
            
            websocket.onopen = function() {
                updateConnectionStatus(true);
                addLog('info', 'WebSocket connected successfully', 'websocket');
            };
            
            websocket.onmessage = function(event) {
                try {
                    const message = JSON.parse(event.data);
                    handleWebSocketMessage(message);
                } catch (error) {
                    console.error('Error parsing WebSocket message:', error);
                    addLog('error', `Failed to parse WebSocket message: ${error.message}`, 'websocket');
                }
            };
            
            websocket.onclose = function() {
                updateConnectionStatus(false);
                addLog('warn', 'WebSocket connection closed', 'websocket');
                
                // Attempt to reconnect after 5 seconds
                setTimeout(() => {
                    addLog('info', 'Attempting to reconnect...', 'websocket');
                    initWebSocket();
                }, 5000);
            };
            
            websocket.onerror = function(error) {
                updateConnectionStatus(false);
                addLog('error', 'WebSocket error occurred', 'websocket');
                console.error('WebSocket error:', error);
            };
            
        } catch (error) {
            updateConnectionStatus(false);
            addLog('error', `Failed to initialize WebSocket: ${error.message}`, 'websocket');
        }
    }
    
    function handleWebSocketMessage(message) {
        switch (message.type) {
            case 'log':
                addLog(message.data.level, message.data.message, message.data.module || 'system');
                break;
                
            case 'page_translation':
                handlePageTranslationResult(message.data);
                break;
                
            case 'translation_progress':
                handleTranslationProgress(message.data);
                break;
                
            case 'translation_complete':
                handleTranslationComplete(message.data);
                break;
                
            case 'translation_error':
                handleTranslationError(message.data);
                break;
                
            default:
                addLog('debug', `Unknown message type: ${message.type}`, 'websocket');
        }
    }
    
    function updateConnectionStatus(connected) {
        if (connected) {
            wsIndicator.className = 'w-2 h-2 bg-green-500 rounded-full';
            wsStatus.textContent = 'Connected';
        } else {
            wsIndicator.className = 'w-2 h-2 bg-red-500 rounded-full';
            wsStatus.textContent = 'Disconnected';
        }
    }
    
    function addLog(level, message, module) {
        const logEntry = document.createElement('div');
        logEntry.className = `log-entry log-${level}`;
        
        const timestamp = new Date().toLocaleTimeString();
        const moduleText = module ? `[${module}] ` : '';
        logEntry.textContent = `${timestamp} ${moduleText}${message}`;
        
        logsContainer.appendChild(logEntry);
        logsContainer.scrollTop = logsContainer.scrollHeight;
        
        // Keep only the last 100 log entries
        const logs = logsContainer.children;
        if (logs.length > 100) {
            logsContainer.removeChild(logs[0]);
        }
    }
    
    function toggleLogsPanel() {
        logsPanel.classList.toggle('hidden');
        
        // Get the main grid container
        const mainGrid = document.getElementById('main-grid');
        
        if (!logsPanel.classList.contains('hidden')) {
            // Logs panel is now visible
            mainGrid.classList.remove('logs-hidden');
            mainGrid.classList.add('logs-visible');
            
            // Scroll to bottom when opening
            setTimeout(() => {
                logsContainer.scrollTop = logsContainer.scrollHeight;
            }, 100);
        } else {
            // Logs panel is now hidden
            mainGrid.classList.remove('logs-visible');
            mainGrid.classList.add('logs-hidden');
        }
    }
    
    function clearLogs() {
        logsContainer.innerHTML = '<div class="log-entry log-info">Logs cleared</div>';
    }
    
    async function loadChapter(chapterId) {
        try {
            addLog('info', `Loading chapter: ${chapterId}`, 'reader');
            
            const response = await fetch(`/api/chapter/${epubId}/${chapterId}`);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            
            const chapterData = await response.json();
            
            currentChapter = chapterId;
            currentChapterData = chapterData;
            showingTranslated = false;
            
            // Update UI
            currentChapterTitle.textContent = chapterData.title;
            chapterControls.classList.remove('hidden');
            
            // Update sidebar selection
            chapterItems.forEach(item => {
                if (item.dataset.chapterId === chapterId) {
                    item.classList.add('bg-blue-50', 'border-l-4', 'border-blue-500');
                } else {
                    item.classList.remove('bg-blue-50', 'border-l-4', 'border-blue-500');
                }
            });
            
            // Update button states
            updateTranslateButtonState();
            updateViewButtons();
            
            // Render current mode
            renderCurrentMode();
            updateURL();
            
            translationStatus.textContent = chapterData.is_translated ? 'Translated' : 'Original';
            
            addLog('info', `Chapter loaded successfully: ${chapterData.title}`, 'reader');
            
        } catch (error) {
            console.error('Error loading chapter:', error);
            addLog('error', `Failed to load chapter: ${error.message}`, 'reader');
            
            chapterContent.innerHTML = `<p class="text-red-500">Error loading chapter: ${error.message}</p>`;
        }
    }
    
    function updateTranslateButtonState() {
        const hasTargetLang = targetLanguageSelect.value !== '';
        const hasCurrentChapter = currentChapter !== null;
        
        translatePageBtn.disabled = !hasTargetLang || !hasCurrentChapter || isTranslating;
        
        if (!hasTargetLang) {
            translatePageBtn.textContent = 'Select Language';
        } else if (isTranslating) {
            translatePageBtn.textContent = 'Translating...';
        } else {
            translatePageBtn.textContent = 'Translate Page';
        }
    }
    
    function updateToggleViewButton() {
        if (!currentChapterData) return;
        
        if (currentChapterData.is_translated) {
            toggleViewBtn.disabled = false;
            toggleViewBtn.textContent = showingTranslated ? 'Show Original' : 'Show Translated';
        } else {
            toggleViewBtn.disabled = true;
            toggleViewBtn.textContent = 'No Translation';
        }
    }
    
    async function translateCurrentPage() {
        if (!currentChapter || !targetLanguageSelect.value || isTranslating) return;
        
        const targetLang = targetLanguageSelect.value;
        const sourceLang = '{{.Language}}'; // This will be replaced by the template
        
        isTranslating = true;
        updateTranslateButtonState();
        
        // Show progress modal
        sourceLangDisplay.textContent = sourceLang;
        targetLangDisplay.textContent = targetLang;
        translationModal.classList.remove('hidden');
        
        // Add translation overlay to content
        chapterContent.classList.add('translation-overlay', 'translating');
        
        addLog('info', `Starting page translation: ${sourceLang} → ${targetLang}`, 'translation');
        
        try {
            const response = await fetch('/api/translate-page', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    epub_id: epubId,
                    chapter_id: currentChapter,
                    content: currentChapterData.content,
                    target_lang: targetLang,
                    source_lang: sourceLang
                })
            });
            
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            
            const result = await response.json();
            addLog('info', 'Page translation request sent successfully', 'translation');
            
        } catch (error) {
            console.error('Error starting translation:', error);
            addLog('error', `Failed to start translation: ${error.message}`, 'translation');
            
            // Hide modal and reset state
            translationModal.classList.add('hidden');
            chapterContent.classList.remove('translation-overlay', 'translating');
            isTranslating = false;
            updateTranslateButtonState();
        }
    }
    
    function handlePageTranslationResult(data) {
        if (data.epub_id === epubId && data.chapter_id === currentChapter) {
            // Update the current chapter data with translation
            if (currentChapterData) {
                currentChapterData.translated_content = data.translated_text;
                currentChapterData.is_translated = true;
                
                // If we're showing the translated view, update the content
                if (showingTranslated) {
                    chapterContent.innerHTML = data.translated_text;
                }
                
                updateToggleViewButton();
                translationStatus.textContent = 'Translated';
                
                addLog('info', 'Page translation completed successfully', 'translation');
            }
        }
        
        // Hide modal and reset state
        translationModal.classList.add('hidden');
        chapterContent.classList.remove('translation-overlay', 'translating');
        isTranslating = false;
        updateTranslateButtonState();
    }
    
    function handleTranslationProgress(data) {
        // Update any global translation progress if needed
        addLog('info', `Translation progress: ${data.progress_percent}% - ${data.current_chapter}`, 'translation');
    }
    
    function handleTranslationComplete(data) {
        addLog('info', 'Full translation completed!', 'translation');
        
        // Reload chapter data if it's the current one
        if (currentChapter) {
            loadChapter(currentChapter);
        }
    }
    
    function handleTranslationError(data) {
        addLog('error', `Translation error: ${data.error || 'Unknown error'}`, 'translation');
        
        // Hide modal and reset state if it's for current page
        if (isTranslating) {
            translationModal.classList.add('hidden');
            chapterContent.classList.remove('translation-overlay', 'translating');
            isTranslating = false;
            updateTranslateButtonState();
        }
    }
    
    // URL management functions
    function updateURL() {
        if (!currentChapter) return;
        
        const newPath = `/reader/${epubId}/${currentChapter}/${currentMode}`;
        window.history.pushState({ 
            chapter: currentChapter, 
            mode: currentMode 
        }, '', newPath);
    }
    
    function setViewMode(mode) {
        if (!currentChapterData) return;
        
        // Validate mode
        const validModes = ['original', 'translated', 'side-by-side'];
        if (!validModes.includes(mode)) {
            mode = 'original';
        }
        
        // Check if translation is available for translated/side-by-side modes
        if ((mode === 'translated' || mode === 'side-by-side') && !currentChapterData.is_translated) {
            addLog('warn', 'No translation available for this chapter', 'reader');
            return;
        }
        
        currentMode = mode;
        updateURL();
        renderCurrentMode();
        updateViewButtons();
        
        addLog('info', `Switched to ${mode} view`, 'reader');
    }
    
    function renderCurrentMode() {
        if (!currentChapterData) return;
        
        switch (currentMode) {
            case 'original':
                renderOriginalView();
                break;
            case 'translated':
                renderTranslatedView();
                break;
            case 'side-by-side':
                renderSideBySideView();
                break;
        }
    }
    
    function renderOriginalView() {
        const content = currentChapterData.content || '<p class="text-gray-500">No content available</p>';
        chapterContent.innerHTML = fixImagePaths(content);
        chapterContent.classList.remove('rtl');
        chapterContent.className = 'chapter-content prose max-w-none';
    }
    
    function renderTranslatedView() {
        if (currentChapterData.translated_content) {
            chapterContent.innerHTML = currentChapterData.translated_content;
            chapterContent.classList.add('rtl');
        } else {
            chapterContent.innerHTML = '<p class="text-gray-500">No translation available</p>';
        }
        chapterContent.className = 'chapter-content prose max-w-none';
    }
    
    function renderSideBySideView() {
        const originalContent = currentChapterData.content || '<p class="text-gray-500">No content available</p>';
        const translatedContent = currentChapterData.translated_content || '<p class="text-gray-500">No translation available</p>';
        
        chapterContent.className = 'side-by-side-container';
        chapterContent.innerHTML = `
            <div class="side-by-side-column">
                <div class="side-by-side-header">Original</div>
                <div class="side-by-side-content ltr">${fixImagePaths(originalContent)}</div>
            </div>
            <div class="side-by-side-column">
                <div class="side-by-side-header">Translation</div>
                <div class="side-by-side-content rtl">${translatedContent}</div>
            </div>
        `;
    }
    
    function updateViewButtons() {
        if (!currentChapterData) return;
        
        // Update button states based on current mode
        toggleViewBtn.classList.remove('bg-blue-500', 'bg-gray-500');
        sideBySideBtn.classList.remove('bg-purple-500', 'bg-gray-500');
        
        if (currentMode === 'translated') {
            toggleViewBtn.classList.add('bg-blue-500');
            toggleViewBtn.textContent = 'Show Original';
        } else {
            toggleViewBtn.classList.add('bg-gray-500');
            toggleViewBtn.textContent = 'Show Translated';
        }
        
        if (currentMode === 'side-by-side') {
            sideBySideBtn.classList.add('bg-purple-500');
            sideBySideBtn.textContent = 'Single View';
        } else {
            sideBySideBtn.classList.add('bg-purple-500');
            sideBySideBtn.textContent = 'Side by Side';
        }
        
        // Enable/disable buttons based on translation availability
        const hasTranslation = currentChapterData.is_translated;
        toggleViewBtn.disabled = !hasTranslation;
        sideBySideBtn.disabled = !hasTranslation;
        
        if (!hasTranslation) {
            toggleViewBtn.textContent = 'No Translation';
            sideBySideBtn.textContent = 'No Translation';
        }
    }
    
    function toggleTranslatedView() {
        // This function is kept for backward compatibility
        if (currentMode === 'translated') {
            setViewMode('original');
        } else {
            setViewMode('translated');
        }
    }
    
    // Keyboard shortcuts
    document.addEventListener('keydown', function(e) {
        // ESC to close modal
        if (e.key === 'Escape' && !translationModal.classList.contains('hidden')) {
            translationModal.classList.add('hidden');
            chapterContent.classList.remove('translation-overlay', 'translating');
            isTranslating = false;
            updateTranslateButtonState();
        }
        
        // Ctrl+L to toggle logs
        if (e.ctrlKey && e.key === 'l') {
            e.preventDefault();
            toggleLogsPanel();
        }
        
        // Ctrl+T to translate (if conditions are met)
        if (e.ctrlKey && e.key === 't') {
            e.preventDefault();
            if (!translatePageBtn.disabled) {
                translateCurrentPage();
            }
        }
        
        // Ctrl+Shift+T to toggle view
        if (e.ctrlKey && e.shiftKey && e.key === 'T') {
            e.preventDefault();
            if (!toggleViewBtn.disabled) {
                toggleTranslatedView();
            }
        }
    });
    
    // Initialize from URL parameters
    function initializeFromURL() {
        if (initialChapter && initialChapter !== '') {
            currentMode = initialMode || 'original';
            loadChapter(initialChapter);
            addLog('info', `Initialized with chapter: ${initialChapter}, mode: ${currentMode}`, 'reader');
        } else if (chapterItems.length > 0) {
            // Load first chapter if no URL parameters
            const firstChapter = chapterItems[0].dataset.chapterId;
            loadChapter(firstChapter);
        }
    }
    
    // Handle browser back/forward navigation
    window.addEventListener('popstate', function(event) {
        if (event.state && event.state.chapter && event.state.mode) {
            currentMode = event.state.mode;
            loadChapter(event.state.chapter);
            addLog('info', `Navigated to chapter: ${event.state.chapter}, mode: ${event.state.mode}`, 'reader');
        }
    });
    
    // Initialize with URL parameters or first chapter
    initializeFromURL();
    
    if (chapterItems.length > 0) {
        addLog('info', 'Reader initialized successfully', 'system');
        addLog('info', 'Keyboard shortcuts: Ctrl+L (logs), Ctrl+T (translate), Ctrl+Shift+T (toggle view)', 'system');
    }
});