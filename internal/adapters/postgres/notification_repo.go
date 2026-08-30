package postgres

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	domainnotif "yadegar/internal/domain/notification"
)

// NotificationRepository implements domainnotif.Repository with Postgres.
type NotificationRepository struct {
	conn *sql.DB
}

func NewNotificationRepository(conn *sql.DB) *NotificationRepository {
	return &NotificationRepository{conn: conn}
}

func (r *NotificationRepository) Create(ctx context.Context, n *domainnotif.Notification) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	_, err := r.conn.ExecContext(ctx,
		`INSERT INTO notifications (id, user_id, type, event_id, from_user_id) VALUES ($1, $2, $3, $4, $5)`,
		n.ID, n.UserID, n.Type, n.EventID, n.FromUserID,
	)
	return err
}

func (r *NotificationRepository) ListForUser(ctx context.Context, userID string) ([]domainnotif.Notification, error) {
	rows, err := r.conn.QueryContext(ctx, `
		SELECT n.id, n.type, n.event_id, e.name, u.username, n.read, n.created_at,
		       m.id, m.status
		FROM notifications n
		JOIN events e ON e.id = n.event_id
		JOIN users u ON u.id = n.from_user_id
		LEFT JOIN event_members m ON m.event_id = n.event_id AND m.user_id = n.user_id
		WHERE n.user_id = $1
		ORDER BY n.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []domainnotif.Notification{}
	for rows.Next() {
		var n domainnotif.Notification
		var memberID, memberStatus sql.NullString
		if err := rows.Scan(&n.ID, &n.Type, &n.EventID, &n.EventName, &n.FromUsername, &n.Read, &n.CreatedAt, &memberID, &memberStatus); err != nil {
			return nil, err
		}
		n.UserID = userID
		n.MemberID = memberID.String
		n.MemberStatus = memberStatus.String
		results = append(results, n)
	}
	return results, rows.Err()
}

func (r *NotificationRepository) MarkRead(ctx context.Context, notificationID, userID string) error {
	_, err := r.conn.ExecContext(ctx,
		`UPDATE notifications SET read = true WHERE id = $1 AND user_id = $2`,
		notificationID, userID,
	)
	return err
}
