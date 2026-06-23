package onvif

import (
	"context"
	"encoding/xml"
	"fmt"
)

// MediaService handles media operations.
type MediaService struct {
	client *Client
}

// NewMediaService creates a new media service.
func NewMediaService(client *Client) *MediaService {
	return &MediaService{client: client}
}

// GetProfiles retrieves media profiles.
func (s *MediaService) GetProfiles(ctx context.Context) ([]MediaProfile, error) {
	// Try Media2 first (Profile T)
	if s.client.endpoints.Media2 != nil {
		profiles, err := s.getMedia2Profiles(ctx)
		if err == nil && len(profiles) > 0 {
			return profiles, nil
		}
	}

	// Fallback to Media1
	if s.client.endpoints.Media != nil {
		return s.getMedia1Profiles(ctx)
	}

	return nil, fmt.Errorf("onvif: no media service available")
}

// getMedia1Profiles retrieves profiles using Media1 service.
func (s *MediaService) getMedia1Profiles(ctx context.Context) ([]MediaProfile, error) {
	body := `<GetProfiles xmlns="http://www.onvif.org/ver10/media/wsdl"/>`

	resp, err := s.client.soap.Send(&SOAPRequest{
		ServiceURL: s.client.endpoints.Media.String(),
		Body:       body,
	})
	if err != nil {
		return nil, fmt.Errorf("onvif: GetProfiles failed: %w", err)
	}

	var result struct {
		XMLName xml.Name `xml:"GetProfilesResponse"`
		Profiles []struct {
			Token string `xml:"token,attr"`
			Name  string `xml:"Name"`
			VideoSourceConfiguration struct {
				Token string `xml:"token,attr"`
			} `xml:"VideoSourceConfiguration"`
			VideoEncoderConfiguration struct {
				Token      string `xml:"token,attr"`
				Encoding   string `xml:"Encoding"`
				Resolution struct {
					Width  int `xml:"Width"`
					Height int `xml:"Height"`
				} `xml:"Resolution"`
				RateControl struct {
					FrameRateLimit int `xml:"FrameRateLimit"`
					BitrateLimit   int `xml:"BitrateLimit"`
				} `xml:"RateControl"`
			} `xml:"VideoEncoderConfiguration"`
			AudioSourceConfiguration struct {
				Token string `xml:"token,attr"`
			} `xml:"AudioSourceConfiguration"`
			AudioEncoderConfiguration struct {
				Token string `xml:"token,attr"`
			} `xml:"AudioEncoderConfiguration"`
		} `xml:"Profiles"`
	}

	if err := ParseResponseBody(resp.Body, &result); err != nil {
		return nil, err
	}

	profiles := make([]MediaProfile, 0, len(result.Profiles))
	for _, p := range result.Profiles {
		profile := MediaProfile{
			Token:        p.Token,
			Name:         p.Name,
			VideoSource:  p.VideoSourceConfiguration.Token,
			VideoEncoder: p.VideoEncoderConfiguration.Token,
			AudioSource:  p.AudioSourceConfiguration.Token,
			AudioEncoder: p.AudioEncoderConfiguration.Token,
			Encoding:     p.VideoEncoderConfiguration.Encoding,
			Resolution: Resolution{
				Width:  p.VideoEncoderConfiguration.Resolution.Width,
				Height: p.VideoEncoderConfiguration.Resolution.Height,
			},
			Framerate: p.VideoEncoderConfiguration.RateControl.FrameRateLimit,
			Bitrate:   p.VideoEncoderConfiguration.RateControl.BitrateLimit,
		}
		profiles = append(profiles, profile)
	}

	return profiles, nil
}

// getMedia2Profiles retrieves profiles using Media2 service (Profile T).
//
// NOTE: The Media2 (ver20) schema differs from Media1: each profile's
// configurations are nested under a <Configurations> wrapper and the encoder
// element is named <VideoEncoder> (not <VideoEncoderConfiguration>). Parsing
// must follow Profiles > Configurations > VideoEncoder > Encoding, otherwise
// Encoding/Resolution come back empty for Profile T devices.
func (s *MediaService) getMedia2Profiles(ctx context.Context) ([]MediaProfile, error) {
	body := `<GetProfiles xmlns="http://www.onvif.org/ver20/media/wsdl">
  <Type>All</Type>
</GetProfiles>`

	resp, err := s.client.soap.Send(&SOAPRequest{
		ServiceURL: s.client.endpoints.Media2.String(),
		Body:       body,
	})
	if err != nil {
		return nil, fmt.Errorf("onvif: GetProfiles (Media2) failed: %w", err)
	}

	var result struct {
		XMLName xml.Name `xml:"GetProfilesResponse"`
		Profiles []struct {
			Token          string `xml:"token,attr"`
			Name           string `xml:"Name"`
			Configurations struct {
				VideoSource struct {
					Token string `xml:"token,attr"`
				} `xml:"VideoSource"`
				VideoEncoder struct {
					Token      string `xml:"token,attr"`
					Encoding   string `xml:"Encoding"`
					Resolution struct {
						Width  int `xml:"Width"`
						Height int `xml:"Height"`
					} `xml:"Resolution"`
					RateControl struct {
						FrameRateLimit int `xml:"FrameRateLimit"`
						BitrateLimit   int `xml:"BitrateLimit"`
					} `xml:"RateControl"`
				} `xml:"VideoEncoder"`
				AudioSource struct {
					Token string `xml:"token,attr"`
				} `xml:"AudioSource"`
				AudioEncoder struct {
					Token string `xml:"token,attr"`
				} `xml:"AudioEncoder"`
			} `xml:"Configurations"`
		} `xml:"Profiles"`
	}

	if err := ParseResponseBody(resp.Body, &result); err != nil {
		return nil, err
	}

	profiles := make([]MediaProfile, 0, len(result.Profiles))
	for _, p := range result.Profiles {
		cfg := p.Configurations
		profile := MediaProfile{
			Token:        p.Token,
			Name:         p.Name,
			VideoSource:  cfg.VideoSource.Token,
			VideoEncoder: cfg.VideoEncoder.Token,
			AudioSource:  cfg.AudioSource.Token,
			AudioEncoder: cfg.AudioEncoder.Token,
			Encoding:     cfg.VideoEncoder.Encoding,
			Resolution: Resolution{
				Width:  cfg.VideoEncoder.Resolution.Width,
				Height: cfg.VideoEncoder.Resolution.Height,
			},
			Framerate: cfg.VideoEncoder.RateControl.FrameRateLimit,
			Bitrate:   cfg.VideoEncoder.RateControl.BitrateLimit,
		}
		profiles = append(profiles, profile)
	}

	return profiles, nil
}

// GetStreamURI retrieves the stream URI for a profile.
func (s *MediaService) GetStreamURI(ctx context.Context, profileToken string) (string, error) {
	// Try Media2 first
	if s.client.endpoints.Media2 != nil {
		uri, err := s.getMedia2StreamURI(ctx, profileToken)
		if err == nil {
			return uri, nil
		}
	}

	// Fallback to Media1
	if s.client.endpoints.Media != nil {
		return s.getMedia1StreamURI(ctx, profileToken)
	}

	return "", fmt.Errorf("onvif: no media service available")
}

// getMedia1StreamURI retrieves stream URI using Media1 service.
func (s *MediaService) getMedia1StreamURI(ctx context.Context, profileToken string) (string, error) {
	body := fmt.Sprintf(`<GetStreamUri xmlns="http://www.onvif.org/ver10/media/wsdl">
  <StreamSetup>
    <Stream xmlns="http://www.onvif.org/ver10/schema">RTP-Unicast</Stream>
    <Transport xmlns="http://www.onvif.org/ver10/schema">
      <Protocol>RTSP</Protocol>
    </Transport>
  </StreamSetup>
  <ProfileToken>%s</ProfileToken>
</GetStreamUri>`, profileToken)

	resp, err := s.client.soap.Send(&SOAPRequest{
		ServiceURL: s.client.endpoints.Media.String(),
		Body:       body,
	})
	if err != nil {
		return "", fmt.Errorf("onvif: GetStreamUri failed: %w", err)
	}

	var result struct {
		XMLName xml.Name `xml:"GetStreamUriResponse"`
		MediaUri struct {
			Uri string `xml:"Uri"`
		} `xml:"MediaUri"`
	}

	if err := ParseResponseBody(resp.Body, &result); err != nil {
		return "", err
	}

	return result.MediaUri.Uri, nil
}

// getMedia2StreamURI retrieves stream URI using Media2 service.
func (s *MediaService) getMedia2StreamURI(ctx context.Context, profileToken string) (string, error) {
	body := fmt.Sprintf(`<GetStreamUri xmlns="http://www.onvif.org/ver20/media/wsdl">
  <Protocol>RTSP</Protocol>
  <ProfileToken>%s</ProfileToken>
</GetStreamUri>`, profileToken)

	resp, err := s.client.soap.Send(&SOAPRequest{
		ServiceURL: s.client.endpoints.Media2.String(),
		Body:       body,
	})
	if err != nil {
		return "", fmt.Errorf("onvif: GetStreamUri (Media2) failed: %w", err)
	}

	var result struct {
		XMLName xml.Name `xml:"GetStreamUriResponse"`
		Uri     string   `xml:"Uri"`
	}

	if err := ParseResponseBody(resp.Body, &result); err != nil {
		return "", err
	}

	return result.Uri, nil
}
