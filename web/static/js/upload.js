// File upload functionality
document.addEventListener('DOMContentLoaded', function() {
    const dropZone = document.getElementById('drop-zone');
    const fileInput = document.getElementById('epub-file');
    const uploadSection = document.getElementById('upload-section');
    const uploadProgress = document.getElementById('upload-progress');
    const progressBar = document.getElementById('progress-bar');
    const errorMessage = document.getElementById('error-message');
    const errorText = document.getElementById('error-text');

    // Drag and drop functionality
    dropZone.addEventListener('dragover', function(e) {
        e.preventDefault();
        dropZone.classList.add('drag-over');
    });

    dropZone.addEventListener('dragleave', function(e) {
        e.preventDefault();
        dropZone.classList.remove('drag-over');
    });

    dropZone.addEventListener('drop', function(e) {
        e.preventDefault();
        dropZone.classList.remove('drag-over');
        
        const files = e.dataTransfer.files;
        if (files.length > 0) {
            handleFile(files[0]);
        }
    });

    // File input change
    fileInput.addEventListener('change', function(e) {
        if (e.target.files.length > 0) {
            handleFile(e.target.files[0]);
        }
    });

    function handleFile(file) {
        // Validate file
        if (!file.name.endsWith('.epub')) {
            showError('Please select an EPUB file');
            return;
        }

        if (file.size > 50 * 1024 * 1024) { // 50MB
            showError('File is too large. Maximum size is 50MB');
            return;
        }

        uploadFile(file);
    }

    function uploadFile(file) {
        const formData = new FormData();
        formData.append('epub', file);

        // Show progress
        uploadSection.style.display = 'none';
        uploadProgress.style.display = 'block';
        hideError();

        // Simulate progress for better UX
        let progress = 0;
        const progressInterval = setInterval(() => {
            progress += Math.random() * 15;
            if (progress > 90) progress = 90;
            progressBar.style.width = progress + '%';
        }, 200);

        fetch('/upload', {
            method: 'POST',
            body: formData
        })
        .then(response => response.json())
        .then(data => {
            clearInterval(progressInterval);
            progressBar.style.width = '100%';

            if (data.error) {
                throw new Error(data.error);
            }

            // Success - redirect to preview
            setTimeout(() => {
                window.location.href = data.redirect_url;
            }, 500);
        })
        .catch(error => {
            clearInterval(progressInterval);
            uploadSection.style.display = 'block';
            uploadProgress.style.display = 'none';
            progressBar.style.width = '0%';
            showError(error.message || 'Upload failed. Please try again.');
        });
    }

    function showError(message) {
        errorText.textContent = message;
        errorMessage.style.display = 'block';
    }

    function hideError() {
        errorMessage.style.display = 'none';
    }
});