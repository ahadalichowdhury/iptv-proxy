package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"tv-proxy-go/store"
)

type DataHandler struct {
	store  *store.Store
	secret string
}

func NewDataHandler(st *store.Store) *DataHandler {
	return &DataHandler{
		store:  st,
		secret: strings.TrimSpace(os.Getenv("DATA_API_SECRET")),
	}
}

func (h *DataHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/data/streams", h.handleStreams)
	mux.HandleFunc("/data/streams/manual", h.handleManualStreams)
	mux.HandleFunc("/data/streams/manual-import", h.handleManualImportStreams)
	mux.HandleFunc("/data/streams/by-source/", h.handleStreamsBySource)
	mux.HandleFunc("/data/streams/", h.handleStreamByID)
	mux.HandleFunc("/data/playlist-sources", h.handlePlaylistSources)
	mux.HandleFunc("/data/playlist-sources/by-url", h.handlePlaylistSourceByURL)
	mux.HandleFunc("/data/playlist-sources/", h.handlePlaylistSourceByID)
	mux.HandleFunc("/data/presence/heartbeat", h.handlePresenceHeartbeat)
	mux.HandleFunc("/data/presence/active-count", h.handlePresenceActiveCount)
}

func (h *DataHandler) run(w http.ResponseWriter, r *http.Request, fn storeFunc) {
	payload, err := h.withStore(r, fn)
	h.writeJSON(w, payload, err)
}

func (h *DataHandler) handleStreams(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.run(w, r, func(ctx context.Context, st *store.Store) (any, error) {
			return st.ListStreams(ctx)
		})
	case http.MethodPost:
		if !h.requireSecret(w, r) {
			return
		}
		var input store.StreamWrite
		if !h.decodeJSON(w, r, &input) {
			return
		}
		h.run(w, r, func(ctx context.Context, st *store.Store) (any, error) {
			return st.InsertStream(ctx, input)
		})
	default:
		methodNotAllowed(w)
	}
}

func (h *DataHandler) handleManualStreams(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	h.run(w, r, func(ctx context.Context, st *store.Store) (any, error) {
		return st.ListManualStreams(ctx)
	})
}

func (h *DataHandler) handleManualImportStreams(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if !h.requireSecret(w, r) {
		return
	}
	h.run(w, r, func(ctx context.Context, st *store.Store) (any, error) {
		return st.ListStreamsForManualImport(ctx)
	})
}

func (h *DataHandler) handleStreamsBySource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	sourceID, err := parseTrailingID(r.URL.Path, "/data/streams/by-source/")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid source id")
		return
	}
	if !h.requireSecret(w, r) {
		return
	}
	h.run(w, r, func(ctx context.Context, st *store.Store) (any, error) {
		return st.ListStreamsBySourceID(ctx, sourceID)
	})
}

func (h *DataHandler) handleStreamByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseTrailingID(r.URL.Path, "/data/streams/")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid stream id")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.run(w, r, func(ctx context.Context, st *store.Store) (any, error) {
			stream, err := st.GetStreamByID(ctx, id)
			if err != nil {
				return nil, err
			}
			if stream == nil {
				return nil, errNotFound("stream not found")
			}
			return stream, nil
		})
	case http.MethodPatch:
		if !h.requireSecret(w, r) {
			return
		}
		var patch store.StreamPatch
		if !h.decodeJSON(w, r, &patch) {
			return
		}
		h.run(w, r, func(ctx context.Context, st *store.Store) (any, error) {
			if err := st.UpdateStream(ctx, id, patch); err != nil {
				return nil, err
			}
			return map[string]bool{"ok": true}, nil
		})
	case http.MethodDelete:
		if !h.requireSecret(w, r) {
			return
		}
		h.run(w, r, func(ctx context.Context, st *store.Store) (any, error) {
			if err := st.DeleteStream(ctx, id); err != nil {
				return nil, err
			}
			return map[string]bool{"ok": true}, nil
		})
	default:
		methodNotAllowed(w)
	}
}

func (h *DataHandler) handlePlaylistSources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.run(w, r, func(ctx context.Context, st *store.Store) (any, error) {
			return st.ListPlaylistSources(ctx)
		})
	case http.MethodPost:
		if !h.requireSecret(w, r) {
			return
		}
		var input store.PlaylistSourceWrite
		if !h.decodeJSON(w, r, &input) {
			return
		}
		h.run(w, r, func(ctx context.Context, st *store.Store) (any, error) {
			return st.InsertPlaylistSource(ctx, input)
		})
	default:
		methodNotAllowed(w)
	}
}

func (h *DataHandler) handlePlaylistSourceByURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if !h.requireSecret(w, r) {
		return
	}
	sourceURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if sourceURL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	h.run(w, r, func(ctx context.Context, st *store.Store) (any, error) {
		source, err := st.GetPlaylistSourceByURL(ctx, sourceURL)
		if err != nil {
			return nil, err
		}
		if source == nil {
			return nil, errNotFound("playlist source not found")
		}
		return source, nil
	})
}

func (h *DataHandler) handlePlaylistSourceByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseTrailingID(r.URL.Path, "/data/playlist-sources/")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid playlist source id")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.run(w, r, func(ctx context.Context, st *store.Store) (any, error) {
			source, err := st.GetPlaylistSourceByID(ctx, id)
			if err != nil {
				return nil, err
			}
			if source == nil {
				return nil, errNotFound("playlist source not found")
			}
			return source, nil
		})
	case http.MethodPatch:
		if !h.requireSecret(w, r) {
			return
		}
		var patch store.PlaylistSourcePatch
		if !h.decodeJSON(w, r, &patch) {
			return
		}
		h.run(w, r, func(ctx context.Context, st *store.Store) (any, error) {
			if err := st.UpdatePlaylistSource(ctx, id, patch); err != nil {
				return nil, err
			}
			return map[string]bool{"ok": true}, nil
		})
	case http.MethodDelete:
		if !h.requireSecret(w, r) {
			return
		}
		h.run(w, r, func(ctx context.Context, st *store.Store) (any, error) {
			if err := st.DeletePlaylistSource(ctx, id); err != nil {
				return nil, err
			}
			return map[string]bool{"ok": true}, nil
		})
	default:
		methodNotAllowed(w)
	}
}

func (h *DataHandler) handlePresenceHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var payload store.PresenceHeartbeat
	if !h.decodeJSON(w, r, &payload) {
		return
	}
	h.run(w, r, func(ctx context.Context, st *store.Store) (any, error) {
		if err := st.RecordPresenceHeartbeat(ctx, payload.SessionID, payload.Path); err != nil {
			return nil, err
		}
		if err := st.DeleteStalePresenceSessions(ctx, time.Now().UTC().Add(-time.Hour)); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})
}

func (h *DataHandler) handlePresenceActiveCount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if !h.requireSecret(w, r) {
		return
	}
	windowMs := int64((24 * time.Minute) / time.Millisecond)
	if raw := strings.TrimSpace(r.URL.Query().Get("windowMs")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "invalid windowMs")
			return
		}
		windowMs = parsed
	}
	h.run(w, r, func(ctx context.Context, st *store.Store) (any, error) {
		count, err := st.CountActivePresenceSessions(ctx, time.Now().UTC().Add(-time.Duration(windowMs)*time.Millisecond))
		if err != nil {
			return nil, err
		}
		return map[string]int64{"count": count}, nil
	})
}

func (h *DataHandler) requireSecret(w http.ResponseWriter, r *http.Request) bool {
	if h.secret == "" {
		return true
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "Bearer "+h.secret {
		return true
	}
	writeError(w, http.StatusUnauthorized, "unauthorized")
	return false
}

func (h *DataHandler) decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return false
	}
	return true
}

type storeFunc func(ctx context.Context, st *store.Store) (any, error)

func (h *DataHandler) withStore(r *http.Request, fn storeFunc) (any, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	return fn(ctx, h.store)
}

func (h *DataHandler) writeJSON(w http.ResponseWriter, payload any, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

type notFoundError string

func (e notFoundError) Error() string { return string(e) }

func errNotFound(msg string) error { return notFoundError(msg) }

func isNotFound(err error) bool {
	_, ok := err.(notFoundError)
	return ok
}

func parseTrailingID(path, prefix string) (int, error) {
	raw := strings.TrimPrefix(path, prefix)
	raw = strings.Trim(raw, "/")
	return strconv.Atoi(raw)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}
