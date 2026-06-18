package onvif

import (
	"context"
	"fmt"
	"sync"

	onviflib "github.com/lalmax-pro/lalmax-nvr/onvif"
)

// SnapshotProviderImpl implements SnapshotProvider using the standalone onvif library.
type SnapshotProviderImpl struct {
	client       *onviflib.Client
	profileToken string
	mu           sync.Mutex
}

func NewSnapshotProviderImpl(client *onviflib.Client, profileToken string) *SnapshotProviderImpl {
	return &SnapshotProviderImpl{
		client:       client,
		profileToken: profileToken,
	}
}

func (s *SnapshotProviderImpl) GetSnapshotUri(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	media := s.client.MediaService()
	uri, err := media.GetSnapshotUri(ctx, s.profileToken)
	if err != nil {
		return "", fmt.Errorf("get snapshot URI failed: %w", err)
	}
	return uri, nil
}
