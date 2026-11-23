package sqlc

import (
	"context"
	"time"
)

type Enrollment struct {
	ID        string     `json:"id"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at"`
	RevokedAt *time.Time `json:"revoked_at"`
}

func (q *Queries) CreateEnrollment(ctx context.Context, id, user, hash string) (Enrollment, error) {
	item := Enrollment{ID: id}
	err := q.db.QueryRow(ctx, `INSERT INTO device_enrollments(id,user_id,token_hash,expires_at)
VALUES ($1,$2,$3,now()+interval '30 minutes') RETURNING expires_at`, id, user, hash).Scan(&item.ExpiresAt)
	return item, err
}

func (q *Queries) ListEnrollments(ctx context.Context, user string) ([]Enrollment, error) {
	rows, err := q.db.Query(ctx, `SELECT id,expires_at,used_at,revoked_at FROM device_enrollments WHERE user_id=$1 ORDER BY expires_at DESC LIMIT 50`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Enrollment{}
	for rows.Next() {
		var item Enrollment
		if err := rows.Scan(&item.ID, &item.ExpiresAt, &item.UsedAt, &item.RevokedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (q *Queries) RevokeEnrollment(ctx context.Context, user, id string) (bool, error) {
	tag, err := q.db.Exec(ctx, `UPDATE device_enrollments SET revoked_at=now() WHERE user_id=$1 AND id=$2 AND used_at IS NULL AND revoked_at IS NULL AND expires_at>now()`, user, id)
	return tag.RowsAffected() == 1, err
}

func (q *Queries) ConsumeEnrollment(ctx context.Context, hash string) (string, error) {
	var user string
	err := q.db.QueryRow(ctx, `UPDATE device_enrollments SET used_at=now() WHERE token_hash=$1 AND used_at IS NULL AND revoked_at IS NULL AND expires_at>now() RETURNING user_id`, hash).Scan(&user)
	return user, err
}

func (q *Queries) LockDeviceEnrollment(ctx context.Context, user string) error {
	_, err := q.db.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 20261123))`, user)
	return err
}
