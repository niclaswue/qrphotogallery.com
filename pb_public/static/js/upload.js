(function () {
    var form = document.getElementById('upload-form');
    var input = document.getElementById('photo');
    var picker = document.getElementById('picker');
    var selected = document.getElementById('dz-preview-wrap');
    var summary = document.getElementById('dz-filename');
    var fileList = document.getElementById('file-list');
    var changeBtn = document.getElementById('change-photo');
    var submitBtn = document.getElementById('submit-btn');
    var clientError = document.getElementById('client-error');
    var nameInput = document.getElementById('guest-name');
    var progress = document.getElementById('upload-progress');
    if (!form || !input || !picker) return;

    var MAX_BYTES = 2 * 1024 * 1024 * 1024;
    var MAX_BATCH_BYTES = 4 * 1024 * 1024 * 1024;
    var EXT_OK = /\.(jpe?g|png|gif|webp|bmp|tiff?|heic|heif|mp4|m4v|mov|webm|avi|mpeg|mpg|3gp)$/i;
    var ERR_BAD = form.dataset.errBadFormat || 'This file type is not supported.';
    var ERR_BIG = form.dataset.errTooLarge || 'One of these files is too large.';
    var ERR_NONE = form.dataset.errNoFile || 'Choose at least one file.';
    var ERR_NAME = form.dataset.errName || 'Please enter your name.';
    var originalButtonLabel = submitBtn.textContent;
    submitBtn.disabled = true;

    function humanSize(bytes) {
        if (bytes >= 1024 * 1024 * 1024) return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB';
        if (bytes >= 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
        return Math.max(1, Math.round(bytes / 1024)) + ' KB';
    }
    function showError(message) { clientError.textContent = message; clientError.hidden = false; }
    function clearError() { clientError.textContent = ''; clientError.hidden = true; }
    function valid(files) {
        if (!files.length) { showError(ERR_NONE); return false; }
        if (files.length > 100) { showError('Please select no more than 100 files at once.'); return false; }
        var total = 0;
        for (var i = 0; i < files.length; i++) {
            if (files[i].size > MAX_BYTES) { showError(ERR_BIG); return false; }
            if (!((files[i].type || '').match(/^(image|video)\//) || EXT_OK.test(files[i].name))) { showError(ERR_BAD); return false; }
            total += files[i].size;
        }
        if (total > MAX_BATCH_BYTES) { showError('This selection is larger than 4 GB. Upload it in two batches.'); return false; }
        return true;
    }
    function renderFiles() {
        clearError();
        var files = Array.prototype.slice.call(input.files || []);
        if (!valid(files)) { submitBtn.disabled = true; return; }
        fileList.textContent = '';
        var total = 0;
        files.forEach(function (file) {
            total += file.size;
            var row = document.createElement('div');
            row.className = 'selected-file';
            var icon = document.createElement('span');
            icon.textContent = (file.type || '').indexOf('video/') === 0 || /\.(mp4|mov|webm|avi|m4v|3gp)$/i.test(file.name) ? '▶' : '▧';
            var label = document.createElement('span');
            var strong = document.createElement('strong'); strong.textContent = file.name;
            var small = document.createElement('small'); small.textContent = humanSize(file.size);
            label.appendChild(strong); label.appendChild(small); row.appendChild(icon); row.appendChild(label); fileList.appendChild(row);
        });
        summary.textContent = files.length === 1 ? '1 file selected · ' + humanSize(total) : files.length + ' files selected · ' + humanSize(total);
        picker.hidden = true; selected.hidden = false; submitBtn.disabled = false;
    }
    function reset() { input.value = ''; fileList.textContent = ''; selected.hidden = true; picker.hidden = false; submitBtn.disabled = true; clearError(); }

    input.addEventListener('change', renderFiles);
    changeBtn && changeBtn.addEventListener('click', reset);
    ['dragenter', 'dragover'].forEach(function (name) { picker.addEventListener(name, function (event) { event.preventDefault(); picker.classList.add('is-dragging'); }); });
    ['dragleave', 'drop'].forEach(function (name) { picker.addEventListener(name, function (event) { event.preventDefault(); picker.classList.remove('is-dragging'); }); });
    picker.addEventListener('drop', function (event) {
        if (!event.dataTransfer || !event.dataTransfer.files.length) return;
        try { input.files = event.dataTransfer.files; renderFiles(); } catch (_) { input.click(); }
    });

    var nameKey = 'qrpg_guest_name';
    if (nameInput) {
        try { if (!nameInput.value) nameInput.value = localStorage.getItem(nameKey) || ''; } catch (_) {}
        nameInput.addEventListener('input', function () { try { localStorage.setItem(nameKey, nameInput.value.trim()); } catch (_) {} });
    }

    form.addEventListener('submit', function (event) {
        event.preventDefault(); clearError();
        if (!valid(Array.prototype.slice.call(input.files || []))) return;
        if (nameInput && !nameInput.value.trim()) { showError(ERR_NAME); nameInput.focus(); return; }
        var xhr = new XMLHttpRequest();
        xhr.open('POST', form.action);
        progress.hidden = false; submitBtn.disabled = true; submitBtn.textContent = 'Uploading…';
        xhr.upload.addEventListener('progress', function (e) {
            if (!e.lengthComputable) return;
            var pct = Math.round((e.loaded / e.total) * 100);
            progress.querySelector('span').style.width = pct + '%'; progress.querySelector('small').textContent = pct + '%';
        });
        xhr.addEventListener('load', function () {
            if (xhr.status >= 200 && xhr.status < 400) { window.location.assign(xhr.responseURL || form.action); return; }
            submitBtn.disabled = false; submitBtn.textContent = originalButtonLabel; progress.hidden = true; showError('The upload could not be completed. Please try again.');
        });
        xhr.addEventListener('error', function () { submitBtn.disabled = false; submitBtn.textContent = originalButtonLabel; progress.hidden = true; showError('Your connection was interrupted. Please try again.'); });
        xhr.send(new FormData(form));
    });
})();
