package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// integrationRecord holds forge-side integration state for a project. Webhook
// secrets are treated as credentials and stored mode 0600.
type integrationRecord struct {
	WebhookSecret string `json:"webhook_secret,omitempty"`
	HookID        int64  `json:"hook_id,omitempty"`
}

// Store persists local state for webhook secrets and hook IDs. Secrets are
// never written to a manifest or stdout; they live here, mode 0600.
type Store struct {
	dir string
}

// NewStore creates a state store rooted at dir, creating it with 0700.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// StateDir returns the per-profile state directory, honouring XDG_STATE_HOME.
func StateDir(profile string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	xdg := os.Getenv("XDG_STATE_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(xdg, "coprctl", profile), nil
}

func (s *Store) recordPath(owner, project string) string {
	return filepath.Join(s.dir, owner+"__"+project+".json")
}

// GetSecret returns the cached webhook secret for a project.
func (s *Store) GetSecret(owner, project string) (string, error) {
	data, err := os.ReadFile(s.recordPath(owner, project))
	if err != nil {
		return "", err
	}
	var rec integrationRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return "", err
	}
	return rec.WebhookSecret, nil
}

// SetSecret caches the webhook secret for a project.
func (s *Store) SetSecret(owner, project, secret string) error {
	rec := integrationRecord{WebhookSecret: secret}
	return s.write(owner, project, rec)
}

// SetHookID records the forge hook id for a project.
func (s *Store) SetHookID(owner, project string, id int64) error {
	rec := s.read(owner, project)
	rec.HookID = id
	return s.write(owner, project, rec)
}

func (s *Store) read(owner, project string) integrationRecord {
	data, err := os.ReadFile(s.recordPath(owner, project))
	if err != nil {
		return integrationRecord{}
	}
	var rec integrationRecord
	_ = json.Unmarshal(data, &rec)
	return rec
}

func (s *Store) write(owner, project string, rec integrationRecord) error {
	data, err := json.Marshal(&rec)
	if err != nil {
		return err
	}
	return os.WriteFile(s.recordPath(owner, project), data, 0o600)
}
