(function () {
    'use strict';

    var switchers = document.querySelectorAll('[data-lang-switcher]');
    if (!switchers.length) return;

    function close(switcher) {
        switcher.classList.remove('open');
        var btn = switcher.querySelector('.lang-current');
        if (btn) btn.setAttribute('aria-expanded', 'false');
    }

    switchers.forEach(function (switcher) {
        var btn = switcher.querySelector('.lang-current');
        if (!btn) return;
        btn.addEventListener('click', function (e) {
            e.preventDefault();
            e.stopPropagation();
            var open = switcher.classList.toggle('open');
            btn.setAttribute('aria-expanded', open ? 'true' : 'false');
            if (open) {
                switchers.forEach(function (other) {
                    if (other !== switcher) close(other);
                });
            }
        });
    });

    document.addEventListener('click', function (e) {
        switchers.forEach(function (switcher) {
            if (!switcher.contains(e.target)) close(switcher);
        });
    });

    document.addEventListener('keydown', function (e) {
        if (e.key === 'Escape') {
            switchers.forEach(close);
        }
    });
})();
