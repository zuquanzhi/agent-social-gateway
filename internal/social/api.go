package social

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/zuwance/agent-social-gateway/internal/types"
)

type API struct {
	actions  *Actions
	timeline *Timeline
	logger   *slog.Logger
}

func NewAPI(actions *Actions, timeline *Timeline, logger *slog.Logger) *API {
	return &API{actions: actions, timeline: timeline, logger: logger}
}

func (a *API) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/social", func(r chi.Router) {
		r.Post("/follow", a.handleFollow)
		r.Post("/unfollow", a.handleUnfollow)
		r.Post("/like", a.handleLike)
		r.Post("/unlike", a.handleUnlike)
		r.Post("/endorse", a.handleEndorse)
		r.Post("/collaborate", a.handleCollaborate)
		r.Get("/graph/{agentID}", a.handleGraphInfo)
		r.Get("/timeline/{agentID}", a.handleTimeline)
		r.Get("/followers/{agentID}", a.handleFollowers)
		r.Get("/following/{agentID}", a.handleFollowing)
	})
}

func (a *API) handleFollow(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FollowerID string `json:"follower_id"`
		TargetID   string `json:"target_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	a.actions.EnsureAgent(types.AgentID(req.FollowerID), req.FollowerID)
	a.actions.EnsureAgent(types.AgentID(req.TargetID), req.TargetID)

	if err := a.actions.Follow(types.AgentID(req.FollowerID), types.AgentID(req.TargetID)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	a.timeline.AddEvent(types.AgentID(req.TargetID), "new_follower", types.AgentID(req.FollowerID), "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "following", "follower": req.FollowerID, "target": req.TargetID})
}

func (a *API) handleUnfollow(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FollowerID string `json:"follower_id"`
		TargetID   string `json:"target_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	if err := a.actions.Unfollow(types.AgentID(req.FollowerID), types.AgentID(req.TargetID)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unfollowed"})
}

func (a *API) handleLike(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID   string `json:"agent_id"`
		MessageID string `json:"message_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	a.actions.EnsureAgent(types.AgentID(req.AgentID), req.AgentID)
	a.actions.EnsureMessage(req.MessageID, types.AgentID(req.AgentID))

	if err := a.actions.Like(types.AgentID(req.AgentID), req.MessageID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	count, _ := a.actions.GetLikeCount(req.MessageID)
	writeJSON(w, http.StatusOK, map[string]any{"status": "liked", "message_id": req.MessageID, "total_likes": count})
}

func (a *API) handleUnlike(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID   string `json:"agent_id"`
		MessageID string `json:"message_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	a.actions.Unlike(types.AgentID(req.AgentID), req.MessageID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "unliked"})
}

func (a *API) handleEndorse(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FromID   string `json:"from_agent_id"`
		TargetID string `json:"target_agent_id"`
		SkillID  string `json:"skill_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	a.actions.EnsureAgent(types.AgentID(req.FromID), req.FromID)
	a.actions.EnsureAgent(types.AgentID(req.TargetID), req.TargetID)

	if err := a.actions.Endorse(types.AgentID(req.FromID), types.AgentID(req.TargetID), req.SkillID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	a.timeline.AddEvent(types.AgentID(req.TargetID), "endorsed", types.AgentID(req.FromID), req.SkillID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "endorsed", "skill": req.SkillID})
}

func (a *API) handleCollaborate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FromID   string         `json:"from_agent_id"`
		TargetID string         `json:"target_agent_id"`
		Proposal map[string]any `json:"proposal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	if err := a.actions.RequestCollaboration(types.AgentID(req.FromID), types.AgentID(req.TargetID), req.Proposal); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	a.timeline.AddEvent(types.AgentID(req.TargetID), "collab_request", types.AgentID(req.FromID), "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "collaboration_requested"})
}

func (a *API) handleGraphInfo(w http.ResponseWriter, r *http.Request) {
	agentID := types.AgentID(chi.URLParam(r, "agentID"))
	followers, _ := a.actions.Graph().GetFollowerCount(agentID)
	following, _ := a.actions.Graph().GetFollowingCount(agentID)
	reputation, _ := a.actions.Graph().GetReputationScore(agentID)

	writeJSON(w, http.StatusOK, map[string]any{
		"agent_id":   string(agentID),
		"followers":  followers,
		"following":  following,
		"reputation": reputation,
	})
}

func (a *API) handleTimeline(w http.ResponseWriter, r *http.Request) {
	agentID := types.AgentID(chi.URLParam(r, "agentID"))
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	events, err := a.timeline.GetTimeline(agentID, 0, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent_id": string(agentID), "events": events, "count": len(events)})
}

func (a *API) handleFollowers(w http.ResponseWriter, r *http.Request) {
	agentID := types.AgentID(chi.URLParam(r, "agentID"))
	followers, err := a.actions.Graph().GetFollowers(agentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent_id": string(agentID), "followers": followers})
}

func (a *API) handleFollowing(w http.ResponseWriter, r *http.Request) {
	agentID := types.AgentID(chi.URLParam(r, "agentID"))
	following, err := a.actions.Graph().GetFollowing(agentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent_id": string(agentID), "following": following})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
