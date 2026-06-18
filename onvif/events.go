package onvif

import (
	"context"
	"encoding/xml"
	"fmt"
	"time"
)

// EventsService handles event operations.
type EventsService struct {
	client *Client
}

// NewEventsService creates a new events service.
func NewEventsService(client *Client) *EventsService {
	return &EventsService{client: client}
}

// PullPointSubscription represents a PullPoint subscription.
type PullPointSubscription struct {
	Reference string
	Timeout   time.Duration
}

// CreatePullPointSubscription creates a PullPoint subscription.
func (s *EventsService) CreatePullPointSubscription(ctx context.Context) (*PullPointSubscription, error) {
	if s.client.endpoints.Events == nil {
		return nil, fmt.Errorf("onvif: Events service not available")
	}

	body := `<CreatePullPointSubscription xmlns="http://www.onvif.org/ver10/events/wsdl">
  <InitialTerminationTime>PT60S</InitialTerminationTime>
</CreatePullPointSubscription>`

	resp, err := s.client.soap.Send(&SOAPRequest{
		ServiceURL: s.client.endpoints.Events.String(),
		Body:       body,
	})
	if err != nil {
		return nil, fmt.Errorf("onvif: CreatePullPointSubscription failed: %w", err)
	}

	var result struct {
		XMLName xml.Name `xml:"CreatePullPointSubscriptionResponse"`
		SubscriptionReference struct {
			Address string `xml:"Address"`
		} `xml:"SubscriptionReference"`
	}

	if err := ParseResponseBody(resp.Body, &result); err != nil {
		return nil, err
	}

	sub := &PullPointSubscription{
		Reference: result.SubscriptionReference.Address,
		Timeout:   60 * time.Second,
	}

	return sub, nil
}

// PullMessages pulls messages from a PullPoint subscription.
func (s *EventsService) PullMessages(ctx context.Context, subscriptionRef string, timeout time.Duration, messageLimit int) ([]Event, error) {
	if messageLimit <= 0 {
		messageLimit = 100
	}

	body := fmt.Sprintf(`<PullMessages xmlns="http://www.onvif.org/ver10/events/wsdl">
  <Timeout>PT%.0fS</Timeout>
  <MessageLimit>%d</MessageLimit>
</PullMessages>`, timeout.Seconds(), messageLimit)

	resp, err := s.client.soap.Send(&SOAPRequest{
		ServiceURL: subscriptionRef,
		Body:       body,
	})
	if err != nil {
		return nil, fmt.Errorf("onvif: PullMessages failed: %w", err)
	}

	var result struct {
		XMLName xml.Name `xml:"PullMessagesResponse"`
		NotificationMessage []struct {
			Topic string `xml:"Topic"`
			Message struct {
				Message struct {
					Source struct {
						SimpleItem []struct {
							Name  string `xml:"Name,attr"`
							Value string `xml:"Value,attr"`
						} `xml:"SimpleItem"`
					} `xml:"Source"`
					Data struct {
						SimpleItem []struct {
							Name  string `xml:"Name,attr"`
							Value string `xml:"Value,attr"`
						} `xml:"SimpleItem"`
					} `xml:"Data"`
					UtcTime string `xml:"UtcTime,attr"`
				} `xml:"Message"`
			} `xml:"Message"`
		} `xml:"NotificationMessage"`
	}

	if err := ParseResponseBody(resp.Body, &result); err != nil {
		return nil, err
	}

	events := make([]Event, 0, len(result.NotificationMessage))
	for _, msg := range result.NotificationMessage {
		event := Event{
			Topic: msg.Topic,
			Source: make(map[string]interface{}),
			Data:   make(map[string]interface{}),
		}

		// Parse timestamp
		if msg.Message.Message.UtcTime != "" {
			if t, err := time.Parse(time.RFC3339, msg.Message.Message.UtcTime); err == nil {
				event.Timestamp = t
			}
		}

		// Parse source
		for _, item := range msg.Message.Message.Source.SimpleItem {
			event.Source[item.Name] = item.Value
		}

		// Parse data
		for _, item := range msg.Message.Message.Data.SimpleItem {
			event.Data[item.Name] = item.Value
		}

		events = append(events, event)
	}

	return events, nil
}

// Renew renews a subscription.
func (s *EventsService) Renew(ctx context.Context, subscriptionRef string, terminationTime time.Duration) error {
	body := fmt.Sprintf(`<Renew xmlns="http://docs.oasis-open.org/wsn/b-2">
  <TerminationTime>PT%.0fS</TerminationTime>
</Renew>`, terminationTime.Seconds())

	_, err := s.client.soap.Send(&SOAPRequest{
		ServiceURL: subscriptionRef,
		Body:       body,
	})
	if err != nil {
		return fmt.Errorf("onvif: Renew failed: %w", err)
	}

	return nil
}

// Unsubscribe unsubscribes from a subscription.
func (s *EventsService) Unsubscribe(ctx context.Context, subscriptionRef string) error {
	body := `<Unsubscribe xmlns="http://docs.oasis-open.org/wsn/b-2"/>`

	_, err := s.client.soap.Send(&SOAPRequest{
		ServiceURL: subscriptionRef,
		Body:       body,
	})
	if err != nil {
		return fmt.Errorf("onvif: Unsubscribe failed: %w", err)
	}

	return nil
}
