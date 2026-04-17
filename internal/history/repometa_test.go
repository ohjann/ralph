package history

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// setDataDir isolates every test to its own RALPH_DATA_DIR.
func setDataDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RALPH_DATA_DIR", dir)
	return dir
}

func TestFingerprint_Stable(t *testing.T) {
	setDataDir(t)
	repo := t.TempDir()
	a, err := Fingerprint(repo)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	b, err := Fingerprint(repo)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	if a != b {
		t.Fatalf("fingerprint not stable: %q vs %q", a, b)
	}
	if len(a) != 12 {
		t.Fatalf("fingerprint len=%d want 12", len(a))
	}
}

func TestTouchRepo_FreshUpsert(t *testing.T) {
	setDataDir(t)
	repo := t.TempDir()
	fp, meta, err := TouchRepo(repo)
	if err != nil {
		t.Fatalf("TouchRepo: %v", err)
	}
	if meta.RunCount != 1 {
		t.Fatalf("RunCount=%d want 1", meta.RunCount)
	}
	if meta.FirstSeen.IsZero() || meta.LastSeen.IsZero() {
		t.Fatalf("timestamps unset")
	}
	absRepo, _ := filepath.Abs(repo)
	if meta.Path != absRepo {
		t.Fatalf("Path=%q want %q", meta.Path, absRepo)
	}
	// meta.json exists
	reposDir, _ := os.ReadFile(filepathMetaJSON(t, fp))
	if len(reposDir) == 0 {
		t.Fatalf("meta.json empty")
	}
}

func filepathMetaJSON(t *testing.T, fp string) string {
	t.Helper()
	dir := os.Getenv("RALPH_DATA_DIR")
	return filepath.Join(dir, "repos", fp, "meta.json")
}

func TestTouchRepo_Idempotent(t *testing.T) {
	setDataDir(t)
	repo := t.TempDir()
	fp1, m1, err := TouchRepo(repo)
	if err != nil {
		t.Fatalf("TouchRepo#1: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	fp2, m2, err := TouchRepo(repo)
	if err != nil {
		t.Fatalf("TouchRepo#2: %v", err)
	}
	if fp1 != fp2 {
		t.Fatalf("fingerprint changed: %q -> %q", fp1, fp2)
	}
	if m2.RunCount != 2 {
		t.Fatalf("RunCount=%d want 2", m2.RunCount)
	}
	if !m2.LastSeen.After(m1.LastSeen) && !m2.LastSeen.Equal(m1.LastSeen) {
		t.Fatalf("LastSeen not advanced")
	}
	if m2.FirstSeen != m1.FirstSeen {
		t.Fatalf("FirstSeen changed on re-touch: %v -> %v", m1.FirstSeen, m2.FirstSeen)
	}
}

// initGitRepo sets up a git repo with a single commit so gitFirstSHA returns
// a stable SHA. Skips the test if git is unavailable.
func initGitRepo(t *testing.T, dir string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "initial")
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	return string(out[:len(out)-1])
}

func TestTouchRepo_ReconcilePathMove(t *testing.T) {
	setDataDir(t)
	base := t.TempDir()
	oldPath := filepath.Join(base, "old")
	newPath := filepath.Join(base, "new")
	if err := os.Mkdir(oldPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	sha := initGitRepo(t, oldPath)

	fpOld, m1, err := TouchRepo(oldPath)
	if err != nil {
		t.Fatalf("TouchRepo old: %v", err)
	}
	if m1.GitFirstSHA != sha {
		t.Fatalf("GitFirstSHA=%q want %q", m1.GitFirstSHA, sha)
	}

	// Move the repo on disk.
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatalf("rename: %v", err)
	}

	fpNew, m2, err := TouchRepo(newPath)
	if err != nil {
		t.Fatalf("TouchRepo new: %v", err)
	}
	if fpOld == fpNew {
		t.Fatalf("expected fingerprint change")
	}
	if m2.GitFirstSHA != sha {
		t.Fatalf("post-move GitFirstSHA=%q want %q", m2.GitFirstSHA, sha)
	}
	if m2.RunCount != 2 {
		t.Fatalf("RunCount=%d want 2 (continued from old meta)", m2.RunCount)
	}
	if !pathEquals(t, m2.Path, newPath) {
		t.Fatalf("Path=%q want %q", m2.Path, newPath)
	}
	// Old fingerprint dir must be gone.
	dataDir := os.Getenv("RALPH_DATA_DIR")
	if _, err := os.Stat(filepath.Join(dataDir, "repos", fpOld)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old fingerprint dir still exists: err=%v", err)
	}
}

func pathEquals(t *testing.T, got, want string) bool {
	t.Helper()
	gAbs, _ := filepath.Abs(got)
	wAbs, _ := filepath.Abs(want)
	if gR, err := filepath.EvalSymlinks(gAbs); err == nil {
		gAbs = gR
	}
	if wR, err := filepath.EvalSymlinks(wAbs); err == nil {
		wAbs = wR
	}
	return gAbs == wAbs
}

func TestTouchRepo_NoReconcileWhenGitFirstSHAEmpty(t *testing.T) {
	setDataDir(t)
	base := t.TempDir()
	oldPath := filepath.Join(base, "old")
	newPath := filepath.Join(base, "new")
	if err := os.Mkdir(oldPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// No git init — GitFirstSHA stays empty.
	fpOld, m1, err := TouchRepo(oldPath)
	if err != nil {
		t.Fatalf("TouchRepo old: %v", err)
	}
	if m1.GitFirstSHA != "" {
		t.Fatalf("expected empty GitFirstSHA, got %q", m1.GitFirstSHA)
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatalf("rename: %v", err)
	}

	fpNew, m2, err := TouchRepo(newPath)
	if err != nil {
		t.Fatalf("TouchRepo new: %v", err)
	}
	if fpOld == fpNew {
		t.Fatalf("expected fingerprint change")
	}
	if m2.RunCount != 1 {
		t.Fatalf("RunCount=%d want 1 (no reconciliation possible)", m2.RunCount)
	}
	// Both fingerprint dirs exist — old one is orphaned.
	dataDir := os.Getenv("RALPH_DATA_DIR")
	if _, err := os.Stat(filepath.Join(dataDir, "repos", fpOld)); err != nil {
		t.Fatalf("old fingerprint dir gone: %v", err)
	}
}

func TestTouchRepo_ConcurrentWithFaultInjection(t *testing.T) {
	setDataDir(t)
	repo := t.TempDir()

	// Inject a one-shot mid-write fault for the first writer.
	var failed atomic.Int32
	prev := writeAtomicFn
	t.Cleanup(func() { writeAtomicFn = prev })
	writeAtomicFn = func(path string, data []byte) error {
		if failed.CompareAndSwap(0, 1) {
			return errors.New("injected write fault")
		}
		return writeAtomic(path, data)
	}

	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, err := TouchRepo(repo)
			errs[i] = err
		}(i)
	}
	wg.Wait()

	failures := 0
	successes := 0
	for _, e := range errs {
		if e != nil {
			failures++
		} else {
			successes++
		}
	}
	if failures != 1 {
		t.Fatalf("expected exactly 1 injected failure, got %d (successes=%d)", failures, successes)
	}
	if successes != 9 {
		t.Fatalf("expected 9 successes, got %d", successes)
	}

	// Meta.json must be valid JSON and reflect the completed writes.
	fp, err := Fingerprint(repo)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	data, err := os.ReadFile(filepathMetaJSON(t, fp))
	if err != nil {
		t.Fatalf("read meta.json: %v", err)
	}
	var m RepoMeta
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse meta.json: %v", err)
	}
	if m.RunCount != successes {
		t.Fatalf("RunCount=%d want %d", m.RunCount, successes)
	}

	// Lock sidecar must be released.
	dataDir := os.Getenv("RALPH_DATA_DIR")
	if _, err := os.Stat(filepath.Join(dataDir, "repos", fp+".lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock sidecar not released: err=%v", err)
	}
}

func TestLoadAllRepos(t *testing.T) {
	setDataDir(t)
	r1 := t.TempDir()
	r2 := t.TempDir()
	if _, _, err := TouchRepo(r1); err != nil {
		t.Fatalf("touch r1: %v", err)
	}
	if _, _, err := TouchRepo(r2); err != nil {
		t.Fatalf("touch r2: %v", err)
	}
	all, err := LoadAllRepos()
	if err != nil {
		t.Fatalf("LoadAllRepos: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("len=%d want 2", len(all))
	}
}

func TestLoadAllRepos_MissingReposDir(t *testing.T) {
	setDataDir(t)
	all, err := LoadAllRepos()
	if err != nil {
		t.Fatalf("LoadAllRepos: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected empty, got %d", len(all))
	}
}

func TestLoadAllReposWithFP(t *testing.T) {
	setDataDir(t)
	r1 := t.TempDir()
	r2 := t.TempDir()
	fp1, _, err := TouchRepo(r1)
	if err != nil {
		t.Fatalf("touch r1: %v", err)
	}
	fp2, _, err := TouchRepo(r2)
	if err != nil {
		t.Fatalf("touch r2: %v", err)
	}
	all, err := LoadAllReposWithFP()
	if err != nil {
		t.Fatalf("LoadAllReposWithFP: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("len=%d want 2", len(all))
	}
	byFP := map[string]RepoMeta{}
	for _, r := range all {
		if r.FP == "" {
			t.Fatalf("empty FP in entry: %+v", r)
		}
		byFP[r.FP] = r.Meta
	}
	if _, ok := byFP[fp1]; !ok {
		t.Fatalf("fp1 %q missing from result", fp1)
	}
	if _, ok := byFP[fp2]; !ok {
		t.Fatalf("fp2 %q missing from result", fp2)
	}
}

func TestLoadAllReposWithFP_MissingReposDir(t *testing.T) {
	setDataDir(t)
	all, err := LoadAllReposWithFP()
	if err != nil {
		t.Fatalf("LoadAllReposWithFP: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected empty, got %d", len(all))
	}
}

func TestLoadAllReposWithFP_SkipsUnparseableAndNonDir(t *testing.T) {
	setDataDir(t)
	repo := t.TempDir()
	fp, _, err := TouchRepo(repo)
	if err != nil {
		t.Fatalf("TouchRepo: %v", err)
	}
	dataDir := os.Getenv("RALPH_DATA_DIR")
	reposDir := filepath.Join(dataDir, "repos")

	// Non-dir entry alongside the valid fingerprint dir.
	if err := os.WriteFile(filepath.Join(reposDir, "stray.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write stray: %v", err)
	}
	// Sibling fingerprint dir with unparseable meta.json.
	badFP := "deadbeef0000"
	badDir := filepath.Join(reposDir, badFP)
	if err := os.Mkdir(badDir, 0o755); err != nil {
		t.Fatalf("mkdir bad: %v", err)
	}
	if err := os.WriteFile(filepath.Join(badDir, "meta.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write bad meta: %v", err)
	}

	all, err := LoadAllReposWithFP()
	if err != nil {
		t.Fatalf("LoadAllReposWithFP: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("len=%d want 1 (bad entries should be skipped)", len(all))
	}
	if all[0].FP != fp {
		t.Fatalf("FP=%q want %q", all[0].FP, fp)
	}
}

func TestLoadAllReposWithFP_FPFromDirNameAfterMove(t *testing.T) {
	setDataDir(t)
	base := t.TempDir()
	oldPath := filepath.Join(base, "old")
	newPath := filepath.Join(base, "new")
	if err := os.Mkdir(oldPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	initGitRepo(t, oldPath)

	if _, _, err := TouchRepo(oldPath); err != nil {
		t.Fatalf("TouchRepo old: %v", err)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatalf("rename: %v", err)
	}
	fpNew, _, err := TouchRepo(newPath)
	if err != nil {
		t.Fatalf("TouchRepo new: %v", err)
	}

	all, err := LoadAllReposWithFP()
	if err != nil {
		t.Fatalf("LoadAllReposWithFP: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("len=%d want 1 (reconcile moved dir)", len(all))
	}
	if all[0].FP != fpNew {
		t.Fatalf("FP=%q want %q (dir name)", all[0].FP, fpNew)
	}
	// FP must come from the dir name, which equals the current fp for newPath.
	recomputed, err := Fingerprint(all[0].Meta.Path)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	if recomputed != all[0].FP {
		t.Fatalf("dir-name FP %q diverges from recomputed %q", all[0].FP, recomputed)
	}
}

func TestUpdateLastRunID(t *testing.T) {
	setDataDir(t)
	repo := t.TempDir()
	fp, _, err := TouchRepo(repo)
	if err != nil {
		t.Fatalf("TouchRepo: %v", err)
	}
	if err := UpdateLastRunID(fp, "run-42"); err != nil {
		t.Fatalf("UpdateLastRunID: %v", err)
	}
	m, err := readMeta(fp)
	if err != nil {
		t.Fatalf("readMeta: %v", err)
	}
	if m.LastRunID != "run-42" {
		t.Fatalf("LastRunID=%q want run-42", m.LastRunID)
	}
	if m.RunCount != 1 {
		t.Fatalf("RunCount bumped by UpdateLastRunID: %d", m.RunCount)
	}
}

// TestDaemonShutdown_UpdateLastRunIDMatchesRunDir exercises the sequence the
// daemon child runs at shutdown: OpenRun creates runs/<runID>, Finalize stamps
// the manifest, UpdateLastRunID writes meta.LastRunID. Afterwards meta's
// LastRunID must resolve to a real directory under runs/.
func TestDaemonShutdown_UpdateLastRunIDMatchesRunDir(t *testing.T) {
	dataDir := setDataDir(t)
	repo := t.TempDir()

	hr, err := OpenRun(repo, "", "test-version", RunOpts{Kind: KindDaemon})
	if err != nil {
		t.Fatalf("OpenRun: %v", err)
	}
	if err := hr.Finalize(StatusComplete, Totals{}, nil); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if err := UpdateLastRunID(hr.RepoFP(), hr.ID()); err != nil {
		t.Fatalf("UpdateLastRunID: %v", err)
	}

	m, err := readMeta(hr.RepoFP())
	if err != nil {
		t.Fatalf("readMeta: %v", err)
	}
	if m.LastRunID != hr.ID() {
		t.Fatalf("LastRunID=%q want %q", m.LastRunID, hr.ID())
	}
	if m.RunCount != 1 {
		t.Fatalf("RunCount=%d want 1 (UpdateLastRunID must not bump)", m.RunCount)
	}

	runDir := filepath.Join(dataDir, "repos", hr.RepoFP(), "runs", m.LastRunID)
	if _, err := os.Stat(runDir); err != nil {
		t.Fatalf("LastRunID does not resolve to a run dir: %v", err)
	}
	if runDir != hr.Dir() {
		t.Fatalf("run dir mismatch: meta points to %q, OpenRun created %q", runDir, hr.Dir())
	}
}

// TestDaemonShutdown_RetroOverwritesLastRunID verifies that a retro run
// completing after a daemon run replaces LastRunID with its own id.
func TestDaemonShutdown_RetroOverwritesLastRunID(t *testing.T) {
	setDataDir(t)
	repo := t.TempDir()

	daemonRun, err := OpenRun(repo, "", "v", RunOpts{Kind: KindDaemon})
	if err != nil {
		t.Fatalf("OpenRun daemon: %v", err)
	}
	if err := daemonRun.Finalize(StatusComplete, Totals{}, nil); err != nil {
		t.Fatalf("Finalize daemon: %v", err)
	}
	if err := UpdateLastRunID(daemonRun.RepoFP(), daemonRun.ID()); err != nil {
		t.Fatalf("UpdateLastRunID daemon: %v", err)
	}

	retroRun, err := OpenRun(repo, "", "v", RunOpts{Kind: KindRetro})
	if err != nil {
		t.Fatalf("OpenRun retro: %v", err)
	}
	if err := retroRun.Finalize(StatusComplete, Totals{}, nil); err != nil {
		t.Fatalf("Finalize retro: %v", err)
	}
	if err := UpdateLastRunID(retroRun.RepoFP(), retroRun.ID()); err != nil {
		t.Fatalf("UpdateLastRunID retro: %v", err)
	}

	m, err := readMeta(retroRun.RepoFP())
	if err != nil {
		t.Fatalf("readMeta: %v", err)
	}
	if m.LastRunID != retroRun.ID() {
		t.Fatalf("LastRunID=%q want retro id %q (daemon id was %q)", m.LastRunID, retroRun.ID(), daemonRun.ID())
	}
}

// TestDaemonShutdown_ConcurrentReposIndependent verifies that two daemons in
// different repos each update their own LastRunID without cross-repo
// interference.
func TestDaemonShutdown_ConcurrentReposIndependent(t *testing.T) {
	setDataDir(t)
	repoA := t.TempDir()
	repoB := t.TempDir()

	runA, err := OpenRun(repoA, "", "v", RunOpts{Kind: KindDaemon})
	if err != nil {
		t.Fatalf("OpenRun A: %v", err)
	}
	runB, err := OpenRun(repoB, "", "v", RunOpts{Kind: KindDaemon})
	if err != nil {
		t.Fatalf("OpenRun B: %v", err)
	}

	if err := UpdateLastRunID(runA.RepoFP(), runA.ID()); err != nil {
		t.Fatalf("UpdateLastRunID A: %v", err)
	}
	if err := UpdateLastRunID(runB.RepoFP(), runB.ID()); err != nil {
		t.Fatalf("UpdateLastRunID B: %v", err)
	}

	mA, err := readMeta(runA.RepoFP())
	if err != nil {
		t.Fatalf("readMeta A: %v", err)
	}
	mB, err := readMeta(runB.RepoFP())
	if err != nil {
		t.Fatalf("readMeta B: %v", err)
	}
	if mA.LastRunID != runA.ID() {
		t.Fatalf("repoA LastRunID=%q want %q", mA.LastRunID, runA.ID())
	}
	if mB.LastRunID != runB.ID() {
		t.Fatalf("repoB LastRunID=%q want %q", mB.LastRunID, runB.ID())
	}
	if mA.LastRunID == mB.LastRunID {
		t.Fatalf("repos share LastRunID %q (cross-repo interference)", mA.LastRunID)
	}
}
