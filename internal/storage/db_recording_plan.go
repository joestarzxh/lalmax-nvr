package storage

import (
	"context"
	"fmt"
	"time"
)

type RecordingPlan struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RecordingPlanTimeRange struct {
	ID          int64  `json:"id"`
	PlanID      int64  `json:"plan_id"`
	DayOfWeek   int    `json:"day_of_week"` // 0=Sunday, 1=Monday, ..., 6=Saturday
	StartTime   string `json:"start_time"`  // "HH:MM"
	EndTime     string `json:"end_time"`    // "HH:MM"
}

type RecordingPlanChannel struct {
	ID       int64  `json:"id"`
	PlanID   int64  `json:"plan_id"`
	CameraID string `json:"camera_id"`
}

type RecordingPlanWithDetails struct {
	RecordingPlan
	TimeRanges []RecordingPlanTimeRange `json:"time_ranges"`
	Channels   []RecordingPlanChannel   `json:"channels"`
}

func (d *DB) createRecordingPlanTables(ctx context.Context) error {
	planSQL := `CREATE TABLE IF NOT EXISTS recording_plans (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		enabled INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	timeRangeSQL := `CREATE TABLE IF NOT EXISTS recording_plan_time_ranges (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		plan_id INTEGER NOT NULL,
		day_of_week INTEGER NOT NULL,
		start_time TEXT NOT NULL,
		end_time TEXT NOT NULL,
		FOREIGN KEY (plan_id) REFERENCES recording_plans(id) ON DELETE CASCADE
	);`

	channelSQL := `CREATE TABLE IF NOT EXISTS recording_plan_channels (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		plan_id INTEGER NOT NULL,
		camera_id TEXT NOT NULL,
		UNIQUE(plan_id, camera_id),
		FOREIGN KEY (plan_id) REFERENCES recording_plans(id) ON DELETE CASCADE
	);`

	if _, err := d.db.ExecContext(ctx, planSQL); err != nil {
		return err
	}
	if _, err := d.db.ExecContext(ctx, timeRangeSQL); err != nil {
		return err
	}
	if _, err := d.db.ExecContext(ctx, channelSQL); err != nil {
		return err
	}

	if _, err := d.db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_recording_plan_time_ranges_plan ON recording_plan_time_ranges(plan_id);"); err != nil {
		return err
	}
	if _, err := d.db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_recording_plan_channels_plan ON recording_plan_channels(plan_id);"); err != nil {
		return err
	}
	if _, err := d.db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_recording_plan_channels_camera ON recording_plan_channels(camera_id);"); err != nil {
		return err
	}

	return nil
}

func (d *DB) ListRecordingPlans(ctx context.Context) ([]RecordingPlanWithDetails, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT id, name, enabled, created_at, updated_at FROM recording_plans ORDER BY id;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plans []RecordingPlanWithDetails
	for rows.Next() {
		var p RecordingPlanWithDetails
		if err := rows.Scan(&p.ID, &p.Name, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}

	for i := range plans {
		plans[i].TimeRanges, _ = d.listPlanTimeRanges(ctx, plans[i].ID)
		plans[i].Channels, _ = d.listPlanChannels(ctx, plans[i].ID)
	}

	return plans, nil
}

func (d *DB) GetRecordingPlan(ctx context.Context, id int64) (*RecordingPlanWithDetails, error) {
	var p RecordingPlanWithDetails
	err := d.db.QueryRowContext(ctx, `SELECT id, name, enabled, created_at, updated_at FROM recording_plans WHERE id = ?;`, id).Scan(
		&p.ID, &p.Name, &p.Enabled, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.TimeRanges, _ = d.listPlanTimeRanges(ctx, p.ID)
	p.Channels, _ = d.listPlanChannels(ctx, p.ID)
	return &p, nil
}

func (d *DB) CreateRecordingPlan(ctx context.Context, plan *RecordingPlanWithDetails) (int64, error) {
	result, err := d.db.ExecContext(ctx, `INSERT INTO recording_plans (name, enabled) VALUES (?, ?);`,
		plan.Name, plan.Enabled)
	if err != nil {
		return 0, err
	}
	planID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	for _, tr := range plan.TimeRanges {
		if _, err := d.db.ExecContext(ctx, `INSERT INTO recording_plan_time_ranges (plan_id, day_of_week, start_time, end_time) VALUES (?, ?, ?, ?);`,
			planID, tr.DayOfWeek, tr.StartTime, tr.EndTime); err != nil {
			return planID, err
		}
	}

	for _, ch := range plan.Channels {
		if _, err := d.db.ExecContext(ctx, `INSERT OR IGNORE INTO recording_plan_channels (plan_id, camera_id) VALUES (?, ?);`,
			planID, ch.CameraID); err != nil {
			return planID, err
		}
	}

	return planID, nil
}

func (d *DB) UpdateRecordingPlan(ctx context.Context, plan *RecordingPlanWithDetails) error {
	_, err := d.db.ExecContext(ctx, `UPDATE recording_plans SET name=?, enabled=?, updated_at=CURRENT_TIMESTAMP WHERE id=?;`,
		plan.Name, plan.Enabled, plan.ID)
	if err != nil {
		return err
	}

	// Replace time ranges
	if _, err := d.db.ExecContext(ctx, `DELETE FROM recording_plan_time_ranges WHERE plan_id=?;`, plan.ID); err != nil {
		return err
	}
	for _, tr := range plan.TimeRanges {
		if _, err := d.db.ExecContext(ctx, `INSERT INTO recording_plan_time_ranges (plan_id, day_of_week, start_time, end_time) VALUES (?, ?, ?, ?);`,
			plan.ID, tr.DayOfWeek, tr.StartTime, tr.EndTime); err != nil {
			return err
		}
	}

	return nil
}

func (d *DB) DeleteRecordingPlan(ctx context.Context, id int64) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM recording_plans WHERE id = ?;", id)
	return err
}

func (d *DB) SetPlanChannels(ctx context.Context, planID int64, cameraIDs []string) error {
	if _, err := d.db.ExecContext(ctx, "DELETE FROM recording_plan_channels WHERE plan_id=?;", planID); err != nil {
		return err
	}
	for _, camID := range cameraIDs {
		if _, err := d.db.ExecContext(ctx, `INSERT OR IGNORE INTO recording_plan_channels (plan_id, camera_id) VALUES (?, ?);`,
			planID, camID); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) AddPlanChannel(ctx context.Context, planID int64, cameraID string) error {
	_, err := d.db.ExecContext(ctx, `INSERT OR IGNORE INTO recording_plan_channels (plan_id, camera_id) VALUES (?, ?);`,
		planID, cameraID)
	return err
}

func (d *DB) RemovePlanChannel(ctx context.Context, planID int64, cameraID string) error {
	_, err := d.db.ExecContext(ctx, `DELETE FROM recording_plan_channels WHERE plan_id=? AND camera_id=?;`,
		planID, cameraID)
	return err
}

func (d *DB) listPlanTimeRanges(ctx context.Context, planID int64) ([]RecordingPlanTimeRange, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT id, plan_id, day_of_week, start_time, end_time FROM recording_plan_time_ranges WHERE plan_id=? ORDER BY day_of_week, start_time;`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ranges []RecordingPlanTimeRange
	for rows.Next() {
		var tr RecordingPlanTimeRange
		if err := rows.Scan(&tr.ID, &tr.PlanID, &tr.DayOfWeek, &tr.StartTime, &tr.EndTime); err != nil {
			return nil, err
		}
		ranges = append(ranges, tr)
	}
	return ranges, nil
}

func (d *DB) listPlanChannels(ctx context.Context, planID int64) ([]RecordingPlanChannel, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT id, plan_id, camera_id FROM recording_plan_channels WHERE plan_id=?;`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []RecordingPlanChannel
	for rows.Next() {
		var ch RecordingPlanChannel
		if err := rows.Scan(&ch.ID, &ch.PlanID, &ch.CameraID); err != nil {
			return nil, err
		}
		channels = append(channels, ch)
	}
	return channels, nil
}

// GetActiveCameraIDs returns camera IDs that should be recording RIGHT NOW
// based on all enabled recording plans.
func (d *DB) GetActiveCameraIDs(ctx context.Context) (map[string]bool, error) {
	now := time.Now()
	dayOfWeek := int(now.Weekday()) // 0=Sunday
	hhmm := now.Format("15:04")

	rows, err := d.db.QueryContext(ctx, `
		SELECT DISTINCT rpc.camera_id
		FROM recording_plans rp
		JOIN recording_plan_time_ranges rptr ON rptr.plan_id = rp.id
		JOIN recording_plan_channels rpc ON rpc.plan_id = rp.id
		WHERE rp.enabled = 1
		  AND rptr.day_of_week = ?
		  AND rptr.start_time <= ?
		  AND rptr.end_time > ?
	`, dayOfWeek, hhmm, hhmm)
	if err != nil {
		return nil, fmt.Errorf("query active recording plans: %w", err)
	}
	defer rows.Close()

	active := make(map[string]bool)
	for rows.Next() {
		var cameraID string
		if err := rows.Scan(&cameraID); err != nil {
			return nil, err
		}
		active[cameraID] = true
	}
	return active, nil
}

// GetCamerasWithPlan returns camera IDs that are associated with any enabled recording plan.
func (d *DB) GetCamerasWithPlan(ctx context.Context) (map[string]bool, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT DISTINCT rpc.camera_id
		FROM recording_plans rp
		JOIN recording_plan_channels rpc ON rpc.plan_id = rp.id
		WHERE rp.enabled = 1
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var cameraID string
		if err := rows.Scan(&cameraID); err != nil {
			return nil, err
		}
		result[cameraID] = true
	}
	return result, nil
}
