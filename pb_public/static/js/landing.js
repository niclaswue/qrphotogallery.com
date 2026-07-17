(function () {
    'use strict';

    function initLiveDemo() {
        var root = document.querySelector('[data-live-demo]');
        if (!root || !window.fetch) return;

        var panels = Array.prototype.slice.call(root.querySelectorAll('[data-demo-panel]'));
        var progress = Array.prototype.slice.call(root.querySelectorAll('[data-demo-progress] i'));
        var qr = root.querySelector('[data-demo-qr]');
        var directLink = root.querySelector('[data-demo-link]');
        var photoButton = root.querySelector('[data-demo-photo]');
        var photo = root.querySelector('[data-demo-image]');
        var lightbox = root.querySelector('[data-demo-lightbox]');
        var lightboxImage = root.querySelector('[data-demo-lightbox-image]');
        var close = root.querySelector('[data-demo-close]');
        var download = root.querySelector('[data-demo-download]');
        var retry = root.querySelector('[data-demo-retry]');
        var errorText = root.querySelector('[data-demo-error]');
        var stateURL = '';
        var pollTimer = 0;
        var currentStage = 'loading';
        var completed = false;
        var lastVersion = -1;

        function show(stage) {
            currentStage = stage;
            panels.forEach(function (panel) {
                panel.hidden = panel.dataset.demoPanel !== stage;
            });
            var count = stage === 'ready' ? 1 : stage === 'scanned' ? 2 : (stage === 'photo' || stage === 'complete') ? 3 : 0;
            progress.forEach(function (dot, index) {
                dot.classList.toggle('is-active', index < count);
            });
        }

        function showError(message) {
            window.clearTimeout(pollTimer);
            if (errorText) errorText.textContent = message || root.dataset.error;
            show('error');
        }

        function applyState(state) {
            if (!state || completed) return;
            if (state.stage === 'photo') {
                if (state.version !== lastVersion) {
                    lastVersion = state.version;
                    photo.src = state.image_url;
                    lightboxImage.src = state.image_url;
                    download.href = state.download_url;
                }
                show('photo');
                return;
            }
            show(state.stage === 'scanned' ? 'scanned' : 'ready');
            schedulePoll();
        }

        function schedulePoll() {
            window.clearTimeout(pollTimer);
            if (!stateURL || completed || currentStage === 'photo' || currentStage === 'error') return;
            pollTimer = window.setTimeout(poll, document.hidden ? 5000 : 1200);
        }

        function poll() {
            if (!stateURL || completed) return;
            fetch(stateURL, { cache: 'no-store', headers: { 'Accept': 'application/json' } })
                .then(function (response) {
                    if (response.status === 410) throw new Error(root.dataset.expired);
                    if (!response.ok) throw new Error(root.dataset.error);
                    return response.json();
                })
                .then(applyState)
                .catch(function (error) { showError(error.message); });
        }

        function start() {
            window.clearTimeout(pollTimer);
            completed = false;
            stateURL = '';
            lastVersion = -1;
            if (lightbox) lightbox.hidden = true;
            show('loading');
            fetch(root.dataset.startUrl, {
                method: 'POST',
                cache: 'no-store',
                headers: {
                    'Accept': 'application/json',
                    'Content-Type': 'application/x-www-form-urlencoded;charset=UTF-8'
                },
                body: 'lang=' + encodeURIComponent(root.dataset.lang || 'en')
            })
                .then(function (response) {
                    if (!response.ok) throw new Error(root.dataset.error);
                    return response.json();
                })
                .then(function (session) {
                    qr.src = session.qr_url;
                    directLink.href = session.demo_url;
                    stateURL = session.state_url;
                    show('ready');
                    poll();
                })
                .catch(function (error) { showError(error.message); });
        }

        if (photoButton) photoButton.addEventListener('click', function () {
            lightbox.hidden = false;
            close.focus();
        });
        if (close) close.addEventListener('click', function () {
            lightbox.hidden = true;
            photoButton.focus();
        });
        if (lightbox) lightbox.addEventListener('click', function (event) {
            if (event.target === lightbox) {
                lightbox.hidden = true;
                photoButton.focus();
            }
        });
        document.addEventListener('keydown', function (event) {
            if (event.key === 'Escape' && lightbox && !lightbox.hidden) {
                lightbox.hidden = true;
                photoButton.focus();
            }
        });
        if (download) download.addEventListener('click', function () {
            completed = true;
            lightbox.hidden = true;
            show('complete');
        });
        if (retry) retry.addEventListener('click', start);
        document.addEventListener('visibilitychange', function () {
            if (!document.hidden && stateURL && currentStage !== 'photo' && currentStage !== 'error') poll();
        });

        start();
    }

    function initExampleGallery() {
        var gallery = document.querySelector('[data-scroll-gallery]');
        if (!gallery || window.matchMedia('(prefers-reduced-motion: reduce)').matches || !('IntersectionObserver' in window)) return;

        var tiles = gallery.querySelectorAll('.gallery-tile');
        gallery.classList.add('is-reveal-ready');
        var observer = new IntersectionObserver(function (entries) {
            entries.forEach(function (entry) {
                if (!entry.isIntersecting) return;
                entry.target.classList.add('is-visible');
                observer.unobserve(entry.target);
            });
        }, { rootMargin: '0px 0px -6% 0px', threshold: 0.15 });
        tiles.forEach(function (tile) { observer.observe(tile); });
    }

    initLiveDemo();
    initExampleGallery();
})();
