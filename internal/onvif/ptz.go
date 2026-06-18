package onvif

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	onviflib "github.com/lalmax-pro/lalmax-nvr/onvif"
)

// PTZControllerImpl implements PTZController using the standalone onvif library.
type PTZControllerImpl struct {
	client       *onviflib.Client
	profileToken string
	mu           sync.Mutex
}

// NewPTZControllerImpl creates a PTZController backed by the onvif library client.
func NewPTZControllerImpl(client *onviflib.Client, profileToken string) *PTZControllerImpl {
	return &PTZControllerImpl{
		client:       client,
		profileToken: profileToken,
	}
}

// serializedPTZController serializes PTZ SOAP calls per device and retries transient transport errors.
type serializedPTZController struct {
	deviceMu *sync.Mutex
	inner    *PTZControllerImpl
}

func newSerializedPTZController(deviceMu *sync.Mutex, client *onviflib.Client, profileToken string) PTZController {
	return &serializedPTZController{
		deviceMu: deviceMu,
		inner:    NewPTZControllerImpl(client, profileToken),
	}
}

func (s *serializedPTZController) ContinuousMove(ctx context.Context, velocity PTZVector) error {
	return s.withRetry(func() error { return s.inner.ContinuousMove(ctx, velocity) })
}

func (s *serializedPTZController) AbsoluteMove(ctx context.Context, position PTZVector) error {
	return s.withRetry(func() error { return s.inner.AbsoluteMove(ctx, position) })
}

func (s *serializedPTZController) RelativeMove(ctx context.Context, displacement PTZVector) error {
	return s.withRetry(func() error { return s.inner.RelativeMove(ctx, displacement) })
}

func (s *serializedPTZController) Stop(ctx context.Context, stopPanTilt, stopZoom bool) error {
	return s.withRetry(func() error { return s.inner.Stop(ctx, stopPanTilt, stopZoom) })
}

func (s *serializedPTZController) GetStatus(ctx context.Context) (PTZVector, bool, error) {
	var pos PTZVector
	var moving bool
	err := s.withRetry(func() error {
		var innerErr error
		pos, moving, innerErr = s.inner.GetStatus(ctx)
		return innerErr
	})
	return pos, moving, err
}

func (s *serializedPTZController) GetPresets(ctx context.Context) ([]PTZPreset, error) {
	var presets []PTZPreset
	err := s.withRetry(func() error {
		var innerErr error
		presets, innerErr = s.inner.GetPresets(ctx)
		return innerErr
	})
	return presets, err
}

func (s *serializedPTZController) SetPreset(ctx context.Context, name string) (string, error) {
	var token string
	err := s.withRetry(func() error {
		var innerErr error
		token, innerErr = s.inner.SetPreset(ctx, name)
		return innerErr
	})
	return token, err
}

func (s *serializedPTZController) GoToPreset(ctx context.Context, token string) error {
	return s.withRetry(func() error { return s.inner.GoToPreset(ctx, token) })
}

func (s *serializedPTZController) RemovePreset(ctx context.Context, token string) error {
	return s.withRetry(func() error { return s.inner.RemovePreset(ctx, token) })
}

func (s *serializedPTZController) withRetry(fn func() error) error {
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	if err := fn(); err == nil || !isPTZTransportError(err) {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	return fn()
}

func isPTZTransportError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection broken") ||
		strings.Contains(msg, "malformed http status") ||
		strings.Contains(msg, "eof") ||
		strings.Contains(msg, "connection reset")
}

func (p *PTZControllerImpl) SetProfileToken(token string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.profileToken = token
}

func (p *PTZControllerImpl) ContinuousMove(ctx context.Context, velocity PTZVector) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ptz := p.client.PTZService()
	return ptz.ContinuousMove(ctx, p.profileToken, onviflib.PTZVelocity{
		PanTilt: onviflib.Vector2D{X: velocity.Pan, Y: velocity.Tilt},
		Zoom:    onviflib.Vector1D{X: velocity.Zoom},
	})
}

func (p *PTZControllerImpl) AbsoluteMove(ctx context.Context, position PTZVector) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ptz := p.client.PTZService()
	return ptz.AbsoluteMove(ctx, p.profileToken, onviflib.PTZPosition{
		PanTilt: onviflib.Vector2D{X: position.Pan, Y: position.Tilt},
		Zoom:    onviflib.Vector1D{X: position.Zoom},
	})
}

func (p *PTZControllerImpl) RelativeMove(ctx context.Context, displacement PTZVector) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ptz := p.client.PTZService()
	return ptz.RelativeMove(ctx, p.profileToken, onviflib.PTZPosition{
		PanTilt: onviflib.Vector2D{X: displacement.Pan, Y: displacement.Tilt},
		Zoom:    onviflib.Vector1D{X: displacement.Zoom},
	})
}

func (p *PTZControllerImpl) Stop(ctx context.Context, stopPanTilt, stopZoom bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ptz := p.client.PTZService()
	return ptz.Stop(ctx, p.profileToken, stopPanTilt, stopZoom)
}

func (p *PTZControllerImpl) GetStatus(ctx context.Context) (position PTZVector, moving bool, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ptz := p.client.PTZService()
	status, err := ptz.GetStatus(ctx, p.profileToken)
	if err != nil {
		return PTZVector{}, false, fmt.Errorf("get PTZ status failed: %w", err)
	}

	return PTZVector{
		Pan:  status.Position.PanTilt.X,
		Tilt: status.Position.PanTilt.Y,
		Zoom: status.Position.Zoom.X,
	}, status.Moving, nil
}

func (p *PTZControllerImpl) GetPresets(ctx context.Context) ([]PTZPreset, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ptz := p.client.PTZService()
	presets, err := ptz.GetPresets(ctx, p.profileToken)
	if err != nil {
		return nil, fmt.Errorf("get PTZ presets failed: %w", err)
	}

	result := make([]PTZPreset, 0, len(presets))
	for _, preset := range presets {
		result = append(result, PTZPreset{
			Token: preset.Token,
			Name:  preset.Name,
			Position: PTZVector{
				Pan:  preset.Position.PanTilt.X,
				Tilt: preset.Position.PanTilt.Y,
				Zoom: preset.Position.Zoom.X,
			},
		})
	}
	return result, nil
}

func (p *PTZControllerImpl) SetPreset(ctx context.Context, name string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ptz := p.client.PTZService()
	token, err := ptz.SetPreset(ctx, p.profileToken, name)
	if err != nil {
		return "", fmt.Errorf("set PTZ preset failed: %w", err)
	}
	return token, nil
}

func (p *PTZControllerImpl) GoToPreset(ctx context.Context, token string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ptz := p.client.PTZService()
	if err := ptz.GotoPreset(ctx, p.profileToken, token); err == nil {
		return nil
	}

	// Fallback: use AbsoluteMove with preset coordinates
	presets, presetsErr := ptz.GetPresets(ctx, p.profileToken)
	if presetsErr != nil {
		return fmt.Errorf("go to PTZ preset failed: %w", presetsErr)
	}

	for _, preset := range presets {
		if preset.Token == token {
			return ptz.AbsoluteMove(ctx, p.profileToken, onviflib.PTZPosition{
				PanTilt: onviflib.Vector2D{X: preset.Position.PanTilt.X, Y: preset.Position.PanTilt.Y},
				Zoom:    onviflib.Vector1D{X: preset.Position.Zoom.X},
			})
		}
	}

	return fmt.Errorf("go to PTZ preset failed: preset %q not found", token)
}

func (p *PTZControllerImpl) RemovePreset(ctx context.Context, token string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ptz := p.client.PTZService()
	return ptz.RemovePreset(ctx, p.profileToken, token)
}
