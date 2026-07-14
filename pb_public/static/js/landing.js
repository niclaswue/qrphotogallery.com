(function () {
    const gallery = document.querySelector('[data-scroll-gallery]');
    if (!gallery || window.matchMedia('(prefers-reduced-motion: reduce)').matches || !('IntersectionObserver' in window)) {
        return;
    }

    const tiles = gallery.querySelectorAll('.gallery-tile');
    gallery.classList.add('is-reveal-ready');

    const observer = new IntersectionObserver((entries) => {
        entries.forEach((entry) => {
            if (!entry.isIntersecting) return;
            entry.target.classList.add('is-visible');
            observer.unobserve(entry.target);
        });
    }, { rootMargin: '0px 0px -6% 0px', threshold: 0.15 });

    tiles.forEach((tile) => observer.observe(tile));
})();
