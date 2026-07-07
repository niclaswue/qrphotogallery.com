// Create page — builds the prompt list client-side and submits it to
// POST /create as a single newline-separated form field (plus a JSON field
// carrying prompt IDs in edit mode, so edits reconcile by identity).
(function () {
    var config = window.__createConfig;
    if (!config) return;

    var form = document.getElementById('creatorForm');
    var questionList = document.getElementById('questionList');
    var questionCount = document.getElementById('questionCount');
    var limitHint = document.getElementById('limitHint');
    var limitWarning = document.getElementById('limitWarning');
    var limitWarningTitle = document.getElementById('limitWarningTitle');
    var limitWarningCta = document.getElementById('limitWarningCta');
    var emptyHint = document.getElementById('emptyHint');
    var customInput = document.getElementById('customQuestion');
    var customCount = document.getElementById('customQuestionCount');
    var addCustomBtn = document.getElementById('addCustomBtn');
    var starterList = document.getElementById('starterList');
    var promptsField = document.getElementById('promptsField');
    var promptsJsonField = document.getElementById('promptsJsonField');
    var saveBtn = document.getElementById('saveBtn');
    var eventTitle = document.getElementById('eventTitle');
    var eventDate = document.getElementById('eventDate');

    var maxPrompts = config.maxPrompts || 5;
    var prompts = Array.isArray(config.initialPrompts) ? config.initialPrompts.slice() : [];
    // promptIds runs parallel to prompts: the existing prompt record ID for each
    // entry, or '' for a newly added prompt. Edits are reconciled by ID on the
    // server so each photo stays bound to its own prompt.
    var promptIds = Array.isArray(config.initialPromptIds) ? config.initialPromptIds.slice() : [];
    while (promptIds.length < prompts.length) promptIds.push('');
    var STORAGE_KEY = 'create_draft';
    var STORAGE_TTL_MS = 7 * 24 * 60 * 60 * 1000;
    var isEdit = !!config.editMode;

    if (!isEdit) restoreDraft();
    render();

    addCustomBtn.addEventListener('click', addCustom);
    customInput.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') { e.preventDefault(); addCustom(); }
    });
    customInput.addEventListener('input', function () {
        var len = this.value.length;
        customCount.textContent = len + '/120';
        customCount.style.color = len >= 115 ? 'var(--color-error)' : '';
    });
    customCount.textContent = '0/120';

    eventTitle.addEventListener('input', saveDraft);
    if (eventDate) eventDate.addEventListener('change', saveDraft);

    if (starterList) {
        starterList.addEventListener('click', function (ev) {
            var item = ev.target.closest('.idea-item');
            if (!item) return;
            addPrompt(item.dataset.text);
        });
    }

    form.addEventListener('submit', function (ev) {
        var title = eventTitle.value.trim();
        if (!title) {
            ev.preventDefault();
            showWarning(config.strings.errorEventName);
            eventTitle.focus();
            return;
        }
        if (prompts.length === 0) {
            ev.preventDefault();
            showWarning(config.strings.errorPromptsEmpty);
            return;
        }
        if (prompts.length > maxPrompts) {
            ev.preventDefault();
            showLimitWarning();
            return;
        }
        promptsField.value = prompts.join('\n');
        if (promptsJsonField) {
            promptsJsonField.value = JSON.stringify(prompts.map(function (text, i) {
                return {id: promptIds[i] || '', text: text};
            }));
        }
        if (!isEdit) clearDraft();
        saveBtn.disabled = true;
    });

    function addCustom() {
        var text = customInput.value.trim();
        if (!text) return;
        addPrompt(text);
        customInput.value = '';
        customCount.textContent = '0/120';
    }

    function addPrompt(text) {
        if (!text) return;
        if (prompts.length >= maxPrompts) {
            showLimitWarning();
            return;
        }
        prompts.push(text);
        promptIds.push('');
        clearWarning();
        render();
        saveDraft();
    }

    function showLimitWarning() {
        var msg = sprintf(config.strings.warning, maxPrompts);
        limitWarningTitle.textContent = msg;
        if (limitWarningCta) limitWarningCta.hidden = false;
        limitWarning.classList.add('is-visible');
        scrollToWarning();
        limitWarning.classList.remove('is-pulse');
        void limitWarning.offsetWidth;
        limitWarning.classList.add('is-pulse');
    }

    function scrollToWarning() {
        try {
            limitWarning.scrollIntoView({behavior: 'smooth', block: 'center'});
        } catch (_) {
            limitWarning.scrollIntoView();
        }
    }

    function removePrompt(index) {
        prompts.splice(index, 1);
        promptIds.splice(index, 1);
        clearWarning();
        render();
        saveDraft();
    }

    function render() {
        questionCount.textContent = prompts.length;
        limitHint.textContent = sprintf(config.strings.counter, prompts.length, maxPrompts);
        limitHint.style.color = prompts.length >= maxPrompts ? 'var(--color-error)' : '';

        if (prompts.length === 0) {
            emptyHint.style.display = '';
            questionList.innerHTML = '';
        } else {
            emptyHint.style.display = 'none';
            questionList.innerHTML = prompts.map(function (text, i) {
                return (
                    '<div class="question-item">' +
                    '<span class="question-num">' + (i + 1) + '</span>' +
                    '<span class="question-text">' + escapeHtml(text) + '</span>' +
                    '<button type="button" class="question-remove" data-index="' + i + '" aria-label="' + escapeHtml(config.strings.removeLabel) + '">' +
                    '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>' +
                    '</button>' +
                    '</div>'
                );
            }).join('');
            questionList.querySelectorAll('.question-remove').forEach(function (btn) {
                btn.addEventListener('click', function () {
                    removePrompt(parseInt(btn.dataset.index, 10));
                });
            });
        }

    }

    function showWarning(msg) {
        limitWarningTitle.textContent = msg;
        if (limitWarningCta) limitWarningCta.hidden = true;
        limitWarning.classList.add('is-visible');
        limitWarning.classList.remove('is-pulse');
        scrollToWarning();
    }

    function clearWarning() {
        limitWarning.classList.remove('is-visible', 'is-pulse');
        limitWarningTitle.textContent = '';
        if (limitWarningCta) limitWarningCta.hidden = true;
    }

    function saveDraft() {
        if (isEdit) return;
        try {
            var payload = {
                title: eventTitle.value,
                date: eventDate ? eventDate.value : '',
                prompts: prompts,
                savedAt: Date.now()
            };
            localStorage.setItem(STORAGE_KEY, JSON.stringify(payload));
        } catch (_) { /* storage disabled — fine to ignore */ }
    }

    function restoreDraft() {
        try {
            var raw = localStorage.getItem(STORAGE_KEY);
            if (!raw) return;
            var payload = JSON.parse(raw);
            if (!payload || !payload.savedAt || Date.now() - payload.savedAt > STORAGE_TTL_MS) {
                localStorage.removeItem(STORAGE_KEY);
                return;
            }
            if (!eventTitle.value && payload.title) eventTitle.value = payload.title;
            if (eventDate && !eventDate.value && payload.date) eventDate.value = payload.date;
            if (prompts.length === 0 && Array.isArray(payload.prompts)) {
                prompts = payload.prompts.slice(0, maxPrompts);
                promptIds = prompts.map(function () { return ''; });
            }
        } catch (_) { /* ignore */ }
    }

    function clearDraft() {
        try { localStorage.removeItem(STORAGE_KEY); } catch (_) { }
    }

    function escapeHtml(str) {
        var div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }

    function sprintf(template) {
        var args = Array.prototype.slice.call(arguments, 1);
        var i = 0;
        return template.replace(/%d/g, function () { return args[i++]; });
    }
})();
