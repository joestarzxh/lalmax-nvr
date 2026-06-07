package storage

import (
	"context"
	"database/sql"
	"time"
)

// GB28181DeviceRow represents a GB28181 device in the database.
type GB28181DeviceRow struct {
	DeviceID        string     `json:"device_id"`
	Name            string     `json:"name"`
	Manufacturer    string     `json:"manufacturer"`
	Model           string     `json:"model"`
	Firmware        string     `json:"firmware"`
	IsOnline        bool       `json:"is_online"`
	Address         string     `json:"address"`
	LastKeepaliveAt *time.Time `json:"last_keepalive_at,omitempty"`
	LastRegisterAt  *time.Time `json:"last_register_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// GB28181ChannelRow represents a channel of a GB28181 device.
type GB28181ChannelRow struct {
	DeviceID  string `json:"device_id"`
	ChannelID string `json:"channel_id"`
	Name      string `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// createGB28181Tables creates the GB28181 device and channel tables.
func (d *DB) createGB28181Tables(ctx context.Context) error {
	deviceSQL := `CREATE TABLE IF NOT EXISTS gb28181_devices (
		device_id TEXT PRIMARY KEY,
		name TEXT DEFAULT '',
		manufacturer TEXT DEFAULT '',
		model TEXT DEFAULT '',
		firmware TEXT DEFAULT '',
		is_online INTEGER DEFAULT 0,
		address TEXT DEFAULT '',
		last_keepalive_at DATETIME,
		last_register_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	
	channelSQL := `CREATE TABLE IF NOT EXISTS gb28181_channels (
		device_id TEXT NOT NULL,
		channel_id TEXT NOT NULL,
		name TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (device_id, channel_id),
		FOREIGN KEY (device_id) REFERENCES gb28181_devices(device_id) ON DELETE CASCADE
	);`
	
	if _, err := d.db.ExecContext(ctx, deviceSQL); err != nil {
		return err
	}
	if _, err := d.db.ExecContext(ctx, channelSQL); err != nil {
		return err
	}
	return nil
}

// ListGB28181Devices returns all GB28181 devices.
func (d *DB) ListGB28181Devices(ctx context.Context) ([]GB28181DeviceRow, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT device_id, name, manufacturer, model, firmware, is_online, address, 
			   last_keepalive_at, last_register_at, created_at, updated_at
		FROM gb28181_devices 
		ORDER BY is_online DESC, device_id;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var devices []GB28181DeviceRow
	for rows.Next() {
		var d GB28181DeviceRow
		var lastKeepalive, lastRegister sql.NullString
		if err := rows.Scan(&d.DeviceID, &d.Name, &d.Manufacturer, &d.Model, &d.Firmware,
			&d.IsOnline, &d.Address, &lastKeepalive, &lastRegister, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		if lastKeepalive.Valid {
			t, _ := parseTime(lastKeepalive.String)
			d.LastKeepaliveAt = &t
		}
		if lastRegister.Valid {
			t, _ := parseTime(lastRegister.String)
			d.LastRegisterAt = &t
		}
		devices = append(devices, d)
	}
	return devices, nil
}

// GetGB28181Device returns a single GB28181 device by ID.
func (d *DB) GetGB28181Device(ctx context.Context, deviceID string) (*GB28181DeviceRow, error) {
	var dev GB28181DeviceRow
	var lastKeepalive, lastRegister sql.NullString
	err := d.db.QueryRowContext(ctx, `
		SELECT device_id, name, manufacturer, model, firmware, is_online, address, 
			   last_keepalive_at, last_register_at, created_at, updated_at
		FROM gb28181_devices WHERE device_id = ?;`, deviceID).Scan(
		&dev.DeviceID, &dev.Name, &dev.Manufacturer, &dev.Model, &dev.Firmware,
		&dev.IsOnline, &dev.Address, &lastKeepalive, &lastRegister, &dev.CreatedAt, &dev.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if lastKeepalive.Valid {
		t, _ := parseTime(lastKeepalive.String)
		dev.LastKeepaliveAt = &t
	}
	if lastRegister.Valid {
		t, _ := parseTime(lastRegister.String)
		dev.LastRegisterAt = &t
	}
	return &dev, nil
}

// UpsertGB28181Device creates or updates a GB28181 device.
func (d *DB) UpsertGB28181Device(ctx context.Context, device *GB28181DeviceRow) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO gb28181_devices (device_id, name, manufacturer, model, firmware, is_online, address, last_keepalive_at, last_register_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(device_id) DO UPDATE SET
			name = COALESCE(NULLIF(excluded.name, ''), name),
			manufacturer = COALESCE(NULLIF(excluded.manufacturer, ''), manufacturer),
			model = COALESCE(NULLIF(excluded.model, ''), model),
			firmware = COALESCE(NULLIF(excluded.firmware, ''), firmware),
			is_online = excluded.is_online,
			address = COALESCE(NULLIF(excluded.address, ''), address),
			last_keepalive_at = COALESCE(excluded.last_keepalive_at, last_keepalive_at),
			last_register_at = COALESCE(excluded.last_register_at, last_register_at),
			updated_at = CURRENT_TIMESTAMP;`,
		device.DeviceID, device.Name, device.Manufacturer, device.Model, device.Firmware,
		device.IsOnline, device.Address, device.LastKeepaliveAt, device.LastRegisterAt)
	return err
}

// UpdateGB28181DeviceStatus updates the online status and address of a device.
func (d *DB) UpdateGB28181DeviceStatus(ctx context.Context, deviceID string, isOnline bool, address string) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO gb28181_devices (device_id, is_online, address, last_keepalive_at, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(device_id) DO UPDATE SET
			is_online = excluded.is_online,
			address = excluded.address,
			last_keepalive_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP;`, deviceID, isOnline, address)
	return err
}

// UpdateGB28181DeviceRegistration updates device info after registration.
func (d *DB) UpdateGB28181DeviceRegistration(ctx context.Context, deviceID, address string) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO gb28181_devices (device_id, is_online, address, last_register_at, updated_at)
		VALUES (?, 1, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(device_id) DO UPDATE SET
			is_online = 1,
			address = excluded.address,
			last_register_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP;`, deviceID, address)
	return err
}

// UpdateGB28181DeviceOnlineStatus sets a device offline.
func (d *DB) UpdateGB28181DeviceOnlineStatus(ctx context.Context, deviceID string, isOnline bool) error {
	_, err := d.db.ExecContext(ctx, `
		UPDATE gb28181_devices 
		SET is_online = ?, updated_at = CURRENT_TIMESTAMP
		WHERE device_id = ?;`, isOnline, deviceID)
	return err
}

// DeleteGB28181Device removes a GB28181 device and its channels.
func (d *DB) DeleteGB28181Device(ctx context.Context, deviceID string) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM gb28181_devices WHERE device_id = ?;", deviceID)
	return err
}

// ListGB28181Channels returns all channels for a device.
func (d *DB) ListGB28181Channels(ctx context.Context, deviceID string) ([]GB28181ChannelRow, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT device_id, channel_id, name, created_at
		FROM gb28181_channels 
		WHERE device_id = ?
		ORDER BY channel_id;`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var channels []GB28181ChannelRow
	for rows.Next() {
		var ch GB28181ChannelRow
		if err := rows.Scan(&ch.DeviceID, &ch.ChannelID, &ch.Name, &ch.CreatedAt); err != nil {
			return nil, err
		}
		channels = append(channels, ch)
	}
	return channels, nil
}

// UpsertGB28181Channel creates or updates a channel.
func (d *DB) UpsertGB28181Channel(ctx context.Context, channel *GB28181ChannelRow) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO gb28181_channels (device_id, channel_id, name)
		VALUES (?, ?, ?)
		ON CONFLICT(device_id, channel_id) DO UPDATE SET
			name = COALESCE(NULLIF(excluded.name, ''), name);`,
		channel.DeviceID, channel.ChannelID, channel.Name)
	return err
}

// ReplaceGB28181Channels replaces all channels for a device.
func (d *DB) ReplaceGB28181Channels(ctx context.Context, deviceID string, channels []GB28181ChannelRow) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	// Delete existing channels
	if _, err := tx.ExecContext(ctx, "DELETE FROM gb28181_channels WHERE device_id = ?;", deviceID); err != nil {
		return err
	}
	
	// Insert new channels
	for _, ch := range channels {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO gb28181_channels (device_id, channel_id, name) VALUES (?, ?, ?);`,
			ch.DeviceID, ch.ChannelID, ch.Name); err != nil {
			return err
		}
	}
	
	return tx.Commit()
}

// DeleteGB28181Channel removes a specific channel.
func (d *DB) DeleteGB28181Channel(ctx context.Context, deviceID, channelID string) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM gb28181_channels WHERE device_id = ? AND channel_id = ?;", deviceID, channelID)
	return err
}

// DeleteGB28181ChannelsForDevice removes all channels for a device.
func (d *DB) DeleteGB28181ChannelsForDevice(ctx context.Context, deviceID string) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM gb28181_channels WHERE device_id = ?;", deviceID)
	return err
}
