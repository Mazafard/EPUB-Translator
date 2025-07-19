// Enhanced preview page functionality with all chapters display
document.addEventListener('DOMContentLoaded', function() {
    // DOM elements
    const targetLanguageSelect = document.getElementById('target-language');
    const startTranslationBtn = document.getElementById('start-translation');
    const translationProgress = document.getElementById('translation-progress');
    const translationComplete = document.getElementById('translation-complete');
    const translationError = document.getElementById('translation-error');
    const progressBar = document.getElementById('translation-progress-bar');
    const progressText = document.getElementById('progress-text');
    const currentChapter = document.getElementById('current-chapter');
    const chaptersCompleted = document.getElementById('chapters-completed');
    const downloadBtn = document.getElementById('download-btn');
    const errorDetails = document.getElementById('error-details');
    
    // New elements for enhanced preview
    const chapterNavItems = document.querySelectorAll('.chapter-nav-item');
    const showAllBtn = document.getElementById('show-all-btn');
    const showTranslatedBtn = document.getElementById('show-translated-btn');
    const currentViewTitle = document.getElementById('current-view-title');
    const viewStatus = document.getElementById('view-status');
    const toggleViewBtn = document.getElementById('toggle-view-btn');
    const allContent = document.getElementById('all-content');
    const singleChapterContent = document.getElementById('single-chapter-content');
    
    // WebSocket and logs elements
    const logsContainer = document.getElementById('logs-container');
    const wsIndicator = document.getElementById('ws-indicator');
    const wsStatus = document.getElementById('ws-status');
    const clearLogsBtn = document.getElementById('clear-logs');
    
    // Single document translation elements
    const translateSingleBtn = document.getElementById('translate-single-btn');
    
    // State
    let translationPolling = null;
    let chaptersData = [];
    let currentViewMode = 'all'; // 'all', 'single', 'translated-only'
    let showingTranslated = false;
    let websocket = null;
    let reconnectAttempts = 0;
    const maxReconnectAttempts = 5;
    
    // Initialize the page
    initializePage();
    
    // Initialize WebSocket connection
    initializeWebSocket();
    
    // Event listeners
    targetLanguageSelect.addEventListener('change', function() {
        startTranslationBtn.disabled = !this.value;
    });
    
    startTranslationBtn.addEventListener('click', function() {
        const targetLang = targetLanguageSelect.value;
        if (!targetLang) return;
        startTranslation(targetLang);
    });
    
    downloadBtn.addEventListener('click', function() {
        window.location.href = `/download/${epubId}`;
    });
    
    // Chapter navigation
    chapterNavItems.forEach(item => {
        item.addEventListener('click', function() {
            const chapterId = this.dataset.chapterId;
            loadSingleChapter(chapterId);
        });
    });
    
    // View mode buttons
    showAllBtn.addEventListener('click', function() {
        showAllChapters();
    });
    
    showTranslatedBtn.addEventListener('click', function() {
        showTranslatedChapters();
    });
    
    toggleViewBtn.addEventListener('click', function() {
        toggleTranslatedView();
    });
    
    // WebSocket and single translation event listeners
    clearLogsBtn.addEventListener('click', function() {
        clearLogs();
    });
    
    translateSingleBtn.addEventListener('click', function() {
        translateCurrentPage();
    });
    
    // Update button state when target language changes
    targetLanguageSelect.addEventListener('change', function() {
        updateTranslateSingleButtonState();
    });
    
    async function initializePage() {
        try {
            // Load all chapters data
            await loadAllChapters();
            
            // Show all content by default
            showAllChapters();
            
            // Check for ongoing translation
            checkOngoingTranslation();
            
            // Initialize button states
            updateTranslateSingleButtonState();
            
        } catch (error) {
            console.error('Error initializing page:', error);
            allContent.innerHTML = '<p class="text-red-500 text-center py-12">Error loading content</p>';
        }
    }
    
    async function loadAllChapters() {
        try {
            const response = await fetch(`/api/chapters/${epubId}?limit=50`);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            
            const data = await response.json();
            chaptersData = data.chapters || [];
            
            // Load detailed content for each chapter
            const detailedChapters = await Promise.all(
                chaptersData.map(async (chapter) => {
                    try {
                        const chapterResponse = await fetch(`/api/chapter/${epubId}/${chapter.id}`);
                        if (chapterResponse.ok) {
                            const chapterData = await chapterResponse.json();
                            return {
                                ...chapter,
                                content: chapterData.content,
                                translated_content: chapterData.translated_content
                            };
                        }
                        return chapter;
                    } catch (error) {
                        console.warn(`Failed to load chapter ${chapter.id}:`, error);
                        return chapter;
                    }
                })
            );
            
            chaptersData = detailedChapters;
            
        } catch (error) {
            console.error('Error loading chapters:', error);
            throw error;
        }
    }
    
    function showAllChapters() {
        currentViewMode = 'all';
        singleChapterContent.classList.add('hidden');
        allContent.classList.remove('hidden');
        
        // Update UI
        currentViewTitle.textContent = `All Chapters - ${document.querySelector('h1').textContent}`;
        viewStatus.textContent = showingTranslated ? 'Showing all chapters (translated)' : 'Showing all chapters (original)';
        
        // Clear active states
        chapterNavItems.forEach(item => {
            item.classList.remove('active');
        });
        
        // Update button states
        showAllBtn.classList.remove('bg-gray-500', 'hover:bg-gray-600');
        showAllBtn.classList.add('bg-blue-500', 'hover:bg-blue-600');
        showTranslatedBtn.classList.remove('bg-blue-500', 'hover:bg-blue-600');
        showTranslatedBtn.classList.add('bg-gray-500', 'hover:bg-gray-600');
        
        // Render all chapters
        renderAllChapters(chaptersData);
        
        // Update translate button state
        updateTranslateSingleButtonState();
    }
    
    function showTranslatedChapters() {
        currentViewMode = 'translated-only';
        singleChapterContent.classList.add('hidden');
        allContent.classList.remove('hidden');
        
        // Filter translated chapters
        const translatedChapters = chaptersData.filter(chapter => chapter.is_translated);
        
        // Update UI
        currentViewTitle.textContent = `Translated Chapters - ${document.querySelector('h1').textContent}`;
        viewStatus.textContent = `Showing ${translatedChapters.length} translated chapters`;
        
        // Clear active states
        chapterNavItems.forEach(item => {
            item.classList.remove('active');
        });
        
        // Update button states
        showTranslatedBtn.classList.remove('bg-gray-500', 'hover:bg-gray-600');
        showTranslatedBtn.classList.add('bg-blue-500', 'hover:bg-blue-600');
        showAllBtn.classList.remove('bg-blue-500', 'hover:bg-blue-600');
        showAllBtn.classList.add('bg-gray-500', 'hover:bg-gray-600');
        
        // Render translated chapters
        if (translatedChapters.length > 0) {
            renderAllChapters(translatedChapters);
        } else {
            allContent.innerHTML = '<p class="text-gray-500 text-center py-12">No translated chapters available yet</p>';
        }
        
        // Update translate button state
        updateTranslateSingleButtonState();
    }
    
    async function loadSingleChapter(chapterId) {
        try {
            currentViewMode = 'single';
            allContent.classList.add('hidden');
            singleChapterContent.classList.remove('hidden');
            
            // Find chapter data
            const chapter = chaptersData.find(ch => ch.id === chapterId);
            if (!chapter) {
                throw new Error('Chapter not found');
            }
            
            // Update UI
            currentViewTitle.textContent = chapter.title;
            viewStatus.textContent = chapter.is_translated ? 'Chapter has translation' : 'Original chapter only';
            
            // Update navigation
            chapterNavItems.forEach(item => {
                if (item.dataset.chapterId === chapterId) {
                    item.classList.add('active');
                } else {
                    item.classList.remove('active');
                }
            });
            
            // Update button states
            showAllBtn.classList.remove('bg-blue-500', 'hover:bg-blue-600');
            showAllBtn.classList.add('bg-gray-500', 'hover:bg-gray-600');
            showTranslatedBtn.classList.remove('bg-blue-500', 'hover:bg-blue-600');
            showTranslatedBtn.classList.add('bg-gray-500', 'hover:bg-gray-600');
            
            // Render single chapter
            renderSingleChapter(chapter);
            
            // Update translate button state
            updateTranslateSingleButtonState();
            
        } catch (error) {
            console.error('Error loading single chapter:', error);
            singleChapterContent.innerHTML = `<p class="text-red-500 text-center py-12">Error loading chapter: ${error.message}</p>`;
        }
    }
    
    function renderAllChapters(chapters) {
        if (!chapters || chapters.length === 0) {
            allContent.innerHTML = '<p class="text-gray-500 text-center py-12">No chapters to display</p>';
            return;
        }
        
        const chaptersHtml = chapters.map(chapter => {
            const content = showingTranslated && chapter.translated_content ? 
                            chapter.translated_content : 
                            (chapter.content || '<p class="text-gray-500">Content not available</p>');
            
            const statusBadge = chapter.is_translated ? 
                '<span class="inline-flex items-center px-2 py-1 rounded-full text-xs bg-green-100 text-green-800 mb-2">Translated</span>' :
                '<span class="inline-flex items-center px-2 py-1 rounded-full text-xs bg-gray-100 text-gray-600 mb-2">Original</span>';
            
            return `
                <div class="chapter-content" id="chapter-${chapter.id}">
                    <div class="chapter-title">
                        ${chapter.title}
                        ${statusBadge}
                    </div>
                    <div class="prose max-w-none">
                        ${content}
                    </div>
                </div>
            `;
        }).join('');
        
        allContent.innerHTML = chaptersHtml;
        
        // Update toggle button
        updateToggleViewButton();
    }
    
    function renderSingleChapter(chapter) {
        const content = showingTranslated && chapter.translated_content ? 
                        chapter.translated_content : 
                        (chapter.content || '<p class="text-gray-500">Content not available</p>');
        
        const statusBadge = chapter.is_translated ? 
            '<span class="inline-flex items-center px-2 py-1 rounded-full text-xs bg-green-100 text-green-800 mb-2">Translated</span>' :
            '<span class="inline-flex items-center px-2 py-1 rounded-full text-xs bg-gray-100 text-gray-600 mb-2">Original</span>';
        
        singleChapterContent.innerHTML = `
            <div class="chapter-content">
                <div class="chapter-title">
                    ${chapter.title}
                    ${statusBadge}
                </div>
                <div class="prose max-w-none">
                    ${content}
                </div>
            </div>
        `;
        
        // Update toggle button
        updateToggleViewButton();
    }
    
    function updateToggleViewButton() {
        const hasTranslations = chaptersData.some(ch => ch.is_translated);
        
        if (hasTranslations) {
            toggleViewBtn.disabled = false;
            toggleViewBtn.textContent = showingTranslated ? 'Show Original' : 'Show Translated';
        } else {
            toggleViewBtn.disabled = true;
            toggleViewBtn.textContent = 'No Translations';
        }
    }
    
    function toggleTranslatedView() {
        showingTranslated = !showingTranslated;
        
        // Re-render current view
        if (currentViewMode === 'all') {
            showAllChapters();
        } else if (currentViewMode === 'translated-only') {
            showTranslatedChapters();
        } else if (currentViewMode === 'single') {
            const activeItem = document.querySelector('.chapter-nav-item.active');
            if (activeItem) {
                loadSingleChapter(activeItem.dataset.chapterId);
            }
        }
    }
    
    function startTranslation(targetLang) {
        fetch('/translate', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                id: epubId,
                target_lang: targetLang
            })
        })
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                throw new Error(data.error);
            }

            // Hide translation settings and show progress
            startTranslationBtn.style.display = 'none';
            targetLanguageSelect.disabled = true;
            translationProgress.classList.remove('hidden');

            // Start polling for progress
            pollTranslationStatus();
        })
        .catch(error => {
            showTranslationError(error.message || 'Failed to start translation');
        });
    }

    function pollTranslationStatus() {
        if (translationPolling) {
            clearInterval(translationPolling);
        }

        translationPolling = setInterval(() => {
            fetch(`/status/${epubId}`)
                .then(response => response.json())
                .then(data => {
                    updateProgress(data);

                    if (data.status === 'completed') {
                        clearInterval(translationPolling);
                        showTranslationComplete();
                    } else if (data.status === 'failed') {
                        clearInterval(translationPolling);
                        showTranslationError(data.error_message || 'Translation failed');
                    }
                })
                .catch(error => {
                    console.error('Error polling status:', error);
                });
        }, 2000);
    }

    function updateProgress(data) {
        const percentage = data.progress_percentage || 0;
        progressBar.style.width = percentage + '%';
        progressText.textContent = Math.round(percentage) + '%';
        
        if (data.current_chapter) {
            currentChapter.textContent = `Translating: ${data.current_chapter}`;
        }
        
        chaptersCompleted.textContent = data.completed_chapters || 0;
    }

    function showTranslationComplete() {
        translationProgress.classList.add('hidden');
        translationComplete.classList.remove('hidden');
        
        // Reload chapters data to get translations
        setTimeout(async () => {
            await loadAllChapters();
            
            // Refresh current view
            if (currentViewMode === 'all') {
                showAllChapters();
            } else if (currentViewMode === 'translated-only') {
                showTranslatedChapters();
            } else if (currentViewMode === 'single') {
                const activeItem = document.querySelector('.chapter-nav-item.active');
                if (activeItem) {
                    loadSingleChapter(activeItem.dataset.chapterId);
                }
            }
            
            // Update chapter nav items
            updateChapterNavigation();
        }, 2000);
    }

    function showTranslationError(message) {
        translationProgress.classList.add('hidden');
        errorDetails.textContent = message;
        translationError.classList.remove('hidden');
        
        // Re-enable translation button
        startTranslationBtn.style.display = 'block';
        targetLanguageSelect.disabled = false;
    }
    
    function updateChapterNavigation() {
        chapterNavItems.forEach((item, index) => {
            const chapter = chaptersData[index];
            if (chapter) {
                const statusIndicator = item.querySelector('.inline-flex');
                if (statusIndicator) {
                    if (chapter.is_translated) {
                        statusIndicator.className = 'inline-flex items-center px-1.5 py-0.5 rounded-full text-xs bg-green-100 text-green-800';
                        statusIndicator.textContent = '✓';
                    } else {
                        statusIndicator.className = 'inline-flex items-center px-1.5 py-0.5 rounded-full text-xs bg-gray-100 text-gray-600';
                        statusIndicator.textContent = '○';
                    }
                }
            }
        });
    }

    function checkOngoingTranslation() {
        fetch(`/status/${epubId}`)
            .then(response => response.json())
            .then(data => {
                if (data.status === 'in_progress') {
                    // Resume showing progress
                    startTranslationBtn.style.display = 'none';
                    targetLanguageSelect.disabled = true;
                    translationProgress.classList.remove('hidden');
                    pollTranslationStatus();
                } else if (data.status === 'completed') {
                    showTranslationComplete();
                } else if (data.status === 'failed') {
                    showTranslationError(data.error_message || 'Translation failed');
                }
            })
            .catch(error => {
                // No existing translation, normal state
                console.log('No ongoing translation');
            });
    }
    
    // WebSocket functionality
    function initializeWebSocket() {
        connectWebSocket();
    }
    
    function connectWebSocket() {
        try {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = `${protocol}//${window.location.host}/ws`;
            
            websocket = new WebSocket(wsUrl);
            
            websocket.onopen = function() {
                addLog('info', 'WebSocket connected successfully');
                updateConnectionStatus(true);
                reconnectAttempts = 0;
            };
            
            websocket.onmessage = function(event) {
                try {
                    const message = JSON.parse(event.data);
                    handleWebSocketMessage(message);
                } catch (error) {
                    console.error('Error parsing WebSocket message:', error);
                    addLog('error', 'Failed to parse WebSocket message');
                }
            };
            
            websocket.onclose = function() {
                addLog('warn', 'WebSocket connection closed');
                updateConnectionStatus(false);
                
                // Attempt to reconnect
                if (reconnectAttempts < maxReconnectAttempts) {
                    reconnectAttempts++;
                    addLog('info', `Attempting to reconnect... (${reconnectAttempts}/${maxReconnectAttempts})`);
                    setTimeout(connectWebSocket, 3000 * reconnectAttempts);
                }
            };
            
            websocket.onerror = function(error) {
                console.error('WebSocket error:', error);
                addLog('error', 'WebSocket connection error');
                updateConnectionStatus(false);
            };
            
        } catch (error) {
            console.error('Failed to create WebSocket connection:', error);
            addLog('error', 'Failed to create WebSocket connection');
            updateConnectionStatus(false);
        }
    }
    
    function handleWebSocketMessage(message) {
        switch (message.type) {
            case 'log':
                addLog(message.level || 'info', message.message, message.category);
                break;
            case 'translation_progress':
                updateProgress(message.data);
                break;
            case 'page_translation':
                handlePageTranslation(message.data);
                break;
            case 'llm_request':
                addLog('debug', `LLM Request: ${JSON.stringify(message.data, null, 2)}`);
                break;
            case 'llm_response':
                addLog('debug', `LLM Response: ${JSON.stringify(message.data, null, 2)}`);
                break;
            default:
                addLog('debug', `Unknown message type: ${message.type}`);
        }
    }
    
    function addLog(level, message, category) {
        const timestamp = new Date().toLocaleTimeString();
        const logEntry = document.createElement('div');
        logEntry.className = `log-entry log-${level}`;
        
        const categoryText = category ? `[${category}] ` : '';
        logEntry.textContent = `${timestamp} ${categoryText}${message}`;
        
        logsContainer.appendChild(logEntry);
        
        // Auto-scroll to bottom
        logsContainer.scrollTop = logsContainer.scrollHeight;
        
        // Limit log entries to prevent memory issues
        const maxLogEntries = 1000;
        while (logsContainer.children.length > maxLogEntries) {
            logsContainer.removeChild(logsContainer.firstChild);
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
    
    function clearLogs() {
        logsContainer.innerHTML = '<div class="log-entry log-info">Logs cleared</div>';
    }
    
    function handlePageTranslation(data) {
        // Handle both camelCase and snake_case field names for compatibility
        const chapterId = data.chapter_id || data.ChapterID;
        const translatedText = data.translated_text || data.TranslatedText;
        
        addLog('info', `Page translation completed for chapter: ${chapterId}`);
        
        // Update the chapter data
        const chapterIndex = chaptersData.findIndex(ch => ch.id === chapterId);
        if (chapterIndex !== -1) {
            chaptersData[chapterIndex].translated_content = translatedText;
            chaptersData[chapterIndex].is_translated = true;
            
            // Refresh current view if needed
            if (currentViewMode === 'single') {
                const activeItem = document.querySelector('.chapter-nav-item.active');
                if (activeItem && activeItem.dataset.chapterId === chapterId) {
                    loadSingleChapter(chapterId);
                }
            }
            
            updateChapterNavigation();
        }
    }
    
    // Single page translation functionality
    function updateTranslateSingleButtonState() {
        const targetLang = targetLanguageSelect.value;
        const hasCurrentPage = (currentViewMode === 'single' && document.querySelector('.chapter-nav-item.active')) || 
                               (currentViewMode === 'all' || currentViewMode === 'translated-only');
        
        translateSingleBtn.disabled = !targetLang || !hasCurrentPage;
        
        if (!targetLang) {
            translateSingleBtn.title = 'Please select a target language first';
        } else if (!hasCurrentPage) {
            translateSingleBtn.title = 'No page available to translate';
        } else {
            translateSingleBtn.title = currentViewMode === 'single' ? 
                'Translate current chapter' : 
                'Translate first visible chapter';
        }
    }
    
    async function translateCurrentPage() {
        const targetLang = targetLanguageSelect.value;
        
        if (!targetLang) {
            addLog('warn', 'Please select a target language first');
            return;
        }
        
        try {
            let chapterToTranslate;
            
            if (currentViewMode === 'single') {
                // Get the currently active chapter
                const activeItem = document.querySelector('.chapter-nav-item.active');
                if (!activeItem) {
                    throw new Error('No active chapter found');
                }
                const chapterId = activeItem.dataset.chapterId;
                chapterToTranslate = chaptersData.find(ch => ch.id === chapterId);
            } else {
                // Get the first visible chapter (all or translated-only mode)
                const visibleChapters = currentViewMode === 'translated-only' ? 
                    chaptersData.filter(ch => ch.is_translated) : 
                    chaptersData;
                    
                if (visibleChapters.length === 0) {
                    throw new Error('No chapters available to translate');
                }
                chapterToTranslate = visibleChapters[0];
            }
            
            if (!chapterToTranslate) {
                throw new Error('Chapter not found');
            }
            
            if (!chapterToTranslate.content) {
                throw new Error('Chapter content not available');
            }
            
            addLog('info', `Starting translation of: ${chapterToTranslate.title}`);
            translateSingleBtn.disabled = true;
            translateSingleBtn.textContent = 'Translating...';
            
            const response = await fetch('/api/translate-page', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    epub_id: epubId,
                    chapter_id: chapterToTranslate.id,
                    content: chapterToTranslate.content,
                    target_lang: targetLang
                })
            });
            
            const result = await response.json();
            
            if (!response.ok) {
                throw new Error(result.error || 'Translation request failed');
            }
            
            addLog('info', `Translation request sent successfully for: ${chapterToTranslate.title}`);
            
        } catch (error) {
            console.error('Error starting translation:', error);
            addLog('error', `Failed to start translation: ${error.message}`);
        } finally {
            translateSingleBtn.disabled = false;
            translateSingleBtn.textContent = 'Translate Current Page';
            updateTranslateSingleButtonState();
        }
    }
    
    // Keyboard shortcuts
    document.addEventListener('keydown', function(e) {
        // 1 key - Show all chapters
        if (e.key === '1') {
            e.preventDefault();
            showAllChapters();
        }
        
        // 2 key - Show translated chapters
        if (e.key === '2') {
            e.preventDefault();
            showTranslatedChapters();
        }
        
        // T key - Toggle view
        if (e.key === 't' || e.key === 'T') {
            e.preventDefault();
            if (!toggleViewBtn.disabled) {
                toggleTranslatedView();
            }
        }
    });
});