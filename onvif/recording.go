package onvif

import (
	"context"
	"encoding/xml"
	"fmt"
	"time"
)

// RecordingService handles recording operations.
type RecordingService struct {
	client *Client
}

// NewRecordingService creates a new recording service.
func NewRecordingService(client *Client) *RecordingService {
	return &RecordingService{client: client}
}

// GetRecordings retrieves all recordings from the device.
func (s *RecordingService) GetRecordings(ctx context.Context) ([]Recording, error) {
	body := `<GetRecordings xmlns="http://www.onvif.org/ver10/recording/wsdl"/>`

	resp, err := s.client.soap.Send(&SOAPRequest{
		ServiceURL: s.client.endpoints.Recording.String(),
		Body:       body,
	})
	if err != nil {
		return nil, fmt.Errorf("onvif: GetRecordings failed: %w", err)
	}

	var result struct {
		XMLName xml.Name `xml:"GetRecordingsResponse"`
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
				MaximumRetentionTime string `xml:"MaximumRetentionTime"`
			} `xml:"Configuration"`
			Tracks struct {
				Track []struct {
					TrackToken string `xml:"TrackToken"`
					Configuration struct {
						TrackType   string `xml:"TrackType"`
						Description string `xml:"Description"`
					} `xml:"Configuration"`
				} `xml:"Track"`
			} `xml:"Tracks"`
		} `xml:"RecordingItem"`
	}

	if err := ParseResponseBody(resp.Body, &result); err != nil {
		return nil, err
	}

	recordings := make([]Recording, 0, len(result.RecordingItem))
	for _, item := range result.RecordingItem {
		rec := Recording{
			Token:       item.RecordingToken,
			Description: item.Configuration.Source.Description,
			Source: RecordingSource{
				SourceID:    item.Configuration.Source.SourceID,
				Name:        item.Configuration.Source.Name,
				Location:    item.Configuration.Source.Location,
				Description: item.Configuration.Source.Description,
			},
			Status: "active",
		}

		// Extract name from content
		if len(item.Configuration.Content) > 0 {
			rec.Name = item.Configuration.Content[0].Name
		}

		// Extract tracks
		for _, t := range item.Tracks.Track {
			rec.Tracks = append(rec.Tracks, Track{
				Token:       t.TrackToken,
				TrackType:   t.Configuration.TrackType,
				Description: t.Configuration.Description,
			})
		}

		recordings = append(recordings, rec)
	}

	return recordings, nil
}

// SearchRecordings searches for recording segments with pagination support.
func (s *RecordingService) SearchRecordings(ctx context.Context, filter SearchFilter) (*SearchResult, error) {
	maxResults := filter.MaxResults
	if maxResults <= 0 {
		maxResults = 100
	}

	// If we have a search token, continue the search
	if filter.SearchToken != "" {
		return s.getSearchResults(ctx, filter.SearchToken, maxResults)
	}

	// Start a new search
	searchToken, err := s.findRecordings(ctx, filter, maxResults)
	if err != nil {
		return nil, err
	}

	if searchToken == "" {
		return &SearchResult{
			Segments: []RecordingSegment{},
		}, nil
	}

	// Get results
	return s.getSearchResults(ctx, searchToken, maxResults)
}

// findRecordings starts a recording search and returns a search token.
func (s *RecordingService) findRecordings(ctx context.Context, filter SearchFilter, maxResults int) (string, error) {
	// Build scope based on filter
	scope := `<tt:IncludedSources>
          <tt:Type>http://www.onvif.org/ver10/schema/RecordingJob</tt:Type>
        </tt:IncludedSources>`

	// Add time range filter if specified
	if !filter.StartTime.IsZero() || !filter.EndTime.IsZero() {
		timeFilter := `<tt:IncludedTimes>`
		if !filter.StartTime.IsZero() {
			timeFilter += fmt.Sprintf(`<tt:From>%s</tt:From>`, filter.StartTime.Format(time.RFC3339))
		}
		if !filter.EndTime.IsZero() {
			timeFilter += fmt.Sprintf(`<tt:Until>%s</tt:Until>`, filter.EndTime.Format(time.RFC3339))
		}
		timeFilter += `</tt:IncludedTimes>`
		scope += timeFilter
	}

	body := fmt.Sprintf(`<FindRecordings xmlns="http://www.onvif.org/ver10/recording/wsdl">
  <Scope>%s</Scope>
  <MaxResults>%d</MaxResults>
  <KeepAliveTime>PT60S</KeepAliveTime>
</FindRecordings>`, scope, maxResults)

	resp, err := s.client.soap.Send(&SOAPRequest{
		ServiceURL: s.client.endpoints.Search.String(),
		Body:       body,
	})
	if err != nil {
		return "", fmt.Errorf("onvif: FindRecordings failed: %w", err)
	}

	var result struct {
		XMLName xml.Name `xml:"FindRecordingsResponse"`
		SearchToken string `xml:"SearchToken"`
	}

	if err := ParseResponseBody(resp.Body, &result); err != nil {
		return "", err
	}

	return result.SearchToken, nil
}

// getSearchResults retrieves search results using a search token.
func (s *RecordingService) getSearchResults(ctx context.Context, searchToken string, maxResults int) (*SearchResult, error) {
	body := fmt.Sprintf(`<GetRecordingSearchResults xmlns="http://www.onvif.org/ver10/search/wsdl">
  <SearchToken>%s</SearchToken>
  <WaitTime>PT5S</WaitTime>
  <MinResults>1</MinResults>
  <MaxResults>%d</MaxResults>
</GetRecordingSearchResults>`, searchToken, maxResults)

	resp, err := s.client.soap.Send(&SOAPRequest{
		ServiceURL: s.client.endpoints.Search.String(),
		Body:       body,
	})
	if err != nil {
		return nil, fmt.Errorf("onvif: GetRecordingSearchResults failed: %w", err)
	}

	var result struct {
		XMLName xml.Name `xml:"GetRecordingSearchResultsResponse"`
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
	}

	if err := ParseResponseBody(resp.Body, &result); err != nil {
		return nil, err
	}

	searchResult := &SearchResult{
		SearchToken: searchToken,
		SearchState: result.ResultList.SearchState,
		HasMore:     result.ResultList.SearchState == "MoreResults",
	}

	// Parse segments
	for _, rec := range result.ResultList.Recording {
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

				searchResult.Segments = append(searchResult.Segments, segment)
			}
		}
	}

	return searchResult, nil
}

// EndSearch ends a search session.
func (s *RecordingService) EndSearch(ctx context.Context, searchToken string) error {
	body := fmt.Sprintf(`<EndSearch xmlns="http://www.onvif.org/ver10/search/wsdl">
  <SearchToken>%s</SearchToken>
</EndSearch>`, searchToken)

	_, err := s.client.soap.Send(&SOAPRequest{
		ServiceURL: s.client.endpoints.Search.String(),
		Body:       body,
	})
	if err != nil {
		return fmt.Errorf("onvif: EndSearch failed: %w", err)
	}

	return nil
}
