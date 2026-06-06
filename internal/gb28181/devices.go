package gb28181

import (
	"fmt"
	"sync"
	"time"

	"github.com/emiago/sipgo/sip"
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

type DeviceStore struct {
	devices sync.Map
}

func NewDeviceStore() *DeviceStore {
	return &DeviceStore{}
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
