package onvif

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// SOAPClient handles SOAP communication with ONVIF devices.
type SOAPClient struct {
	httpClient *http.Client
	username   string
	password   string
	timeShift  time.Duration

	mu sync.Mutex
}

// NewSOAPClient creates a new SOAP client.
func NewSOAPClient(httpClient *http.Client, username, password string) *SOAPClient {
	return &SOAPClient{
		httpClient: httpClient,
		username:   username,
		password:   password,
	}
}

// SetTimeShift sets the time difference with the device.
func (s *SOAPClient) SetTimeShift(shift time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timeShift = shift
}

// SOAPRequest represents a SOAP request.
type SOAPRequest struct {
	ServiceURL string
	Action     string
	Body       string
	Namespace  string
	NoAuth     bool // If true, don't add WS-Security header
}

// SOAPResponse represents a SOAP response.
type SOAPResponse struct {
	Body     []byte
	Header   []byte
	Fault    *SOAPFault
	HTTPCode int
}

// SOAPFault represents a SOAP fault.
type SOAPFault struct {
	Code   string
	Reason string
	Detail string
}

func (f *SOAPFault) Error() string {
	return fmt.Sprintf("SOAP Fault: %s - %s", f.Code, f.Reason)
}

// Send sends a SOAP request and returns the response.
func (s *SOAPClient) Send(req *SOAPRequest) (*SOAPResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.sendInternal(req)
}

// sendInternal sends a SOAP request (caller must hold lock).
func (s *SOAPClient) sendInternal(req *SOAPRequest) (*SOAPResponse, error) {
	// Build SOAP envelope
	envelope := s.buildEnvelope(req.Body, req.Action, req.NoAuth)

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", req.ServiceURL, bytes.NewBufferString(envelope))
	if err != nil {
		return nil, fmt.Errorf("onvif: create request failed: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	if req.Action != "" {
		httpReq.Header.Set("SOAPAction", req.Action)
	}

	// Send request
	httpResp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("onvif: send request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("onvif: read response failed: %w", err)
	}

	resp := &SOAPResponse{
		Body:     body,
		Header:   []byte(httpResp.Header.Get("WWW-Authenticate")),
		HTTPCode: httpResp.StatusCode,
	}

	// Handle 401 with HTTP Digest Auth fallback
	if httpResp.StatusCode == http.StatusUnauthorized {
		wwwAuth := httpResp.Header.Get("WWW-Authenticate")
		if wwwAuth != "" && s.username != "" {
			return s.sendWithDigestAuth(req, wwwAuth)
		}
		return nil, fmt.Errorf("onvif: authentication failed (401)")
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("onvif: HTTP error %d", httpResp.StatusCode)
	}

	// Parse SOAP fault
	fault, err := s.parseFault(body)
	if err == nil && fault != nil {
		resp.Fault = fault
		return resp, fault
	}

	return resp, nil
}

// sendWithDigestAuth retries a request with HTTP Digest Authentication.
func (s *SOAPClient) sendWithDigestAuth(req *SOAPRequest, wwwAuth string) (*SOAPResponse, error) {
	// Parse digest challenge
	realm, nonce, qop, opaque, algorithm := parseDigestChallenge(wwwAuth)
	if realm == "" {
		return nil, fmt.Errorf("onvif: invalid digest challenge")
	}

	// Build SOAP envelope (no WS-Security for digest auth)
	envelope := s.buildEnvelope(req.Body, req.Action, true)

	uri, err := url.Parse(req.ServiceURL)
	if err != nil {
		return nil, fmt.Errorf("onvif: invalid URL: %w", err)
	}
	uriPath := uri.Path

	// Calculate digest
	nc := "00000001"
	cnonce := randomHex(8)
	ha1 := md5Sum(s.username + ":" + realm + ":" + s.password)
	ha2 := md5Sum("POST:" + uriPath)

	var response string
	if qop == "auth" || qop == "auth-int" {
		response = md5Sum(ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":" + qop + ":" + ha2)
	} else {
		response = md5Sum(ha1 + ":" + nonce + ":" + ha2)
	}

	// Build Authorization header
	authHeader := fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s"`,
		s.username, realm, nonce, uriPath, response)
	if qop != "" {
		authHeader += fmt.Sprintf(", qop=%s, nc=%s, cnonce=%s", qop, nc, cnonce)
	}
	if opaque != "" {
		authHeader += fmt.Sprintf(`, opaque="%s"`, opaque)
	}
	if algorithm != "" {
		authHeader += fmt.Sprintf(`, algorithm=%s`, algorithm)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", req.ServiceURL, bytes.NewBufferString(envelope))
	if err != nil {
		return nil, fmt.Errorf("onvif: create request failed: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	httpReq.Header.Set("Authorization", authHeader)
	if req.Action != "" {
		httpReq.Header.Set("SOAPAction", req.Action)
	}

	// Send request
	httpResp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("onvif: send request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("onvif: read response failed: %w", err)
	}

	resp := &SOAPResponse{
		Body:     body,
		HTTPCode: httpResp.StatusCode,
	}

	if httpResp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("onvif: digest auth failed (401)")
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("onvif: HTTP error %d", httpResp.StatusCode)
	}

	// Parse SOAP fault
	fault, err := s.parseFault(body)
	if err == nil && fault != nil {
		resp.Fault = fault
		return resp, fault
	}

	return resp, nil
}

// buildEnvelope builds a complete SOAP envelope with WS-Security.
func (s *SOAPClient) buildEnvelope(body, action string, noAuth bool) string {
	var buf bytes.Buffer

	// Envelope header
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://www.w3.org/2005/08/addressing">
<s:Header>`)

	// Add WS-Security header
	if !noAuth && s.username != "" {
		nonce, timestamp := s.generateWSSE()
		buf.WriteString(fmt.Sprintf(`
<Security s:mustUnderstand="1" xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd">
<UsernameToken>
<Username>%s</Username>
<Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest">%s</Password>
<Nonce EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-soap-message-security-1.0#Base64Binary">%s</Nonce>
<Created xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">%s</Created>
</UsernameToken>
</Security>`,
			s.username,
			s.passwordDigest(nonce, timestamp),
			base64.StdEncoding.EncodeToString(nonce),
			timestamp,
		))
	}

	// Close header and start body
	buf.WriteString(`
</s:Header>
<s:Body xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema">`)

	buf.WriteString(body)
	buf.WriteString(`
</s:Body>
</s:Envelope>`)

	return buf.String()
}

// generateWSSE generates WS-Security nonce and timestamp.
func (s *SOAPClient) generateWSSE() ([]byte, string) {
	nonce := make([]byte, 16)
	rand.Read(nonce)

	now := time.Now().Add(s.timeShift)
	timestamp := now.Format("2006-01-02T15:04:05.000Z")

	return nonce, timestamp
}

// passwordDigest calculates WS-Security password digest.
func (s *SOAPClient) passwordDigest(nonce []byte, timestamp string) string {
	h := sha1.New()
	h.Write(nonce)
	h.Write([]byte(timestamp))
	h.Write([]byte(s.password))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// parseFault parses a SOAP fault from response body.
func (s *SOAPClient) parseFault(body []byte) (*SOAPFault, error) {
	var envelope struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Fault struct {
				Code struct {
					Value string `xml:"Value"`
				} `xml:"Code"`
				Reason struct {
					Text string `xml:"Text"`
				} `xml:"Reason"`
				Detail struct {
					Text string `xml:",chardata"`
				} `xml:"Detail"`
			} `xml:"Fault"`
		} `xml:"Body"`
	}

	if err := xml.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}

	if envelope.Body.Fault.Code.Value == "" {
		return nil, nil
	}

	return &SOAPFault{
		Code:   envelope.Body.Fault.Code.Value,
		Reason: envelope.Body.Fault.Reason.Text,
		Detail: envelope.Body.Fault.Detail.Text,
	}, nil
}

// ParseResponseBody parses the SOAP body into the given struct.
func ParseResponseBody(body []byte, v interface{}) error {
	// Extract body content
	var envelope struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Content []byte `xml:",innerxml"`
		} `xml:"Body"`
	}

	if err := xml.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("onvif: parse envelope failed: %w", err)
	}

	if err := xml.Unmarshal(envelope.Body.Content, v); err != nil {
		return fmt.Errorf("onvif: parse body failed: %w", err)
	}

	return nil
}

// parseDigestChallenge parses the WWW-Authenticate Digest header.
func parseDigestChallenge(header string) (realm, nonce, qop, opaque, algorithm string) {
	// Remove "Digest " prefix
	header = strings.TrimPrefix(header, "Digest ")

	// Split by comma
	parts := strings.Split(header, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		value := strings.Trim(strings.TrimSpace(kv[1]), `"`)

		switch key {
		case "realm":
			realm = value
		case "nonce":
			nonce = value
		case "qop":
			qop = value
		case "opaque":
			opaque = value
		case "algorithm":
			algorithm = value
		}
	}
	return
}

// md5Sum calculates MD5 hash of a string and returns hex.
func md5Sum(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// randomHex generates a random hex string of n bytes.
func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
