package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/jimgcampbell/mathgames/internal/ai"
	"github.com/jimgcampbell/mathgames/internal/game"
)

// GameHandler serves the full API surface: sessions, serving, attempts,
// daily, profile, collection, catch, quests, parents, settings, export
// (phase 3), plus AI generation and question-review (phase 4). aiGen is nil
// when ANTHROPIC_API_KEY isn't configured; generate returns 503 in that case.
type GameHandler struct {
	svc   *game.Service
	aiGen *ai.Generator
	log   *slog.Logger
}

func NewGameHandler(svc *game.Service, aiGen *ai.Generator, log *slog.Logger) *GameHandler {
	return &GameHandler{svc: svc, aiGen: aiGen, log: log}
}

func (h *GameHandler) Routes(r chi.Router) {
	r.Post("/sessions", h.createSession)
	r.Post("/sessions/{id}/end", h.endSession)

	r.Get("/next", h.next)
	r.Post("/attempts", h.createAttempt)

	r.Get("/daily", h.daily)

	r.Get("/profile", h.profile)
	r.Get("/collection", h.collection)
	r.Post("/catch", h.catch)

	r.Get("/quests", h.quests)
	r.Get("/quests/{id}", h.questChapter)

	r.Get("/parents/summary", h.parentsSummary)

	r.Get("/settings", h.getSettings)
	r.Put("/settings", h.updateSettings)

	r.Get("/screentime", h.screenTime)
	r.Post("/screentime/reset", h.resetScreenTime)
	r.Get("/screentime/log", h.screenTimeLog)

	r.Get("/export", h.export)

	r.Post("/generate", h.generate)
	r.Get("/questions", h.listQuestions)
	r.Post("/questions/{id}/retire", h.retireQuestion)
	r.Post("/questions/{id}/unretire", h.unretireQuestion)

	r.Post("/reset", h.reset)
}

// ---- sessions ----

func (h *GameHandler) createSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	switch body.Mode {
	case game.ModeTraining, game.ModeQuest, game.ModeDaily:
	default:
		writeError(w, http.StatusBadRequest, "mode must be one of training, quest, daily")
		return
	}
	sess, err := h.svc.CreateSession(r.Context(), body.Mode)
	if err != nil {
		h.fail(w, "create session", err)
		return
	}
	writeJSON(w, http.StatusCreated, sess)
}

func (h *GameHandler) endSession(w http.ResponseWriter, r *http.Request) {
	id, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	if err := h.svc.EndSession(r.Context(), id); err != nil {
		h.fail(w, "end session", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- serving / attempts ----

func (h *GameHandler) next(w http.ResponseWriter, r *http.Request) {
	skill := r.URL.Query().Get("skill")
	if skill == "" {
		writeError(w, http.StatusBadRequest, "skill is required")
		return
	}
	count := 1
	if c := r.URL.Query().Get("count"); c != "" {
		n, err := strconv.Atoi(c)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "count must be a positive integer")
			return
		}
		count = n
	}
	var sessionID int64
	if sid := r.URL.Query().Get("session_id"); sid != "" {
		n, err := parseInt64(sid)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session_id")
			return
		}
		sessionID = n
	}

	qs, bankLow, err := h.svc.NextQuestions(r.Context(), skill, count, sessionID)
	if err != nil {
		h.fail(w, "get next questions", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"questions": toQuestionsOut(qs),
		"bank_low":  bankLow,
	})
}

func (h *GameHandler) createAttempt(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SessionID  int64           `json:"session_id"`
		QuestionID int64           `json:"question_id"`
		Given      json.RawMessage `json:"given"`
		ElapsedMS  int             `json:"elapsed_ms"`
		Day        string          `json:"day"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if body.Day != "" && !validLocalDay(body.Day) {
		writeError(w, http.StatusBadRequest, "day must be YYYY-MM-DD")
		return
	}
	result, err := h.svc.Attempt(r.Context(), body.SessionID, body.QuestionID, body.Given, body.ElapsedMS, body.Day)
	if err != nil {
		h.fail(w, "record attempt", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ---- daily ----

func (h *GameHandler) daily(w http.ResponseWriter, r *http.Request) {
	day := r.URL.Query().Get("day")
	if day == "" {
		day = time.Now().UTC().Format("2006-01-02")
	}
	if !validLocalDay(day) {
		writeError(w, http.StatusBadRequest, "day must be YYYY-MM-DD")
		return
	}
	view, err := h.svc.Daily(r.Context(), day)
	if err != nil {
		h.fail(w, "get daily", err)
		return
	}
	writeJSON(w, http.StatusOK, dailyViewOut{
		Day:       view.Day,
		Questions: toQuestionsOutOmitEmpty(view.Questions),
		Answered:  view.Answered,
		Correct:   view.Correct,
		ElapsedMS: view.ElapsedMS,
		XPEarned:  view.XPEarned,
		Completed: view.Completed,
		Streak:    view.Streak,
		Calendar:  view.Calendar,
	})
}

// validLocalDay reports whether day is a well-formed YYYY-MM-DD string.
func validLocalDay(day string) bool {
	_, err := time.Parse("2006-01-02", day)
	return err == nil
}

// dailyViewOut mirrors game.DailyView but with Questions run through the
// answer-stripping serializer.
type dailyViewOut struct {
	Day       string             `json:"day"`
	Questions []questionOut      `json:"questions,omitempty"`
	Answered  int                `json:"answered"`
	Correct   int                `json:"correct"`
	ElapsedMS int                `json:"elapsed_ms"`
	XPEarned  int                `json:"xp_earned"`
	Completed bool               `json:"completed"`
	Streak    int                `json:"streak"`
	Calendar  []game.CalendarDay `json:"calendar"`
}

func toQuestionsOutOmitEmpty(qs []game.Question) []questionOut {
	if len(qs) == 0 {
		return nil
	}
	return toQuestionsOut(qs)
}

// ---- profile / collection / catch ----

func (h *GameHandler) profile(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Profile(r.Context())
	if err != nil {
		h.fail(w, "get profile", err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *GameHandler) collection(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Collection(r.Context())
	if err != nil {
		h.fail(w, "get collection", err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *GameHandler) catch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pokemon string `json:"pokemon"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	u, err := h.svc.Catch(r.Context(), body.Pokemon)
	if err != nil {
		h.fail(w, "catch", err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// ---- quests ----

func (h *GameHandler) quests(w http.ResponseWriter, r *http.Request) {
	sagas, err := h.svc.Quests(r.Context())
	if err != nil {
		h.fail(w, "list quests", err)
		return
	}
	writeJSON(w, http.StatusOK, sagas)
}

func (h *GameHandler) questChapter(w http.ResponseWriter, r *http.Request) {
	id, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid chapter id")
		return
	}
	ch, err := h.svc.QuestChapterByID(r.Context(), id)
	if err != nil {
		h.fail(w, "get quest chapter", err)
		return
	}
	if ch == nil {
		writeError(w, http.StatusNotFound, "quest chapter not found")
		return
	}
	writeJSON(w, http.StatusOK, ch)
}

// ---- parents ----

func (h *GameHandler) parentsSummary(w http.ResponseWriter, r *http.Request) {
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		n, err := strconv.Atoi(d)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "days must be a positive integer")
			return
		}
		days = n
	}
	summary, err := h.svc.ParentsSummary(r.Context(), days)
	if err != nil {
		h.fail(w, "get parents summary", err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// ---- settings ----

func (h *GameHandler) getSettings(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.GetSettings(r.Context())
	if err != nil {
		h.fail(w, "get settings", err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *GameHandler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var s game.Settings
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	updated, err := h.svc.UpdateSettings(r.Context(), &s)
	if err != nil {
		h.fail(w, "update settings", err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// ---- screen time ----

func (h *GameHandler) screenTime(w http.ResponseWriter, r *http.Request) {
	day := r.URL.Query().Get("day")
	if !validLocalDay(day) {
		writeError(w, http.StatusBadRequest, "day must be YYYY-MM-DD")
		return
	}
	st, err := h.svc.ScreenTime(r.Context(), day)
	if err != nil {
		h.fail(w, "get screen time", err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h *GameHandler) resetScreenTime(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Day string `json:"day"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if !validLocalDay(body.Day) {
		writeError(w, http.StatusBadRequest, "day must be YYYY-MM-DD")
		return
	}
	reset, err := h.svc.ResetScreenTime(r.Context(), body.Day)
	if err != nil {
		h.fail(w, "reset screen time", err)
		return
	}
	writeJSON(w, http.StatusOK, reset)
}

func (h *GameHandler) screenTimeLog(w http.ResponseWriter, r *http.Request) {
	log, err := h.svc.ListScreenTimeLog(r.Context())
	if err != nil {
		h.fail(w, "list screen time log", err)
		return
	}
	writeJSON(w, http.StatusOK, log)
}

// ---- export ----

func (h *GameHandler) export(w http.ResponseWriter, r *http.Request) {
	doc, err := h.svc.Export(r.Context())
	if err != nil {
		h.fail(w, "export", err)
		return
	}
	filename := "mathgames-export-" + time.Now().UTC().Format("20060102") + ".json"
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	writeJSON(w, http.StatusOK, doc)
}

// ---- AI generation / question review (parent view) ----

func (h *GameHandler) generate(w http.ResponseWriter, r *http.Request) {
	if h.aiGen == nil {
		writeError(w, http.StatusServiceUnavailable, "AI content generation is not configured (set ANTHROPIC_API_KEY)")
		return
	}
	var body struct {
		Kind       string `json:"kind"`
		Skill      string `json:"skill"`
		Difficulty int    `json:"difficulty"`
		Count      int    `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	var result *ai.BatchResult
	var err error
	switch body.Kind {
	case "story":
		result, err = h.aiGen.GenerateStorySaga(r.Context(), body.Skill)
	case "word_problems", "logic":
		result, err = h.aiGen.GenerateBatch(r.Context(), body.Skill, body.Difficulty, body.Count)
	default:
		writeError(w, http.StatusBadRequest, "kind must be one of word_problems, logic, story")
		return
	}
	if err != nil {
		h.fail(w, "generate content", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *GameHandler) listQuestions(w http.ResponseWriter, r *http.Request) {
	skill := r.URL.Query().Get("skill")
	source := r.URL.Query().Get("source")
	var retired *bool
	if rv := r.URL.Query().Get("retired"); rv != "" {
		b, err := strconv.ParseBool(rv)
		if err != nil {
			writeError(w, http.StatusBadRequest, "retired must be true or false")
			return
		}
		retired = &b
	}
	qs, err := h.svc.ListQuestions(r.Context(), skill, source, retired)
	if err != nil {
		h.fail(w, "list questions", err)
		return
	}
	writeJSON(w, http.StatusOK, qs)
}

func (h *GameHandler) retireQuestion(w http.ResponseWriter, r *http.Request) {
	h.setQuestionRetired(w, r, true)
}

func (h *GameHandler) unretireQuestion(w http.ResponseWriter, r *http.Request) {
	h.setQuestionRetired(w, r, false)
}

func (h *GameHandler) setQuestionRetired(w http.ResponseWriter, r *http.Request, retired bool) {
	id, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid question id")
		return
	}
	if err := h.svc.SetQuestionRetired(r.Context(), id, retired); err != nil {
		h.fail(w, "set question retired", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- admin ----

// reset requires the caller to send {"confirm":"RESET"} — the bearer key
// alone isn't enough friction for an irreversible wipe of all progress.
func (h *GameHandler) reset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Confirm string `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if body.Confirm != "RESET" {
		writeError(w, http.StatusBadRequest, `send {"confirm":"RESET"} to confirm this wipes all progress`)
		return
	}
	if err := h.svc.ResetProgress(r.Context()); err != nil {
		h.fail(w, "reset progress", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- helpers ----

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// fail maps service errors to status codes via sentinel prefixes ("invalid:",
// "not found:", "conflict:"), same convention as the sibling apps.
func (h *GameHandler) fail(w http.ResponseWriter, op string, err error) {
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "invalid:"):
		writeError(w, http.StatusBadRequest, strings.TrimSpace(strings.TrimPrefix(msg, "invalid:")))
	case strings.HasPrefix(msg, "not found:"):
		writeError(w, http.StatusNotFound, strings.TrimSpace(strings.TrimPrefix(msg, "not found:")))
	case strings.HasPrefix(msg, "conflict:"):
		writeError(w, http.StatusConflict, strings.TrimSpace(strings.TrimPrefix(msg, "conflict:")))
	default:
		h.log.Error(op+" failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to "+op)
	}
}
