(() => {
    function watchID(pathname) {
        return /^\/watch\/((?:(?:sm|nm|so)?[0-9]{1,20}))\/?$/i.exec(pathname)?.[1].toLowerCase();
    }

    if (window.__nicoDesktopMedia || !watchID(location.pathname) || location.protocol !== 'https:' ||
        !/^(?:(?:www|sp|video)\.)?nicovideo\.jp$/.test(location.hostname)) return;

    const contentSelector = '[data-name="stage"] video[data-name="video-content"]';
    const watchRoute = 'routes/_web.watch.$id.$';
    const generation = new WeakMap();
    let entryURL = location.href;
    let targetURL = entryURL;
    let lastSample;
    let pausedRestore;

    function removesWatchOptions() {
        const previous = new URL(targetURL);
        const next = new URL(location.href);
        if (previous.pathname !== next.pathname || next.hash && next.hash !== previous.hash) return false;
        const options = [...previous.searchParams];
        return [...next.searchParams].every(([key, value]) => {
            const i = options.findIndex(([k, v]) => k === key && v === value);
            if (i < 0) return false;
            options.splice(i, 1);
            return true;
        });
    }

    // Nico can remove consumed options with replaceState after loading a watch
    // URL. Keep the requested options, including the alias path, for recovery.
    window.navigation?.addEventListener('currententrychange', event => {
        if (event.navigationType !== 'replace' || !watchID(location.pathname) || !removesWatchOptions()) {
            targetURL = location.href;
        }
        entryURL = location.href;
    });

    function committedVideo() {
        const requested = watchID(location.pathname);
        const state = window.__reactRouterDataRouter?.state;
        if (!requested || !state || state.errors || state.location?.pathname !== location.pathname ||
            !state.matches?.some(match => match.route.id === watchRoute && match.params.id === requested)) return;
        const response = state.loaderData?.[watchRoute];
        const id = response?.data?.response?.$watchV4?.data?.video?.id;
        if (response?.ok === true && typeof id === 'string' && /^(?:(?:sm|nm|so)?[0-9]{1,20})$/.test(id) &&
            (requested === id || /^[0-9]+$/.test(requested))) return id;
    }

    // The official desktop page mounts the controller's stage under a React
    // host element. Read the controller value already stored by its presenter.
    // A route match alone cannot tell which session owns the current resource.
    function currentMedia() {
        const id = committedVideo();
        if (!id) return;
        const videos = document.querySelectorAll(contentSelector);
        if (videos.length !== 1) return;
        const video = videos[0];
        const source = video.currentSrc;
        if (!source || video.readyState < 1 || video.error || !Number.isFinite(video.duration) || video.duration <= 0) return;
        const stage = video.closest('[data-name="stage"]');
        const parent = stage?.parentElement;
        if (!parent) return;
        const key = Object.keys(parent).find(name => name.startsWith('__reactFiber$'));
        if (!key) return;
        const fiber = parent[key];
        let owner;
        for (const branch of [fiber, fiber?.alternate]) {
            for (let node = branch; node; node = node.return) {
                for (let hook = node.memoizedState; hook; hook = hook.next) {
                    const controller = Array.isArray(hook.memoizedState) && hook.memoizedState[0];
                    if (controller?.stage !== stage || controller.view?.videoContent !== video) continue;
                    const session = controller.session;
                    const hls = session?.hlsjs;
                    const content = session?.accessRights?.contentUrl;
                    if (session?.video !== video || !content ||
                        session.watch?.video?.id !== controller.watch?.video?.id ||
                        controller._isInterrupting !== false || controller._isDummyVideo !== false) continue;
                    // Native playback assigns the content URL directly. HLS.js
                    // owns the object URL through its attached buffer controller.
                    const buffer = hls?.bufferController;
                    const ownsSource = hls ? hls.media === video && buffer?.media === video &&
                        buffer._objectUrl === source && buffer.mediaSrc === source && hls.url === content &&
                        !hls.interstitialsManager?.playingItem?.event : video.src === source && source === content;
                    if (ownsSource) {
                        if (owner && owner !== session) return;
                        owner = session;
                    }
                }
            }
        }
        if (owner?.watch?.video?.id === id) return {id, video, source, owner, generation: generation.get(video)};
    }

    // Reloading the same URL can reset currentTime without changing currentSrc.
    document.addEventListener('emptied', event => {
        if (event.isTrusted && event.target instanceof HTMLVideoElement) {
            generation.set(event.target, (generation.get(event.target) ?? 0) + 1);
        }
    }, true);

    function sameMedia(a, b) {
        return b && a.id === b.id && a.video === b.video && a.owner === b.owner &&
            a.source === b.source && a.generation === b.generation;
    }

    function remember(target) {
        const media = currentMedia();
        const video = media?.video;
        if (video && (!target || target === video) && Number.isFinite(video.currentTime) && video.currentTime >= 0) {
            const url = entryURL === location.href ? targetURL : location.href;
            lastSample = [url, media.id, video.currentTime, video.paused || video.ended];
        }
    }

    // The official player can request autoplay after its initial seek has
    // completed. Keep a restored pause until the viewer takes over.
    document.addEventListener('play', event => {
        if (!event.isTrusted || !pausedRestore) return;
        const video = pausedRestore.video;
        if (event.target !== video) return;
        if (sameMedia(pausedRestore, currentMedia())) {
            video.pause();
            if (Math.abs(video.currentTime - pausedRestore.at) > 0.05) video.currentTime = pausedRestore.at;
        } else {
            pausedRestore = undefined;
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
        if (!event.cancelable || !watchID(location.pathname) || dest.origin !== location.origin ||
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
        const media = currentMedia();
        if (media?.id !== id) return 'wait';
        const {video} = media;
        const at = Math.min(seconds, video.duration);
        if (restoreAttempt?.token !== token || !sameMedia(restoreAttempt, media)) {
            restoreAttempt = {...media, token, at};
            pausedRestore = undefined;
        }
        const attempt = restoreAttempt;
        if (attempt.playing) return attempt.playing === 'applied' && video.paused ? 'wait' : attempt.playing;
        if (video.seeking) return 'wait';
        try {
            if (paused && !video.paused) video.pause();
            if (Math.abs(video.currentTime - at) > 0.05) {
                for (let i = 0; i < video.seekable.length; i++) {
                    if (at >= video.seekable.start(i) && at <= video.seekable.end(i)) {
                        if (!video.paused) video.pause();
                        video.currentTime = at;
                        break;
                    }
                }
                return 'wait';
            }
            if (paused) {
                pausedRestore = attempt;
                return 'applied';
            }
            if (!video.paused) return 'applied';
            if (!resume) return 'play';
            attempt.playing = 'wait';
            video.play().then(() => {
                attempt.playing = restoreAttempt === attempt && sameMedia(attempt, currentMedia()) &&
                    Math.abs(video.currentTime - at) <= 0.25 ? 'applied' : undefined;
            }, () => { attempt.playing = 'blocked'; });
        } catch {
            return 'blocked';
        }
        return 'wait';
    }

    Object.defineProperty(window, '__nicoDesktopMedia', {
        value: Object.freeze({snapshot() { remember(); return lastSample; }, restore})
    });
})();
