package types

import "time"

type AgentID string

type ConnectionType string

const (
	ConnMCPSSE    ConnectionType = "mcp_sse"
	ConnA2A       ConnectionType = "a2a"
	ConnWebSocket ConnectionType = "websocket"
)

type MessageType string

const (
	MsgDirect    MessageType = "direct"
	MsgBroadcast MessageType = "broadcast"
	MsgGroup     MessageType = "group"
)

type RelationType string

const (
	RelFollow      RelationType = "follow"
	RelEndorse     RelationType = "endorse"
	RelCollaborate RelationType = "collaborate"
)

type TaskState string

const (
	TaskStateUnspecified   TaskState = "UNSPECIFIED"
	TaskStateSubmitted     TaskState = "SUBMITTED"
	TaskStateWorking       TaskState = "WORKING"
	TaskStateCompleted     TaskState = "COMPLETED"
	TaskStateFailed        TaskState = "FAILED"
	TaskStateCanceled      TaskState = "CANCELED"
	TaskStateInputRequired TaskState = "INPUT_REQUIRED"
	TaskStateRejected      TaskState = "REJECTED"
	TaskStateAuthRequired  TaskState = "AUTH_REQUIRED"
)

func (s TaskState) IsTerminal() bool {
	switch s {
	case TaskStateCompleted, TaskStateFailed, TaskStateCanceled, TaskStateRejected:
		return true
	}
	return false
}

func (s TaskState) IsInterrupted() bool {
	return s == TaskStateInputRequired || s == TaskStateAuthRequired
}

type Part struct {
	Text      string         `json:"text,omitempty"`
	Raw       []byte         `json:"raw,omitempty"`
	URL       string         `json:"url,omitempty"`
	Data      any            `json:"data,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Filename  string         `json:"filename,omitempty"`
	MediaType string         `json:"mediaType,omitempty"`
}

type Message struct {
	MessageID        string         `json:"messageId"`
	ContextID        string         `json:"contextId,omitempty"`
	TaskID           string         `json:"taskId,omitempty"`
	Role             string         `json:"role"`
	Parts            []Part         `json:"parts"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	Extensions       []string       `json:"extensions,omitempty"`
	ReferenceTaskIDs []string       `json:"referenceTaskIds,omitempty"`
}

type TaskStatus struct {
	State     TaskState `json:"state"`
	Message   *Message  `json:"message,omitempty"`
	Timestamp string    `json:"timestamp,omitempty"`
}

type Artifact struct {
	ArtifactID  string         `json:"artifactId"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Parts       []Part         `json:"parts"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Extensions  []string       `json:"extensions,omitempty"`
}

type Task struct {
	ID        string     `json:"id"`
	ContextID string     `json:"contextId,omitempty"`
	Status    TaskStatus `json:"status"`
	Artifacts []Artifact `json:"artifacts,omitempty"`
	History   []Message  `json:"history,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type AgentSkill struct {
	ID          string   `json:"id" yaml:"id"`
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description" yaml:"description"`
	Tags        []string `json:"tags" yaml:"tags"`
	Examples    []string `json:"examples,omitempty" yaml:"examples,omitempty"`
	InputModes  []string `json:"inputModes,omitempty" yaml:"input_modes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty" yaml:"output_modes,omitempty"`
}

type AgentCapabilities struct {
	Streaming         bool `json:"streaming,omitempty" yaml:"streaming"`
	PushNotifications bool `json:"pushNotifications,omitempty" yaml:"push_notifications"`
	ExtendedAgentCard bool `json:"extendedAgentCard,omitempty" yaml:"extended_agent_card"`
	SocialExtensions  bool `json:"socialExtensions,omitempty" yaml:"social_extensions"`
}

type AgentProvider struct {
	URL          string `json:"url" yaml:"url"`
	Organization string `json:"organization" yaml:"organization"`
}

type AgentInterface struct {
	URL             string `json:"url" yaml:"url"`
	ProtocolBinding string `json:"protocolBinding" yaml:"protocol_binding"`
	Tenant          string `json:"tenant,omitempty" yaml:"tenant,omitempty"`
	ProtocolVersion string `json:"protocolVersion" yaml:"protocol_version"`
}

type AgentCard struct {
	Name                string            `json:"name" yaml:"name"`
	Description         string            `json:"description" yaml:"description"`
	SupportedInterfaces []AgentInterface  `json:"supportedInterfaces" yaml:"supported_interfaces"`
	Provider            *AgentProvider    `json:"provider,omitempty" yaml:"provider,omitempty"`
	Version             string            `json:"version" yaml:"version"`
	DocumentationURL    string            `json:"documentationUrl,omitempty" yaml:"documentation_url,omitempty"`
	Capabilities        AgentCapabilities `json:"capabilities" yaml:"capabilities"`
	DefaultInputModes   []string          `json:"defaultInputModes" yaml:"default_input_modes"`
	DefaultOutputModes  []string          `json:"defaultOutputModes" yaml:"default_output_modes"`
	Skills              []AgentSkill      `json:"skills" yaml:"skills"`
	IconURL             string            `json:"iconUrl,omitempty" yaml:"icon_url,omitempty"`
	SocialProfile       *SocialProfile    `json:"socialProfile,omitempty" yaml:"social_profile,omitempty"`
}

// SocialProfile extends the standard Agent Card with social metadata (A2A Social Extension Layer 1).
type SocialProfile struct {
	Followers    int                `json:"followers"`
	Following    int                `json:"following"`
	Reputation   float64            `json:"reputation"`
	TrustLevel   string             `json:"trustLevel,omitempty"`
	Tags         []string           `json:"tags,omitempty"`
	Endorsements map[string]int     `json:"endorsements,omitempty"`
	JoinedAt     string             `json:"joinedAt,omitempty"`
}

type Agent struct {
	ID              AgentID    `json:"id"`
	Name            string     `json:"name"`
	Card            *AgentCard `json:"card,omitempty"`
	ReputationScore float64    `json:"reputationScore"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type Session struct {
	ID             string         `json:"id"`
	AgentID        AgentID        `json:"agentId"`
	ConnectionType ConnectionType `json:"connectionType"`
	Status         string         `json:"status"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	LastActiveAt   time.Time      `json:"lastActiveAt"`
}

type SocialRelation struct {
	ID           int64        `json:"id"`
	FromAgent    AgentID      `json:"fromAgent"`
	ToAgent      AgentID      `json:"toAgent"`
	RelationType RelationType `json:"relationType"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time    `json:"createdAt"`
}

type Group struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedBy   AgentID   `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
}

type GroupMember struct {
	GroupID  string    `json:"groupId"`
	AgentID  AgentID   `json:"agentId"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joinedAt"`
}

type Subscription struct {
	AgentID   AgentID   `json:"agentId"`
	Topic     string    `json:"topic"`
	CreatedAt time.Time `json:"createdAt"`
}

type TimelineEvent struct {
	ID          int64     `json:"id"`
	AgentID     AgentID   `json:"agentId"`
	EventType   string    `json:"eventType"`
	SourceAgent AgentID   `json:"sourceAgent"`
	MessageID   string    `json:"messageId,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type RoutedMessage struct {
	ID          string      `json:"id"`
	FromAgent   AgentID     `json:"fromAgent"`
	ToAgent     AgentID     `json:"toAgent,omitempty"`
	GroupID     string      `json:"groupId,omitempty"`
	Topic       string      `json:"topic,omitempty"`
	Content     *Message    `json:"content"`
	MessageType MessageType `json:"messageType"`
	CreatedAt   time.Time   `json:"createdAt"`
}

// ─── A2A Social Extensions ──────────────────────────────────────

// SocialEvent is a lightweight event that doesn't create a Task (Layer 2).
type SocialEvent struct {
	Type      string         `json:"type"`
	From      string         `json:"from"`
	To        string         `json:"to,omitempty"`
	Skill     string         `json:"skill,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Timestamp string         `json:"timestamp"`
}

// SocialEventType constants for the social/event JSON-RPC method.
const (
	SocialEventFollow     = "follow"
	SocialEventUnfollow   = "unfollow"
	SocialEventLike       = "like"
	SocialEventUnlike     = "unlike"
	SocialEventEndorse    = "endorse"
	SocialEventCollabReq  = "collaborate.request"
	SocialEventCollabAck  = "collaborate.accept"
	SocialEventCollabNack = "collaborate.reject"
	SocialEventMention    = "mention"
	SocialEventRepUpdate  = "reputation.update"
)

// RoutingStrategy controls relationship-aware message routing (Layer 3).
type RoutingStrategy struct {
	Strategy     string   `json:"strategy"`
	TrustMinimum float64  `json:"trustMinimum,omitempty"`
	Exclude      []string `json:"exclude,omitempty"`
	Topic        string   `json:"topic,omitempty"`
	GroupID      string   `json:"groupId,omitempty"`
}

const (
	RouteDirect       = "direct"
	RouteFollowers    = "followers"
	RouteMutualFollow = "mutual_follows"
	RouteGroup        = "group"
	RouteTopic        = "topic"
	RouteTrustCircle  = "trust_circle"
)

// ConversationContext is a persistent, structured conversation room (Layer 4).
type ConversationContext struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	Topic        string   `json:"topic,omitempty"`
	Participants []string `json:"participants"`
	Status       string   `json:"status"`
	MessageCount int      `json:"messageCount"`
	CreatedAt    string   `json:"createdAt"`
	UpdatedAt    string   `json:"updatedAt"`
}

const (
	ConvoStatusActive   = "active"
	ConvoStatusArchived = "archived"
	ConvoStatusClosed   = "closed"
)
