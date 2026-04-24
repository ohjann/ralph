package testintegrity

import (
	"strings"
	"testing"
)

// stubReader maps repo-relative paths to file contents for deterministic tests.
type stubReader struct {
	files map[string]string
}

func (s stubReader) ReadFile(path string) ([]byte, error) {
	// Caller passes an absolute projectDir + path; we keep only the tail
	// after the synthetic projectDir prefix so tests stay readable.
	for rel, content := range s.files {
		if strings.HasSuffix(path, rel) {
			return []byte(content), nil
		}
	}
	return nil, &notFoundErr{path: path}
}

type notFoundErr struct{ path string }

func (e *notFoundErr) Error() string { return "not found: " + e.path }

func runCheck(diff string, files map[string]string) Report {
	return check(diff, "/proj", stubReader{files: files})
}

func TestCheck_EmptyDiff(t *testing.T) {
	r := runCheck("", nil)
	if len(r.Findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(r.Findings))
	}
	if r.HasBlocker() {
		t.Fatalf("expected no blockers")
	}
}

func TestCheck_NonTestFileIgnored(t *testing.T) {
	diff := `diff --git a/internal/foo/foo.go b/internal/foo/foo.go
--- a/internal/foo/foo.go
+++ b/internal/foo/foo.go
@@ -1,2 +1,3 @@
 package foo
+func Bar() int { return 1 }
`
	r := runCheck(diff, map[string]string{
		"internal/foo/foo.go": "package foo\nfunc Bar() int { return 1 }\n",
	})
	if len(r.Findings) != 0 {
		t.Fatalf("expected no findings on non-test file, got %+v", r.Findings)
	}
}

func TestCheck_Go_TautologicalEqual(t *testing.T) {
	diff := `diff --git a/foo_test.go b/foo_test.go
--- a/foo_test.go
+++ b/foo_test.go
@@ -1,1 +1,5 @@
 package foo
+func TestThing(t *testing.T) {
+	assert.Equal(t, 1, 1)
+}
`
	content := `package foo

import "testing"

func TestThing(t *testing.T) {
	assert.Equal(t, 1, 1)
}
`
	r := runCheck(diff, map[string]string{"foo_test.go": content})
	if !r.HasBlocker() {
		t.Fatalf("expected blocker for tautological assertion, got %+v", r.Findings)
	}
	found := false
	for _, f := range r.Findings {
		if f.Rule == "tautological-assertion" && f.Severity == SeverityCritical {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected tautological-assertion CRITICAL, got %+v", r.Findings)
	}
}

func TestCheck_Go_AssertTrueLiteral(t *testing.T) {
	diff := `diff --git a/foo_test.go b/foo_test.go
--- a/foo_test.go
+++ b/foo_test.go
@@ -1,1 +1,5 @@
 package foo
+func TestThing(t *testing.T) {
+	require.True(t, true)
+}
`
	content := `package foo

import "testing"

func TestThing(t *testing.T) {
	require.True(t, true)
}
`
	r := runCheck(diff, map[string]string{"foo_test.go": content})
	if !r.HasBlocker() {
		t.Fatalf("expected blocker for require.True(t, true), got %+v", r.Findings)
	}
}

func TestCheck_Go_EmptyTestBody(t *testing.T) {
	diff := `diff --git a/foo_test.go b/foo_test.go
--- a/foo_test.go
+++ b/foo_test.go
@@ -1,1 +1,4 @@
 package foo
+func TestEmpty(t *testing.T) {
+	// TODO: implement
+}
`
	content := `package foo

import "testing"

func TestEmpty(t *testing.T) {
	// TODO: implement
}
`
	r := runCheck(diff, map[string]string{"foo_test.go": content})
	if !r.HasBlocker() {
		t.Fatalf("expected blocker for empty test body, got %+v", r.Findings)
	}
	found := false
	for _, f := range r.Findings {
		if f.Rule == "empty-test-body" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected empty-test-body finding, got %+v", r.Findings)
	}
}

func TestCheck_Go_RealTest_NoFindings(t *testing.T) {
	diff := `diff --git a/foo_test.go b/foo_test.go
--- a/foo_test.go
+++ b/foo_test.go
@@ -1,1 +1,5 @@
 package foo
+func TestThing(t *testing.T) {
+	got := Bar()
+	if got != 42 { t.Fatalf("want 42, got %d", got) }
+}
`
	content := `package foo

import "testing"

func TestThing(t *testing.T) {
	got := Bar()
	if got != 42 {
		t.Fatalf("want 42, got %d", got)
	}
}
`
	r := runCheck(diff, map[string]string{"foo_test.go": content})
	if r.HasBlocker() {
		t.Fatalf("did not expect blocker on a real test, got %+v", r.Findings)
	}
}

func TestCheck_TS_NoSourceImport(t *testing.T) {
	diff := `diff --git a/app/thing.test.ts b/app/thing.test.ts
--- a/app/thing.test.ts
+++ b/app/thing.test.ts
@@ -0,0 +1,6 @@
+import { describe, it, expect } from 'vitest';
+
+describe('thing', () => {
+  it('works', () => {
+    expect(2).toBe(2);
+  });
+});
`
	content := `import { describe, it, expect } from 'vitest';

describe('thing', () => {
  it('works', () => {
    expect(2).toBe(2);
  });
});
`
	r := runCheck(diff, map[string]string{"app/thing.test.ts": content})
	if !r.HasBlocker() {
		t.Fatalf("expected blocker for no-source-import + tautology, got %+v", r.Findings)
	}
	hasNoImport := false
	hasTauto := false
	for _, f := range r.Findings {
		if f.Rule == "no-source-import" {
			hasNoImport = true
		}
		if f.Rule == "tautological-assertion" {
			hasTauto = true
		}
	}
	if !hasNoImport {
		t.Fatalf("expected no-source-import finding")
	}
	if !hasTauto {
		t.Fatalf("expected tautological-assertion finding")
	}
}

func TestCheck_TS_RealTest_NoFindings(t *testing.T) {
	diff := `diff --git a/app/thing.test.ts b/app/thing.test.ts
--- a/app/thing.test.ts
+++ b/app/thing.test.ts
@@ -0,0 +1,8 @@
+import { describe, it, expect } from 'vitest';
+import { thing } from './thing';
+
+describe('thing', () => {
+  it('works', () => {
+    expect(thing(2)).toBe(4);
+  });
+});
`
	content := `import { describe, it, expect } from 'vitest';
import { thing } from './thing';

describe('thing', () => {
  it('works', () => {
    expect(thing(2)).toBe(4);
  });
});
`
	r := runCheck(diff, map[string]string{"app/thing.test.ts": content})
	if r.HasBlocker() {
		t.Fatalf("did not expect blocker on a real TS test, got %+v", r.Findings)
	}
}

func TestCheck_TS_EmptyTestBody(t *testing.T) {
	diff := `diff --git a/app/thing.test.ts b/app/thing.test.ts
--- a/app/thing.test.ts
+++ b/app/thing.test.ts
@@ -0,0 +1,6 @@
+import { thing } from './thing';
+import { describe, it } from 'vitest';
+
+describe('thing', () => {
+  it('works', () => {});
+});
`
	content := `import { thing } from './thing';
import { describe, it } from 'vitest';

describe('thing', () => {
  it('works', () => {});
});
`
	r := runCheck(diff, map[string]string{"app/thing.test.ts": content})
	if !r.HasBlocker() {
		t.Fatalf("expected blocker for empty TS test body, got %+v", r.Findings)
	}
}

func TestCheck_Python_AssertTrue(t *testing.T) {
	diff := `diff --git a/tests/test_thing.py b/tests/test_thing.py
--- a/tests/test_thing.py
+++ b/tests/test_thing.py
@@ -0,0 +1,3 @@
+def test_thing():
+    assert True
`
	content := `def test_thing():
    assert True
`
	r := runCheck(diff, map[string]string{"tests/test_thing.py": content})
	if !r.HasBlocker() {
		t.Fatalf("expected blocker for assert True, got %+v", r.Findings)
	}
}

func TestCheck_Python_EmptyTest(t *testing.T) {
	diff := `diff --git a/tests/test_thing.py b/tests/test_thing.py
--- a/tests/test_thing.py
+++ b/tests/test_thing.py
@@ -0,0 +1,3 @@
+def test_empty():
+    pass
`
	content := `def test_empty():
    pass
`
	r := runCheck(diff, map[string]string{"tests/test_thing.py": content})
	if !r.HasBlocker() {
		t.Fatalf("expected HIGH blocker for empty python test, got %+v", r.Findings)
	}
}

func TestCheck_Python_RealTest_NoFindings(t *testing.T) {
	diff := `diff --git a/tests/test_thing.py b/tests/test_thing.py
--- a/tests/test_thing.py
+++ b/tests/test_thing.py
@@ -0,0 +1,5 @@
+from mymod import thing
+
+def test_thing():
+    assert thing(2) == 4
`
	content := `from mymod import thing

def test_thing():
    assert thing(2) == 4
`
	r := runCheck(diff, map[string]string{"tests/test_thing.py": content})
	if r.HasBlocker() {
		t.Fatalf("did not expect blocker on real python test, got %+v", r.Findings)
	}
}

func TestCheck_Mutation_SignalOnly(t *testing.T) {
	diff := `diff --git a/foo_test.go b/foo_test.go
--- a/foo_test.go
+++ b/foo_test.go
@@ -1,10 +1,10 @@
 package foo

 func TestA(t *testing.T) {
-	assert.Equal(t, got, 1)
+	assert.Equal(t, got, 2)
 }
 func TestB(t *testing.T) {
-	assert.Equal(t, got, 5)
+	assert.Equal(t, got, 6)
 }
`
	content := `package foo

import "testing"

func TestA(t *testing.T) {
	got := 2
	assert.Equal(t, got, 2)
}
func TestB(t *testing.T) {
	got := 6
	assert.Equal(t, got, 6)
}
`
	r := runCheck(diff, map[string]string{"foo_test.go": content})
	// Mutation is Low severity — must NOT block.
	if r.HasBlocker() {
		t.Fatalf("mutation detector must not block: got blockers %+v", r.Blockers())
	}
	found := false
	for _, f := range r.Findings {
		if f.Rule == "assertion-value-churn" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected assertion-value-churn signal, got %+v", r.Findings)
	}
}

func TestCheck_Mutation_SingleChangeNotFlagged(t *testing.T) {
	diff := `diff --git a/foo_test.go b/foo_test.go
--- a/foo_test.go
+++ b/foo_test.go
@@ -1,5 +1,5 @@
 package foo
 func TestA(t *testing.T) {
-	assert.Equal(t, got, 1)
+	assert.Equal(t, got, 2)
 }
`
	content := `package foo

import "testing"

func TestA(t *testing.T) {
	got := 2
	assert.Equal(t, got, 2)
}
`
	r := runCheck(diff, map[string]string{"foo_test.go": content})
	for _, f := range r.Findings {
		if f.Rule == "assertion-value-churn" {
			t.Fatalf("single mutation should not signal, got %+v", r.Findings)
		}
	}
}

func TestCheck_DeletedFileSkipped(t *testing.T) {
	diff := `diff --git a/foo_test.go b/foo_test.go
--- a/foo_test.go
+++ /dev/null
@@ -1,3 +0,0 @@
-package foo
-
-func TestA(t *testing.T) {}
`
	r := runCheck(diff, map[string]string{})
	if len(r.Findings) != 0 {
		t.Fatalf("expected deleted files to be skipped, got %+v", r.Findings)
	}
}

func TestCheck_SkippedTestLowSignal(t *testing.T) {
	diff := `diff --git a/foo_test.go b/foo_test.go
--- a/foo_test.go
+++ b/foo_test.go
@@ -0,0 +1,5 @@
+package foo
+func TestA(t *testing.T) {
+	t.Skip("flaky")
+	assert.Equal(t, Bar(), 42)
+}
`
	content := `package foo

import "testing"

func TestA(t *testing.T) {
	t.Skip("flaky")
	assert.Equal(t, Bar(), 42)
}
`
	r := runCheck(diff, map[string]string{"foo_test.go": content})
	if r.HasBlocker() {
		t.Fatalf("skip is Low severity; must not block, got %+v", r.Findings)
	}
	found := false
	for _, f := range r.Findings {
		if f.Rule == "skipped-test" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected skipped-test signal, got %+v", r.Findings)
	}
}

func TestIsTestFile(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"foo_test.go", true},
		{"internal/foo/bar_test.go", true},
		{"foo.go", false},
		{"app/thing.test.ts", true},
		{"app/thing.spec.tsx", true},
		{"src/__tests__/thing.ts", true},
		{"src/thing.ts", false},
		{"tests/test_thing.py", true},
		{"tests/thing_test.py", true},
		{"src/thing.py", false},
	}
	for _, c := range cases {
		if got := isTestFile(c.path); got != c.want {
			t.Errorf("isTestFile(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestParseDiffFiles_MultipleFiles(t *testing.T) {
	diff := `diff --git a/a_test.go b/a_test.go
--- a/a_test.go
+++ b/a_test.go
@@ -1,2 +1,3 @@
 package a
+// hi
diff --git a/b_test.go b/b_test.go
--- a/b_test.go
+++ b/b_test.go
@@ -1,1 +1,2 @@
 package b
+// yo
`
	files := parseDiffFiles(diff)
	if len(files) != 2 {
		t.Fatalf("want 2 files, got %d: %+v", len(files), files)
	}
	if files[0].path != "a_test.go" || files[1].path != "b_test.go" {
		t.Fatalf("unexpected paths: %+v", files)
	}
}

func TestFormatReport_NonEmpty(t *testing.T) {
	r := Report{Findings: []Finding{
		{Severity: SeverityCritical, File: "a_test.go", Line: 3, Rule: "tautological-assertion", Message: "noop"},
		{Severity: SeverityLow, File: "b_test.go", Rule: "skipped-test", Message: "skipped"},
	}}
	got := FormatReport(r)
	if !strings.Contains(got, "CRITICAL") || !strings.Contains(got, "Blockers") {
		t.Fatalf("expected blockers section in format, got:\n%s", got)
	}
	if !strings.Contains(got, "LOW") || !strings.Contains(got, "Signals") {
		t.Fatalf("expected signals section in format, got:\n%s", got)
	}
}
