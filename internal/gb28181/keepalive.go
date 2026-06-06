package gb28181

import (
	"log/slog"
	"time"

	"github.com/emiago/sipgo/sip"
)

func (g *GB28181API) handlerNotify(req *sip.Request, tx sip.ServerTransaction) {
	deviceID := req.From().Address.User

	var msg MessageNotify
	body := req.Body()
	if len(body) > 0 {
		if err := xmlUnmarshal(body, &msg); err != nil {
			slog.Error("keepalive xml decode error", "device_id", deviceID, "error", err)
			tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil))
			return
		}
	}

	g.store.LoadOrStore(deviceID, &Device{
		DeviceID: deviceID,
		source:   req.Source(),
	})

	g.store.Change(deviceID, func(d *Device) {
		d.LastKeepaliveAt = time.Now()
		d.IsOnline = msg.Status == "OK" || msg.Status == "ON"
		d.Address = req.Source()
	})

	tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil))
}
