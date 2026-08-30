package postgres

import (
	"context"
	"database/sql"

	domaingallery "yadegar/internal/domain/gallery"
)

// GalleryRepository implements domaingallery.Repository with Postgres.
type GalleryRepository struct {
	conn *sql.DB
}

func NewGalleryRepository(conn *sql.DB) *GalleryRepository {
	return &GalleryRepository{conn: conn}
}

func (r *GalleryRepository) ListForUser(ctx context.Context, userID string) ([]domaingallery.Event, error) {
	rows, err := r.conn.QueryContext(ctx, `
		SELECT e.id, e.name, m.created_at,
		       (SELECT count(*) FROM photos p WHERE p.event_id = e.id) AS photo_count
		FROM event_members m
		JOIN events e ON e.id = m.event_id
		WHERE m.user_id = $1 AND m.status = 'approved'
		ORDER BY m.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []domaingallery.Event{}
	for rows.Next() {
		var e domaingallery.Event
		if err := rows.Scan(&e.ID, &e.Name, &e.ApprovedAt, &e.PhotoCount); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
