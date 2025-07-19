// Reader page functionality with WebSocket support
document.addEventListener('DOMContentLoaded', function() {
    // DOM elements
    const chapterItems = document.querySelectorAll('.chapter-item');
    const currentChapterTitle = document.getElementById('current-chapter-title');
    const chapterContent = document.getElementById('chapter-content');
    const chapterControls = document.getElementById('chapter-controls');
    const translatePageBtn = document.getElementById('translate-page-btn');
    const toggleViewBtn = document.getElementById('toggle-view-btn');
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
    let websocket = null;
    let isTranslating = false;
    
    // Initialize WebSocket connection
    initWebSocket();
    
    // Event listeners
    chapterItems.forEach(item => {
        item.addEventListener('click', () => loadChapter(item.dataset.chapterId));
    });
    
    translatePageBtn.addEventListener('click', translateCurrentPage);
    toggleViewBtn.addEventListener('click', toggleTranslatedView);
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
        
        if (!logsPanel.classList.contains('hidden')) {
            // Scroll to bottom when opening
            setTimeout(() => {
                logsContainer.scrollTop = logsContainer.scrollHeight;
            }, 100);
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
            chapterContent.innerHTML = chapterData.content || '<p class="text-gray-500">No content available</p>';
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
            updateToggleViewButton();
            
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
    
    function toggleTranslatedView() {
        if (!currentChapterData || !currentChapterData.is_translated) return;
        
        showingTranslated = !showingTranslated;
        
        if (showingTranslated && currentChapterData.translated_content) {
            chapterContent.innerHTML = currentChapterData.translated_content;
            chapterContent.classList.add('rtl'); // Assuming translation might be RTL
        } else {
            chapterContent.innerHTML = currentChapterData.content;
            chapterContent.classList.remove('rtl');
        }
        
        updateToggleViewButton();
        
        const viewType = showingTranslated ? 'translated' : 'original';
        addLog('info', `Switched to ${viewType} view`, 'reader');
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
    
    // Initialize with first chapter if available
    if (chapterItems.length > 0) {
        addLog('info', 'Reader initialized successfully', 'system');
        addLog('info', 'Keyboard shortcuts: Ctrl+L (logs), Ctrl+T (translate), Ctrl+Shift+T (toggle view)', 'system');
    }
});