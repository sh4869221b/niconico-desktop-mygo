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
    const listeners = new Map();
    const navigation = new Map();
    const location = new URL(a);
    const router = {state: {}};
    const window = {
        __reactRouterDataRouter: router,
        navigation: {addEventListener: (name, listener) => navigation.set(name, listener)}
    };
    const presenter = {};
    const stage = {parentElement: {'__reactFiber$fixture': {return: presenter}}};
    let content;
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
    class HTMLVideoElement {
        currentSrc = 'blob:a';
        currentTime = 60;
        paused = true;
        ended = false;
        seeking = false;
        readyState = 1;
        duration = 172;
        seekable = {length: 1, start: () => 0, end: () => 172};
        closest() { return stage; }
        pause() { this.paused = true; }
        play() { this.paused = false; return Promise.resolve(); }
    }
    const video = new HTMLVideoElement();
    content = [video];
    runInNewContext(preload, {window, document, location, URL, HTMLVideoElement, queueMicrotask});

    function route(requested, href, id = requested, ok = true, navigationType = 'push') {
        location.href = href;
        navigation.get('currententrychange')({navigationType});
        router.state = {
            location: {pathname: location.pathname},
            matches: [{route: {id: watchRoute}, params: {id: requested}}],
            errors: ok ? null : {[watchRoute]: 'failure'},
            loaderData: {[watchRoute]: {ok, data: {response: {$watchV4: {data: {video: {id, title: '同じタイトル'}}}}}}}
        };
    }
    function controller(id, source, element = video, native = false) {
        const watch = {video: {id}};
        const contentUrl = native ? source : origin + '/fixture/' + id + '.m3u8';
        const session = {video: element, watch, accessRights: {contentUrl}};
        if (native) element.src = source;
        else session.hlsjs = {url: contentUrl, media: element,
            bufferController: {media: element, _objectUrl: source, mediaSrc: source}};
        return {stage, view: {videoContent: element}, watch, session, _isInterrupting: false, _isDummyVideo: false};
    }
    function present(...controllers) {
        presenter.memoizedState = controllers.reduceRight((next, c) => ({memoizedState: [c], next}), null);
    }
    function mediaEvent(name, target = video, isTrusted = true) {
        for (const handle of listeners.get(name) ?? []) handle({target, isTrusted});
    }
    const old = controller('sm9', video.currentSrc);
    present(old);
    route('sm9', a);
    mediaEvent('loadedmetadata');

    return {
        old, video, route, controller, present, mediaEvent,
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

test('B can load before its route and survive an immediate search while paused or playing', () => {
    for (const paused of [true, false]) {
        const p = player();
        p.video.currentSrc = 'blob:b';
        p.video.currentTime = 85.25;
        p.video.paused = paused;
        p.present(p.controller('sm38468751', p.video.currentSrc));
        p.mediaEvent('loadedmetadata');
        assert.deepEqual(p.snapshot(), [a, 'sm9', 60, true]);

        p.route('sm38468751', b);
        p.leave();
        assert.deepEqual(p.snapshot(), [b, 'sm38468751', 85.25, paused]);
    }
});

test('a delayed A reload is still A even after B commits, and a failed B does not displace A', () => {
    for (const ok of [true, false]) {
        const p = player();
        p.route('sm38468751', b, 'sm38468751', ok);
        p.mediaEvent('emptied');
        p.video.currentSrc = 'blob:reloaded-a';
        p.video.currentTime = 2;
        const reloaded = p.controller('sm9', p.video.currentSrc);
        p.present(p.controller('sm38468751', 'blob:pending-b'), reloaded);
        p.mediaEvent('loadstart');
        p.mediaEvent('loadedmetadata');
        p.leave();
        assert.deepEqual(p.snapshot(), [a, 'sm9', 60, true]);
    }
});

test('a verified B can load after its route while unmatched sources, ads, and dummy videos are ignored', () => {
    const p = player();
    p.route('sm38468751', b);
    const next = p.controller('sm38468751', 'blob:b');
    p.present(next);
    assert.deepEqual(p.snapshot(), [a, 'sm9', 60, true]);
    p.video.currentSrc = 'blob:ad';
    p.video.currentTime = 5;
    assert.deepEqual(p.snapshot(), [a, 'sm9', 60, true]);
    p.video.currentSrc = 'blob:b';
    for (const flag of ['_isInterrupting', '_isDummyVideo']) {
        next[flag] = true;
        assert.deepEqual(p.snapshot(), [a, 'sm9', 60, true]);
        next[flag] = false;
    }
    next.session.hlsjs.interstitialsManager = {playingItem: {event: {}}};
    assert.deepEqual(p.snapshot(), [a, 'sm9', 60, true]);
    delete next.session.hlsjs.interstitialsManager;
    p.present(); // A future official player shape without a verifiable controller.
    assert.deepEqual(p.snapshot(), [a, 'sm9', 60, true]);
    p.present(next);
    p.video.currentTime = 85.25;
    p.mediaEvent('pause', new p.video.constructor());
    p.leave();
    assert.deepEqual(p.snapshot(), [b, 'sm38468751', 85.25, true]);
});

test('a numeric alias keeps its requested URL, while its metadata must match the session', () => {
    const p = player();
    const alias = origin + '/watch/1173108780?ref=x+y#comments';
    p.route('1173108780', alias, 'sm38468751');
    assert.deepEqual(p.snapshot(), [a, 'sm9', 60, true]);
    assert.equal(p.restore(1, 'sm38468751', 85.25, true, false), 'wait');
    p.route('sm38468751', b, 'sm9');
    assert.deepEqual(p.snapshot(), [a, 'sm9', 60, true]);
    p.route('1173108780', alias, 'sm9');
    assert.deepEqual(p.snapshot(), [alias, 'sm9', 60, true]);
    p.route('1173108780', origin + '/watch/1173108780', 'sm9', true, 'replace');
    assert.deepEqual(p.snapshot(), [alias, 'sm9', 60, true]);
    assert.equal(p.restore(2, 'sm9', 85.25, true, false), 'wait');
    assert.equal(p.restore(2, 'sm9', 85.25, true, false), 'applied');
    p.leave();
    assert.deepEqual(p.snapshot(), [alias, 'sm9', 85.25, true]);
});

test('a clean watch URL never inherits the options of a different history entry', () => {
    const p = player();
    const next = origin + '/watch/sm9?playlist=new&ref=other';
    p.route('sm9', next, 'sm9', true, 'replace');
    assert.deepEqual(p.snapshot(), [next, 'sm9', 60, true]);
    const bare = origin + '/watch/sm9';
    p.route('sm9', bare);
    assert.deepEqual(p.snapshot(), [bare, 'sm9', 60, true]);
});

test('restore does not reuse a seek or play acknowledgement across a new media generation', async () => {
    const p = player();
    const restore = resume => p.restore(1, 'sm9', 85.25, false, resume);
    assert.equal(restore(false), 'wait');
    p.video.seeking = true;
    assert.equal(restore(false), 'wait');
    p.video.seeking = false;
    assert.equal(restore(false), 'play');

    p.video.currentSrc = 'https://media.example/new.mp4';
    p.video.currentTime = 0.46;
    p.present(p.controller('sm9', p.video.currentSrc, p.video, true));
    assert.equal(restore(true), 'wait'); // The old gesture must not start the new source at 0.46.
    assert.equal(p.video.currentTime, 85.25);
    assert.equal(restore(false), 'play');
    assert.equal(restore(true), 'wait');
    await Promise.resolve();
    assert.equal(restore(false), 'applied');

    p.mediaEvent('emptied');
    p.video.currentTime = 0.46;
    assert.equal(restore(false), 'wait'); // A reload of the same currentSrc is a new generation too.
    assert.equal(p.video.currentTime, 85.25);
    assert(p.video.paused);
});

test('an old play promise resolving after replacement cannot acknowledge the new source', async () => {
    const p = player();
    let finishOldPlay;
    p.video.currentTime = 85.25;
    p.video.play = () => new Promise(resolve => { finishOldPlay = resolve; });
    assert.equal(p.restore(1, 'sm9', 85.25, false, true), 'wait');
    p.video.currentSrc = 'blob:replacement';
    p.video.currentTime = 0.46;
    p.present(p.controller('sm9', p.video.currentSrc));
    finishOldPlay();
    await Promise.resolve();
    assert.equal(p.restore(1, 'sm9', 85.25, false, false), 'wait');
    assert.equal(p.video.currentTime, 85.25);
    assert.equal(p.restore(1, 'sm9', 85.25, false, false), 'play');
});
