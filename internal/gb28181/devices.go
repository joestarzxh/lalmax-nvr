package gb28181

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/emiago/sipgo/sip"
	"github.com/lalmax-pro/lalmax-nvr/internal/storage"
)

type Device struct {
	DeviceID string
	Channels sync.Map

	registerWithKeepaliveMutex sync.Mutex
	playMutex                  sync.Mutex

	IsOnline bool
	Address  string
	Password string
	region   string

	conn   sip.Connection
	source string

	LastKeepaliveAt time.Time
	LastRegisterAt  time.Time

	keepaliveInterval uint16
	keepaliveTimeout  uint16
}

func (d *Device) Conn() sip.Connection {
	return d.conn
}

func (d *Device) Source() string {
	return d.source
}

func (d *Device) GetChannel(channelID string) (*Channel, bool) {
	v, ok := d.Channels.Load(channelID)
	if !ok {
		return nil, false
	}
	return v.(*Channel), true
}

type Channel struct {
	ChannelID string
	uriStr    string
	device    *Device
}

func (c *Channel) Conn() sip.Connection {
	return c.device.conn
}

func (c *Channel) Source() string {
	return c.device.source
}

func (c *Channel) init(domain string) {
	c.uriStr = fmt.Sprintf("sip:%s@%s", c.ChannelID, domain)
}

// DeviceStore manages GB28181 devices with both memory cache and database persistence.
type DeviceStore struct {
	devices sync.Map
	db      *storage.DB
}

func NewDeviceStore(db *storage.DB) *DeviceStore {
	return &DeviceStore{
		db: db,
	}
}

// LoadFromDB loads all devices from database into memory.
func (s *DeviceStore) LoadFromDB() error {
	ctx := context.Background()
	dbDevices, err := s.db.ListGB28181Devices(ctx)
	if err != nil {
		return err
	}

	for _, dbDev := range dbDevices {
		dev := &Device{
			DeviceID: dbDev.DeviceID,
			IsOnline: false, // Will be updated when device registers/heartbeats
			Address:  dbDev.Address,
		}
		if dbDev.LastKeepaliveAt != nil {
			dev.LastKeepaliveAt = *dbDev.LastKeepaliveAt
		}
		if dbDev.LastRegisterAt != nil {
			dev.LastRegisterAt = *dbDev.LastRegisterAt
		}

		// Load channels
		channels, err := s.db.ListGB28181Channels(ctx, dbDev.DeviceID)
		if err != nil {
			slog.Warn("Failed to load channels for device", "device_id", dbDev.DeviceID, "error", err)
		} else {
			for _, ch := range channels {
				channel := &Channel{
					ChannelID: ch.ChannelID,
					device:    dev,
				}
				dev.Channels.Store(ch.ChannelID, channel)
			}
		}

		s.devices.Store(dbDev.DeviceID, dev)
	}

	slog.Info("Loaded GB28181 devices from database", "count", len(dbDevices))
	return nil
}

func (s *DeviceStore) LoadOrStore(deviceID string, value *Device) {
	existing, loaded := s.devices.LoadOrStore(deviceID, value)
	if loaded {
		dev := existing.(*Device)
		if value.source != "" {
			dev.source = value.source
		}
	}
}

func (s *DeviceStore) Load(deviceID string) (*Device, bool) {
	v, ok := s.devices.Load(deviceID)
	if !ok {
		return nil, false
	}
	return v.(*Device), true
}

func (s *DeviceStore) Store(deviceID string, value *Device) {
	s.devices.Store(deviceID, value)
}

func (s *DeviceStore) GetChannel(deviceID, channelID string) (*Channel, bool) {
	dev, ok := s.Load(deviceID)
	if !ok {
		return nil, false
	}
	return dev.GetChannel(channelID)
}

func (s *DeviceStore) Change(deviceID string, changeFn func(*Device)) error {
	v, ok := s.devices.Load(deviceID)
	if !ok {
		return ErrDeviceNotExist
	}
	dev := v.(*Device)
	dev.registerWithKeepaliveMutex.Lock()
	defer dev.registerWithKeepaliveMutex.Unlock()
	changeFn(dev)
	return nil
}

func (s *DeviceStore) RangeDevices(fn func(key string, value *Device) bool) {
	s.devices.Range(func(key, value any) bool {
		return fn(key.(string), value.(*Device))
	})
}

// SaveDevice persists device info to database.
func (s *DeviceStore) SaveDevice(deviceID string, dev *Device) error {
	ctx := context.Background()
	dbDev := &storage.GB28181DeviceRow{
		DeviceID: deviceID,
		IsOnline: dev.IsOnline,
		Address:  dev.Address,
	}
	if !dev.LastKeepaliveAt.IsZero() {
		dbDev.LastKeepaliveAt = &dev.LastKeepaliveAt
	}
	if !dev.LastRegisterAt.IsZero() {
		dbDev.LastRegisterAt = &dev.LastRegisterAt
	}
	return s.db.UpsertGB28181Device(ctx, dbDev)
}

// UpdateDeviceStatus updates device online status in database.
func (s *DeviceStore) UpdateDeviceStatus(deviceID string, isOnline bool, address string) error {
	ctx := context.Background()
	return s.db.UpdateGB28181DeviceStatus(ctx, deviceID, isOnline, address)
}

// UpdateDeviceRegistration updates device registration info in database.
func (s *DeviceStore) UpdateDeviceRegistration(deviceID string, address string) error {
	ctx := context.Background()
	return s.db.UpdateGB28181DeviceRegistration(ctx, deviceID, address)
}

// UpdateDeviceOnlineStatus sets device offline in database.
func (s *DeviceStore) UpdateDeviceOnlineStatus(deviceID string, isOnline bool) error {
	ctx := context.Background()
	return s.db.UpdateGB28181DeviceOnlineStatus(ctx, deviceID, isOnline)
}

// SaveChannels persists channels to database.
func (s *DeviceStore) SaveChannels(deviceID string, channels []Channel) error {
	ctx := context.Background()
	dbChannels := make([]storage.GB28181ChannelRow, len(channels))
	for i, ch := range channels {
		dbChannels[i] = storage.GB28181ChannelRow{
			DeviceID:  deviceID,
			ChannelID: ch.ChannelID,
		}
	}
	return s.db.ReplaceGB28181Channels(ctx, deviceID, dbChannels)
}

// SaveDeviceInfo persists device info (manufacturer, model, firmware) to database.
func (s *DeviceStore) SaveDeviceInfo(deviceID, manufacturer, model, firmware string) error {
	ctx := context.Background()
	dbDev := &storage.GB28181DeviceRow{
		DeviceID:     deviceID,
		Manufacturer: manufacturer,
		Model:        model,
		Firmware:     firmware,
	}
	return s.db.UpsertGB28181Device(ctx, dbDev)
}

// DeleteDevice removes a device from memory and database.
func (s *DeviceStore) DeleteDevice(deviceID string) error {
	s.devices.Delete(deviceID)
	ctx := context.Background()
	return s.db.DeleteGB28181Device(ctx, deviceID)
}

func filterUnknowDevices(deviceID string) error {
	if len(deviceID) < 18 {
		return fmt.Errorf("device id too short")
	}
	if len(deviceID) > 20 {
		return fmt.Errorf("device id too long")
	}
	for _, ch := range deviceID {
		if ch < '0' || ch > '9' {
			return fmt.Errorf("device id must be all numbers")
		}
	}
	return nil
}

type ChannelsXML struct {
	DeviceID     string `xml:"DeviceID"`
	ChannelID    string `xml:"-"`
	Name         string `xml:"Name"`
	Manufacturer string `xml:"Manufacturer"`
	Model        string `xml:"Model"`
	Status       string `xml:"Status"`
	PTZType      int    `xml:"PTZType"`
	CameraType   int    `xml:"CameraType"`
}

type MessageNotify struct {
	CmdType  string `xml:"CmdType"`
	SN       int    `xml:"SN"`
	DeviceID string `xml:"DeviceID"`
	Status   string `xml:"Status"`
	Info     string `xml:"Info"`
}
