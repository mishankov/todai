// Package project implements project application operations and PostgreSQL persistence.
package project

import "time"

// Layout describes how a project presents its sections.
type Layout string

const (
	LayoutList  Layout = "list"
	LayoutBoard Layout = "board"
)

// Project groups tasks owned by one user.
type Project struct {
	ID             string     `db:"id" json:"id"`
	UserID         string     `db:"user_id" json:"-"`
	Name           string     `db:"name" json:"name"`
	Layout         Layout     `db:"layout" json:"layout"`
	Position       int64      `db:"position" json:"position"`
	Version        int64      `db:"version" json:"version"`
	ArchivedAt     *time.Time `db:"archived_at" json:"archivedAt"`
	CreatedAt      time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updatedAt"`
	LastModifiedBy string     `db:"last_modified_by" json:"lastModifiedBy"`
}

// Update contains editable project fields and the version observed by the caller.
type Update struct {
	Version  int64
	Name     *string
	Archived *bool
	Layout   *Layout
}

// Section groups tasks within one project.
type Section struct {
	ID             string    `db:"id" json:"id"`
	UserID         string    `db:"user_id" json:"-"`
	ProjectID      string    `db:"project_id" json:"projectId"`
	Name           string    `db:"name" json:"name"`
	Position       int64     `db:"position" json:"position"`
	Version        int64     `db:"version" json:"version"`
	CreatedAt      time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt      time.Time `db:"updated_at" json:"updatedAt"`
	LastModifiedBy string    `db:"last_modified_by" json:"lastModifiedBy"`
}

// SectionUpdate contains editable section fields and the observed version.
type SectionUpdate struct {
	Version int64
	Name    *string
}
