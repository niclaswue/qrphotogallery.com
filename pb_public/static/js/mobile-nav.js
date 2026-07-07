(function () {
    'use strict';

    var navToggle = document.getElementById('navToggle');
    var navMobile = document.getElementById('navMobile');
    var navMobileOverlay = document.getElementById('navMobileOverlay');
    var navMobileClose = document.getElementById('navMobileClose');

    if (!navToggle || !navMobile) return;

    function openMobileNav() {
        navMobile.classList.add('is-open');
        if (navMobileOverlay) navMobileOverlay.classList.add('is-open');
        navToggle.setAttribute('aria-expanded', 'true');
        document.body.style.overflow = 'hidden';
    }

    function closeMobileNav() {
        navMobile.classList.remove('is-open');
        if (navMobileOverlay) navMobileOverlay.classList.remove('is-open');
        navToggle.setAttribute('aria-expanded', 'false');
        document.body.style.overflow = '';
    }

    navToggle.addEventListener('click', openMobileNav);
    if (navMobileClose) {
        navMobileClose.addEventListener('click', closeMobileNav);
    }
    if (navMobileOverlay) {
        navMobileOverlay.addEventListener('click', closeMobileNav);
    }

    document.addEventListener('keydown', function (e) {
        if (e.key === 'Escape' && navMobile.classList.contains('is-open')) {
            closeMobileNav();
        }
    });
})();