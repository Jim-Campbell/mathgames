package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/jimgcampbell/mathgames/internal/game"
)

// maxClipBytes is the upload size cap -- a 30-second phone clip fits; if
// Jim's clip is rejected he can trim/compress and re-upload.
const maxClipBytes = 64 << 20 // 64 MB

var clipExtByContentType = map[string]string{
	"video/mp4":       "mp4",
	"video/quicktime": "mov",
	"video/webm":      "webm",
}

// ClipStore is the subset of *storage.R2Client the handler depends on.
type ClipStore interface {
	Upload(ctx context.Context, key, contentType string, data io.Reader) (string, error)
	Delete(ctx context.Context, key string) error
}

// ClipHandler serves the video-clips manage API. store is nil when R2 isn't
// configured -- upload 503s in that case; list/update/delete/plays still
// work against whatever metadata already exists.
type ClipHandler struct {
	svc   *game.Service
	store ClipStore
	log   *slog.Logger
}

func NewClipHandler(svc *game.Service, store ClipStore, log *slog.Logger) *ClipHandler {
	return &ClipHandler{svc: svc, store: store, log: log}
}

func (h *ClipHandler) Routes(r chi.Router) {
	r.Get("/clips", h.list)
	r.Post("/clips", h.upload)
	r.Put("/clips/{id}", h.update)
	r.Delete("/clips/{id}", h.delete)
	r.Get("/clips/plays", h.plays)
}

func (h *ClipHandler) list(w http.ResponseWriter, r *http.Request) {
	clips, err := h.svc.ListClips(r.Context())
	if err != nil {
		h.fail(w, "list clips", err)
		return
	}
	writeJSON(w, http.StatusOK, clips)
}

func (h *ClipHandler) upload(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "video storage is not configured (set R2_* env vars)")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxClipBytes+2<<20) // headroom for multipart overhead
	if err := r.ParseMultipartForm(maxClipBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form or clip over the 64MB limit (trim/compress and try again)")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	if header.Size > maxClipBytes {
		writeError(w, http.StatusBadRequest, "clip exceeds the 64MB limit (trim/compress and try again)")
		return
	}

	contentType := header.Header.Get("Content-Type")
	ext, ok := clipExtByContentType[contentType]
	if !ok {
		writeError(w, http.StatusBadRequest, "unsupported content type "+contentType+" (want video/mp4, video/quicktime, or video/webm)")
		return
	}

	title := r.FormValue("title")
	if title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	key := clipObjectKey(ext)
	url, err := h.store.Upload(r.Context(), key, contentType, file)
	if err != nil {
		h.log.Error("upload clip failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to upload clip")
		return
	}

	clip := &game.Clip{
		Title:       title,
		R2Key:       key,
		URL:         url,
		ContentType: contentType,
		SizeBytes:   header.Size,
		DurationMS:  parseOptionalIntForm(r, "duration_ms"),
		Enabled:     parseBoolForm(r, "enabled", true),
		OnCorrect:   parseBoolForm(r, "on_correct", true),
		OnWrong:     parseBoolForm(r, "on_wrong", false),
		Weight:      parseIntFormDefault(r, "weight", 1),
	}
	if err := h.svc.CreateClip(r.Context(), clip); err != nil {
		h.fail(w, "create clip", err)
		return
	}

	h.log.Info("clip uploaded", "key", key, "url", url, "bytes", header.Size, "content_type", contentType)
	writeJSON(w, http.StatusCreated, clip)
}

func (h *ClipHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid clip id")
		return
	}
	var body struct {
		Title     string `json:"title"`
		Enabled   bool   `json:"enabled"`
		OnCorrect bool   `json:"on_correct"`
		OnWrong   bool   `json:"on_wrong"`
		Weight    int    `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := h.svc.UpdateClip(r.Context(), id, body.Title, body.Enabled, body.OnCorrect, body.OnWrong, body.Weight); err != nil {
		h.fail(w, "update clip", err)
		return
	}
	clip, err := h.svc.GetClip(r.Context(), id)
	if err != nil {
		h.fail(w, "get clip", err)
		return
	}
	writeJSON(w, http.StatusOK, clip)
}

func (h *ClipHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid clip id")
		return
	}
	clip, err := h.svc.GetClip(r.Context(), id)
	if err != nil {
		h.fail(w, "get clip", err)
		return
	}
	if clip == nil {
		writeError(w, http.StatusNotFound, "clip not found")
		return
	}
	if h.store != nil {
		if err := h.store.Delete(r.Context(), clip.R2Key); err != nil {
			// Log and continue: an already-gone object shouldn't block
			// removing the row.
			h.log.Warn("delete clip object failed, continuing to delete row", "key", clip.R2Key, "error", err)
		}
	}
	if err := h.svc.DeleteClip(r.Context(), id); err != nil {
		h.fail(w, "delete clip", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ClipHandler) plays(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = n
	}
	plays, err := h.svc.ListClipPlays(r.Context(), limit)
	if err != nil {
		h.fail(w, "list clip plays", err)
		return
	}
	writeJSON(w, http.StatusOK, plays)
}

// clipObjectKey builds a clips/<random>.<ext> R2 key.
func clipObjectKey(ext string) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("generate clip key: %v", err)) // unreachable: crypto/rand.Read only fails if the OS RNG is broken
	}
	return fmt.Sprintf("clips/%s.%s", hex.EncodeToString(b), ext)
}

func parseBoolForm(r *http.Request, name string, def bool) bool {
	v := r.FormValue(name)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func parseIntFormDefault(r *http.Request, name string, def int) int {
	v := r.FormValue(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func parseOptionalIntForm(r *http.Request, name string) *int {
	v := r.FormValue(name)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return nil
	}
	return &n
}

// fail mirrors GameHandler.fail's sentinel-prefix convention.
func (h *ClipHandler) fail(w http.ResponseWriter, op string, err error) {
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "invalid:"):
		writeError(w, http.StatusBadRequest, strings.TrimSpace(strings.TrimPrefix(msg, "invalid:")))
	case strings.HasPrefix(msg, "not found:"):
		writeError(w, http.StatusNotFound, strings.TrimSpace(strings.TrimPrefix(msg, "not found:")))
	default:
		h.log.Error(op+" failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to "+op)
	}
}
