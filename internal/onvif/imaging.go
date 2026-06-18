package onvif

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	onviflib "github.com/lalmax-pro/lalmax-nvr/onvif"
)

// ImagingControllerImpl implements ImagingController using the standalone onvif library.
type ImagingControllerImpl struct {
	client       *onviflib.Client
	profileToken string
	mu           sync.Mutex
	logger       *slog.Logger
}

// NewImagingControllerImpl creates an ImagingController backed by the onvif library client.
func NewImagingControllerImpl(client *onviflib.Client, profileToken string) *ImagingControllerImpl {
	return &ImagingControllerImpl{
		client:       client,
		profileToken: profileToken,
		logger:       slog.Default().With("component", "onvif-imaging"),
	}
}

// SetImagingEndpoint is a no-op. The standalone library discovers endpoints automatically.
func (c *ImagingControllerImpl) SetImagingEndpoint(endpoint string) {
	// No-op: endpoint discovery is handled by the standalone library
}

func (c *ImagingControllerImpl) GetImagingSettings(ctx context.Context) (*ImagingSettings, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	imaging := c.client.ImagingService()
	if imaging == nil {
		return nil, fmt.Errorf("imaging service not available")
	}

	c.logger.Debug("Getting imaging settings", "profile_token", c.profileToken)
	settings, err := imaging.GetImagingSettings(ctx, c.profileToken)
	if err != nil {
		c.logger.Error("Failed to get imaging settings", "error", err, "profile_token", c.profileToken)
		return nil, fmt.Errorf("get imaging settings failed: %w", err)
	}

	return &ImagingSettings{
		Brightness: settings.Brightness,
		Contrast:   settings.Contrast,
		Saturation: settings.ColorSaturation,
		Sharpness:  settings.Sharpness,
		Exposure: ExposureSettings{
			Mode: settings.ExposureMode,
		},
		WhiteBalance: WhiteBalanceSettings{
			Mode: settings.WhiteBalance,
		},
	}, nil
}

func (c *ImagingControllerImpl) SetImagingSettings(ctx context.Context, settings ImagingSettings) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	imaging := c.client.ImagingService()
	if imaging == nil {
		return fmt.Errorf("imaging service not available")
	}

	c.logger.Debug("Setting imaging settings", "profile_token", c.profileToken)
	return imaging.SetImagingSettings(ctx, c.profileToken, onviflib.ImagingSettings{
		Brightness:      settings.Brightness,
		Contrast:        settings.Contrast,
		ColorSaturation: settings.Saturation,
		Sharpness:       settings.Sharpness,
		WhiteBalance:    settings.WhiteBalance.Mode,
		FocusMode:       settings.Exposure.Mode,
		ExposureMode:    settings.Exposure.Mode,
	})
}

func (c *ImagingControllerImpl) GetImagingOptions(ctx context.Context) (*ImagingOptions, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	imaging := c.client.ImagingService()
	if imaging == nil {
		return nil, fmt.Errorf("imaging service not available")
	}

	c.logger.Debug("Getting imaging options", "profile_token", c.profileToken)
	options, err := imaging.GetOptions(ctx, c.profileToken)
	if err != nil {
		c.logger.Error("Failed to get imaging options", "error", err, "profile_token", c.profileToken)
		return nil, fmt.Errorf("get imaging options failed: %w", err)
	}

	return &ImagingOptions{
		Brightness: (*Range)(options.Brightness),
		Contrast:   (*Range)(options.Contrast),
		Saturation: (*Range)(options.Saturation),
		Sharpness:  (*Range)(options.Sharpness),
	}, nil
}

var _ ImagingController = (*ImagingControllerImpl)(nil)
