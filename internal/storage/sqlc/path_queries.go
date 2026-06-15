package sqlc

import (
	"context"
	"time"
)

func (q *Queries) UpdateUserPathMode(ctx context.Context, userID, mode string) error {
	_, err := q.db.Exec(ctx, `UPDATE users SET path_mode = $2, relay_mode = ($2 = 'relay') WHERE id = $1`, userID, mode)
	return err
}

func (q *Queries) ListPeerPathsByUser(ctx context.Context, userID string) ([]PeerPath, error) {
	rows, err := q.db.Query(ctx, `SELECT user_id, client_device_id, main_site_device_id, current_path, desired_path, state, generation, switched_at, updated_at FROM peer_paths WHERE user_id=$1 ORDER BY client_device_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PeerPath
	for rows.Next() {
		var item PeerPath
		if err := rows.Scan(&item.UserID, &item.ClientDeviceID, &item.MainSiteDeviceID, &item.CurrentPath, &item.DesiredPath, &item.State, &item.Generation, &item.SwitchedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (q *Queries) EnsurePeerPath(ctx context.Context, userID, clientID, mainSiteID, desired string) (PeerPath, error) {
	row := q.db.QueryRow(ctx, `INSERT INTO peer_paths (user_id, client_device_id, main_site_device_id, current_path, desired_path, state)
VALUES ($1,$2,$3,$4,$4,$4)
ON CONFLICT (user_id, client_device_id) DO UPDATE SET main_site_device_id=EXCLUDED.main_site_device_id
RETURNING user_id, client_device_id, main_site_device_id, current_path, desired_path, state, generation, switched_at, updated_at`, userID, clientID, mainSiteID, desired)
	var item PeerPath
	err := row.Scan(&item.UserID, &item.ClientDeviceID, &item.MainSiteDeviceID, &item.CurrentPath, &item.DesiredPath, &item.State, &item.Generation, &item.SwitchedAt, &item.UpdatedAt)
	return item, err
}

func (q *Queries) SetPeerPathDesired(ctx context.Context, userID, clientID, desired, state string) (PeerPath, error) {
	row := q.db.QueryRow(ctx, `UPDATE peer_paths SET desired_path=$3, state=$4, generation=generation+1, updated_at=now() WHERE user_id=$1 AND client_device_id=$2
RETURNING user_id, client_device_id, main_site_device_id, current_path, desired_path, state, generation, switched_at, updated_at`, userID, clientID, desired, state)
	var item PeerPath
	err := row.Scan(&item.UserID, &item.ClientDeviceID, &item.MainSiteDeviceID, &item.CurrentPath, &item.DesiredPath, &item.State, &item.Generation, &item.SwitchedAt, &item.UpdatedAt)
	return item, err
}

func (q *Queries) SetPeerPathState(ctx context.Context, userID, clientID, state string) error {
	_, err := q.db.Exec(ctx, `UPDATE peer_paths SET state=$3, updated_at=now() WHERE user_id=$1 AND client_device_id=$2`, userID, clientID, state)
	return err
}

func (q *Queries) CompletePeerPath(ctx context.Context, userID, clientID string) error {
	_, err := q.db.Exec(ctx, `UPDATE peer_paths SET current_path=desired_path, state=desired_path, switched_at=now(), updated_at=now() WHERE user_id=$1 AND client_device_id=$2`, userID, clientID)
	return err
}

func (q *Queries) DeletePeerPathsNotIn(ctx context.Context, userID, mainSiteID string, clientIDs []string) error {
	_, err := q.db.Exec(ctx, `DELETE FROM peer_paths WHERE user_id=$1 AND (main_site_device_id<>$2 OR NOT (client_device_id = ANY($3::text[])))`, userID, mainSiteID, clientIDs)
	return err
}

func (q *Queries) UpsertDevicePeerStat(ctx context.Context, deviceID, publicKey string, handshake *time.Time, rx, tx int64) error {
	_, err := q.db.Exec(ctx, `INSERT INTO device_peer_stats (device_id,peer_public_key,latest_handshake_at,rx_bytes,tx_bytes,last_rx_at,reported_at)
VALUES ($1,$2,$3,$4::bigint,$5::bigint,CASE WHEN $4::bigint>0 THEN now() END,now())
ON CONFLICT (device_id,peer_public_key) DO UPDATE SET
 latest_handshake_at=EXCLUDED.latest_handshake_at,
 last_rx_at=CASE WHEN EXCLUDED.rx_bytes>device_peer_stats.rx_bytes THEN now() ELSE device_peer_stats.last_rx_at END,
 rx_bytes=EXCLUDED.rx_bytes, tx_bytes=EXCLUDED.tx_bytes, reported_at=now()`, deviceID, publicKey, handshake, rx, tx)
	return err
}

func (q *Queries) GetDevicePeerStat(ctx context.Context, deviceID, publicKey string) (DevicePeerStat, error) {
	row := q.db.QueryRow(ctx, `SELECT device_id,peer_public_key,latest_handshake_at,rx_bytes,tx_bytes,last_rx_at,reported_at FROM device_peer_stats WHERE device_id=$1 AND peer_public_key=$2`, deviceID, publicKey)
	var item DevicePeerStat
	err := row.Scan(&item.DeviceID, &item.PeerPublicKey, &item.LatestHandshake, &item.RxBytes, &item.TxBytes, &item.LastRxAt, &item.ReportedAt)
	return item, err
}

func (q *Queries) UpsertAppliedPath(ctx context.Context, deviceID, clientID string, generation int64) error {
	_, err := q.db.Exec(ctx, `INSERT INTO device_path_applied(device_id,client_device_id,generation) VALUES($1,$2,$3)
ON CONFLICT(device_id,client_device_id) DO UPDATE SET generation=GREATEST(device_path_applied.generation,EXCLUDED.generation),updated_at=now()`, deviceID, clientID, generation)
	return err
}

func (q *Queries) GetAppliedPathGeneration(ctx context.Context, deviceID, clientID string) (int64, error) {
	var generation int64
	err := q.db.QueryRow(ctx, `SELECT generation FROM device_path_applied WHERE device_id=$1 AND client_device_id=$2`, deviceID, clientID).Scan(&generation)
	return generation, err
}
