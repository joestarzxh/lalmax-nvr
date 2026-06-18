package onvif

import (
	"context"
	"encoding/xml"
	"fmt"
)

// MediaService (snapshot part) - GetSnapshotUri

// GetSnapshotUri retrieves the snapshot URI for a profile.
func (s *MediaService) GetSnapshotUri(ctx context.Context, profileToken string) (string, error) {
	if s.client.endpoints.Media == nil {
		return "", fmt.Errorf("onvif: Media service not available")
	}

	body := fmt.Sprintf(`<GetSnapshotUri xmlns="http://www.onvif.org/ver10/media/wsdl">
  <ProfileToken>%s</ProfileToken>
</GetSnapshotUri>`, profileToken)

	resp, err := s.client.soap.Send(&SOAPRequest{
		ServiceURL: s.client.endpoints.Media.String(),
		Body:       body,
	})
	if err != nil {
		return "", fmt.Errorf("onvif: GetSnapshotUri failed: %w", err)
	}

	var result struct {
		XMLName xml.Name `xml:"GetSnapshotUriResponse"`
		MediaUri struct {
			Uri string `xml:"Uri"`
		} `xml:"MediaUri"`
	}

	if err := ParseResponseBody(resp.Body, &result); err != nil {
		return "", err
	}

	return result.MediaUri.Uri, nil
}
