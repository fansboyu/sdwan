package sqlc

import (
	"context"
	"time"
)

type LANVerification struct {
	Device    string    `json:"device"`
	Target    string    `json:"target"`
	Reachable bool      `json:"reachable"`
	CheckedAt time.Time `json:"checked_at"`
}

func (q *Queries) UpsertLANVerification(ctx context.Context, device, route, target string, reachable bool) error {
	_, err := q.db.Exec(ctx, `INSERT INTO employee_lan_verifications(device_id,route_id,target,reachable)
VALUES ($1,$2,$3,$4) ON CONFLICT (device_id,route_id,target) DO UPDATE SET reachable=EXCLUDED.reachable,checked_at=now()`, device, route, target, reachable)
	return err
}

func (q *Queries) ListLANVerifications(ctx context.Context, user string) ([]LANVerification, error) {
	rows, err := q.db.Query(ctx, `SELECT d.hostname,v.target,v.reachable,v.checked_at
FROM employee_lan_verifications v JOIN devices d ON d.id=v.device_id
JOIN subnet_routes r ON r.id=v.route_id JOIN devices hub ON hub.id=r.device_id
WHERE d.user_id=$1 AND r.user_id=$1 AND d.status='active' AND d.site_role<>'main_site'
AND hub.status='active' AND hub.site_role='main_site'
AND r.advertised AND r.approved AND r.status='active'
ORDER BY v.checked_at DESC LIMIT 20`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []LANVerification{}
	for rows.Next() {
		var item LANVerification
		if err := rows.Scan(&item.Device, &item.Target, &item.Reachable, &item.CheckedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
