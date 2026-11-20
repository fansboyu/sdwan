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
	rows, err := q.db.Query(ctx, `
SELECT p.user_id, p.client_device_id, p.main_site_device_id, p.current_path, p.desired_path, p.state,
       p.generation, p.verified_path, p.verified_generation, p.verification_state, p.verification_error,
       p.verified_at, p.switched_at, p.updated_at, client.generation, hub.generation,
       p.hub_endpoint, p.client_endpoint, p.endpoint_trial
FROM peer_paths p
LEFT JOIN device_path_applied client ON client.device_id=p.client_device_id AND client.client_device_id=p.client_device_id
LEFT JOIN device_path_applied hub ON hub.device_id=p.main_site_device_id AND hub.client_device_id=p.client_device_id
WHERE p.user_id=$1 ORDER BY p.client_device_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PeerPath
	for rows.Next() {
		var item PeerPath
		if err := rows.Scan(&item.UserID, &item.ClientDeviceID, &item.MainSiteDeviceID, &item.CurrentPath, &item.DesiredPath, &item.State, &item.Generation, &item.VerifiedPath, &item.VerifiedGeneration, &item.VerificationState, &item.VerificationError, &item.VerifiedAt, &item.SwitchedAt, &item.UpdatedAt, &item.ClientAppliedGeneration, &item.HubAppliedGeneration, &item.HubEndpoint, &item.ClientEndpoint, &item.EndpointTrial); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (q *Queries) TryDirectEndpoints(ctx context.Context, path PeerPath, clientEndpoint, hubEndpoint string) (bool, error) {
	tag, err := q.db.Exec(ctx, `UPDATE peer_paths SET client_endpoint=$4, hub_endpoint=$5,
 generation=generation+1, endpoint_trial=endpoint_trial+1, verification_state='pending',
 verification_error='', verified_at=NULL, updated_at=now()
 WHERE user_id=$1 AND client_device_id=$2 AND generation=$3 AND main_site_device_id=$6
   AND desired_path='direct' AND current_path='direct'`,
		path.UserID, path.ClientDeviceID, path.Generation, clientEndpoint, hubEndpoint, path.MainSiteDeviceID)
	return tag.RowsAffected() > 0, err
}

func (q *Queries) EnsurePeerPath(ctx context.Context, userID, clientID, mainSiteID, desired string) (PeerPath, error) {
	row := q.db.QueryRow(ctx, `INSERT INTO peer_paths (user_id, client_device_id, main_site_device_id, current_path, desired_path, state)
VALUES ($1,$2,$3,$4,$4,$4)
ON CONFLICT (user_id, client_device_id) DO UPDATE SET main_site_device_id=EXCLUDED.main_site_device_id
RETURNING user_id, client_device_id, main_site_device_id, current_path, desired_path, state, generation, verified_path, verified_generation, verification_state, verification_error, verified_at, switched_at, updated_at`, userID, clientID, mainSiteID, desired)
	var item PeerPath
	err := row.Scan(&item.UserID, &item.ClientDeviceID, &item.MainSiteDeviceID, &item.CurrentPath, &item.DesiredPath, &item.State, &item.Generation, &item.VerifiedPath, &item.VerifiedGeneration, &item.VerificationState, &item.VerificationError, &item.VerifiedAt, &item.SwitchedAt, &item.UpdatedAt)
	return item, err
}

func (q *Queries) SetPeerPathDesired(ctx context.Context, userID, clientID, desired, state string) (PeerPath, error) {
	row := q.db.QueryRow(ctx, `UPDATE peer_paths SET desired_path=$3, state=$4, generation=generation+1,
 verification_state='pending', verification_error='', verified_at=NULL, updated_at=now() WHERE user_id=$1 AND client_device_id=$2
RETURNING user_id, client_device_id, main_site_device_id, current_path, desired_path, state, generation, verified_path, verified_generation, verification_state, verification_error, verified_at, switched_at, updated_at`, userID, clientID, desired, state)
	var item PeerPath
	err := row.Scan(&item.UserID, &item.ClientDeviceID, &item.MainSiteDeviceID, &item.CurrentPath, &item.DesiredPath, &item.State, &item.Generation, &item.VerifiedPath, &item.VerifiedGeneration, &item.VerificationState, &item.VerificationError, &item.VerifiedAt, &item.SwitchedAt, &item.UpdatedAt)
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

func (q *Queries) UpdatePeerPathVerification(ctx context.Context, userID, clientID, state, verifiedPath, message string, generation int64, verified bool) error {
	_, err := q.db.Exec(ctx, `UPDATE peer_paths SET verification_state=$3, verified_path=CASE WHEN $7 THEN $4 ELSE verified_path END,
 verified_generation=CASE WHEN $7 THEN $6 ELSE verified_generation END, verification_error=$5,
 endpoint_trial=CASE WHEN $7 THEN 0 ELSE endpoint_trial END,
 verified_at=CASE WHEN $7 THEN (
   SELECT min(a.verified_at) FROM device_path_applied a
   WHERE a.client_device_id=peer_paths.client_device_id
     AND a.device_id IN (peer_paths.client_device_id, peer_paths.main_site_device_id)
     AND a.generation=$6 AND a.effective_path=$4 AND a.verification_status='verified'
   HAVING count(a.verified_at)=2
 ) ELSE verified_at END, verification_updated_at=now()
 WHERE user_id=$1 AND client_device_id=$2 AND generation=$6`, userID, clientID, state, verifiedPath, message, generation, verified)
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

type UpsertDevicePathQualityParams struct {
	DeviceID          string
	PeerDeviceID      string
	PathType          string
	CurrentPath       bool
	Endpoint          string
	LatestHandshakeAt *time.Time
	RxBytes           int64
	TxBytes           int64
	LastRxAt          *time.Time
	FailCount         int32
	LastOKAt          *time.Time
	LastFailedAt      *time.Time
}

func (q *Queries) UpsertDevicePathQuality(ctx context.Context, arg UpsertDevicePathQualityParams) error {
	_, err := q.db.Exec(ctx, `INSERT INTO device_path_quality (
  device_id, peer_device_id, path_type, current_path, endpoint, latest_handshake_at,
  rx_bytes, tx_bytes, last_rx_at, fail_count, last_ok_at, last_failed_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7::bigint,$8::bigint,$9,$10,$11,$12,now())
ON CONFLICT (device_id, peer_device_id, path_type) DO UPDATE SET
  current_path=EXCLUDED.current_path,
  endpoint=EXCLUDED.endpoint,
  latest_handshake_at=EXCLUDED.latest_handshake_at,
  rx_bytes=EXCLUDED.rx_bytes,
  tx_bytes=EXCLUDED.tx_bytes,
  last_rx_at=EXCLUDED.last_rx_at,
  fail_count=EXCLUDED.fail_count,
  last_ok_at=EXCLUDED.last_ok_at,
  last_failed_at=EXCLUDED.last_failed_at,
  updated_at=now()`,
		arg.DeviceID,
		arg.PeerDeviceID,
		arg.PathType,
		arg.CurrentPath,
		arg.Endpoint,
		arg.LatestHandshakeAt,
		arg.RxBytes,
		arg.TxBytes,
		arg.LastRxAt,
		arg.FailCount,
		arg.LastOKAt,
		arg.LastFailedAt,
	)
	return err
}

func (q *Queries) ListPathQualityByUser(ctx context.Context, userID string) ([]DevicePathQuality, error) {
	rows, err := q.db.Query(ctx, `SELECT q.device_id, q.peer_device_id, q.path_type, q.current_path, q.endpoint,
       q.latest_handshake_at, q.rx_bytes, q.tx_bytes, q.last_rx_at, q.fail_count,
       q.last_ok_at, q.last_failed_at, q.updated_at
FROM device_path_quality q
JOIN devices d ON d.id = q.device_id
WHERE d.user_id = $1
ORDER BY q.updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []DevicePathQuality
	for rows.Next() {
		var item DevicePathQuality
		if err := rows.Scan(
			&item.DeviceID,
			&item.PeerDeviceID,
			&item.PathType,
			&item.CurrentPath,
			&item.Endpoint,
			&item.LatestHandshakeAt,
			&item.RxBytes,
			&item.TxBytes,
			&item.LastRxAt,
			&item.FailCount,
			&item.LastOKAt,
			&item.LastFailedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (q *Queries) GetDevicePeerStat(ctx context.Context, deviceID, publicKey string) (DevicePeerStat, error) {
	row := q.db.QueryRow(ctx, `SELECT device_id,peer_public_key,latest_handshake_at,rx_bytes,tx_bytes,last_rx_at,reported_at FROM device_peer_stats WHERE device_id=$1 AND peer_public_key=$2`, deviceID, publicKey)
	var item DevicePeerStat
	err := row.Scan(&item.DeviceID, &item.PeerPublicKey, &item.LatestHandshake, &item.RxBytes, &item.TxBytes, &item.LastRxAt, &item.ReportedAt)
	return item, err
}

func (q *Queries) UpsertAppliedPath(ctx context.Context, deviceID, clientID string, generation int64, effectivePath, verificationStatus, verificationError string) error {
	_, err := q.db.Exec(ctx, `INSERT INTO device_path_applied(device_id,client_device_id,generation,effective_path,verification_status,verification_error,verified_at)
VALUES($1,$2,$3,$4,$5,$6,CASE WHEN $5='verified' THEN now() END)
ON CONFLICT(device_id,client_device_id) DO UPDATE SET
 generation=GREATEST(device_path_applied.generation,EXCLUDED.generation),
 effective_path=CASE WHEN EXCLUDED.generation>=device_path_applied.generation THEN EXCLUDED.effective_path ELSE device_path_applied.effective_path END,
 verification_status=CASE WHEN EXCLUDED.generation>=device_path_applied.generation THEN EXCLUDED.verification_status ELSE device_path_applied.verification_status END,
 verification_error=CASE WHEN EXCLUDED.generation>=device_path_applied.generation THEN EXCLUDED.verification_error ELSE device_path_applied.verification_error END,
 verified_at=CASE WHEN EXCLUDED.generation>=device_path_applied.generation THEN EXCLUDED.verified_at ELSE device_path_applied.verified_at END, updated_at=now()`, deviceID, clientID, generation, effectivePath, verificationStatus, verificationError)
	return err
}

type AppliedPathStatus struct {
	Generation         int64
	EffectivePath      string
	VerificationStatus string
	VerificationError  string
}

func (q *Queries) GetAppliedPathStatus(ctx context.Context, deviceID, clientID string) (AppliedPathStatus, error) {
	var item AppliedPathStatus
	err := q.db.QueryRow(ctx, `SELECT generation,effective_path,verification_status,verification_error FROM device_path_applied WHERE device_id=$1 AND client_device_id=$2`, deviceID, clientID).Scan(&item.Generation, &item.EffectivePath, &item.VerificationStatus, &item.VerificationError)
	return item, err
}

func (q *Queries) GetAppliedPathGeneration(ctx context.Context, deviceID, clientID string) (int64, error) {
	var generation int64
	err := q.db.QueryRow(ctx, `SELECT generation FROM device_path_applied WHERE device_id=$1 AND client_device_id=$2`, deviceID, clientID).Scan(&generation)
	return generation, err
}
