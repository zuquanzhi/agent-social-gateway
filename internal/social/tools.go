package social

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/zuwance/agent-social-gateway/internal/types"
)

// RegisterMCPTools registers all social actions as MCP tools on the provided registrar.
type ToolRegistrar interface {
	AddTool(name, description string, schema map[string]any, handler func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error))
}

func RegisterMCPTools(reg ToolRegistrar, actions *Actions, timeline *Timeline, logger *slog.Logger) {
	reg.AddTool("social_follow", "Follow another agent to receive their updates", map[string]any{
		"target_agent_id": map[string]any{"type": "string", "description": "ID of the agent to follow", "required": true},
		"follower_id":     map[string]any{"type": "string", "description": "ID of the follower agent", "required": true},
	}, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		target, _ := args["target_agent_id"].(string)
		follower, _ := args["follower_id"].(string)
		if target == "" || follower == "" {
			return mcp.NewToolResultError("target_agent_id and follower_id are required"), nil
		}
		if err := actions.Follow(types.AgentID(follower), types.AgentID(target)); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Now following %s", target)), nil
	})

	reg.AddTool("social_unfollow", "Unfollow an agent", map[string]any{
		"target_agent_id": map[string]any{"type": "string", "description": "ID of the agent to unfollow", "required": true},
		"follower_id":     map[string]any{"type": "string", "description": "ID of the follower agent", "required": true},
	}, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		target, _ := args["target_agent_id"].(string)
		follower, _ := args["follower_id"].(string)
		if err := actions.Unfollow(types.AgentID(follower), types.AgentID(target)); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Unfollowed %s", target)), nil
	})

	reg.AddTool("social_like", "Like a message", map[string]any{
		"agent_id":   map[string]any{"type": "string", "description": "ID of the agent liking", "required": true},
		"message_id": map[string]any{"type": "string", "description": "ID of the message to like", "required": true},
	}, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		agentID, _ := args["agent_id"].(string)
		messageID, _ := args["message_id"].(string)
		if err := actions.Like(types.AgentID(agentID), messageID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		count, _ := actions.GetLikeCount(messageID)
		return mcp.NewToolResultText(fmt.Sprintf("Liked message %s (total likes: %d)", messageID, count)), nil
	})

	reg.AddTool("social_endorse", "Endorse an agent's skill", map[string]any{
		"from_agent_id":   map[string]any{"type": "string", "description": "ID of the endorsing agent", "required": true},
		"target_agent_id": map[string]any{"type": "string", "description": "ID of the agent being endorsed", "required": true},
		"skill_id":        map[string]any{"type": "string", "description": "ID of the skill being endorsed", "required": true},
	}, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		from, _ := args["from_agent_id"].(string)
		target, _ := args["target_agent_id"].(string)
		skillID, _ := args["skill_id"].(string)
		if err := actions.Endorse(types.AgentID(from), types.AgentID(target), skillID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Endorsed %s for skill %s", target, skillID)), nil
	})

	reg.AddTool("social_request_collaboration", "Request collaboration with another agent", map[string]any{
		"from_agent_id":   map[string]any{"type": "string", "description": "ID of the requesting agent", "required": true},
		"target_agent_id": map[string]any{"type": "string", "description": "ID of the target agent", "required": true},
		"proposal":        map[string]any{"type": "string", "description": "JSON proposal for collaboration", "required": true},
	}, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		from, _ := args["from_agent_id"].(string)
		target, _ := args["target_agent_id"].(string)
		proposalStr, _ := args["proposal"].(string)

		var proposal map[string]interface{}
		if err := json.Unmarshal([]byte(proposalStr), &proposal); err != nil {
			proposal = map[string]interface{}{"description": proposalStr}
		}

		if err := actions.RequestCollaboration(types.AgentID(from), types.AgentID(target), proposal); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Collaboration request sent to %s", target)), nil
	})

	reg.AddTool("social_timeline", "Get timeline feed for an agent", map[string]any{
		"agent_id": map[string]any{"type": "string", "description": "ID of the agent", "required": true},
	}, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		agentID, _ := args["agent_id"].(string)

		events, err := timeline.GetTimeline(types.AgentID(agentID), 0, 20)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		data, _ := json.Marshal(events)
		return mcp.NewToolResultText(string(data)), nil
	})

	reg.AddTool("social_graph_info", "Get social graph info for an agent", map[string]any{
		"agent_id": map[string]any{"type": "string", "description": "ID of the agent", "required": true},
	}, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		agentID, _ := args["agent_id"].(string)
		aid := types.AgentID(agentID)

		followers, _ := actions.Graph().GetFollowerCount(aid)
		following, _ := actions.Graph().GetFollowingCount(aid)
		reputation, _ := actions.Graph().GetReputationScore(aid)

		result := map[string]interface{}{
			"agent_id":   agentID,
			"followers":  followers,
			"following":  following,
			"reputation": reputation,
		}
		data, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(data)), nil
	})

	logger.Info("social MCP tools registered", "count", 7)
}
