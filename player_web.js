(() => {
    function watchID(raw) {
        const pathname = new URL(raw, location.href).pathname;
        return /^\/watch\/((?:(?:sm|nm|so)?[0-9]{1,20}))\/?$/i.exec(pathname)?.[1].toLowerCase();
    }

    const initialID = watchID(location.href);
    if (window.__nicoDesktopMedia || !initialID || location.protocol !== 'https:' ||
        !/^(?:(?:www|sp|video)\.)?nicovideo\.jp$/.test(location.hostname)) return;

    let mediaID = initialID;
    let previousMedia;
    let previousSource;
    let lastSample;
    let pausedRestore;

    function mediaFor(id) {
        if (!id || watchID(location.href) !== id) return;
        const videos = [...document.querySelectorAll('video')].filter(video =>
            video.readyState >= 1 && !video.error && Number.isFinite(video.duration) && video.duration > 0);
        if (videos.length !== 1) return;
        const video = videos[0];
        if (id !== mediaID) {
            // A new URL can precede the new video element or source in a SPA.
            if (!previousMedia || video === previousMedia && video.currentSrc === previousSource) return;
            try {
                const response = JSON.parse(document.querySelector('meta[name="server-response"]').content);
                if (response.data.response.$watchV4.data.video.id !== id) return;
            } catch {
                return;
            }
            mediaID = id;
        }
        previousMedia = video;
        previousSource = video.currentSrc;
        return video;
    }

    function remember() {
        const id = watchID(location.href);
        const video = mediaFor(id);
        if (video && Number.isFinite(video.currentTime) && video.currentTime >= 0) {
            lastSample = [id, video.currentTime, video.paused || video.ended];
        }
    }

    // The official player can request autoplay after its initial seek has
    // completed. Keep a restored pause until the viewer takes over.
    document.addEventListener('play', event => {
        if (!pausedRestore) return;
        if (watchID(location.href) !== pausedRestore.id) {
            pausedRestore = undefined;
            return;
        }
        const video = pausedRestore.video;
        if (event.target === video && video.currentSrc === pausedRestore.source) {
            video.pause();
            if (Math.abs(video.currentTime - pausedRestore.at) > 0.05) video.currentTime = pausedRestore.at;
        }
    }, true);
    for (const event of ['pointerdown', 'keydown']) {
        document.addEventListener(event, action => {
            if (action.isTrusted) pausedRestore = undefined;
        }, true);
    }

    for (const event of ['loadedmetadata', 'timeupdate', 'seeked', 'play', 'pause', 'ended']) {
        document.addEventListener(event, remember, true);
    }

    function forwardSearch(event, dest) {
        if (!event.cancelable || !watchID(location.href) || dest.origin !== location.origin ||
            !/^\/(?:tag|search)\/[^/]+\/?$/.test(dest.pathname)) return;
        remember();
        const source = location.href;
        event.preventDefault();
        event.stopImmediatePropagation();
        // A normal top-level request lets the host send the complete URL to the main window.
        queueMicrotask(() => {
            if (location.href === source) location.assign(dest.href);
        });
    }

    document.addEventListener('click', event => {
        if (!event.isTrusted || event.defaultPrevented || event.button !== 0 ||
            event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
        const link = event.target?.closest?.('a[href]');
        if (link && (!link.target || link.target.toLowerCase() === '_self') && !link.hasAttribute('download')) {
            forwardSearch(event, new URL(link.href));
        }
    }, true);

    window.navigation?.addEventListener('navigate', event => {
        if (event.destination.sameDocument) {
            remember();
            if (!event.formData && event.downloadRequest === null) {
                forwardSearch(event, new URL(event.destination.url));
            }
        }
    });

    let restoreAttempt;
    function restore(token, id, seconds, paused, resume) {
        const video = mediaFor(id);
        if (!video || video.seekable.length === 0) return 'wait';
        if (restoreAttempt?.token !== token || restoreAttempt.video !== video) {
            const at = Math.min(seconds, video.duration);
            let seekable = false;
            for (let i = 0; i < video.seekable.length; i++) {
                if (at >= video.seekable.start(i) && at <= video.seekable.end(i)) seekable = true;
            }
            if (!seekable) return 'wait';

            const attempt = {token, video, result: 'wait'};
            restoreAttempt = attempt;
            try {
                pausedRestore = paused ? {id, at, video, source: video.currentSrc} : undefined;
                if (paused && !video.paused) video.pause();
                video.currentTime = at;
                if (paused && video.seeking) {
                    video.addEventListener('seeked', () => {
                        if (restoreAttempt === attempt && mediaFor(id) === video) {
                            video.pause();
                            attempt.result = 'applied';
                        }
                    }, {once: true});
                } else {
                    attempt.result = paused || !video.paused ? 'applied' : 'play';
                }
            } catch {
                pausedRestore = undefined;
                attempt.result = 'blocked';
            }
        }
        const attempt = restoreAttempt;
        if (attempt.result === 'play' && resume) {
            attempt.result = 'wait';
            try {
                video.play().then(
                    () => { attempt.result = 'applied'; },
                    () => { attempt.result = 'blocked'; }
                );
            } catch {
                attempt.result = 'blocked';
            }
        }
        return attempt.result;
    }

    Object.defineProperty(window, '__nicoDesktopMedia', {
        value: Object.freeze({snapshot() { remember(); return lastSample; }, restore})
    });
})();
