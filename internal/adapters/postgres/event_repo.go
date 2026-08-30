package postgres

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	domainevent "yadegar/internal/domain/event"
)

// EventRepository implements domainevent.Repository with Postgres.
type EventRepository struct {
	conn *sql.DB
}

func NewEventRepository(conn *sql.DB) *EventRepository {
	return &EventRepository{conn: conn}
}

func (r *EventRepository) Create(ctx context.Context, e *domainevent.Event) error {
	tx, err := r.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO events (id, name, location, creator_id) VALUES ($1, $2, $3, $4)`,
		e.ID, e.Name, e.Location, e.CreatorID,
	)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO event_members (id, event_id, user_id, status, tagged_by) VALUES ($1, $2, $3, 'approved', $4)`,
		uuid.New().String(), e.ID, e.CreatorID, e.CreatorID,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *EventRepository) ListForUser(ctx context.Context, userID, search string) ([]domainevent.Event, error) {
	rows, err := r.conn.QueryContext(ctx, `
		SELECT e.id, e.name, e.location, e.creator_id
		FROM events e
		JOIN event_members m ON m.event_id = e.id
		WHERE m.user_id = $1 AND m.status = 'approved'
		  AND ($2 = '' OR e.name ILIKE '%' || $2 || '%')
		ORDER BY e.created_at DESC`,
		userID, search,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []domainevent.Event{}
	for rows.Next() {
		var e domainevent.Event
		if err := rows.Scan(&e.ID, &e.Name, &e.Location, &e.CreatorID); err != nil {
			return nil, err
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

func (r *EventRepository) Get(ctx context.Context, eventID string) (*domainevent.Event, error) {
	var e domainevent.Event
	err := r.conn.QueryRowContext(ctx,
		`SELECT id, name, location, creator_id FROM events WHERE id = $1`,
		eventID,
	).Scan(&e.ID, &e.Name, &e.Location, &e.CreatorID)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *EventRepository) IsApprovedMember(ctx context.Context, eventID, userID string) (bool, error) {
	var approved bool
	err := r.conn.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM event_members WHERE event_id = $1 AND user_id = $2 AND status = 'approved')`,
		eventID, userID,
	).Scan(&approved)
	return approved, err
}

func (r *EventRepository) ListMembers(ctx context.Context, eventID string) ([]domainevent.Member, error) {
	rows, err := r.conn.QueryContext(ctx, `
		SELECT m.id, m.user_id, u.username, m.status, m.tagged_by
		FROM event_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.event_id = $1`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []domainevent.Member{}
	for rows.Next() {
		var m domainevent.Member
		var status string
		if err := rows.Scan(&m.ID, &m.UserID, &m.Username, &status, &m.TaggedBy); err != nil {
			return nil, err
		}
		m.EventID = eventID
		m.Status = domainevent.MemberStatus(status)
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *EventRepository) MemberStatus(ctx context.Context, eventID, userID string) (domainevent.MemberStatus, bool, error) {
	var status string
	err := r.conn.QueryRowContext(ctx,
		`SELECT status FROM event_members WHERE event_id = $1 AND user_id = $2`,
		eventID, userID,
	).Scan(&status)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return domainevent.MemberStatus(status), true, nil
}

func (r *EventRepository) AddMember(ctx context.Context, m *domainevent.Member) error {
	_, err := r.conn.ExecContext(ctx,
		`INSERT INTO event_members (id, event_id, user_id, status, tagged_by) VALUES ($1, $2, $3, 'invited', $4)`,
		m.ID, m.EventID, m.UserID, m.TaggedBy,
	)
	return err
}

func (r *EventRepository) ApproveMember(ctx context.Context, memberID, userID string) (eventID, taggedBy string, err error) {
	err = r.conn.QueryRowContext(ctx,
		`UPDATE event_members SET status = 'approved' WHERE id = $1 AND user_id = $2
		 RETURNING event_id, tagged_by`,
		memberID, userID,
	).Scan(&eventID, &taggedBy)
	return eventID, taggedBy, err
}

func (r *EventRepository) RejectMember(ctx context.Context, memberID, userID string) (eventID, taggedBy string, err error) {
	err = r.conn.QueryRowContext(ctx,
		`UPDATE event_members SET status = 'rejected' WHERE id = $1 AND user_id = $2
		 RETURNING event_id, tagged_by`,
		memberID, userID,
	).Scan(&eventID, &taggedBy)
	return eventID, taggedBy, err
}

func (r *EventRepository) RemoveMember(ctx context.Context, memberID, userID string) error {
	result, err := r.conn.ExecContext(ctx,
		`DELETE FROM event_members WHERE id = $1 AND user_id = $2 AND status = 'approved'`,
		memberID, userID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
