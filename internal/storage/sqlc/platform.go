package sqlc

import "context"

type PlatformTenant struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	CIDR       string `json:"cidr"`
	Status     string `json:"status"`
	MaxDevices int    `json:"max_devices"`
	Devices    int    `json:"devices"`
	Online     int    `json:"online"`
}

func (q *Queries) PlatformTenants(ctx context.Context) ([]PlatformTenant, error) {
	rows, err := q.db.Query(ctx, `
SELECT u.id, u.email, u.overlay_cidr::text, u.status, u.max_devices,
       count(d.id), count(d.id) FILTER (WHERE d.last_seen_at >= now() - interval '60 seconds')
FROM users u
LEFT JOIN devices d ON d.user_id = u.id AND d.status = 'active'
GROUP BY u.id ORDER BY u.email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []PlatformTenant{}
	for rows.Next() {
		var item PlatformTenant
		if err := rows.Scan(&item.ID, &item.Email, &item.CIDR, &item.Status, &item.MaxDevices, &item.Devices, &item.Online); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
