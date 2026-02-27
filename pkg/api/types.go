package api

import "time"

// Session types

type CreateSessionResponse struct {
	SessionID string `json:"session_id"`
}

type CommitRequest struct {
	Message  string            `json:"message"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type CommitResponse struct {
	CommitID string `json:"commit_id,omitempty"`

	// Fields present when approval is required (202)
	ApprovalRequired bool   `json:"approval_required,omitempty"`
	SessionID        string `json:"session_id,omitempty"`
	Message          string `json:"message,omitempty"`
	APIURL           string `json:"api_url,omitempty"`
	WebURL           string `json:"web_url,omitempty"`
}

// Object types

type StageResponse struct {
	UploadURL       string    `json:"upload_url"`
	PhysicalAddress string    `json:"physical_address"`
	Signature       string    `json:"signature"`
	ExpiresAt       time.Time `json:"expires_at"`
}

type FinalizeRequest struct {
	PhysicalAddress string `json:"physical_address"`
	Signature       string `json:"signature"`
	ContentType     string `json:"content_type,omitempty"`
}

type FinalizeResponse struct {
	Path string `json:"path"`
	ETag string `json:"etag"`
}

type UploadResponse struct {
	Path string `json:"path"`
	ETag string `json:"etag"`
}

type BulkDeleteRequest struct {
	Paths []string `json:"paths"`
}

type BulkDeleteResponse struct {
	Deleted int `json:"deleted"`
}

// Listing types

type SourceMetadata struct {
	ConnectorID   string    `json:"connector_id,omitempty"`
	ConnectorType string    `json:"connector_type,omitempty"`
	SourcePath    string    `json:"source_path,omitempty"`
	VersionID     string    `json:"version_id,omitempty"`
	SourceETag    string    `json:"source_etag,omitempty"`
	ImportTime    time.Time `json:"import_time,omitempty"`
	ImportJobID   string    `json:"import_job_id,omitempty"`
}

type Entry struct {
	Address        string            `json:"address,omitempty"`
	LastModified   time.Time         `json:"last_modified,omitempty"`
	Size           int64             `json:"size,omitempty"`
	ETag           string            `json:"e_tag,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	AddressType    int               `json:"address_type,omitempty"`
	ContentType    string            `json:"content_type,omitempty"`
	SourceMetadata *SourceMetadata   `json:"source_metadata,omitempty"`
}

type ListingEntry struct {
	Path   string `json:"path"`
	Type   string `json:"type,omitempty"`   // "object" or "prefix"
	Status string `json:"status,omitempty"` // "added", "modified", "removed"
	Entry  *Entry `json:"entry,omitempty"`
}

type Pagination struct {
	HasMore    bool   `json:"has_more"`
	NextOffset string `json:"next_offset"`
	MaxPerPage int    `json:"max_per_page"`
}

type ListObjectsResponse struct {
	Results    []ListingEntry `json:"results"`
	Pagination Pagination     `json:"pagination"`
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
