package onvif

import (
	"context"
	"fmt"
	"time"

	onviflib "github.com/lalmax-pro/lalmax-nvr/onvif"
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
	Duration       int64     `json:"duration"`
	Size           int64     `json:"size"`
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

	recSvc := c.client.RecordingService()
	recordings, err := recSvc.GetRecordings(ctx)
	if err != nil {
		return nil, fmt.Errorf("get recordings: %w", err)
	}

	result := make([]Recording, 0, len(recordings))
	for _, rec := range recordings {
		result = append(result, Recording{
			Token:       rec.Token,
			Name:        rec.Name,
			Description: rec.Description,
			Source: RecordingSource{
				SourceID:    rec.Source.SourceID,
				Name:        rec.Source.Name,
				Location:    rec.Source.Location,
				Description: rec.Source.Description,
			},
			Status: rec.Status,
		})
	}

	return result, nil
}

// SearchRecordings searches for recording segments on an ONVIF device.
func (c *Client) SearchRecordings(ctx context.Context, req SearchRequest) ([]RecordingSegment, error) {
	if !c.ready {
		return nil, fmt.Errorf("onvif client not connected, call Connect() first")
	}

	recSvc := c.client.RecordingService()
	searchResult, err := recSvc.SearchRecordings(ctx, onviflib.SearchFilter{
		RecordingToken: req.RecordingToken,
		StartTime:      req.StartTime,
		EndTime:        req.EndTime,
		MaxResults:     req.MaxResults,
	})
	if err != nil {
		return nil, fmt.Errorf("search recordings: %w", err)
	}

	result := make([]RecordingSegment, 0, len(searchResult.Segments))
	for _, seg := range searchResult.Segments {
		result = append(result, RecordingSegment{
			Token:          seg.Token,
			RecordingToken: seg.RecordingToken,
			StartTime:      seg.StartTime,
			EndTime:        seg.EndTime,
			FilePath:       seg.FilePath,
			Size:           seg.Size,
		})
	}

	// End search session if complete
	if !searchResult.HasMore && searchResult.SearchToken != "" {
		_ = recSvc.EndSearch(ctx, searchResult.SearchToken)
	}

	return result, nil
}

// GetReplayURI retrieves the replay URI for a recording.
func (c *Client) GetReplayURI(ctx context.Context, recordingToken string) (string, error) {
	if !c.ready {
		return "", fmt.Errorf("onvif client not connected, call Connect() first")
	}

	replaySvc := c.client.ReplayService()
	uri, err := replaySvc.GetReplayURI(ctx, recordingToken)
	if err != nil {
		return "", fmt.Errorf("get replay URI: %w", err)
	}

	return uri, nil
}
