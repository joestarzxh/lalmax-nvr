package gb28181

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/emiago/sipgo/sip"
)

func randInt(min, max int) int {
	return rand.Intn(max-min+1) + min
}

func xmlUnmarshal(data []byte, v interface{}) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	return decoder.Decode(v)
}

func (g *GB28181API) sendMessage(targetID string, dev *Device, body []byte) error {
	uri := sip.Uri{
		Scheme: "sip",
		User:   targetID,
		Host:   dev.Address,
		Port:   5060,
	}
	if host, port, err := net.SplitHostPort(dev.Address); err == nil {
		uri.Host = host
		if p := portInt(port); p > 0 {
			uri.Port = p
		}
	}

	req := sip.NewRequest(sip.MESSAGE, uri)
	req.SetBody(body)
	req.AppendHeader(sip.NewHeader("Content-Type", "Application/MANSCDP+xml"))

	_, err := g.client.Do(context.Background(), req)
	return err
}

func portInt(s string) int {
	var p int
	fmt.Sscanf(s, "%d", &p)
	return p
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
