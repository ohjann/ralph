#!/bin/bash
# Ralph - Long-running AI agent loop
# Usage: ralph [--tool amp|claude] [--dir <path>] [max_iterations]

set -e

# --- Defaults ---
TOOL="claude"
MAX_ITERATIONS=10
PROJECT_DIR=""

# --- Help ---
usage() {
  cat <<'EOF'
Usage: ralph [options] [max_iterations]

Run the Ralph autonomous agent loop against a prd.json in the current directory.

Options:
  --tool <amp|claude>   AI tool to use (default: claude)
  --dir <path>          Project directory containing prd.json (default: current directory)
  --help, -h            Show this help message

Arguments:
  max_iterations        Maximum loop iterations (default: 10)

Examples:
  ralph                 Run with claude, 10 iterations, prd.json in current dir
  ralph 5               Run with claude, 5 iterations
  ralph --tool amp      Run with amp instead of claude
  ralph --dir ~/myapp   Run against prd.json in ~/myapp
EOF
  exit 0
}

# --- Parse arguments ---
while [[ $# -gt 0 ]]; do
  case $1 in
    --help|-h)
      usage
      ;;
    --tool)
      TOOL="$2"
      shift 2
      ;;
    --tool=*)
      TOOL="${1#*=}"
      shift
      ;;
    --dir)
      PROJECT_DIR="$2"
      shift 2
      ;;
    --dir=*)
      PROJECT_DIR="${1#*=}"
      shift
      ;;
    *)
      if [[ "$1" =~ ^[0-9]+$ ]]; then
        MAX_ITERATIONS="$1"
      else
        echo "Error: Unknown argument '$1'. Use --help for usage."
        exit 1
      fi
      shift
      ;;
  esac
done

# --- Validate ---
if [[ "$TOOL" != "amp" && "$TOOL" != "claude" ]]; then
  echo "Error: Invalid tool '$TOOL'. Must be 'amp' or 'claude'."
  exit 1
fi

# RALPH_HOME is where the prompts live (the ralph repo)
RALPH_HOME="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# PROJECT_DIR is where prd.json and working files live (default: cwd)
PROJECT_DIR="${PROJECT_DIR:-$(pwd)}"
PROJECT_DIR="$(cd "$PROJECT_DIR" && pwd)"  # resolve to absolute path

PRD_FILE="$PROJECT_DIR/prd.json"
PROGRESS_FILE="$PROJECT_DIR/progress.txt"
ARCHIVE_DIR="$PROJECT_DIR/.ralph/archive"
LAST_BRANCH_FILE="$PROJECT_DIR/.ralph/.last-branch"
LOG_DIR="$PROJECT_DIR/.ralph/logs"

# Check prd.json exists
if [ ! -f "$PRD_FILE" ]; then
  echo "Error: No prd.json found in $PROJECT_DIR"
  echo "Use the /ralph skill in Claude Code to create one from a PRD."
  exit 1
fi

# Ensure .ralph directory exists
mkdir -p "$PROJECT_DIR/.ralph"
mkdir -p "$LOG_DIR"

# --- Archive previous run if branch changed ---
if [ -f "$LAST_BRANCH_FILE" ]; then
  CURRENT_BRANCH=$(jq -r '.branchName // empty' "$PRD_FILE" 2>/dev/null || echo "")
  LAST_BRANCH=$(cat "$LAST_BRANCH_FILE" 2>/dev/null || echo "")

  if [ -n "$CURRENT_BRANCH" ] && [ -n "$LAST_BRANCH" ] && [ "$CURRENT_BRANCH" != "$LAST_BRANCH" ]; then
    DATE=$(date +%Y-%m-%d)
    FOLDER_NAME=$(echo "$LAST_BRANCH" | sed 's|^ralph/||')
    ARCHIVE_FOLDER="$ARCHIVE_DIR/$DATE-$FOLDER_NAME"

    echo "Archiving previous run: $LAST_BRANCH"
    mkdir -p "$ARCHIVE_FOLDER"
    [ -f "$PRD_FILE" ] && cp "$PRD_FILE" "$ARCHIVE_FOLDER/"
    [ -f "$PROGRESS_FILE" ] && cp "$PROGRESS_FILE" "$ARCHIVE_FOLDER/"
    echo "   Archived to: $ARCHIVE_FOLDER"

    echo "# Ralph Progress Log" > "$PROGRESS_FILE"
    echo "Started: $(date)" >> "$PROGRESS_FILE"
    echo "---" >> "$PROGRESS_FILE"
  fi
fi

# Track current branch
CURRENT_BRANCH=$(jq -r '.branchName // empty' "$PRD_FILE" 2>/dev/null || echo "")
if [ -n "$CURRENT_BRANCH" ]; then
  echo "$CURRENT_BRANCH" > "$LAST_BRANCH_FILE"
fi

# Initialize progress file if it doesn't exist
if [ ! -f "$PROGRESS_FILE" ]; then
  echo "# Ralph Progress Log" > "$PROGRESS_FILE"
  echo "Started: $(date)" >> "$PROGRESS_FILE"
  echo "---" >> "$PROGRESS_FILE"
fi

echo "Starting Ralph - Tool: $TOOL - Max iterations: $MAX_ITERATIONS"
echo "  Project: $PROJECT_DIR"

for i in $(seq 1 $MAX_ITERATIONS); do
  echo ""
  echo "==============================================================="
  echo "  Ralph Iteration $i of $MAX_ITERATIONS ($TOOL)"
  echo "==============================================================="

  # Find the next incomplete story
  NEXT_STORY=$(jq -r '[.userStories[] | select(.passes == false)] | sort_by(.priority) | .[0].id // empty' "$PRD_FILE")

  if [ -z "$NEXT_STORY" ]; then
    echo "All stories already complete!"
    exit 0
  fi

  echo "  Target story: $NEXT_STORY"

  # Capture progress file state before iteration
  PRE_LINES=$(wc -l < "$PROGRESS_FILE" 2>/dev/null | tr -d ' ')

  # Build prompt with story constraint injected
  if [[ "$TOOL" == "amp" ]]; then
    BASE_PROMPT=$(cat "$RALPH_HOME/prompt.md")
  else
    BASE_PROMPT=$(cat "$RALPH_HOME/CLAUDE.md")
  fi

  PROMPT="$BASE_PROMPT

---
## THIS ITERATION
You MUST only work on story **$NEXT_STORY**. Do NOT implement any other story. After completing $NEXT_STORY, stop immediately.
If progress.txt contains a [CONTEXT EXHAUSTED] entry for $NEXT_STORY, continue from where it left off."

  # Run the selected tool with the dynamic prompt
  LOG_FILE="$LOG_DIR/iteration-$i.log"

  if [[ "$TOOL" == "amp" ]]; then
    cd "$PROJECT_DIR" && echo "$PROMPT" | amp --dangerously-allow-all 2>&1 | tee "$LOG_FILE" || true
  else
    cd "$PROJECT_DIR" && echo "$PROMPT" | claude --dangerously-skip-permissions --print 2>&1 | tee "$LOG_FILE" || true
  fi

  OUTPUT=$(cat "$LOG_FILE")

  # Show what was added to progress.txt this iteration
  POST_LINES=$(wc -l < "$PROGRESS_FILE" 2>/dev/null | tr -d ' ')
  if [ "$POST_LINES" -gt "$PRE_LINES" ]; then
    echo ""
    echo "--- Progress from iteration $i ---"
    tail -n +$((PRE_LINES + 1)) "$PROGRESS_FILE"
    echo "---"
  fi

  # Check for completion signal
  if echo "$OUTPUT" | grep -q "<promise>COMPLETE</promise>"; then
    echo ""
    echo "Ralph completed all tasks!"
    echo "Completed at iteration $i of $MAX_ITERATIONS"
    exit 0
  fi

  echo "Iteration $i complete. Continuing..."
  sleep 2
done

echo ""
echo "Ralph reached max iterations ($MAX_ITERATIONS) without completing all tasks."
echo "Check $PROGRESS_FILE for status."
exit 1
