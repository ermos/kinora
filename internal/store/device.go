package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// CreateDeviceCode stores a pending code, and drops the expired ones on the way.
func (s *Store) CreateDeviceCode(ctx context.Context, code, tokenHash string, expires time.Time) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM device_codes WHERE expires_at <= extract(epoch FROM now())::bigint"); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, "INSERT INTO device_codes (code, token_hash, expires_at) VALUES ($1, $2, $3)", code, tokenHash, expires.Unix())
	return err
}

// ApproveDeviceCode signs the device waiting on code in as userID. ErrNotFound if the code is unknown, expired or
// already approved.
func (s *Store) ApproveDeviceCode(ctx context.Context, code string, userID int64) error {
	return s.affectOne(s.db.ExecContext(ctx, `UPDATE device_codes SET user_id = $2
		WHERE code = $1 AND user_id IS NULL AND expires_at > extract(epoch FROM now())::bigint`, code, userID))
}

// ClaimDeviceCode is the device's poll: the approving user once (the code is used up), 0 while pending, ErrNotFound
// once expired or unknown.
func (s *Store) ClaimDeviceCode(ctx context.Context, tokenHash string) (int64, error) {
	var userID int64
	err := s.db.QueryRowContext(ctx, `DELETE FROM device_codes
		WHERE token_hash = $1 AND user_id IS NOT NULL AND expires_at > extract(epoch FROM now())::bigint
		RETURNING user_id`, tokenHash).Scan(&userID)
	if !errors.Is(err, sql.ErrNoRows) {
		return userID, err
	}
	var pending bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM device_codes
		WHERE token_hash = $1 AND expires_at > extract(epoch FROM now())::bigint)`, tokenHash).Scan(&pending); err != nil {
		return 0, err
	}
	if !pending {
		return 0, ErrNotFound
	}
	return 0, nil
}
