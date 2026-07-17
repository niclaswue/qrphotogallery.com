(function () {
    'use strict';

    var root = document.querySelector('[data-demo-mobile]');
    if (!root) return;

    var form = document.getElementById('demo-upload-form');
    var input = document.getElementById('demo-photo');
    var picker = document.getElementById('demo-picker');
    var selection = document.getElementById('demo-selection');
    var filename = document.getElementById('demo-filename');
    var filesize = document.getElementById('demo-filesize');
    var change = document.getElementById('demo-change');
    var upload = document.getElementById('demo-upload');
    var sample = document.getElementById('demo-sample');
    var progress = document.getElementById('demo-progress');
    var errorBox = document.getElementById('demo-error');
    var card = document.getElementById('demo-upload-card');
    var success = document.getElementById('demo-success');
    var successImage = document.getElementById('demo-success-image');
    var download = document.getElementById('demo-download');
    var timer = document.querySelector('[data-demo-timer]');
    var MAX_BYTES = 15 * 1024 * 1024;
    var EXT_OK = /\.(jpe?g|png|gif|webp|heic|heif)$/i;
    var originalUploadLabel = upload ? upload.innerHTML : '';
    var originalSampleLabel = sample ? sample.innerHTML : '';

    function copy(name, fallback) {
        return root.dataset[name] || fallback;
    }

    function humanSize(bytes) {
        if (bytes >= 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
        return Math.max(1, Math.round(bytes / 1024)) + ' KB';
    }

    function showError(message) {
        if (!errorBox) return;
        errorBox.textContent = message;
        errorBox.hidden = false;
    }

    function clearError() {
        if (!errorBox) return;
        errorBox.textContent = '';
        errorBox.hidden = true;
    }

    function valid(file) {
        if (!file) {
            showError(copy('errorNoFile', 'Choose a photo first.'));
            return false;
        }
        if (file.size <= 0 || file.size > MAX_BYTES) {
            showError(copy('errorTooLarge', 'Choose a photo smaller than 15 MB.'));
            return false;
        }
        if (!/^image\//.test(file.type || '') && !EXT_OK.test(file.name || '')) {
            showError(copy('errorBadFormat', 'Choose a JPG, PNG, WebP, GIF, HEIC or HEIF photo.'));
            return false;
        }
        return true;
    }

    function renderSelection() {
        clearError();
        var file = input && input.files && input.files[0];
        if (!valid(file)) {
            if (upload) upload.disabled = true;
            return;
        }
        filename.textContent = file.name;
        filesize.textContent = copy('selected', 'Ready to upload') + ' · ' + humanSize(file.size);
        picker.hidden = true;
        selection.hidden = false;
        upload.disabled = false;
    }

    function resetSelection() {
        if (!input) return;
        input.value = '';
        picker.hidden = false;
        selection.hidden = true;
        upload.disabled = true;
        clearError();
    }

    function setBusy(busy, source) {
        if (input) input.disabled = busy;
        if (upload) {
            upload.disabled = busy || !(input && input.files && input.files.length);
            upload.innerHTML = busy && source === 'upload' ? copy('uploading', 'Uploading…') : originalUploadLabel;
        }
        if (sample) {
            sample.disabled = busy;
            sample.innerHTML = busy && source === 'sample' ? copy('uploading', 'Preparing…') : originalSampleLabel;
        }
    }

    function parseError(text) {
        try {
            var body = JSON.parse(text);
            return body.error || copy('errorFailed', 'That did not work. Please try again.');
        } catch (_) {
            return copy('errorFailed', 'That did not work. Please try again.');
        }
    }

    function showSuccess(state) {
        if (!state || !state.image_url) return;
        clearError();
        card.hidden = true;
        successImage.src = state.image_url;
        download.href = state.download_url;
        success.hidden = false;
        success.scrollIntoView({ behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth', block: 'start' });
    }

    if (input) input.addEventListener('change', renderSelection);
    if (change) change.addEventListener('click', resetSelection);

    if (picker) {
        ['dragenter', 'dragover'].forEach(function (name) {
            picker.addEventListener(name, function (event) {
                event.preventDefault();
                picker.classList.add('is-dragging');
            });
        });
        ['dragleave', 'drop'].forEach(function (name) {
            picker.addEventListener(name, function (event) {
                event.preventDefault();
                picker.classList.remove('is-dragging');
            });
        });
        picker.addEventListener('drop', function (event) {
            if (!event.dataTransfer || !event.dataTransfer.files.length || !input) return;
            try {
                input.files = event.dataTransfer.files;
                renderSelection();
            } catch (_) {
                input.click();
            }
        });
    }

    if (form) form.addEventListener('submit', function (event) {
        event.preventDefault();
        clearError();
        var file = input && input.files && input.files[0];
        if (!valid(file)) return;

        var xhr = new XMLHttpRequest();
        xhr.open('POST', root.dataset.uploadUrl);
        xhr.setRequestHeader('Accept', 'application/json');
        setBusy(true, 'upload');
        progress.hidden = false;
        progress.querySelector('span').style.width = '4%';
        xhr.upload.addEventListener('progress', function (event) {
            if (!event.lengthComputable) return;
            progress.querySelector('span').style.width = Math.max(4, Math.round(event.loaded / event.total * 100)) + '%';
        });
        xhr.addEventListener('load', function () {
            if (xhr.status >= 200 && xhr.status < 300) {
                progress.querySelector('span').style.width = '100%';
                try { showSuccess(JSON.parse(xhr.responseText)); } catch (_) { showError(copy('errorFailed', 'That did not work. Please try again.')); }
                return;
            }
            progress.hidden = true;
            setBusy(false, 'upload');
            showError(parseError(xhr.responseText));
        });
        xhr.addEventListener('error', function () {
            progress.hidden = true;
            setBusy(false, 'upload');
            showError(copy('errorConnection', 'The connection was interrupted. Please try again.'));
        });
        var body = new FormData();
        body.append('image', file, file.name);
        xhr.send(body);
    });

    if (sample) sample.addEventListener('click', function () {
        clearError();
        setBusy(true, 'sample');
        fetch(root.dataset.sampleUrl, { method: 'POST', headers: { 'Accept': 'application/json' } })
            .then(function (response) {
                return response.text().then(function (text) {
                    if (!response.ok) throw new Error(parseError(text));
                    return JSON.parse(text);
                });
            })
            .then(showSuccess)
            .catch(function (error) {
                setBusy(false, 'sample');
                showError(error.message || copy('errorConnection', 'The connection was interrupted. Please try again.'));
            });
    });

    if (timer) {
        var expiresAt = new Date(timer.dataset.expiresAt).getTime();
        var timerText = timer.querySelector('span');
        var tick = function () {
            var seconds = Math.max(0, Math.floor((expiresAt - Date.now()) / 1000));
            if (!seconds) {
                timerText.textContent = copy('timerExpired', 'Demo expired');
                setBusy(true, 'expired');
                return false;
            }
            var minutes = Math.floor(seconds / 60);
            timerText.textContent = minutes + ':' + String(seconds % 60).padStart(2, '0');
            return true;
        };
        tick();
        var interval = window.setInterval(function () {
            if (!tick()) window.clearInterval(interval);
        }, 1000);
    }
})();
