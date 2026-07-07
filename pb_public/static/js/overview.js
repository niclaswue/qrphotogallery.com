// Keeps the viewport anchored when htmx re-renders the overview page.
//
// The QR-mode and language forms persist a change and then swap the whole
// #overview-app container (outerHTML) so the scattered bits they affect — the
// print buttons, QR downloads, the translate CTA — all refresh at once.
// Replacing that large subtree mid-flight makes the browser scroll-anchor jump
// (it lands at the very bottom of the page for a beat), which reads as the page
// "scrolling to the bottom" every time the host touches a setting.
//
// We capture the scroll offset just before the swap and restore it the instant
// the new content lands — afterSwap runs before the browser paints, so there's
// no visible flash, and we re-pin once afterSettle has the final layout (a
// language change can grow/shrink the page height). The settings checkboxes
// don't swap at all (hx-swap="none"), so they never reach this path.
(function () {
    'use strict';

    var savedScroll = null;

    function targetsOverview(evt) {
        var t = evt && evt.detail && evt.detail.target;
        return !!t && t.id === 'overview-app';
    }

    document.body.addEventListener('htmx:beforeSwap', function (evt) {
        if (targetsOverview(evt)) savedScroll = window.scrollY;
    });

    document.body.addEventListener('htmx:afterSwap', function (evt) {
        if (savedScroll !== null && targetsOverview(evt)) window.scrollTo(0, savedScroll);
    });

    document.body.addEventListener('htmx:afterSettle', function (evt) {
        if (savedScroll !== null && targetsOverview(evt)) {
            window.scrollTo(0, savedScroll);
            savedScroll = null;
        }
    });
})();
