// Package task implements task application operations and PostgreSQL persistence.
package task

import "time"

// Status describes whether a task still requires attention.
type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
)

// Task is the task representation returned by application operations.
type Task struct {
	ID             string     `db:"id" json:"id"`
	UserID         string     `db:"user_id" json:"-"`
	ProjectID      *string    `db:"project_id" json:"projectId"`
	ParentID       *string    `db:"parent_id" json:"parentId"`
	Title          string     `db:"title" json:"title"`
	Description    *string    `db:"description" json:"description"`
	Status         Status     `db:"status" json:"status"`
	Priority       int        `db:"priority" json:"priority"`
	DueDate        *Date      `db:"due_date" json:"dueDate"`
	DueTime        *TimeOfDay `db:"due_time" json:"dueTime"`
	DueTimezone    *string    `db:"due_timezone" json:"dueTimezone"`
	Position       int64      `db:"position" json:"position"`
	Version        int64      `db:"version" json:"version"`
	CompletedAt    *time.Time `db:"completed_at" json:"completedAt"`
	CreatedAt      time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updatedAt"`
	LastModifiedBy string     `db:"last_modified_by" json:"lastModifiedBy"`
}

// Nullable represents an explicitly provided nullable field in an update.
type Nullable[T any] struct {
	Value *T
}

// Update contains the editable fields and the version the caller observed.
type Update struct {
	Version     int64
	Title       *string
	Description *Nullable[string]
	ProjectID   *Nullable[string]
	Priority    *int
	DueDate     *Nullable[Date]
	DueTime     *Nullable[TimeOfDay]
	DueTimezone *Nullable[string]
}
