package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const skylarProfile = "Skylar is a gifted 8-year-old heading into 3rd grade, working through " +
	"4th-grade math content. He is a voracious reader and loves Dragon Ball Z. Write for a " +
	"bright, fast kid: clear wording, no busywork, playful DBZ flavor where it fits naturally " +
	"(character names, training/battle framing) but never forced into every single item."

// wordProblemRubric maps a 1-10 difficulty level to the grade-band /
// complexity rubric for word_problems, per ARCHITECTURE.md's skill table
// (L1-2 ~ 3rd grade, L3-5 ~ 4th grade, L6-8 ~ 5th grade, L9-10 ~ 6th grade).
func wordProblemRubric(level int) string {
	switch {
	case level <= 2:
		return "3rd-grade level: one or two-step story problems using addition, subtraction, or " +
			"simple multiplication/division, numbers under 100."
	case level <= 5:
		return "4th-grade level: 2-3 step story problems mixing operations (multiplication, " +
			"division with remainders, multi-digit addition/subtraction, a fraction-of-a-quantity " +
			"step), numbers up to a few thousand."
	case level <= 8:
		return "5th-grade level: 2-3 step story problems with larger numbers (multi-digit " +
			"multiplication/division, multi-step reasoning chaining 3+ quantities), numbers up to " +
			"the tens of thousands."
	default:
		return "6th-grade level: multi-step problems chaining several operations together, larger " +
			"numbers, and dense wording that requires careful reading to extract the right quantities."
	}
}

func logicRubric(level int) string {
	switch {
	case level <= 2:
		return "Simple: 'which one doesn't belong' (4 items, one category outlier) or a 2-clue " +
			"deduction with 3 entities."
	case level <= 5:
		return "Moderate: 3-4 clue grid deduction puzzles (e.g. matching characters to techniques) " +
			"or balance-scale weighing puzzles with 3-4 items."
	case level <= 8:
		return "Harder: 4-5 clue grid deduction with 4-5 entities and 2 attributes, or multi-step " +
			"balance/logic puzzles requiring combining two clues to eliminate an option."
	default:
		return "Hardest: 5+ clue grid deduction across 3+ attributes, or layered deduction puzzles " +
			"requiring chaining 3+ inferences."
	}
}

const wordProblemShape = `Each item is a JSON object:
{"payload": {"kind": "numeric", "prompt": "<the story problem>"}, "answer": {"value": <integer>}, "explanation": "<short worked explanation a kid can read after answering wrong>", "check": "<integer arithmetic expression using only + - * / ( ) and the numbers from the problem, that evaluates to the answer, e.g. \"34*12+50\">"}
The check expression is mandatory and must exactly equal answer.value when evaluated with integer division. Never use decimals anywhere.`

const logicShape = `Each item is a JSON object in one of these two shapes:
- Multiple choice: {"payload": {"kind": "mc", "prompt": "<puzzle>", "choices": ["...", "..."]}, "answer": {"index": <0-based index of correct choice>}, "explanation": "<why>"}
- Short answer: {"payload": {"kind": "text", "prompt": "<puzzle>"}, "answer": {"value": "<canonical answer>", "accept": ["<canonical answer>", "<alternate phrasing>"]}, "explanation": "<why>"}
choices must have between 2 and 5 entries for "mc". For grid-deduction or sequence puzzles that benefit from a visual, add a "display" key inside payload: {"grid": {"rows": [...], "cols": [...]}} or {"sequence": [1,2,null,4]} (whatever shape best represents the puzzle -- the display is loosely typed and AI-authored).`

// questionsSystemPrompt builds the system prompt for a word_problems or
// logic batch call.
func questionsSystemPrompt(kind string, difficulty int, recentPrompts []string) string {
	var rubric, shape string
	switch kind {
	case "word_problems":
		rubric = wordProblemRubric(difficulty)
		shape = wordProblemShape
	case "logic":
		rubric = logicRubric(difficulty)
		shape = logicShape
	default:
		rubric = ""
		shape = ""
	}

	var sb strings.Builder
	sb.WriteString(skylarProfile)
	sb.WriteString("\n\nYou are generating a batch of ")
	sb.WriteString(kind)
	sb.WriteString(fmt.Sprintf(" questions at difficulty level %d/10.\n\nRubric for this level: %s\n\n", difficulty, rubric))
	sb.WriteString(shape)
	sb.WriteString("\n\nRespond with a JSON array of these objects and nothing else -- no markdown code fence, no commentary, no surrounding object.")

	if len(recentPrompts) > 0 {
		sb.WriteString("\n\nDo not repeat or closely paraphrase any of these already-used prompts:\n")
		for _, p := range recentPrompts {
			sb.WriteString("- ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// GenerateQuestions makes one non-agentic batch call for a word_problems or
// logic skill at the given difficulty, returning the system prompt sent
// (stored in ai_batches.prompt), the raw response text (stored in
// ai_batches.raw), and the parsed items.
func GenerateQuestions(ctx context.Context, msgr Messenger, model, kind string, difficulty, count int, recentPrompts []string) (systemPrompt, rawText string, items []RawItem, err error) {
	systemPrompt = questionsSystemPrompt(kind, difficulty, recentPrompts)
	user := fmt.Sprintf("Generate exactly %d new questions now, as a JSON array and nothing else.", count)

	resp, err := msgr.CreateMessage(ctx, Request{
		Model:    model,
		System:   systemPrompt,
		Messages: []Message{{Role: RoleUser, Content: user}},
	})
	if err != nil {
		return systemPrompt, "", nil, fmt.Errorf("generate %s: %w", kind, err)
	}
	rawText = resp.Text()

	items, err = parseItems(rawText)
	if err != nil {
		return systemPrompt, rawText, nil, fmt.Errorf("parse %s response: %w", kind, err)
	}
	return systemPrompt, rawText, items, nil
}

// ChapterInput describes one quest chapter for a story batch call: enough
// context for the model to write a hook that references the chapter's
// actual requirement.
type ChapterInput struct {
	Chapter            int
	CurrentTitle       string
	RequirementSkills  []string
	RequirementCorrect int
	RequirementMinDiff int
}

const storyShape = `Respond with a JSON array, one object per chapter, in this shape:
{"chapter": <chapter number>, "title": "<short punchy chapter title>", "story": "<~120 word narrative>"}
Each story must end with a hook tying directly into that chapter's requirement (e.g. "Vegeta blocks the path -- land 12 multiplication hits to push through!"). Adventurous, funny, reading-level generous tone -- he's a strong reader. Respond with the JSON array and nothing else -- no markdown code fence, no commentary.`

// GenerateStory makes one non-agentic batch call rewriting every chapter's
// title/story for one saga.
func GenerateStory(ctx context.Context, msgr Messenger, model, saga string, chapters []ChapterInput) (systemPrompt, rawText string, items []StoryItem, err error) {
	var sb strings.Builder
	sb.WriteString(skylarProfile)
	sb.WriteString(fmt.Sprintf("\n\nYou are writing the story text for the %q saga, %d chapters, of a Dragon Ball Z themed math training app. ", saga, len(chapters)))
	sb.WriteString("This is a private, non-commercial fan project -- playful references to DBZ characters and lore are fine, but write original prose, not copied text.\n\n")
	for _, c := range chapters {
		sb.WriteString(fmt.Sprintf("Chapter %d (current placeholder title %q): requires %d correct answers in skills %v at difficulty %d+.\n",
			c.Chapter, c.CurrentTitle, c.RequirementCorrect, c.RequirementSkills, c.RequirementMinDiff))
	}
	sb.WriteString("\n")
	sb.WriteString(storyShape)
	systemPrompt = sb.String()

	resp, err := msgr.CreateMessage(ctx, Request{
		Model:    model,
		System:   systemPrompt,
		Messages: []Message{{Role: RoleUser, Content: "Write the story batch now."}},
	})
	if err != nil {
		return systemPrompt, "", nil, fmt.Errorf("generate story for saga %s: %w", saga, err)
	}
	rawText = resp.Text()

	if err := json.Unmarshal([]byte(stripCodeFence(rawText)), &items); err != nil {
		return systemPrompt, rawText, nil, fmt.Errorf("parse story response: %w", err)
	}
	return systemPrompt, rawText, items, nil
}

func parseItems(text string) ([]RawItem, error) {
	var items []RawItem
	if err := json.Unmarshal([]byte(stripCodeFence(text)), &items); err != nil {
		return nil, err
	}
	return items, nil
}

// stripCodeFence removes a wrapping ```json ... ``` (or plain ```) fence if
// the model added one despite instructions not to.
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	lines := strings.SplitN(s, "\n", 2)
	if len(lines) < 2 {
		return s
	}
	s = strings.TrimSpace(lines[1])
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
