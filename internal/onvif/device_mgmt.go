package onvif

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"

	onviflib "github.com/lalmax-pro/lalmax-nvr/onvif"
)

var ErrUnsupported = errors.New("operation not supported")
var deviceMgmtLogger = slog.Default().With("component", "onvif-device-mgmt")

// DeviceManagerImpl implements DeviceManager using the standalone onvif library.
type DeviceManagerImpl struct {
	client *onviflib.Client
	mu     sync.Mutex
}

var _ DeviceManager = (*DeviceManagerImpl)(nil)

func NewDeviceManagerImpl(client *onviflib.Client) *DeviceManagerImpl {
	return &DeviceManagerImpl{client: client}
}

func (d *DeviceManagerImpl) SystemReboot(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	mgr := d.client.DeviceManager()
	message, err := mgr.SystemReboot(ctx)
	if err != nil {
		return fmt.Errorf("system reboot failed: %w", err)
	}

	deviceMgmtLogger.Info("device rebooted", "message", message)
	return nil
}

func (d *DeviceManagerImpl) GetNetworkInterfaces(ctx context.Context) ([]NetworkInterface, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	mgr := d.client.DeviceManager()
	ifaces, err := mgr.GetNetworkInterfaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("get network interfaces failed: %w", err)
	}

	result := make([]NetworkInterface, 0, len(ifaces))
	for _, iface := range ifaces {
		n := NetworkInterface{
			Name:    iface.Name,
			Enabled: iface.Enabled,
		}
		n.IPv4 = NetworkIPv4{
			Enabled: iface.IPv4.Enabled,
			DHCP:    iface.IPv4.DHCP,
			Address: iface.IPv4.Address,
			Netmask: iface.IPv4.Netmask,
			Gateway: iface.IPv4.Gateway,
		}
		n.IPv6 = NetworkIPv6{
			Enabled: iface.IPv6.Enabled,
			DHCP:    iface.IPv6.DHCP,
			Address: iface.IPv6.Address,
		}
		result = append(result, n)
	}
	return result, nil
}

func (d *DeviceManagerImpl) SetNetworkInterfaces(ctx context.Context, interfaces []NetworkInterface) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_ = ctx
	_ = interfaces
	return fmt.Errorf("set network interfaces: %w", ErrUnsupported)
}

func (d *DeviceManagerImpl) GetUsers(ctx context.Context) ([]ONVIFUser, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	mgr := d.client.DeviceManager()
	users, err := mgr.GetUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("get users failed: %w", err)
	}

	result := make([]ONVIFUser, 0, len(users))
	for _, u := range users {
		result = append(result, ONVIFUser{
			Username: u.Username,
			Level:    u.UserLevel,
		})
	}
	return result, nil
}

func (d *DeviceManagerImpl) CreateUsers(ctx context.Context, users []ONVIFUser) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	mgr := d.client.DeviceManager()
	onvifUsers := make([]onviflib.User, len(users))
	for i, u := range users {
		onvifUsers[i] = onviflib.User{
			Username:  u.Username,
			Password:  u.Password,
			UserLevel: u.Level,
		}
	}

	if err := mgr.CreateUsers(ctx, onvifUsers); err != nil {
		return fmt.Errorf("create users failed: %w", err)
	}

	deviceMgmtLogger.Info("users created", "count", len(users))
	return nil
}

func (d *DeviceManagerImpl) DeleteUsers(ctx context.Context, usernames []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	mgr := d.client.DeviceManager()
	if err := mgr.DeleteUsers(ctx, usernames); err != nil {
		return fmt.Errorf("delete users failed: %w", err)
	}

	deviceMgmtLogger.Info("users deleted", "count", len(usernames))
	return nil
}

func (d *DeviceManagerImpl) SetUser(ctx context.Context, username, password string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	mgr := d.client.DeviceManager()
	if err := mgr.SetUser(ctx, username, password); err != nil {
		return fmt.Errorf("set user failed: %w", err)
	}

	deviceMgmtLogger.Info("user updated", "username", username)
	return nil
}

func formatPrefixMask(prefixLength int) string {
	if prefixLength <= 0 || prefixLength > 32 {
		return ""
	}
	mask := net.IPv4Mask(0, 0, 0, 0)
	for i := 0; i < 4; i++ {
		bits := prefixLength - i*8
		if bits >= 8 {
			mask[i] = 0xFF
		} else if bits > 0 {
			mask[i] = ^byte(0xFF >> bits)
		}
	}
	return mask.String()
}
