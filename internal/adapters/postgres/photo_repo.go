package postgres

import (
	"context"
	"database/sql"

	domainphoto "yadegar/internal/domain/photo"
)

// PhotoRepository implements domainphoto.Repository with Postgres.
type PhotoRepository struct {
	conn *sql.DB
}

func NewPhotoRepository(conn *sql.DB) *PhotoRepository {
	return &PhotoRepository{conn: conn}
}

func (r *PhotoRepository) Create(ctx context.Context, p *domainphoto.Photo) error {
	_, err := r.conn.ExecContext(ctx,
		`INSERT INTO photos (id, event_id, uploader_id, storage_key) VALUES ($1, $2, $3, $4)`,
		p.ID, p.EventID, p.UploaderID, p.URL,
	)
	return err
}

func (r *PhotoRepository) CountForEvent(ctx context.Context, eventID string) (int, error) {
	var count int
	err := r.conn.QueryRowContext(ctx,
		`SELECT count(*) FROM photos WHERE event_id = $1`,
		eventID,
	).Scan(&count)
	return count, err
}

func (r *PhotoRepository) ListForEvent(ctx context.Context, eventID string, limit int) ([]domainphoto.Photo, error) {
	rows, err := r.conn.QueryContext(ctx,
		`SELECT id, event_id, uploader_id, storage_key
		 FROM photos
		 WHERE event_id = $1
		 ORDER BY created_at ASC
		 LIMIT $2`,
		eventID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	photos := []domainphoto.Photo{}
	for rows.Next() {
		var p domainphoto.Photo
		if err := rows.Scan(&p.ID, &p.EventID, &p.UploaderID, &p.URL); err != nil {
			return nil, err
		}
		photos = append(photos, p)
	}
	return photos, rows.Err()
}
