package a2a

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zuwance/agent-social-gateway/internal/storage"
	"github.com/zuwance/agent-social-gateway/internal/types"
)

type TaskStore struct {
	db *storage.DB
}

func NewTaskStore(db *storage.DB) *TaskStore {
	return &TaskStore{db: db}
}

func (s *TaskStore) Create(task *types.Task) error {
	statusJSON, _ := json.Marshal(task.Status)
	artifactsJSON, _ := json.Marshal(task.Artifacts)
	historyJSON, _ := json.Marshal(task.History)
	metaJSON, _ := json.Marshal(task.Metadata)

	_, err := s.db.Exec(`INSERT INTO tasks (id, context_id, state, status_message_json, artifacts_json, history_json, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.ContextID, string(task.Status.State), string(statusJSON),
		string(artifactsJSON), string(historyJSON), string(metaJSON))
	return err
}

func (s *TaskStore) Get(id string) (*types.Task, error) {
	row := s.db.QueryRow(`SELECT id, context_id, state, status_message_json, artifacts_json, history_json, metadata_json, created_at, updated_at FROM tasks WHERE id = ?`, id)
	return s.scanTask(row)
}

func (s *TaskStore) Update(task *types.Task) error {
	statusJSON, _ := json.Marshal(task.Status)
	artifactsJSON, _ := json.Marshal(task.Artifacts)
	historyJSON, _ := json.Marshal(task.History)
	metaJSON, _ := json.Marshal(task.Metadata)

	_, err := s.db.Exec(`UPDATE tasks SET state = ?, status_message_json = ?, artifacts_json = ?, history_json = ?, metadata_json = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		string(task.Status.State), string(statusJSON), string(artifactsJSON),
		string(historyJSON), string(metaJSON), task.ID)
	return err
}

func (s *TaskStore) ListByContext(contextID string, limit, offset int) ([]*types.Task, error) {
	rows, err := s.db.Query(`SELECT id, context_id, state, status_message_json, artifacts_json, history_json, metadata_json, created_at, updated_at FROM tasks WHERE context_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		contextID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanTasks(rows)
}

func (s *TaskStore) ListByState(state types.TaskState, limit, offset int) ([]*types.Task, error) {
	rows, err := s.db.Query(`SELECT id, context_id, state, status_message_json, artifacts_json, history_json, metadata_json, created_at, updated_at FROM tasks WHERE state = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		string(state), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanTasks(rows)
}

func (s *TaskStore) List(limit, offset int) ([]*types.Task, int, error) {
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(`SELECT id, context_id, state, status_message_json, artifacts_json, history_json, metadata_json, created_at, updated_at FROM tasks ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	tasks, err := s.scanTasks(rows)
	return tasks, total, err
}

func (s *TaskStore) Cancel(id string) error {
	_, err := s.db.Exec(`UPDATE tasks SET state = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND state NOT IN (?, ?, ?, ?)`,
		string(types.TaskStateCanceled), id,
		string(types.TaskStateCompleted), string(types.TaskStateFailed),
		string(types.TaskStateCanceled), string(types.TaskStateRejected))
	return err
}

func (s *TaskStore) scanTask(row *sql.Row) (*types.Task, error) {
	var (
		id, contextID, state                          string
		statusJSON, artifactsJSON, historyJSON, metaJSON sql.NullString
		createdAt, updatedAt                          time.Time
	)
	if err := row.Scan(&id, &contextID, &state, &statusJSON, &artifactsJSON, &historyJSON, &metaJSON, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task not found: %s", id)
		}
		return nil, err
	}

	task := &types.Task{
		ID:        id,
		ContextID: contextID,
		Status:    types.TaskStatus{State: types.TaskState(state)},
	}

	if statusJSON.Valid {
		json.Unmarshal([]byte(statusJSON.String), &task.Status)
	}
	if artifactsJSON.Valid {
		json.Unmarshal([]byte(artifactsJSON.String), &task.Artifacts)
	}
	if historyJSON.Valid {
		json.Unmarshal([]byte(historyJSON.String), &task.History)
	}
	if metaJSON.Valid {
		json.Unmarshal([]byte(metaJSON.String), &task.Metadata)
	}

	return task, nil
}

func (s *TaskStore) scanTasks(rows *sql.Rows) ([]*types.Task, error) {
	var tasks []*types.Task
	for rows.Next() {
		var (
			id, contextID, state                          string
			statusJSON, artifactsJSON, historyJSON, metaJSON sql.NullString
			createdAt, updatedAt                          time.Time
		)
		if err := rows.Scan(&id, &contextID, &state, &statusJSON, &artifactsJSON, &historyJSON, &metaJSON, &createdAt, &updatedAt); err != nil {
			return nil, err
		}

		task := &types.Task{
			ID:        id,
			ContextID: contextID,
			Status:    types.TaskStatus{State: types.TaskState(state)},
		}
		if statusJSON.Valid {
			json.Unmarshal([]byte(statusJSON.String), &task.Status)
		}
		if artifactsJSON.Valid {
			json.Unmarshal([]byte(artifactsJSON.String), &task.Artifacts)
		}
		if historyJSON.Valid {
			json.Unmarshal([]byte(historyJSON.String), &task.History)
		}
		if metaJSON.Valid {
			json.Unmarshal([]byte(metaJSON.String), &task.Metadata)
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

// Push notification config storage
func (s *TaskStore) CreatePushConfig(cfg *PushNotificationConfig) error {
	_, err := s.db.Exec(`INSERT INTO push_notification_configs (id, task_id, url, token, auth_scheme, auth_credentials) VALUES (?, ?, ?, ?, ?, ?)`,
		cfg.ID, cfg.TaskID, cfg.URL, cfg.Token, cfg.AuthScheme, cfg.AuthCredentials)
	return err
}

func (s *TaskStore) GetPushConfig(taskID, configID string) (*PushNotificationConfig, error) {
	var cfg PushNotificationConfig
	err := s.db.QueryRow(`SELECT id, task_id, url, token, auth_scheme, auth_credentials FROM push_notification_configs WHERE id = ? AND task_id = ?`,
		configID, taskID).Scan(&cfg.ID, &cfg.TaskID, &cfg.URL, &cfg.Token, &cfg.AuthScheme, &cfg.AuthCredentials)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *TaskStore) ListPushConfigs(taskID string) ([]*PushNotificationConfig, error) {
	rows, err := s.db.Query(`SELECT id, task_id, url, token, auth_scheme, auth_credentials FROM push_notification_configs WHERE task_id = ?`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*PushNotificationConfig
	for rows.Next() {
		var cfg PushNotificationConfig
		if err := rows.Scan(&cfg.ID, &cfg.TaskID, &cfg.URL, &cfg.Token, &cfg.AuthScheme, &cfg.AuthCredentials); err != nil {
			return nil, err
		}
		configs = append(configs, &cfg)
	}
	return configs, rows.Err()
}

func (s *TaskStore) DeletePushConfig(taskID, configID string) error {
	_, err := s.db.Exec(`DELETE FROM push_notification_configs WHERE id = ? AND task_id = ?`, configID, taskID)
	return err
}

type PushNotificationConfig struct {
	ID              string `json:"id"`
	TaskID          string `json:"taskId"`
	URL             string `json:"url"`
	Token           string `json:"token,omitempty"`
	AuthScheme      string `json:"authScheme,omitempty"`
	AuthCredentials string `json:"authCredentials,omitempty"`
}
