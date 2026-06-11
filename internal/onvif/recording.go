package onvif

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// Recording represents a recording on an ONVIF device.
type Recording struct {
	Token       string          `json:"token"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Source      RecordingSource `json:"source"`
	StartTime   time.Time       `json:"start_time"`
	EndTime     time.Time       `json:"end_time"`
	Status      string          `json:"status"`
}

// RecordingSource represents the source of a recording.
type RecordingSource struct {
	SourceID    string `json:"source_id"`
	Name        string `json:"name"`
	Location    string `json:"location"`
	Description string `json:"description"`
}

// RecordingSegment represents a segment of a recording.
type RecordingSegment struct {
	Token          string    `json:"token"`
	RecordingToken string    `json:"recording_token"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	FilePath       string    `json:"file_path"`
	Duration       int64     `json:"duration"` // seconds
	Size           int64     `json:"size"`     // bytes
}

// SearchRequest represents a search request for recordings.
type SearchRequest struct {
	RecordingToken string
	StartTime      time.Time
	EndTime        time.Time
	MaxResults     int
}

// GetRecordings retrieves all recordings from an ONVIF device.
func (c *Client) GetRecordings(ctx context.Context) ([]Recording, error) {
	if !c.ready {
		return nil, fmt.Errorf("onvif client not connected, call Connect() first")
	}

	c.soapMu.Lock()
	defer c.soapMu.Unlock()

	soapBody := `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:trc="http://www.onvif.org/ver10/recording/wsdl">
  <s:Body>
    <trc:GetRecordings/>
  </s:Body>
</s:Envelope>`

	body, err := c.doAuthenticatedSOAPRequestTo(ctx, c.recordingEndpoint(), soapBody)
	if err != nil {
		return nil, fmt.Errorf("get recordings: %w", err)
	}

	var envelope struct {
		Body struct {
			GetRecordingsResponse struct {
				RecordingItem []struct {
					RecordingToken string `xml:"RecordingToken"`
					Configuration  struct {
						Source struct {
							SourceID    string `xml:"SourceId"`
							Name        string `xml:"Name"`
							Location    string `xml:"Location"`
							Description string `xml:"Description"`
						} `xml:"Source"`
						Content []struct {
							Name string `xml:"Name"`
						} `xml:"Content"`
					} `xml:"Configuration"`
					Tracks struct {
						Track []struct {
							TrackToken string `xml:"TrackToken"`
						} `xml:"Track"`
					} `xml:"Tracks"`
				} `xml:"RecordingItem"`
			} `xml:"GetRecordingsResponse"`
		} `xml:"Body"`
	}

	if err := xml.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse recordings response: %w", err)
	}

	result := make([]Recording, 0, len(envelope.Body.GetRecordingsResponse.RecordingItem))
	for _, item := range envelope.Body.GetRecordingsResponse.RecordingItem {
		name := ""
		if len(item.Configuration.Content) > 0 {
			name = item.Configuration.Content[0].Name
		}
		rec := Recording{
			Token:       item.RecordingToken,
			Name:        name,
			Description: item.Configuration.Source.Description,
			Source: RecordingSource{
				SourceID:    item.Configuration.Source.SourceID,
				Name:        item.Configuration.Source.Name,
				Location:    item.Configuration.Source.Location,
				Description: item.Configuration.Source.Description,
			},
			Status: "active",
		}
		result = append(result, rec)
	}

	return result, nil
}

// SearchRecordings searches for recording segments on an ONVIF device.
func (c *Client) SearchRecordings(ctx context.Context, req SearchRequest) ([]RecordingSegment, error) {
	if !c.ready {
		return nil, fmt.Errorf("onvif client not connected, call Connect() first")
	}

	c.soapMu.Lock()
	defer c.soapMu.Unlock()

	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 100
	}

	soapBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:trc="http://www.onvif.org/ver10/recording/wsdl"
 xmlns:tt="http://www.onvif.org/ver10/schema">
  <s:Body>
    <trc:FindRecordings>
      <trc:Scope>
        <tt:IncludedSources>
          <tt:Type>http://www.onvif.org/ver10/schema/RecordingJob</tt:Type>
        </tt:IncludedSources>
      </trc:Scope>
      <trc:MaxResults>%d</trc:MaxResults>
      <trc:KeepAliveTime>PT60S</trc:KeepAliveTime>
    </trc:FindRecordings>
  </s:Body>
</s:Envelope>`, maxResults)

	// First, start the search
	searchBody, err := c.doAuthenticatedSOAPRequestTo(ctx, c.searchEndpoint(), soapBody)
	if err != nil {
		return nil, fmt.Errorf("find recordings: %w", err)
	}

	var searchEnvelope struct {
		Body struct {
			FindRecordingsResponse struct {
				SearchToken string `xml:"SearchToken"`
			} `xml:"FindRecordingsResponse"`
		} `xml:"Body"`
	}

	if err := xml.Unmarshal(searchBody, &searchEnvelope); err != nil {
		return nil, fmt.Errorf("parse find recordings response: %w", err)
	}

	searchToken := searchEnvelope.Body.FindRecordingsResponse.SearchToken
	if searchToken == "" {
		return []RecordingSegment{}, nil
	}

	// Get search results
	getResultsBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:trc="http://www.onvif.org/ver10/recording/wsdl"
 xmlns:tt="http://www.onvif.org/ver10/schema">
  <s:Body>
    <trc:GetRecordingSearchResults>
      <trc:SearchToken>%s</trc:SearchToken>
      <trc:WaitTime>PT5S</trc:WaitTime>
      <trc:MinResults>1</trc:MinResults>
      <trc:MaxResults>%d</trc:MaxResults>
    </trc:GetRecordingSearchResults>
  </s:Body>
</s:Envelope>`, searchToken, maxResults)

	resultsBody, err := c.doAuthenticatedSOAPRequestTo(ctx, c.searchEndpoint(), getResultsBody)
	if err != nil {
		return nil, fmt.Errorf("get recording search results: %w", err)
	}

	var resultsEnvelope struct {
		Body struct {
			GetRecordingSearchResultsResponse struct {
				ResultList struct {
					SearchState string `xml:"SearchState"`
					Recording   []struct {
						RecordingToken string `xml:"RecordingToken"`
						Track          []struct {
							TrackToken string `xml:"TrackToken"`
							Segment    []struct {
								SegmentToken string `xml:"SegmentToken"`
								Start        string `xml:"Start"`
								End          string `xml:"End"`
								FileURL      string `xml:"FileURL"`
								Duration     string `xml:"Duration"`
								Size         int64  `xml:"Size"`
							} `xml:"Segment"`
						} `xml:"Track"`
					} `xml:"Recording"`
				} `xml:"ResultList"`
			} `xml:"GetRecordingSearchResultsResponse"`
		} `xml:"Body"`
	}

	if err := xml.Unmarshal(resultsBody, &resultsEnvelope); err != nil {
		return nil, fmt.Errorf("parse search results: %w", err)
	}

	var segments []RecordingSegment
	for _, rec := range resultsEnvelope.Body.GetRecordingSearchResultsResponse.ResultList.Recording {
		for _, track := range rec.Track {
			for _, seg := range track.Segment {
				segment := RecordingSegment{
					Token:          seg.SegmentToken,
					RecordingToken: rec.RecordingToken,
					FilePath:       seg.FileURL,
					Size:           seg.Size,
				}

				if t, err := time.Parse(time.RFC3339, seg.Start); err == nil {
					segment.StartTime = t
				}
				if t, err := time.Parse(time.RFC3339, seg.End); err == nil {
					segment.EndTime = t
				}

				segments = append(segments, segment)
			}
		}
	}

	return segments, nil
}

// GetReplayURI retrieves the replay URI for a recording.
func (c *Client) GetReplayURI(ctx context.Context, recordingToken string) (string, error) {
	if !c.ready {
		return "", fmt.Errorf("onvif client not connected, call Connect() first")
	}

	c.soapMu.Lock()
	defer c.soapMu.Unlock()

	soapBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:trp="http://www.onvif.org/ver10/replay/wsdl"
 xmlns:tt="http://www.onvif.org/ver10/schema">
  <s:Body>
    <trp:GetReplayUri>
      <trp:StreamSetup>
        <tt:Stream>RTP-Unicast</tt:Stream>
        <tt:Transport>
          <tt:Protocol>RTSP</tt:Protocol>
        </tt:Transport>
      </trp:StreamSetup>
      <trp:RecordingToken>%s</trp:RecordingToken>
    </trp:GetReplayUri>
  </s:Body>
</s:Envelope>`, recordingToken)

	body, err := c.doAuthenticatedSOAPRequestTo(ctx, c.replayEndpoint(), soapBody)
	if err != nil {
		return "", fmt.Errorf("get replay URI: %w", err)
	}

	var envelope struct {
		Body struct {
			GetReplayUriResponse struct {
				Uri string `xml:"Uri"`
			} `xml:"GetReplayUriResponse"`
		} `xml:"Body"`
	}

	if err := xml.Unmarshal(body, &envelope); err != nil {
		return "", fmt.Errorf("parse replay URI response: %w", err)
	}

	return envelope.Body.GetReplayUriResponse.Uri, nil
}

// recordingEndpoint returns the ONVIF Recording service endpoint.
func (c *Client) recordingEndpoint() string {
	return strings.Replace(c.endpoint, "/device_service", "/recording_service", 1)
}

// searchEndpoint returns the ONVIF Search service endpoint.
func (c *Client) searchEndpoint() string {
	return strings.Replace(c.endpoint, "/device_service", "/search_service", 1)
}

// replayEndpoint returns the ONVIF Replay service endpoint.
func (c *Client) replayEndpoint() string {
	return strings.Replace(c.endpoint, "/device_service", "/replay_service", 1)
}
