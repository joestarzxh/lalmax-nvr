package storage

import (
	"context"
	"database/sql"
	"strings"
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
	DeviceID     string     `json:"device_id"`
	ChannelID    string     `json:"channel_id"`
	Name         string     `json:"name"`
	LastSeenAt   *time.Time `json:"last_seen_at,omitempty"`
	Status       string     `json:"status"`
	MissingCount int        `json:"missing_count"`
	CreatedAt    time.Time  `json:"created_at"`
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
		last_seen_at DATETIME,
		status TEXT DEFAULT '',
		missing_count INTEGER DEFAULT 0,
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
	
	// 迁移：添加新列（如果不存在）
	migrations := []string{
		`ALTER TABLE gb28181_channels ADD COLUMN last_seen_at DATETIME;`,
		`ALTER TABLE gb28181_channels ADD COLUMN status TEXT DEFAULT '';`,
		`ALTER TABLE gb28181_channels ADD COLUMN missing_count INTEGER DEFAULT 0;`,
	}
	
	for _, migration := range migrations {
		_, err := d.db.ExecContext(ctx, migration)
		if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
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
		SELECT device_id, channel_id, name, last_seen_at, status, missing_count, created_at
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
		var lastSeenAt sql.NullString
		if err := rows.Scan(&ch.DeviceID, &ch.ChannelID, &ch.Name, &lastSeenAt, &ch.Status, &ch.MissingCount, &ch.CreatedAt); err != nil {
			return nil, err
		}
		if lastSeenAt.Valid {
			t, _ := parseTime(lastSeenAt.String)
			ch.LastSeenAt = &t
		}
		channels = append(channels, ch)
	}
	return channels, nil
}

// GetGB28181Channel returns a single channel by device ID and channel ID.
func (d *DB) GetGB28181Channel(ctx context.Context, deviceID, channelID string) (*GB28181ChannelRow, error) {
	var ch GB28181ChannelRow
	var lastSeenAt sql.NullString
	err := d.db.QueryRowContext(ctx, `
		SELECT device_id, channel_id, name, last_seen_at, status, missing_count, created_at
		FROM gb28181_channels 
		WHERE device_id = ? AND channel_id = ?;`, deviceID, channelID).Scan(
		&ch.DeviceID, &ch.ChannelID, &ch.Name, &lastSeenAt, &ch.Status, &ch.MissingCount, &ch.CreatedAt)
	if err != nil {
		return nil, err
	}
	if lastSeenAt.Valid {
		t, _ := parseTime(lastSeenAt.String)
		ch.LastSeenAt = &t
	}
	return &ch, nil
}

// UpsertGB28181Channel creates or updates a channel.
func (d *DB) UpsertGB28181Channel(ctx context.Context, channel *GB28181ChannelRow) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO gb28181_channels (device_id, channel_id, name, last_seen_at, status, missing_count)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(device_id, channel_id) DO UPDATE SET
			name = COALESCE(NULLIF(excluded.name, ''), name),
			last_seen_at = CASE
				WHEN excluded.last_seen_at > gb28181_channels.last_seen_at OR gb28181_channels.last_seen_at IS NULL
				THEN excluded.last_seen_at
				ELSE gb28181_channels.last_seen_at
			END,
			status = CASE
				WHEN excluded.last_seen_at > gb28181_channels.last_seen_at OR gb28181_channels.last_seen_at IS NULL
				THEN excluded.status
				ELSE gb28181_channels.status
			END,
			missing_count = CASE
				WHEN excluded.last_seen_at > gb28181_channels.last_seen_at OR gb28181_channels.last_seen_at IS NULL
				THEN excluded.missing_count
				ELSE gb28181_channels.missing_count
			END;`,
		channel.DeviceID, channel.ChannelID, channel.Name, channel.LastSeenAt, channel.Status, channel.MissingCount)
	return err
}

// BatchUpsertChannels performs incremental update of channels for a device.
// It deletes channels that no longer exist and inserts/updates new channels.
func (d *DB) BatchUpsertChannels(ctx context.Context, deviceID string, channels []GB28181ChannelRow) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	// 1. 查询现有通道
	existingRows, err := tx.QueryContext(ctx, 
		"SELECT channel_id FROM gb28181_channels WHERE device_id = ?;", deviceID)
	if err != nil {
		return err
	}
	
	existing := make(map[string]bool)
	for existingRows.Next() {
		var chID string
		if err := existingRows.Scan(&chID); err != nil {
			existingRows.Close()
			return err
		}
		existing[chID] = true
	}
	existingRows.Close()
	
	// 2. 构建新通道集合
	newSet := make(map[string]bool)
	for _, ch := range channels {
		newSet[ch.ChannelID] = true
	}
	
	// 3. 删除不存在的通道
	for chID := range existing {
		if !newSet[chID] {
			if _, err := tx.ExecContext(ctx,
				"DELETE FROM gb28181_channels WHERE device_id = ? AND channel_id = ?;",
				deviceID, chID); err != nil {
				return err
			}
		}
	}
	
	// 4. 批量插入/更新通道
	for _, ch := range channels {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO gb28181_channels (device_id, channel_id, name, last_seen_at, status, missing_count)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP, 'online', 0)
			ON CONFLICT(device_id, channel_id) DO UPDATE SET
				name = COALESCE(NULLIF(excluded.name, ''), name),
				last_seen_at = CURRENT_TIMESTAMP,
				status = 'online',
				missing_count = 0;`,
			ch.DeviceID, ch.ChannelID, ch.Name); err != nil {
			return err
		}
	}
	
	return tx.Commit()
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

// ListMissingChannels returns channels with missing_count >= threshold.
func (d *DB) ListMissingChannels(ctx context.Context, threshold int) ([]GB28181ChannelRow, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT device_id, channel_id, name, last_seen_at, status, missing_count, created_at
		FROM gb28181_channels 
		WHERE missing_count >= ?
		ORDER BY device_id, channel_id;`, threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var channels []GB28181ChannelRow
	for rows.Next() {
		var ch GB28181ChannelRow
		var lastSeenAt sql.NullString
		if err := rows.Scan(&ch.DeviceID, &ch.ChannelID, &ch.Name, &lastSeenAt, &ch.Status, &ch.MissingCount, &ch.CreatedAt); err != nil {
			return nil, err
		}
		if lastSeenAt.Valid {
			t, _ := parseTime(lastSeenAt.String)
			ch.LastSeenAt = &t
		}
		channels = append(channels, ch)
	}
	return channels, nil
}

// UpdateChannelStatus updates the status of a channel.
func (d *DB) UpdateChannelStatus(ctx context.Context, deviceID, channelID, status string) error {
	_, err := d.db.ExecContext(ctx, `
		UPDATE gb28181_channels 
		SET status = ?
		WHERE device_id = ? AND channel_id = ?;`, status, deviceID, channelID)
	return err
}

// IncrementMissingCount increments the missing_count for a channel.
func (d *DB) IncrementMissingCount(ctx context.Context, deviceID, channelID string) error {
	_, err := d.db.ExecContext(ctx, `
		UPDATE gb28181_channels 
		SET missing_count = missing_count + 1
		WHERE device_id = ? AND channel_id = ?;`, deviceID, channelID)
	return err
}
