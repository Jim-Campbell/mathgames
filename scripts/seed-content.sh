#!/usr/bin/env bash
# Fills the AI question bank (word_problems, logic) to ~40 non-retired
# questions per level 1-10, then rewrites the story text for all 5 sagas.
# Idempotent-ish: reads current bank counts from /api/parents/summary and
# skips any skill x level already at target, so re-running just tops up.
#
# Usage: MATHGAMES_API_KEY=... scripts/seed-content.sh
# Optional env: MATHGAMES_BASE_URL (default http://localhost:8083),
#   SEED_TARGET_PER_LEVEL (default 40), SEED_BATCH_COUNT (default 10).
# Requires: curl, jq.

set -uo pipefail

BASE="${MATHGAMES_BASE_URL:-http://localhost:8083}/api"
API_KEY="${MATHGAMES_API_KEY:?MATHGAMES_API_KEY must be set}"
TARGET="${SEED_TARGET_PER_LEVEL:-40}"
BATCH_COUNT="${SEED_BATCH_COUNT:-10}"

auth() {
  curl -sS -H "Authorization: Bearer ${API_KEY}" -H "Content-Type: application/json" "$@"
}

echo "== checking AI is configured on the server =="
ai_ok="$(auth "${BASE}/health" | jq -r '.ai')"
if [ "$ai_ok" != "true" ]; then
  echo "ANTHROPIC_API_KEY not configured on the server; nothing to do." >&2
  exit 1
fi

echo "== filling word_problems and logic banks to ${TARGET} questions per level =="
for skill in word_problems logic; do
  for level in $(seq 1 10); do
    while :; do
      available="$(auth "${BASE}/parents/summary?days=1" \
        | jq -r --arg s "$skill" --argjson l "$level" \
          '.bank[] | select(.skill==$s and .level==$l) | .available')"
      available="${available:-0}"

      if [ "$available" -ge "$TARGET" ]; then
        echo "  ${skill} L${level}: ${available} >= ${TARGET}, skipping"
        break
      fi

      echo "  ${skill} L${level}: ${available} < ${TARGET}, generating a batch of ${BATCH_COUNT}..."
      resp="$(auth -X POST "${BASE}/generate" -d "$(jq -n \
        --arg kind "$skill" --arg skill "$skill" --argjson difficulty "$level" --argjson count "$BATCH_COUNT" \
        '{kind:$kind, skill:$skill, difficulty:$difficulty, count:$count}')")"

      accepted="$(echo "$resp" | jq -r '.accepted // 0')"
      rejected="$(echo "$resp" | jq -r '.rejected // 0')"
      echo "    accepted=${accepted} rejected=${rejected}"

      if [ "$accepted" = "0" ]; then
        echo "    no accepted questions this round, moving on (check server logs for the rejection reasons)." >&2
        break
      fi
    done
  done
done

echo "== rewriting saga story text =="
for saga in saiyan namek android cell buu; do
  echo "  ${saga}..."
  resp="$(auth -X POST "${BASE}/generate" -d "$(jq -n --arg saga "$saga" '{kind:"story", skill:$saga}')")"
  accepted="$(echo "$resp" | jq -r '.accepted // 0')"
  rejected="$(echo "$resp" | jq -r '.rejected // 0')"
  echo "    accepted=${accepted} rejected=${rejected}"
done

echo "done."
