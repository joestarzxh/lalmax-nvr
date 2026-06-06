package gb28181

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// MessageRecordInfoResponse is the XML structure for RecordInfo response.
type MessageRecordInfoResponse struct {
	XMLName  struct{}     `xml:"Response"`
	CmdType  string       `xml:"CmdType"`
	SN       int          `xml:"SN"`
	DeviceID string       `xml:"DeviceID"`
	SumNum   int          `xml:"SumNum"`
	Item     []RecordItem `xml:"RecordList>Item"`
}

// RecordItem represents a single recording entry on a device.
type RecordItem struct {
	DeviceID  string `xml:"DeviceID"`
	Name      string `xml:"Name"`
	FilePath  string `xml:"FilePath"`
	Address   string `xml:"Address"`
	StartTime string `xml:"StartTime"`
	EndTime   string `xml:"EndTime"`
	Secrecy   int    `xml:"Secrecy"`
	Type      string `xml:"Type"`
}

// Records is the aggregated result of recording query, grouped by date.
type Records struct {
	DayTotal int         `json:"day_total"`
	TimeNum  int         `json:"time_num"`
	Data     []RecordDay `json:"data"`
}

// RecordDay represents recordings on a single day.
type RecordDay struct {
	Date  string       `json:"date"`  // "2006-01-02"
	Items []RecordTime `json:"items"`
}

// RecordTime represents a continuous recording time segment.
type RecordTime struct {
	Start int64 `json:"start"` // Unix timestamp
	End   int64 `json:"end"`   // Unix timestamp
}

// recordListState tracks the state of an in-progress RecordInfo query.
type recordListState struct {
	channelID string
	sn        int
	resp      chan Records
	total     int
	data      [][]int64
	mu        sync.Mutex
	start     int64
	end       int64
}

var recordListStore sync.Map // key = "channelID:sn" -> recordListState

// QueryRecordInfo sends a RecordInfo query to a device to get its recording list.
func (g *GB28181API) QueryRecordInfo(deviceID, channelID string, startTime, endTime time.Time) (*Records, error) {
	slog.Debug("QueryRecordInfo", "device_id", deviceID, "channel_id", channelID,
		"start", startTime.Format(time.RFC3339), "end", endTime.Format(time.RFC3339))

	dev, ok := g.store.Load(deviceID)
	if !ok || !dev.IsOnline {
		return nil, ErrDeviceOffline
	}

	_, ok = dev.GetChannel(channelID)
	if !ok {
		return nil, ErrChannelNotExist
	}

	sn := randInt(100000, 999999)
	key := fmt.Sprintf("%s:%d", channelID, sn)

	state := recordListState{
		channelID: channelID,
		sn:        sn,
		resp:      make(chan Records, 1),
		data:      make([][]int64, 0),
		start:     startTime.Unix(),
		end:       endTime.Unix(),
	}
	recordListStore.Store(key, &state)
	defer recordListStore.Delete(key)

	xmlBody := recordInfoXML(channelID, sn, startTime, endTime)
	if err := g.sendMessage(channelID, dev, xmlBody); err != nil {
		return nil, fmt.Errorf("send RecordInfo query failed: %w", err)
	}

	// Wait for complete response or timeout
	select {
	case res := <-state.resp:
		return &res, nil
	case <-time.After(10 * time.Second):
		// Return partial data on timeout
		state.mu.Lock()
		defer state.mu.Unlock()
		if len(state.data) > 0 {
			result := aggregateRecordList(state.data)
			return &result, nil
		}
		return nil, fmt.Errorf("RecordInfo query timeout")
	}
}

// handleRecordInfoResponse processes a RecordInfo response from a device.
func (g *GB28181API) handleRecordInfoResponse(deviceID string, body []byte) {
	var msg MessageRecordInfoResponse
	if err := xml.Unmarshal(body, &msg); err != nil {
		slog.Error("RecordInfo xml decode error", "device_id", deviceID, "error", err)
		return
	}

	key := fmt.Sprintf("%s:%d", msg.DeviceID, msg.SN)
	val, ok := recordListStore.Load(key)
	if !ok {
		slog.Debug("RecordInfo response for unknown query", "device_id", deviceID, "sn", msg.SN)
		return
	}
	state := val.(*recordListState)

	state.mu.Lock()
	defer state.mu.Unlock()

	state.total += len(msg.Item)
	for _, item := range msg.Item {
		s, _ := time.ParseInLocation("2006-01-02T15:04:05", item.StartTime, time.Local)
		e, _ := time.ParseInLocation("2006-01-02T15:04:05", item.EndTime, time.Local)
		sint := s.Unix()
		eint := e.Unix()
		if sint < state.start {
			sint = state.start
		}
		if eint > state.end {
			eint = state.end
		}
		state.data = append(state.data, []int64{sint, eint})
	}

	// Check if we've received all items
	if state.total >= msg.SumNum && msg.SumNum > 0 {
		result := aggregateRecordList(state.data)
		select {
		case state.resp <- result:
		default:
		}
	}
}

// recordInfoXML generates the XML body for a RecordInfo query.
func recordInfoXML(channelID string, sn int, start, end time.Time) []byte {
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="GB2312"?>
<Query>
<CmdType>RecordInfo</CmdType>
<SN>%d</SN>
<DeviceID>%s</DeviceID>
<StartTime>%s</StartTime>
<EndTime>%s</EndTime>
<Secrecy>0</Secrecy>
<Type>time</Type>
</Query>`, sn, channelID, start.Format("2006-01-02T15:04:05"), end.Format("2006-01-02T15:04:05")))
}

// aggregateRecordList merges overlapping time segments and groups by date.
func aggregateRecordList(data [][]int64) Records {
	if len(data) == 0 {
		return Records{}
	}

	// Sort by start time
	sort.Slice(data, func(i, j int) bool {
		return data[i][0] < data[j][0]
	})

	// Merge overlapping/adjacent segments
	merged := [][]int64{}
	current := data[0]
	for i := 1; i < len(data); i++ {
		if data[i][0] <= current[1] {
			if data[i][1] > current[1] {
				current[1] = data[i][1]
			}
		} else {
			merged = append(merged, current)
			current = data[i]
		}
	}
	merged = append(merged, current)

	// Group by date
	result := Records{}
	dayMap := map[string][]RecordTime{}
	var dates []string

	for _, seg := range merged {
		s := time.Unix(seg[0], 0)
		e := time.Unix(seg[1], 0)
		dayStart, _ := time.ParseInLocation("20060102", s.Format("20060102"), time.Local)

		for {
			dayEnd := dayStart.Add(24 * time.Hour)
			if e.Unix() >= dayEnd.Unix() {
				// Spans to next day
				dateStr := dayStart.Format("2006-01-02")
				if _, exists := dayMap[dateStr]; !exists {
					dates = append(dates, dateStr)
					result.DayTotal++
				}
				startVal := seg[0]
				if dayStart.Unix() > startVal {
					startVal = dayStart.Unix()
				}
				dayMap[dateStr] = append(dayMap[dateStr], RecordTime{
					Start: startVal,
					End:   dayEnd.Unix() - 1,
				})
				result.TimeNum++
				dayStart = dayEnd
			} else {
				dateStr := dayStart.Format("2006-01-02")
				if _, exists := dayMap[dateStr]; !exists {
					dates = append(dates, dateStr)
					result.DayTotal++
				}
				startVal := seg[0]
				if dayStart.Unix() > startVal {
					startVal = dayStart.Unix()
				}
				dayMap[dateStr] = append(dayMap[dateStr], RecordTime{
					Start: startVal,
					End:   seg[1],
				})
				result.TimeNum++
				break
			}
		}
	}

	// Build ordered result
	for _, date := range dates {
		result.Data = append(result.Data, RecordDay{
			Date:  date,
			Items: dayMap[date],
		})
	}

	return result
}
