// Package gallery holds a read-only projection. Gallery combines Event
// and Photo data for one screen. Gallery owns no business rules — it
// only shapes a query result.
package gallery

// Event is one row of a user's gallery: an Event they belong to, with a
// photo count.
type Event struct {
	ID         string
	Name       string
	PhotoCount int
	ApprovedAt string
}
