package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

const settingsWorkerDirectory = "MYGO_TEST_SHARED_SETTINGS"

func TestLibraryAndNGKeepEditsFromTwoProcesses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")
	entryFor := func(id string) entry {
		e, _ := makeEntry(home+"watch/sm"+id, "既存の動画")
		return e
	}
	seed := []entry{entryFor("11"), entryFor("12"), entryFor("13")}
	if err := saveLibrary(path, library{Pins: seed, History: seed}); err != nil {
		t.Fatal(err)
	}
	ng, err := loadCommentNG(filepath.Join(dir, "comment-ng.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"11", "12", "13"} {
		if err := ng.change(commentAuthor{ID: id}, true); err != nil {
			t.Fatal(err)
		}
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	type child struct {
		process *exec.Cmd
		output  bytes.Buffer
	}
	children := []child{{}, {}}
	for i := range children {
		id := strconv.Itoa(i + 1)
		c := &children[i]
		c.process = exec.CommandContext(ctx, exe, "-test.run=^TestSettingsProcessWorker$")
		c.process.Env = append(os.Environ(), settingsWorkerDirectory+"="+dir, "MYGO_TEST_WORKER="+id)
		c.process.Stdout, c.process.Stderr = &c.output, &c.output
		if err := c.process.Start(); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if c.process.ProcessState == nil {
				c.process.Process.Kill()
				c.process.Wait()
			}
		}()
	}
	for _, i := range []string{"1", "2"} {
		waitForTestFile(t, ctx, filepath.Join(dir, "ready-"+i))
	}
	if err := os.WriteFile(filepath.Join(dir, "start"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, c := range children {
		if err := c.process.Wait(); err != nil {
			t.Fatalf("worker failed: %v\n%s", err, c.output.String())
		}
	}
	library, err := loadLibrary(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, list := range [][]entry{library.Pins, library.History} {
		var ids []string
		for _, e := range list {
			ids = append(ids, routeFor(e.URL).key)
		}
		slices.Sort(ids)
		if !slices.Equal(ids, []string{"sm13", "sm21", "sm22"}) {
			t.Fatalf("an addition or removal was lost: %v", ids)
		}
	}
	ng, err = loadCommentNG(ng.path)
	if err != nil || !slices.Equal(ng.entries, []commentAuthor{{ID: "13"}, {ID: "21"}, {ID: "22"}}) {
		t.Fatalf("an NG change was lost: %+v, %v", ng.entries, err)
	}
}

func waitForTestFile(t *testing.T, ctx context.Context, path string) {
	t.Helper()
	for {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %s", filepath.Base(path))
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestSettingsProcessWorker(t *testing.T) {
	dir, id := os.Getenv(settingsWorkerDirectory), os.Getenv("MYGO_TEST_WORKER")
	if dir == "" {
		t.Skip("subprocess helper")
	}
	p := &playerWindow{}
	b := &browser{index: -1, libraryPath: filepath.Join(dir, "library.json"), player: p, closed: true}
	var err error
	b.data, err = loadLibrary(b.libraryPath)
	if err != nil {
		t.Fatal(err)
	}
	b.commentNG, err = loadCommentNG(filepath.Join(dir, "comment-ng.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ready-"+id), nil, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	waitForTestFile(t, ctx, filepath.Join(dir, "start"))
	url := home + "watch/sm2" + id
	b.navigate(url)
	b.result.title = "新しい動画 " + id
	view := ui.NewTester(b.draw, 960, 650)
	if err := view.Click("☆ 登録"); err != nil {
		t.Fatal(err)
	}
	b.recordVideo(p, url, b.result.title)
	old, _ := makeEntry(home+"watch/sm1"+id, "")
	b.remove(old)
	b.history = true
	b.remove(old)
	n := &nativePlayer{}
	for _, edit := range []commentNGEdit{
		{author: commentAuthor{ID: "2" + id}, block: true},
		{author: commentAuthor{ID: "1" + id}, block: false},
	} {
		n.pendingNG = edit
		b.applyPendingCommentNG(n)
		if n.ngNotice == "NG設定を保存できませんでした。" {
			t.Fatal(n.ngNotice)
		}
	}
}

func TestLibraryMutationDoesNotOverwriteNewlyCorruptedFile(t *testing.T) {
	b := &browser{index: -1, closed: true, libraryPath: filepath.Join(t.TempDir(), "library.json")}
	const corrupt = "{"
	if err := os.WriteFile(b.libraryPath, []byte(corrupt), 0600); err != nil {
		t.Fatal(err)
	}
	b.navigate(home + "watch/sm9")
	b.pinCurrent()
	data, err := os.ReadFile(b.libraryPath)
	if err != nil || string(data) != corrupt {
		t.Fatalf("failed operation changed saved data: %q (%v)", data, err)
	}
	if len(b.data.Pins) != 1 || b.status == "" {
		t.Fatal("save failure was not reflected in the current session")
	}
}
