(() => {
    function watchID(raw) {
        const pathname = new URL(raw, location.href).pathname;
        return /^\/watch\/((?:(?:sm|nm|so)?[0-9]{1,20}))\/?$/i.exec(pathname)?.[1].toLowerCase();
    }

    const initialID = watchID(location.href);
    if (window.__nicoDesktopMedia || !initialID || location.protocol !== 'https:' ||
        !/^(?:(?:www|sp|video)\.)?nicovideo\.jp$/.test(location.hostname)) return;

    // The watch route consumes the server-response meta on hydration. Use its
    // live loader data and the official content element (excluding ads/previews).
    const contentSelector = '[data-name="stage"] video[data-name="video-content"]';
    const watchRoute = 'routes/_web.watch.$id.$';
    const mediaOwners = new Map();
    let lastSample;
    let pausedRestore;

    function committedWatch(id) {
        const state = window.__reactRouterDataRouter?.state;
        if (!state) return id === initialID;
        const data = state.loaderData?.[watchRoute];
        return !state.errors && state.navigation?.state === 'idle' &&
            state.matches?.some(match => match.route.id === watchRoute && match.params.id === id) &&
            data?.ok === true && data.data?.response?.$watchV4?.data?.video?.id === id;
    }

    function contentVideo() {
        const videos = document.querySelectorAll(contentSelector);
        if (videos.length === 1) return videos[0];
    }

    document.addEventListener('loadstart', event => {
        const video = contentVideo();
        if (!event.isTrusted || !video || event.target !== video) return;
        const source = video.currentSrc;
        const id = watchID(location.href);
        // A changed URL or a late event for the old source cannot relabel it.
        if (source && !mediaOwners.has(source) && id && committedWatch(id)) {
            mediaOwners.set(source, id);
        }
    }, true);

    function mediaFor(id) {
        if (!id || watchID(location.href) !== id) return;
        const video = contentVideo();
        if (!video || video.readyState < 1 || video.error || !Number.isFinite(video.duration) || video.duration <= 0) return;
        if (mediaOwners.get(video.currentSrc) === id) return video;
    }

    function remember(target) {
        const url = location.href;
        const video = mediaFor(watchID(url));
        if (video && (!target || target === video) && Number.isFinite(video.currentTime) && video.currentTime >= 0) {
            lastSample = [url, video.currentTime, video.paused || video.ended];
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
        document.addEventListener(event, action => {
            if (action.isTrusted) remember(action.target);
        }, true);
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
