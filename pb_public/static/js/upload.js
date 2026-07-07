// Shared photo-picker behaviour for the guest upload form. Used by both the
// per-prompt upload page (upload.html) and the single-QR reveal page
// (challenge.html). Expects a form with id="upload-form" carrying
// data-err-bad-format / data-err-too-large, plus the picker/preview element ids
// below. No-ops gracefully if the markup isn't present.
(function () {
    var form = document.getElementById('upload-form');
    var input = document.getElementById('photo');
    var picker = document.getElementById('picker');
    var cameraTile = document.getElementById('tile-camera');
    var galleryTile = document.getElementById('tile-gallery');
    var previewWrap = document.getElementById('dz-preview-wrap');
    var preview = document.getElementById('dz-preview');
    var filename = document.getElementById('dz-filename');
    var changeBtn = document.getElementById('change-photo');
    var clientError = document.getElementById('client-error');
    var nameInput = document.getElementById('guest-name');
    if (!input || !picker || !form) return;
    var ERR_BAD = form.dataset.errBadFormat || 'Unsupported file type.';
    var ERR_BIG = form.dataset.errTooLarge || 'File is too large.';
    var ERR_NONE = form.dataset.errNoFile || 'Please choose a photo first.';
    var ERR_NAME = form.dataset.errName || 'Please enter your name.';

    // When the owner collects names, remember the guest's name across prompts so
    // they don't retype it for every photo. Scoped per-browser via localStorage;
    // the server still treats the field as required and validates it.
    var NAME_KEY = 'pcw_guest_name';
    if (nameInput) {
        try {
            if (!nameInput.value) {
                var saved = window.localStorage.getItem(NAME_KEY);
                if (saved) nameInput.value = saved;
            }
        } catch (e) { /* storage may be unavailable (private mode) */ }
        nameInput.addEventListener('input', function () {
            try { window.localStorage.setItem(NAME_KEY, nameInput.value.trim()); } catch (e) {}
        });
    }

    var MAX_BYTES = 50 * 1024 * 1024;
    var EXT_OK = /\.(jpe?g|png|gif|webp|bmp|tiff?|heic|heif)$/i;

    function showError(msg) {
        clientError.textContent = msg;
        clientError.hidden = false;
    }
    function clearError() {
        clientError.textContent = '';
        clientError.hidden = true;
    }
    function reset() {
        input.value = '';
        picker.hidden = false;
        previewWrap.hidden = true;
        preview.removeAttribute('src');
        filename.textContent = '';
        if (changeBtn) changeBtn.hidden = true;
    }

    // Both tiles drive the single #photo input so the form keeps one `image`
    // field. The camera tile adds capture="environment" to open the device
    // camera directly; the gallery tile (a <label for="photo">) clears it for
    // the normal file/gallery picker. Clearing on the label's own click runs
    // before the label forwards the click to the input, so the gallery picker
    // never inherits a stale capture attribute left by the camera tile.
    if (cameraTile) {
        cameraTile.addEventListener('click', function () {
            clearError();
            input.setAttribute('capture', 'environment');
            input.click();
        });
    }
    if (galleryTile) {
        galleryTile.addEventListener('click', function () {
            clearError();
            input.removeAttribute('capture');
        });
    }

    input.addEventListener('change', function () {
        clearError();
        var file = input.files && input.files[0];
        if (!file) { reset(); return; }

        if (file.size > MAX_BYTES) {
            showError(ERR_BIG);
            reset();
            return;
        }
        var typeOK = (file.type && file.type.indexOf('image/') === 0) || EXT_OK.test(file.name);
        if (!typeOK) {
            showError(ERR_BAD);
            reset();
            return;
        }

        filename.textContent = file.name;
        picker.hidden = true;
        previewWrap.hidden = false;
        if (changeBtn) changeBtn.hidden = false;

        // HEIC/HEIF (the iPhone default) can't be rendered by <img> in most
        // browsers, so we skip the inline preview entirely and show the
        // filename only. The server transcodes it to JPEG after upload.
        var isHeic = /\.(heic|heif)$/i.test(file.name) ||
            file.type === 'image/heic' || file.type === 'image/heif';
        if (isHeic) {
            preview.hidden = true;
            preview.removeAttribute('src');
            return;
        }

        // FileReader gives us a data URL preview that works for JPEG/PNG/etc.
        preview.hidden = false;
        var reader = new FileReader();
        reader.onload = function (ev) {
            preview.src = ev.target.result;
        };
        reader.onerror = function () {
            preview.hidden = true;
            preview.removeAttribute('src');
        };
        try { reader.readAsDataURL(file); } catch (e) { preview.hidden = true; preview.removeAttribute('src'); }
    });

    if (changeBtn) {
        changeBtn.addEventListener('click', function () {
            // Drop the current photo and return to the picker so the guest can
            // pick the camera or gallery again. We deliberately don't re-open
            // the file dialog here: doing so cleared the input first, and a
            // cancelled dialog left an empty (black) preview that then failed
            // to upload.
            reset();
            input.removeAttribute('capture');
        });
    }

    // Guard against submitting with no file selected (which the server rejects
    // with a full page reload). Show the inline error instead.
    form.addEventListener('submit', function (ev) {
        if (!input.files || !input.files[0]) {
            ev.preventDefault();
            showError(ERR_NONE);
            return;
        }
        // Backstop the native `required` on the name field so the chosen photo
        // isn't lost to a server-side reload when the name is missing.
        if (nameInput && !nameInput.value.trim()) {
            ev.preventDefault();
            showError(ERR_NAME);
            nameInput.focus();
        }
    });
})();
