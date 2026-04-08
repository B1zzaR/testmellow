package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vpnplatform/internal/domain"
)

type DeviceRepo struct {
	db *pgxpool.Pool
}

func NewDeviceRepo(db *pgxpool.Pool) *DeviceRepo {
	return &DeviceRepo{db: db}
}

// ListByUser returns all active devices for a user, ordered by last_active desc.
func (r *DeviceRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]*domain.Device, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, device_name, last_active, created_at, is_active
		FROM devices
		WHERE user_id = $1 AND is_active = true
		ORDER BY last_active DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*domain.Device
	for rows.Next() {
		d := &domain.Device{}
		if err := rows.Scan(&d.ID, &d.UserID, &d.DeviceName, &d.LastActive, &d.CreatedAt, &d.IsActive); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

// CountActive returns the number of active devices for a user.
func (r *DeviceRepo) CountActive(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM devices WHERE user_id = $1 AND is_active = true`,
		userID,
	).Scan(&count)
	return count, err
}

// GetByID returns a device by its ID.
func (r *DeviceRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Device, error) {
	d := &domain.Device{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, device_name, last_active, created_at, is_active
		FROM devices WHERE id = $1`,
		id,
	).Scan(&d.ID, &d.UserID, &d.DeviceName, &d.LastActive, &d.CreatedAt, &d.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return d, err
}

// Upsert inserts or updates a device record (keyed by id).
// Creates a new row on first call; subsequent calls update last_active / is_active.
func (r *DeviceRepo) Upsert(ctx context.Context, d *domain.Device) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO devices (id, user_id, device_name, last_active, created_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE
		  SET last_active = EXCLUDED.last_active,
		      is_active   = EXCLUDED.is_active`,
		d.ID, d.UserID, d.DeviceName, d.LastActive, d.CreatedAt, d.IsActive,
	)
	return err
}

// Disconnect marks a single device as inactive.
func (r *DeviceRepo) Disconnect(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE devices SET is_active = false WHERE id = $1`,
		id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("device not found")
	}
	return nil
}

// Touch updates last_active and ensures is_active = true for a device.
func (r *DeviceRepo) Touch(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE devices SET last_active = $1, is_active = true WHERE id = $2`,
		time.Now().UTC(), id,
	)
	return err
}
