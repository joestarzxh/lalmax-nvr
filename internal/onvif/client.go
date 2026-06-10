package onvif

import (
	"context"
	"crypto/md5" //nolint:gosec // MD5 required for HTTP digest auth
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	onvifgo "github.com/0x524a/onvif-go"
)

var logger = slog.Default().With("component", "onvif-client")

// Client wraps an onvif-go Client for ONVIF device operations.
type Client struct {
	endpoint     string
	ptzEndpoint  string // PTZ service endpoint (may differ from device endpoint)
	username     string
	password     string
	client       *onvifgo.Client
	http         *http.Client // lazy-init HTTP client with DisableKeepAlives
	mu           sync.Mutex
	soapMu       sync.Mutex // serializes SOAP requests (many cameras mishandle concurrent HTTP)
	ready        bool
}

// newONVIFHTTPClient returns an HTTP client tuned for embedded ONVIF devices.
// DisableKeepAlives avoids intermittent "malformed HTTP status" errors when
// move/stop commands overlap on a reused connection.
func newONVIFHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives:     true,
			DisableCompression:    true,
			MaxConnsPerHost:       1,
			ResponseHeaderTimeout: 15 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 0,
			}).DialContext,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// NewClient creates a new ONVIF client for a specific device.
// Call Connect() before using device operations.
func NewClient(endpoint, username, password string) *Client {
	return &Client{
		endpoint: endpoint,
		username: username,
		password: password,
	}
}

// Connect initializes the ONVIF connection and discovers service endpoints.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	onvifClient, err := onvifgo.NewClient(c.endpoint,
		onvifgo.WithCredentials(c.username, c.password),
		onvifgo.WithHTTPClient(newONVIFHTTPClient()),
	)
	if err != nil {
		return fmt.Errorf("create ONVIF client: %w", err)
	}

	if err := onvifClient.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize ONVIF client: %w", err)
	}

	c.client = onvifClient
	c.ready = true

	// Derive PTZ service endpoint for raw SOAP fallback.
	// Most cameras serve PTZ on the same host with /onvif/ptz_service path.
	c.ptzEndpoint = strings.Replace(c.endpoint, "/device_service", "/ptz_service", 1)

	logger.Info("connected to ONVIF device", "endpoint", c.endpoint)
	return nil
}

// GetDeviceInformation retrieves device info (manufacturer, model, firmware).
func (c *Client) GetDeviceInformation(ctx context.Context) (*DeviceInfo, error) {
	if !c.ready {
		return nil, fmt.Errorf("onvif client not connected, call Connect() first")
	}

	info, err := c.client.GetDeviceInformation(ctx)
	if err != nil {
		return nil, fmt.Errorf("get device information: %w", err)
	}

	return mapDeviceInfo(info), nil
}

// GetProfiles retrieves media profiles from the device.
func (c *Client) GetProfiles(ctx context.Context) ([]DeviceProfile, error) {
	if !c.ready {
		return nil, fmt.Errorf("onvif client not connected, call Connect() first")
	}

	profiles, err := c.client.GetProfiles(ctx)
	if err != nil {
		if isHTTPAuthFailure(err) {
			logger.Warn("onvif-go GetProfiles failed with HTTP auth challenge, trying raw SOAP fallback", "endpoint", c.endpoint)
			rawProfiles, rawErr := c.getRawProfiles(ctx)
			if rawErr == nil {
				return rawProfiles, nil
			}
			logger.Warn("raw SOAP GetProfiles fallback failed", "error", rawErr)
		}
		return nil, fmt.Errorf("get profiles: %w", err)
	}

	result := make([]DeviceProfile, 0, len(profiles))
	for _, p := range profiles {
		result = append(result, mapProfile(p))
	}
	return result, nil
}

func (c *Client) GetStreamURI(ctx context.Context, profileToken string) (*StreamInfo, error) {
	if !c.ready {
		return nil, fmt.Errorf("onvif client not connected, call Connect() first")
	}

	uri, err := c.client.GetStreamURI(ctx, profileToken)
	if err != nil {
		if isHTTPAuthFailure(err) {
			logger.Warn("onvif-go GetStreamURI failed with HTTP auth challenge, trying raw SOAP fallback", "profile_token", profileToken)
			rawURI, rawErr := c.getRawStreamURI(ctx, profileToken)
			if rawErr == nil && strings.TrimSpace(rawURI) != "" {
				return mapStreamURI(&onvifgo.MediaURI{URI: rawURI}, profileToken), nil
			}
			logger.Warn("raw SOAP GetStreamURI fallback failed", "error", rawErr)
		}
		return nil, fmt.Errorf("get stream URI: %w", err)
	}

	// onvif-go may return empty URI due to XML namespace parsing issues
	// with some devices. Fallback to raw SOAP request if URI is empty.
	if strings.TrimSpace(uri.URI) == "" {
		logger.Warn("onvif-go returned empty URI, trying raw SOAP fallback", "profile_token", profileToken)
		rawURI, rawErr := c.getRawStreamURI(ctx, profileToken)
		if rawErr != nil {
			logger.Warn("raw SOAP fallback failed", "error", rawErr)
		} else if strings.TrimSpace(rawURI) != "" {
			uri.URI = rawURI
		}
	}

	logger.Info("GetStreamURI response", "profile_token", profileToken, "uri", uri.URI)

	return mapStreamURI(uri, profileToken), nil
}

// getRawStreamURI sends a raw SOAP GetStreamUri request and parses the response.
// This works around XML namespace parsing issues in onvif-go with some devices.
func (c *Client) getRawStreamURI(ctx context.Context, profileToken string) (string, error) {
	soapBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:trt="http://www.onvif.org/ver10/media/wsdl"
 xmlns:tt="http://www.onvif.org/ver10/schema">
  <s:Body>
    <trt:GetStreamUri>
      <trt:StreamSetup>
        <tt:Stream>RTP-Unicast</tt:Stream>
        <tt:Transport>
          <tt:Protocol>RTSP</tt:Protocol>
        </tt:Transport>
      </trt:StreamSetup>
      <trt:ProfileToken>%s</trt:ProfileToken>
    </trt:GetStreamUri>
  </s:Body>
</s:Envelope>`, profileToken)

	body, err := c.doAuthenticatedSOAPRequest(ctx, soapBody)
	if err != nil {
		return "", err
	}

	// Parse URI from XML response using regex-like approach
	// Look for <tt:Uri> or <Uri> tag content
	var envelope struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			XMLName              xml.Name `xml:"Body"`
			GetStreamURIResponse struct {
				XMLName  xml.Name `xml:"GetStreamUriResponse"`
				MediaURI struct {
					URI string `xml:"Uri"`
				} `xml:"MediaUri"`
			} `xml:"GetStreamUriResponse"`
		} `xml:"Body"`
	}

	if err := xml.Unmarshal(body, &envelope); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	return envelope.Body.GetStreamURIResponse.MediaURI.URI, nil
}

func (c *Client) getRawProfiles(ctx context.Context) ([]DeviceProfile, error) {
	soapBody := `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:trt="http://www.onvif.org/ver10/media/wsdl">
  <s:Body>
    <trt:GetProfiles/>
  </s:Body>
</s:Envelope>`

	body, err := c.doAuthenticatedSOAPRequest(ctx, soapBody)
	if err != nil {
		return nil, err
	}

	var envelope struct {
		Body struct {
			GetProfilesResponse struct {
				Profiles []struct {
					Token                     string `xml:"token,attr"`
					Name                      string `xml:"Name"`
					VideoEncoderConfiguration *struct {
						Encoding   string `xml:"Encoding"`
						Resolution *struct {
							Width  int `xml:"Width"`
							Height int `xml:"Height"`
						} `xml:"Resolution"`
					} `xml:"VideoEncoderConfiguration"`
				} `xml:"Profiles"`
			} `xml:"GetProfilesResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	result := make([]DeviceProfile, 0, len(envelope.Body.GetProfilesResponse.Profiles))
	for _, p := range envelope.Body.GetProfilesResponse.Profiles {
		profile := DeviceProfile{
			Token: p.Token,
			Name:  p.Name,
		}
		if p.VideoEncoderConfiguration != nil {
			profile.Encoding = p.VideoEncoderConfiguration.Encoding
			if p.VideoEncoderConfiguration.Resolution != nil {
				profile.Width = p.VideoEncoderConfiguration.Resolution.Width
				profile.Height = p.VideoEncoderConfiguration.Resolution.Height
			}
		}
		result = append(result, profile)
	}
	return result, nil
}

func (c *Client) doAuthenticatedSOAPRequest(ctx context.Context, soapBody string) ([]byte, error) {
	return c.doAuthenticatedSOAPRequestTo(ctx, c.endpoint, soapBody)
}

func (c *Client) doAuthenticatedSOAPRequestTo(ctx context.Context, endpoint, soapBody string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(soapBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized && c.username != "" {
		challenge := resp.Header.Get("WWW-Authenticate")
		switch {
		case strings.Contains(strings.ToLower(challenge), "digest"):
			return c.doDigestSOAPRequest(ctx, soapBody, challenge)
		case strings.Contains(strings.ToLower(challenge), "basic"):
			return c.doBasicSOAPRequest(ctx, soapBody)
		}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func (c *Client) doBasicSOAPRequest(ctx context.Context, soapBody string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(soapBody))
	if err != nil {
		return nil, fmt.Errorf("create basic auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	req.SetBasicAuth(c.username, c.password)
	return c.executeSOAPRequest(req)
}

func (c *Client) doDigestSOAPRequest(ctx context.Context, soapBody, challenge string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(soapBody))
	if err != nil {
		return nil, fmt.Errorf("create digest auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	req.Header.Set("Authorization", buildDigestAuthHeader(req, challenge, c.username, c.password))
	return c.executeSOAPRequest(req)
}

func (c *Client) httpClient() *http.Client {
	if c.http != nil {
		return c.http
	}
	c.http = newONVIFHTTPClient()
	return c.http
}

func (c *Client) executeSOAPRequest(req *http.Request) ([]byte, error) {
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("send authenticated request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read authenticated response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func isHTTPAuthFailure(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "status 401") || strings.Contains(msg, "unauthorized")
}

func buildDigestAuthHeader(req *http.Request, authHeader, username, password string) string {
	realm := extractAuthParam(authHeader, "realm")
	nonce := extractAuthParam(authHeader, "nonce")
	qop := extractAuthParam(authHeader, "qop")
	opaque := extractAuthParam(authHeader, "opaque")
	uri := req.URL.RequestURI()
	ha1 := md5Hex(username + ":" + realm + ":" + password)
	ha2 := md5Hex(req.Method + ":" + uri)
	nc := "00000001"
	cnonce := randomHex(16)

	var response string
	if qop == "auth" {
		response = md5Hex(ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":auth:" + ha2)
	} else {
		response = md5Hex(ha1 + ":" + nonce + ":" + ha2)
	}

	header := fmt.Sprintf(`Digest username=%q, realm=%q, nonce=%q, uri=%q, response=%q`, username, realm, nonce, uri, response)
	if qop == "auth" {
		header += fmt.Sprintf(`, qop=%s, nc=%s, cnonce=%q`, qop, nc, cnonce)
	}
	if opaque != "" {
		header += fmt.Sprintf(`, opaque=%q`, opaque)
	}
	return header
}

func extractAuthParam(authHeader, param string) string {
	prefix := param + `="`
	idx := strings.Index(authHeader, prefix)
	if idx == -1 {
		return ""
	}
	start := idx + len(prefix)
	end := strings.Index(authHeader[start:], `"`)
	if end == -1 {
		return ""
	}
	return authHeader[start : start+end]
}

func md5Hex(s string) string {
	h := md5.New() //nolint:gosec // HTTP digest auth requires MD5
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func randomHex(size int) string {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// --- Raw PTZ SOAP fallback methods ---
// These use doAuthenticatedSOAPRequest which handles HTTP Basic/Digest auth.
// They serve as fallbacks when the onvif-go library's WS-Security auth gets HTTP 401.

func (c *Client) ptzSOAPFallbackEndpoint() string {
	if c.ptzEndpoint != "" {
		return c.ptzEndpoint
	}
	return c.endpoint
}

func (c *Client) rawContinuousMove(ctx context.Context, profileToken string, pan, tilt, zoom float64) error {
	soapBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl"
 xmlns:tt="http://www.onvif.org/ver10/schema">
  <s:Body>
    <tptz:ContinuousMove>
      <tptz:ProfileToken>%s</tptz:ProfileToken>
      <tptz:Velocity>
        <tt:PanTilt x="%f" y="%f"/>
        <tt:Zoom x="%f"/>
      </tptz:Velocity>
      <tptz:Timeout>PT10S</tptz:Timeout>
    </tptz:ContinuousMove>
  </s:Body>
</s:Envelope>`, profileToken, pan, tilt, zoom)
	_, err := c.doAuthenticatedSOAPRequestTo(ctx, c.ptzSOAPFallbackEndpoint(), soapBody)
	return err
}

func (c *Client) rawAbsoluteMove(ctx context.Context, profileToken string, pan, tilt, zoom float64) error {
	soapBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl"
 xmlns:tt="http://www.onvif.org/ver10/schema">
  <s:Body>
    <tptz:AbsoluteMove>
      <tptz:ProfileToken>%s</tptz:ProfileToken>
      <tptz:Position>
        <tt:PanTilt x="%f" y="%f"/>
        <tt:Zoom x="%f"/>
      </tptz:Position>
    </tptz:AbsoluteMove>
  </s:Body>
</s:Envelope>`, profileToken, pan, tilt, zoom)
	_, err := c.doAuthenticatedSOAPRequestTo(ctx, c.ptzSOAPFallbackEndpoint(), soapBody)
	return err
}

func (c *Client) rawRelativeMove(ctx context.Context, profileToken string, pan, tilt, zoom float64) error {
	soapBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl"
 xmlns:tt="http://www.onvif.org/ver10/schema">
  <s:Body>
    <tptz:RelativeMove>
      <tptz:ProfileToken>%s</tptz:ProfileToken>
      <tptz:Translation>
        <tt:PanTilt x="%f" y="%f"/>
        <tt:Zoom x="%f"/>
      </tptz:Translation>
    </tptz:RelativeMove>
  </s:Body>
</s:Envelope>`, profileToken, pan, tilt, zoom)
	_, err := c.doAuthenticatedSOAPRequestTo(ctx, c.ptzSOAPFallbackEndpoint(), soapBody)
	return err
}

func (c *Client) rawStop(ctx context.Context, profileToken string) error {
	soapBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
  <s:Body>
    <tptz:Stop>
      <tptz:ProfileToken>%s</tptz:ProfileToken>
      <tptz:PanTilt>true</tptz:PanTilt>
      <tptz:Zoom>true</tptz:Zoom>
    </tptz:Stop>
  </s:Body>
</s:Envelope>`, profileToken)
	_, err := c.doAuthenticatedSOAPRequestTo(ctx, c.ptzSOAPFallbackEndpoint(), soapBody)
	return err
}

func (c *Client) rawGetPresets(ctx context.Context, profileToken string) ([]PTZPreset, error) {
	soapBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
  <s:Body>
    <tptz:GetPresets>
      <tptz:ProfileToken>%s</tptz:ProfileToken>
    </tptz:GetPresets>
  </s:Body>
</s:Envelope>`, profileToken)

	body, err := c.doAuthenticatedSOAPRequestTo(ctx, c.ptzSOAPFallbackEndpoint(), soapBody)
	if err != nil {
		return nil, err
	}

	var envelope struct {
		Body struct {
			GetPresetsResponse struct {
				Presets []struct {
					Token       string `xml:"token,attr"`
					Name        string `xml:"Name"`
					PTZPosition *struct {
						PanTilt *struct {
							X float64 `xml:"x,attr"`
							Y float64 `xml:"y,attr"`
						} `xml:"PanTilt"`
						Zoom *struct {
							X float64 `xml:"x,attr"`
						} `xml:"Zoom"`
					} `xml:"PTZPosition"`
				} `xml:"Preset"`
			} `xml:"GetPresetsResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse presets response: %w", err)
	}

	result := make([]PTZPreset, 0, len(envelope.Body.GetPresetsResponse.Presets))
	for _, p := range envelope.Body.GetPresetsResponse.Presets {
		preset := PTZPreset{
			Token: p.Token,
			Name:  p.Name,
		}
		if p.PTZPosition != nil {
			pos := PTZVector{}
			if p.PTZPosition.PanTilt != nil {
				pos.Pan = p.PTZPosition.PanTilt.X
				pos.Tilt = p.PTZPosition.PanTilt.Y
			}
			if p.PTZPosition.Zoom != nil {
				pos.Zoom = p.PTZPosition.Zoom.X
			}
			preset.Position = pos
		}
		result = append(result, preset)
	}
	return result, nil
}

// GetCapabilities retrieves device capabilities (PTZ, streaming, etc.).
func (c *Client) GetCapabilities(ctx context.Context) (*DeviceCapabilities, error) {
	if !c.ready {
		return nil, fmt.Errorf("onvif client not connected, call Connect() first")
	}

	caps, err := c.client.GetCapabilities(ctx)
	if err != nil {
		return nil, fmt.Errorf("get capabilities: %w", err)
	}

	return mapCapabilities(caps), nil
}

// mapDeviceInfo converts onvif-go DeviceInformation to project DeviceInfo.
func mapDeviceInfo(info *onvifgo.DeviceInformation) *DeviceInfo {
	return &DeviceInfo{
		Manufacturer: info.Manufacturer,
		Model:        info.Model,
		Firmware:     info.FirmwareVersion,
		SerialNumber: info.SerialNumber,
		HardwareID:   info.HardwareID,
	}
}

// mapCapabilities converts onvif-go Capabilities to project DeviceCapabilities.
func mapCapabilities(caps *onvifgo.Capabilities) *DeviceCapabilities {
	return &DeviceCapabilities{
		PTZ:       caps.PTZ != nil,
		Streaming: caps.Media != nil,
	}
}

// mapProfile converts onvif-go Profile to project DeviceProfile.
func mapProfile(p *onvifgo.Profile) DeviceProfile {
	profile := DeviceProfile{
		Token: p.Token,
		Name:  p.Name,
	}
	if p.VideoEncoderConfiguration != nil {
		profile.Encoding = p.VideoEncoderConfiguration.Encoding
		if p.VideoEncoderConfiguration.Resolution != nil {
			profile.Width = p.VideoEncoderConfiguration.Resolution.Width
			profile.Height = p.VideoEncoderConfiguration.Resolution.Height
		}
	}
	return profile
}

// mapStreamURI converts onvif-go MediaURI to project StreamInfo.
func mapStreamURI(uri *onvifgo.MediaURI, profileToken string) *StreamInfo {
	return &StreamInfo{
		URI:          uri.URI,
		Protocol:     "RTSP",
		Encoding:     "",
		ProfileToken: profileToken,
	}
}

// NewPTZController creates a PTZController backed by this client's onvif-go connection.
// Requires Connect() to have been called first. Returns nil if not connected.
func (c *Client) NewPTZController(profileToken string) PTZController {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil {
		return nil
	}
	return newSerializedPTZController(&c.soapMu, c.client, profileToken, c)
}

// NewImagingController creates an ImagingController backed by this client's onvif-go connection.
// Requires Connect() to have been called first. Returns nil if not connected.
func (c *Client) NewImagingController(profileToken string) *ImagingControllerImpl {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil {
		return nil
	}
	ctrl := NewImagingController(c.client, profileToken)
	ctrl.SetCredentials(c.username, c.password)
	return ctrl
}

// GetEndpoint returns the device service endpoint URL.
func (c *Client) GetEndpoint() string {
	return c.endpoint
}

// NewSnapshotProvider creates a SnapshotProvider backed by this client's onvif-go connection.
// Requires Connect() to have been called first. Returns nil if not connected.
func (c *Client) NewSnapshotProvider(profileToken string) SnapshotProvider {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil {
		return nil
	}
	return NewSnapshotProvider(c.client, profileToken)
}

// NewEventSubscriber creates an EventSubscriber backed by this client's onvif-go connection.
// Requires Connect() to have been called first. Returns nil if not connected.
func (c *Client) NewEventSubscriber(opts ...EventSubscriberOption) EventSubscriber {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil {
		return nil
	}
	return NewEventSubscriber(c.client, opts...)
}

// NewDeviceManager creates a DeviceManager backed by this client's onvif-go connection.
// Requires Connect() to have been called first. Returns nil if not connected.
func (c *Client) NewDeviceManager() DeviceManager {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil {
		return nil
	}
	return NewDeviceManager(c.client)
}
