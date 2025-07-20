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
    const exportEpubBtn = document.getElementById('export-epub-btn');
    
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
    let currentTargetLanguage = null;
    
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
        const targetLang = currentTargetLanguage || targetLanguageSelect.value;
        if (!targetLang) {
            showTranslationAlert('Cannot determine target language for download.', 'error');
            return;
        }
        window.location.href = `/download-translated/${epubId}/${targetLang}`;
    });

    exportEpubBtn.addEventListener('click', function() {
        // This will download the current state of the processed EPUB, including any single-page translations.
        const targetLang = currentTargetLanguage || targetLanguageSelect.value;
        if (!targetLang) {
            showTranslationAlert('Please select a language. The downloaded file will be configured for this language.', 'warning');
            return;
        }
        addLog('info', `Preparing download for language: ${targetLang}`);
        window.location.href = `/download/processed/${epubId}/${targetLang}`;
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
            
            // Restore state from URL
            restoreStateFromURL();
            
            // Check for ongoing translation
            checkOngoingTranslation();
            
            // Initialize button states
            updateTranslateSingleButtonState();
            
            // Enable start translation button if Persian is selected by default
            startTranslationBtn.disabled = !targetLanguageSelect.value;
            
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
        
        // Apply language-specific styles if showing translated content
        applyLanguageStyles();
        
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
                    // Check if event.data exists and is not empty
                    if (!event.data || event.data.trim() === '') {
                        addLog('warn', 'Received empty WebSocket message');
                        return;
                    }
                    
                    // Handle multiple JSON messages separated by newlines
                    const messages = event.data.trim().split('\n');
                    
                    for (const messageData of messages) {
                        if (messageData.trim() === '') continue;
                        
                        try {
                            const message = JSON.parse(messageData);
                            
                            // Validate message structure
                            if (!message || typeof message !== 'object') {
                                addLog('warn', 'Received invalid WebSocket message format');
                                continue;
                            }
                            
                            handleWebSocketMessage(message);
                        } catch (parseError) {
                            console.error('Error parsing individual WebSocket message:', parseError);
                            addLog('error', `Failed to parse WebSocket message: ${messageData.substring(0, 100)}...`);
                        }
                    }
                } catch (error) {
                    console.error('Error processing WebSocket message:', error);
                    addLog('error', `WebSocket message processing error: ${error.message}`);
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
        // Validate inputs
        if (!level) level = 'info';
        if (message === undefined || message === null) {
            message = '[undefined message]';
        }
        if (typeof message !== 'string') {
            message = String(message);
        }
        
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
        
        if (!chapterId) {
            addLog('warn', 'Received page translation without chapter ID');
            return;
        }
        
        addLog('info', `Page translation completed for chapter: ${chapterId}`);
        
        // Update the chapter data
        const chapterIndex = chaptersData.findIndex(ch => ch.id === chapterId);
        if (chapterIndex !== -1) {
            chaptersData[chapterIndex].translated_content = translatedText;
            chaptersData[chapterIndex].is_translated = true;
            
            // Show success notification
            const chapter = chaptersData[chapterIndex];
            showTranslationAlert(`Translation completed for "${chapter.title}"! Content updated automatically.`, 'success');
            
            // Automatically refresh the current view to show translated content
            refreshCurrentView(chapterId);
            
            // Update chapter navigation indicators
            updateChapterNavigation();
            
            // Update toggle button state
            updateToggleViewButton();
            
            addLog('info', `Chapter "${chapter.title}" translation applied and view refreshed`);
        } else {
            addLog('warn', `Chapter with ID ${chapterId} not found in current data`);
        }
    }
    
    // Helper function to refresh the current view intelligently
    function refreshCurrentView(translatedChapterId) {
        if (currentViewMode === 'single') {
            // If viewing single chapter and it's the one that was translated, refresh it
            const activeItem = document.querySelector('.chapter-nav-item.active');
            if (activeItem && activeItem.dataset.chapterId === translatedChapterId) {
                loadSingleChapter(translatedChapterId);
            }
        } else if (currentViewMode === 'all') {
            // Refresh all chapters view to show the new translation
            renderAllChapters(chaptersData);
        } else if (currentViewMode === 'translated-only') {
            // Refresh translated-only view to include the new translation
            showTranslatedChapters();
        }
        
        // If we're currently showing translated content, update the view
        if (showingTranslated) {
            // Re-render to show the new translated content
            if (currentViewMode === 'single') {
                const activeItem = document.querySelector('.chapter-nav-item.active');
                if (activeItem && activeItem.dataset.chapterId === translatedChapterId) {
                    const chapter = chaptersData.find(ch => ch.id === translatedChapterId);
                    if (chapter) {
                        renderSingleChapter(chapter);
                    }
                }
            } else {
                // Re-render the chapters with updated translated content
                const chaptersToShow = currentViewMode === 'translated-only' ? 
                    chaptersData.filter(ch => ch.is_translated) : 
                    chaptersData;
                renderAllChapters(chaptersToShow);
            }
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
            // Show a user-friendly error message
            showTranslationAlert('Please select a target language first. Persian (fa) is recommended as the default.', 'warning');
            addLog('warn', 'Please select a target language first');
            
            // Highlight the language selector
            targetLanguageSelect.focus();
            targetLanguageSelect.style.borderColor = '#f59e0b';
            setTimeout(() => {
                targetLanguageSelect.style.borderColor = '';
            }, 3000);
            return;
        }
        
        try {
            let chapterToTranslate;
            
            if (currentViewMode === 'single') {
                // Get the currently active chapter
                const activeItem = document.querySelector('.chapter-nav-item.active');
                if (!activeItem) {
                    throw new Error('No active chapter found. Please select a chapter first.');
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
            
            // Show translation starting feedback
            showTranslationAlert(`Starting translation of "${chapterToTranslate.title}" to ${getLanguageName(targetLang)}...`, 'info');
            addLog('info', `Starting translation of: ${chapterToTranslate.title} to ${targetLang}`);
            
            // Update button state with loading animation
            translateSingleBtn.disabled = true;
            translateSingleBtn.innerHTML = `
                <svg class="animate-spin -ml-1 mr-3 h-4 w-4 text-white inline" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                Translating...
            `;
            
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
            
            showTranslationAlert(`Translation request submitted successfully for "${chapterToTranslate.title}". Please wait for completion...`, 'success');
            addLog('info', `Translation request sent successfully for: ${chapterToTranslate.title}`);
            
        } catch (error) {
            console.error('Error starting translation:', error);
            showTranslationAlert(`Translation failed: ${error.message}`, 'error');
            addLog('error', `Failed to start translation: ${error.message}`);
        } finally {
            translateSingleBtn.disabled = false;
            translateSingleBtn.textContent = 'Translate Current Page';
            updateTranslateSingleButtonState();
        }
    }
    
    // Helper function to show translation alerts
    function showTranslationAlert(message, type) {
        // Remove any existing alerts
        const existingAlert = document.getElementById('translation-alert');
        if (existingAlert) {
            existingAlert.remove();
        }
        
        const alertColors = {
            'info': 'bg-blue-100 border-blue-500 text-blue-700',
            'success': 'bg-green-100 border-green-500 text-green-700',
            'warning': 'bg-yellow-100 border-yellow-500 text-yellow-700',
            'error': 'bg-red-100 border-red-500 text-red-700'
        };
        
        const alert = document.createElement('div');
        alert.id = 'translation-alert';
        alert.className = `${alertColors[type]} border-l-4 p-4 mb-4 rounded`;
        alert.innerHTML = `
            <div class="flex items-center justify-between">
                <p class="text-sm font-medium">${message}</p>
                <button onclick="this.parentElement.parentElement.remove()" class="text-gray-400 hover:text-gray-600">
                    <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                        <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                    </svg>
                </button>
            </div>
        `;
        
        // Insert the alert at the top of the content area
        const contentHeader = document.getElementById('content-header');
        contentHeader.parentNode.insertBefore(alert, contentHeader.nextSibling);
        
        // Auto-remove after 5 seconds for success messages
        if (type === 'success' || type === 'info') {
            setTimeout(() => {
                if (alert && alert.parentNode) {
                    alert.remove();
                }
            }, 5000);
        }
    }
    
    // Helper function to get language name from code
    function getLanguageName(code) {
        const languages = {
            'en': 'English',
            'es': 'Spanish', 
            'fr': 'French',
            'de': 'German',
            'it': 'Italian',
            'pt': 'Portuguese',
            'ru': 'Russian',
            'ja': 'Japanese',
            'ko': 'Korean',
            'zh': 'Chinese',
            'ar': 'Arabic',
            'fa': 'Persian/Farsi',
            'he': 'Hebrew',
            'hi': 'Hindi',
            'tr': 'Turkish',
            'pl': 'Polish',
            'nl': 'Dutch',
            'sv': 'Swedish',
            'da': 'Danish',
            'no': 'Norwegian'
        };
        return languages[code] || code;
    }
    
    // URL State Management Functions
    function updateURL() {
        const params = new URLSearchParams();
        
        // Add language if set
        if (currentTargetLanguage) {
            params.set('lang', currentTargetLanguage);
        }
        
        // Add view mode
        params.set('view', currentViewMode);
        
        // Add translated state
        if (showingTranslated) {
            params.set('translated', 'true');
        }
        
        // Add chapter if in single view
        if (currentViewMode === 'single') {
            const activeItem = document.querySelector('.chapter-nav-item.active');
            if (activeItem) {
                params.set('chapter', activeItem.dataset.chapterId);
            }
        }
        
        // Update URL without page reload
        const newURL = `${window.location.pathname}${params.toString() ? '?' + params.toString() : ''}`;
        window.history.replaceState({}, '', newURL);
    }
    
    function restoreStateFromURL() {
        const params = new URLSearchParams(window.location.search);
        
        // Restore target language
        const langParam = params.get('lang');
        if (langParam && targetLanguageSelect) {
            targetLanguageSelect.value = langParam;
            currentTargetLanguage = langParam;
        }
        
        // Restore translated state
        const translatedParam = params.get('translated');
        if (translatedParam === 'true') {
            showingTranslated = true;
        }
        
        // Restore view mode and chapter
        const viewParam = params.get('view');
        const chapterParam = params.get('chapter');
        
        if (viewParam === 'single' && chapterParam) {
            // Load specific chapter
            loadSingleChapter(chapterParam);
        } else if (viewParam === 'translated-only') {
            // Show translated chapters
            showTranslatedChapters();
        } else {
            // Default to all chapters
            showAllChapters();
        }
    }
    
    // Update all view change functions to update URL
    const originalShowAllChapters = showAllChapters;
    showAllChapters = function() {
        originalShowAllChapters();
        updateURL();
    };
    
    const originalShowTranslatedChapters = showTranslatedChapters;
    showTranslatedChapters = function() {
        originalShowTranslatedChapters();
        updateURL();
    };
    
    const originalLoadSingleChapter = loadSingleChapter;
    loadSingleChapter = function(chapterId) {
        return originalLoadSingleChapter(chapterId).then(() => {
            updateURL();
        });
    };
    
    const originalToggleTranslatedView = toggleTranslatedView;
    toggleTranslatedView = function() {
        originalToggleTranslatedView();
        updateURL();
    };
    
    // Update target language tracking
    targetLanguageSelect.addEventListener('change', function() {
        currentTargetLanguage = this.value;
        updateURL();
        startTranslationBtn.disabled = !this.value;
        updateTranslateSingleButtonState();
    });
    
    // Browser back/forward button support
    window.addEventListener('popstate', function(event) {
        restoreStateFromURL();
    });
    
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
    
    // Function to apply language-specific styles
    function applyLanguageStyles() {
        // Remove any previously injected language styles
        const existingLanguageStyles = document.querySelectorAll('link[data-language-style], style[data-language-style]');
        existingLanguageStyles.forEach(element => element.remove());
        
        // Only apply styles if showing translated content and we have a target language
        if (!showingTranslated || !currentTargetLanguage) {
            return;
        }
        
        // Check if we have any translated chapters
        const hasTranslatedChapters = chaptersData.some(ch => ch.is_translated);
        if (!hasTranslatedChapters) {
            return;
        }
        
        // Try to load styles from the translated EPUB directory
        tryLoadTranslatedStyles(currentTargetLanguage);
    }
    
    function tryLoadTranslatedStyles(targetLang) {
        // Define possible paths where CSS files might be located in translated EPUB
        const possibleCSSPaths = [
            `/epub_files/${epubId}_translated_${targetLang}/styles/default.css`,
            `/epub_files/${epubId}_translated_${targetLang}/OEBPS/styles/styles.css`,
            `/epub_files/${epubId}_translated_${targetLang}/OEBPS/styles/default.css`,
            `/epub_files/${epubId}_translated_${targetLang}/OEBPS/styles/stylesheet.css`,
            `/epub_files/${epubId}_translated_${targetLang}/OEBPS/css/styles.css`,
            `/epub_files/${epubId}_translated_${targetLang}/css/styles.css`,
            `/epub_files/${epubId}_translated_${targetLang}/styles/styles.css`,
            `/epub_files/${epubId}_translated_${targetLang}/fonts/vazirmatn.css`,
            `/epub_files/${epubId}_translated_${targetLang}/fonts/Vazirmatn-font-face.css`,
            `/epub_files/${epubId}_translated_${targetLang}/style.css`
        ];

        // Try to load each CSS file
        possibleCSSPaths.forEach((cssPath, index) => {
            const link = document.createElement('link');
            link.rel = 'stylesheet';
            link.type = 'text/css';
            link.href = cssPath;
            link.setAttribute('data-language-style', targetLang);
            link.setAttribute('data-css-source', 'translated-epub');
            
            // Add load and error handlers
            link.onload = () => {
                console.log(`Successfully loaded translated styles from: ${cssPath}`);
                addLog('info', `Loaded translated EPUB styles: ${cssPath}`);
            };
            
            link.onerror = () => {
                // Silently fail for most paths, but log for debugging
                console.debug(`CSS file not found: ${cssPath}`);
                link.remove();
            };
            
            document.head.appendChild(link);
        });

        // Also apply RTL styles for supported languages
        if (isRTLLanguage(targetLang)) {
            applyRTLStyles(targetLang);
        }
    }
    
    function isRTLLanguage(languageCode) {
        const rtlLanguages = ['ar', 'fa', 'he', 'ur', 'yi', 'ji', 'iw', 'ku', 'ps', 'sd'];
        return rtlLanguages.includes(languageCode.toLowerCase());
    }
    
    function applyRTLStyles(targetLang) {
        // Create RTL stylesheet
        const rtlStyle = document.createElement('style');
        rtlStyle.setAttribute('data-language-style', targetLang);
        
        let fontFamily = '"Noto Sans", "Iranian Sans", "Tahoma", Arial, sans-serif';
        if (targetLang === 'fa') {
            fontFamily = '"Vazirmatn", "Noto Sans", "Iranian Sans", "B Nazanin", "Tahoma", Arial, sans-serif';
        } else if (targetLang === 'ar') {
            fontFamily = '"Noto Sans Arabic", "Arabic UI Text", "Tahoma", Arial, sans-serif';
        } else if (targetLang === 'he') {
            fontFamily = '"Noto Sans Hebrew", "Hebrew UI Text", "David", "Tahoma", Arial, sans-serif';
        }
        
        rtlStyle.textContent = `
            /* Applied RTL styles for ${targetLang} */
            .chapter-content, .prose {
                direction: rtl !important;
                text-align: right !important;
                unicode-bidi: embed !important;
                font-family: ${fontFamily} !important;
                line-height: 1.8 !important;
            }
            
            .chapter-content p, .prose p,
            .chapter-content div, .prose div,
            .chapter-content span, .prose span,
            .chapter-content h1, .prose h1,
            .chapter-content h2, .prose h2,
            .chapter-content h3, .prose h3,
            .chapter-content h4, .prose h4,
            .chapter-content h5, .prose h5,
            .chapter-content h6, .prose h6,
            .chapter-content li, .prose li,
            .chapter-content td, .prose td,
            .chapter-content th, .prose th,
            .chapter-content blockquote, .prose blockquote {
                direction: rtl !important;
                text-align: right !important;
                unicode-bidi: embed !important;
                font-family: ${fontFamily} !important;
            }
            
            .chapter-content table, .prose table {
                direction: rtl !important;
            }
            
            .chapter-content ul, .prose ul,
            .chapter-content ol, .prose ol {
                direction: rtl !important;
                text-align: right !important;
            }
            
            .chapter-content blockquote, .prose blockquote {
                border-right: 4px solid #ccc !important;
                border-left: none !important;
                padding-right: 1em !important;
                padding-left: 0 !important;
                margin-right: 0 !important;
                margin-left: 1em !important;
            }
            
            /* Chapter title styling for RTL */
            .chapter-title {
                direction: rtl !important;
                text-align: right !important;
                font-family: ${fontFamily} !important;
            }
        `;
        
        document.head.appendChild(rtlStyle);
        addLog('info', `Applied RTL styles for language: ${targetLang}`);
    }
});