package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ohjann/ralphplusplus/internal/debuglog"
	"github.com/ohjann/ralphplusplus/internal/history"
	"github.com/ohjann/ralphplusplus/internal/prd"
)

// displayNameTimeout caps how long we wait for the utility-model CLI to
// return a slug. Missing this budget falls back to the deterministic
// adjective-noun name — a better run still starts on time.
const displayNameTimeout = 12 * time.Second

// deriveRunDisplayName asks the configured utility model (usually haiku)
// for a short kebab-case summary of the PRD's first few stories. Returns
// "" on any failure — callers treat that as "use the deterministic default".
//
// The prompt deliberately asks for ONLY the slug so we can trust the whole
// response after trimming. We still validate against the slug shape before
// accepting; random prose or an apology sentence will fail IsValidDisplayName
// and we fall back gracefully.
func deriveRunDisplayName(ctx context.Context, prdFile, utilityModel string) string {
	if prdFile == "" {
		return ""
	}
	p, err := prd.Load(prdFile)
	if err != nil {
		return ""
	}
	stories := incompleteStoryContext(p, 3)
	if stories == "" {
		return ""
	}

	prompt := buildDisplayNamePrompt(p.Project, stories)
	cctx, cancel := context.WithTimeout(ctx, displayNameTimeout)
	defer cancel()

	args := []string{"--dangerously-skip-permissions", "-p", "--output-format", "text"}
	if utilityModel != "" {
		args = append(args, "--model", utilityModel)
	}
	cmd := exec.CommandContext(cctx, "claude", args...)
	cmd.Stdin = strings.NewReader(prompt)
	out, err := cmd.Output()
	if err != nil {
		debuglog.Log("displayname: claude invocation failed: %v", err)
		return ""
	}

	name := extractSlug(string(out))
	if !history.IsValidDisplayName(name) {
		debuglog.Log("displayname: rejected LLM output %q", strings.TrimSpace(string(out)))
		return ""
	}
	return name
}

func buildDisplayNamePrompt(project, stories string) string {
	projectLine := ""
	if project != "" {
		projectLine = "Project: " + project + "\n\n"
	}
	return fmt.Sprintf(`%sSummarise the work described below as a short kebab-case slug (2-4 words, lowercase, hyphen-separated, no quotes, no punctuation).

Examples of good slugs: add-tailscale-integration, fix-login-race, migrate-postgres-schema.

Work to summarise:
%s

Respond with ONLY the slug on a single line. No preamble, no explanation, no markdown.`, projectLine, stories)
}

// incompleteStoryContext returns up to max titles (+ one-line descriptions) of
// stories that have not passed, trimmed to keep the prompt short.
func incompleteStoryContext(p *prd.PRD, max int) string {
	var b strings.Builder
	n := 0
	for _, s := range p.UserStories {
		if s.Passes {
			continue
		}
		title := strings.TrimSpace(s.Title)
		if title == "" {
			continue
		}
		fmt.Fprintf(&b, "- %s", title)
		if d := trimFirstLine(s.Description, 140); d != "" {
			fmt.Fprintf(&b, " — %s", d)
		}
		b.WriteByte('\n')
		n++
		if n >= max {
			break
		}
	}
	return strings.TrimSpace(b.String())
}

func trimFirstLine(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}

// extractSlug collapses the CLI response to a single kebab token. Models
// occasionally wrap the slug in backticks or add a trailing period; be
// tolerant of both without opening the door to fully-freeform output
// (IsValidDisplayName enforces the rest).
func extractSlug(out string) string {
	s := strings.TrimSpace(out)
	// First non-empty line only — multi-line answers fail validation anyway.
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	s = strings.Trim(s, "`\"' .,")
	s = strings.ToLower(s)
	return s
}
