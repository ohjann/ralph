# Appellate Review Prompt

A primary judge has rejected this story **{{REJECTION_COUNT}} times**. The implementer maintains the work is complete. Your role is not to re-review the code; it is to decide whether the judge's repeated objections are grounded in the acceptance criteria or have become pedantic.

## Story Under Review

**ID:** {{STORY_ID}}
**Title:** {{STORY_TITLE}}
**Description:** {{STORY_DESCRIPTION}}

## Acceptance Criteria

{{ACCEPTANCE_CRITERIA}}

## Code Diff

```diff
{{DIFF}}
```

## Judge's Rejection History

{{FEEDBACK_HISTORY}}

## Your Task

Evaluate the judge's reasoning against the acceptance criteria above. You are **not** a second code reviewer. Do not introduce new objections. Do not score code style, test coverage, or architecture unless an acceptance criterion explicitly demands it.

### PASS (override the judge) when:
- The judge's objections are stylistic, pedantic, or about concerns not listed in the acceptance criteria.
- The judge moved the goalposts across rejections (e.g. rejected on reason A, implementer fixed A, judge then rejected on reason B that was not previously flagged and is not in the acceptance criteria).
- The acceptance criteria are all visibly satisfied by the diff, and the judge's objections are not tied to any specific criterion.

### FAIL (uphold the judge) when:
- The judge's objections are tied to concrete acceptance criteria and the diff genuinely does not satisfy them.
- A required behaviour named in the acceptance criteria is missing, placeholder, or stubbed.
- The diff contains work that contradicts an acceptance criterion.

### Bias
When the acceptance criteria are met and the judge's complaints are stylistic, PASS. When an acceptance criterion has no matching code, FAIL — even if the judge's stated reason was phrased clumsily.

## Required Output

Return ONLY raw JSON (no markdown fences, no commentary):

{"verdict":"PASS or FAIL","criteria_met":["list of criteria satisfied"],"criteria_failed":["list of criteria not met, empty if PASS"],"reason":"one sentence explaining why the judge was (or was not) correct","suggestion":"if FAIL, specific guidance tied to the failed criteria. empty string if PASS"}
