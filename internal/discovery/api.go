package discovery

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type API struct {
	cache    *Cache
	resolver *Resolver
	logger   *slog.Logger
}

func NewAPI(cache *Cache, resolver *Resolver, logger *slog.Logger) *API {
	return &API{
		cache:    cache,
		resolver: resolver,
		logger:   logger.With("component", "discovery-api"),
	}
}

func (a *API) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/discover", func(r chi.Router) {
		r.Get("/", a.handleDiscover)
		r.Get("/agents", a.handleListAgents)
		r.Post("/resolve", a.handleResolve)
	})
}

func (a *API) handleDiscover(w http.ResponseWriter, r *http.Request) {
	skill := r.URL.Query().Get("skill")
	name := r.URL.Query().Get("name")
	minRepStr := r.URL.Query().Get("min_reputation")

	var minReputation float64
	if minRepStr != "" {
		if v, err := strconv.ParseFloat(minRepStr, 64); err == nil {
			minReputation = v
		}
	}

	type result struct {
		Agents []CachedAgent `json:"agents"`
		Query  interface{}   `json:"query"`
	}

	var agents []CachedAgent
	var err error

	switch {
	case skill != "":
		cards, e := a.cache.SearchBySkillTag(skill)
		if e != nil {
			writeError(w, http.StatusInternalServerError, e.Error())
			return
		}
		for _, c := range cards {
			agents = append(agents, CachedAgent{Card: c})
		}
	case name != "":
		cards, e := a.cache.SearchByName(name)
		if e != nil {
			writeError(w, http.StatusInternalServerError, e.Error())
			return
		}
		for _, c := range cards {
			agents = append(agents, CachedAgent{Card: c})
		}
	case minReputation > 0:
		agents, err = a.cache.SearchByReputation(minReputation)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	default:
		agents, err = a.cache.ListAll()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result{
		Agents: agents,
		Query: map[string]interface{}{
			"skill":          skill,
			"name":           name,
			"min_reputation": minReputation,
		},
	})
}

func (a *API) handleListAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := a.cache.ListAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agents": agents,
		"total":  len(agents),
	})
}

func (a *API) handleResolve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	card, err := a.resolver.Resolve(r.Context(), req.URL)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(card)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
