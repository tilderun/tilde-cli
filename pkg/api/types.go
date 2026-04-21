package api

import "time"

// Pagination is used across list endpoints.
type Pagination struct {
	HasMore    bool   `json:"has_more"`
	NextOffset string `json:"next_offset"`
	MaxPerPage int    `json:"max_per_page"`
}

// Repository types

type Repository struct {
	OrganizationSlug string `json:"organization_slug"`
	Name             string `json:"name"`
	CreatedAt        string `json:"created_at,omitempty"`
}

type ListRepositoriesResponse struct {
	Results    []Repository `json:"results"`
	Pagination Pagination   `json:"pagination"`
}

// Sandbox types

type CreateSandboxRequest struct {
	Image          string            `json:"image"`
	Command        []string          `json:"command,omitempty"`
	Mountpoint     string            `json:"mountpoint,omitempty"`
	PathPrefix     string            `json:"path_prefix,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	EnvVars        map[string]string `json:"env_vars,omitempty"`
	RunAs          string            `json:"run_as,omitempty"`
	// Mode is one of SandboxMode* constants. Omit to get the server default
	// (one-shot).
	Mode string `json:"mode,omitempty"`
}

type CreateSandboxResponse struct {
	SandboxID string `json:"sandbox_id"`
}

type Sandbox struct {
	ID             string            `json:"id"`
	RepositoryID   string            `json:"repository_id,omitempty"`
	Image          string            `json:"image"`
	Command        []string          `json:"command,omitempty"`
	Mountpoint     string            `json:"mountpoint,omitempty"`
	PathPrefix     string            `json:"path_prefix,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	Mode           string            `json:"mode,omitempty"`
	EnvVars        map[string]string `json:"env_vars,omitempty"`
	Status         string            `json:"status"`
	StatusReason   string            `json:"status_reason,omitempty"`
	ErrorMessage   string            `json:"error_message,omitempty"`
	ExitCode       *int              `json:"exit_code,omitempty"`
	CommitID       string            `json:"commit_id,omitempty"`
	WebURL         string            `json:"web_url,omitempty"`
	CreatedByType  string            `json:"created_by_type,omitempty"`
	CreatedBy      string            `json:"created_by,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	FinishedAt     *time.Time        `json:"finished_at,omitempty"`
}

type SandboxStatusResponse struct {
	Status       string `json:"status"`
	StatusReason string `json:"status_reason,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	ExitCode     *int   `json:"exit_code,omitempty"`
	CommitID     string `json:"commit_id,omitempty"`
	WebURL       string `json:"web_url,omitempty"`
}

// Sandbox lifecycle states. Mirrors the enum in the server OpenAPI spec.
const (
	SandboxStatusStarting         = "starting"
	SandboxStatusRunning          = "running"
	SandboxStatusCommitted        = "committed"
	SandboxStatusAwaitingApproval = "awaiting_approval"
	SandboxStatusFailed           = "failed"
	SandboxStatusCancelled        = "cancelled"
)

// Sandbox execution modes. Mirrors the enum in the server OpenAPI spec.
const (
	SandboxModeOneShot     = "one-shot"
	SandboxModeInteractive = "interactive"
	SandboxModeService     = "service"
)

// IsTerminal reports whether the sandbox has reached a final state from which
// it will not transition out of.
func (s *SandboxStatusResponse) IsTerminal() bool {
	switch s.Status {
	case SandboxStatusCommitted, SandboxStatusAwaitingApproval, SandboxStatusFailed, SandboxStatusCancelled:
		return true
	}
	return false
}

// IsAttachable reports whether an interactive terminal may be attached now.
// Only the `running` state accepts terminal connections.
func (s *SandboxStatusResponse) IsAttachable() bool {
	return s.Status == SandboxStatusRunning
}

// LogsAvailable reports whether log streams can be read now. Logs are
// unavailable while the sandbox is still being prepared (`starting`).
func (s *SandboxStatusResponse) LogsAvailable() bool {
	return s.Status != SandboxStatusStarting && s.Status != ""
}

type ListSandboxesResponse struct {
	Results    []Sandbox  `json:"results"`
	Pagination Pagination `json:"pagination"`
}
