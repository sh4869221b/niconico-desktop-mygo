const assert = require('node:assert/strict');
const {readFileSync} = require('node:fs');
const {test} = require('node:test');
const {runInNewContext} = require('node:vm');

const preload = readFileSync(require('node:path').join(__dirname, 'player_web.js'), 'utf8');
const origin = 'https://www.nicovideo.jp';
const a = origin + '/watch/sm9?from=60';
const b = origin + '/watch/sm38468751?playlist=related%3Asm9&ref=x+y#comments';
const search = origin + '/tag/test?l_range=1';
const watchRoute = 'routes/_web.watch.$id.$';

function player() {
    let content = [];
    const listeners = new Map();
    const navigation = new Map();
    const location = new URL(a);
    const router = {state: {}};
    const window = {
        __reactRouterDataRouter: router,
        navigation: {addEventListener: (name, listener) => navigation.set(name, listener)}
    };
    const document = {
        querySelectorAll(selector) {
            assert.equal(selector, '[data-name="stage"] video[data-name="video-content"]');
            return content;
        },
        addEventListener(name, listener) {
            if (!listeners.has(name)) listeners.set(name, []);
            listeners.get(name).push(listener);
        }
    };
    runInNewContext(preload, {window, document, location, URL, queueMicrotask});
    const video = {
        currentSrc: 'blob:media-a', currentTime: 60, paused: true, ended: false, readyState: 1, duration: 172,
        pause() { this.paused = true; },
        seekable: {length: 1, start: () => 0, end: () => 172}
    };
    function route(id, href, state = 'idle', ok = true) {
        location.href = href;
        router.state = {
            matches: [{route: {id: watchRoute}, params: {id}}],
            navigation: {state},
            loaderData: {[watchRoute]: {ok, data: {response: {$watchV4: {data: {video: {id, title: '同じタイトル'}}}}}}}
        };
    }
    function mediaEvent(name, target = video, isTrusted = true) {
        for (const handle of listeners.get(name)) handle({target, isTrusted});
    }
    content = [video];
    route('sm9', a);
    mediaEvent('loadstart');
    mediaEvent('loadedmetadata');

    return {
        video, route, mediaEvent,
        setContent(...videos) { content = videos; },
        snapshot() { return Array.from(window.__nicoDesktopMedia.snapshot()); },
        restore(...params) { return window.__nicoDesktopMedia.restore(...params); },
        leave() {
            navigation.get('navigate')({
                cancelable: false, downloadRequest: null, destination: {sameDocument: true, url: search}
            });
            location.href = search;
            content = [];
        }
    };
}

test('the confirmed same-document media keeps its full URL and state across immediate search', async t => {
    for (const paused of [true, false]) {
        await t.test(paused ? 'paused' : 'playing', () => {
            const p = player();
            p.route('sm38468751', b);
            const next = {...p.video, currentSrc: 'blob:media-b', currentTime: 85.25, paused};
            // Give the new element a real source lifecycle. No server-response meta exists.
            p.setContent(next);
            p.mediaEvent('loadstart', next);
            p.mediaEvent('pause', p.video); // delayed A event cannot supply B's time
            p.leave(); // snapshot in the navigation callback, before the host polls
            assert.deepEqual(p.snapshot(), [b, 85.25, paused]);
        });
    }
});

test('a pending or failed route cannot take ownership of A or capture unrelated media', () => {
    const p = player();
    p.route('sm38468751', b, 'loading');
    p.video.currentSrc = 'blob:pending';
    p.mediaEvent('loadstart');
    p.route('sm38468751', b, 'idle', false);
    p.mediaEvent('loadedmetadata');
    p.leave();
    assert.deepEqual(p.snapshot(), [a, 60, true]);
});

test('a reused element requires a new current source and a trusted content load', () => {
    const p = player();
    p.route('sm38468751', b);
    p.mediaEvent('loadstart'); // a late notification still has A's source
    p.mediaEvent('timeupdate');
    assert.deepEqual(p.snapshot(), [a, 60, true]);
    assert.equal(p.restore(1, 'sm38468751', 85.25, true, false), 'wait');

    p.video.currentSrc = 'blob:reuse-for-b';
    p.video.currentTime = 10;
    p.mediaEvent('loadstart', p.video, false);
    p.mediaEvent('loadedmetadata');
    assert.deepEqual(p.snapshot(), [a, 60, true]);
    p.mediaEvent('loadstart', {...p.video}); // unrelated player element
    assert.deepEqual(p.snapshot(), [a, 60, true]);
    p.mediaEvent('loadstart');
    assert.equal(p.restore(2, 'sm38468751', 85.25, true, false), 'applied');
    p.mediaEvent('seeked');
    p.video.currentSrc = 'blob:media-a'; // a superseded resource reappears
    p.video.currentTime = 60;
    p.mediaEvent('loadstart');
    p.mediaEvent('timeupdate');
    p.leave();
    assert.deepEqual(p.snapshot(), [b, 85.25, true]);
});
