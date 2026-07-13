(function () {
    var lightbox = document.getElementById('lightbox');
    if (!lightbox) return;
    var track = document.getElementById('lightbox-track');
    var slides = track.children;
    var counter = document.getElementById('lightbox-counter');
    var current = 0;

    function show(index) {
        current = Math.max(0, Math.min(slides.length - 1, index));
        lightbox.hidden = false;
        document.body.classList.add('lightbox-open');
        track.scrollTo({ left: slides[current].offsetLeft, behavior: 'auto' });
        counter.textContent = (current + 1) + ' / ' + slides.length;
    }
    function close() {
        lightbox.hidden = true;
        document.body.classList.remove('lightbox-open');
        Array.prototype.forEach.call(track.querySelectorAll('video'), function (video) { video.pause(); });
    }
    Array.prototype.forEach.call(document.querySelectorAll('[data-slide]'), function (tile) {
        tile.addEventListener('click', function () { show(parseInt(tile.dataset.slide, 10)); });
    });
    document.getElementById('lightbox-close').addEventListener('click', close);
    document.getElementById('lightbox-prev').addEventListener('click', function () { show(current - 1); });
    document.getElementById('lightbox-next').addEventListener('click', function () { show(current + 1); });
    document.addEventListener('keydown', function (event) {
        if (lightbox.hidden) return;
        if (event.key === 'Escape') close();
        if (event.key === 'ArrowRight') show(current + 1);
        if (event.key === 'ArrowLeft') show(current - 1);
    });
})();
