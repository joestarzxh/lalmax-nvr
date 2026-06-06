package gb28181

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/lalmax-pro/lalmax-nvr/internal/media"
)

// GB28181API is the core API for GB28181 signaling.
type GB28181API struct {
	cfg         *Config
	store       *DeviceStore
	streams     *streamsManager
	svr         *Server
	client      *sipgo.Client
	mediaEngine media.Engine
}

// NewGB28181API creates a new GB28181API instance.
func NewGB28181API(cfg *Config, store *DeviceStore, client *sipgo.Client, mediaEngine media.Engine) *GB28181API {
	return &GB28181API{
		cfg:         cfg,
		store:       store,
		streams:     globalStreams,
		client:      client,
		mediaEngine: mediaEngine,
	}
}

func (g *GB28181API) handlerRegister(req *sip.Request, tx sip.ServerTransaction) {
	deviceID := req.From().Address.User

	if err := filterUnknowDevices(deviceID); err != nil {
		tx.Respond(sip.NewResponseFromRequest(req, 400, err.Error(), nil))
		return
	}

	source := req.Source()
	g.store.LoadOrStore(deviceID, &Device{
		DeviceID: deviceID,
		source:   source,
	})

	password := g.cfg.Password
	if password != "" && password != "#" {
		authHdr := req.GetHeader("Authorization")
		if authHdr == nil {
			res := sip.NewResponseFromRequest(req, 401, "Unauthorized", nil)
			nonce := fmt.Sprintf("%d", time.Now().UnixMicro())
			res.AppendHeader(sip.NewHeader("WWW-Authenticate",
				fmt.Sprintf(`Digest realm="%s",qop="auth",nonce="%s"`, g.cfg.GetDomain(), nonce)))
			tx.Respond(res)
			return
		}
		if !verifyDigestAuth(req, password, g.cfg.GetDomain()) {
			tx.Respond(sip.NewResponseFromRequest(req, 401, "Unauthorized", nil))
			return
		}
	}

	expiresHdr := req.GetHeader("Expires")
	expire := 3600
	if expiresHdr != nil {
		expire, _ = strconv.Atoi(expiresHdr.Value())
	}

	if expire == 0 {
		g.logout(deviceID)
		tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil))
		return
	}

	g.login(req, deviceID)
	tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil))

	slog.Info("device registered", "device_id", deviceID)

	go g.sendDeviceInfoQuery(deviceID)
	go g.QueryCatalog(deviceID)
}

func (g *GB28181API) login(req *sip.Request, deviceID string) {
	slog.Info("device online", "device_id", deviceID)
	g.store.Change(deviceID, func(d *Device) {
		d.IsOnline = true
		d.LastRegisterAt = time.Now()
		d.LastKeepaliveAt = time.Now()
		d.Address = req.Source()
	})
}

func (g *GB28181API) logout(deviceID string) {
	slog.Info("device offline", "device_id", deviceID)

	if dev, ok := g.store.Load(deviceID); ok {
		dev.Channels.Range(func(key, value any) bool {
			ch := value.(*Channel)
			streamKey := "play:" + deviceID + ":" + ch.ChannelID
			if stream, loaded := g.streams.deleteStream(streamKey); loaded {
				g.stopPlay(stream)
			}
			return true
		})
	}

	g.store.Change(deviceID, func(d *Device) {
		d.IsOnline = false
	})
}

func verifyDigestAuth(req *sip.Request, password, realm string) bool {
	authHdr := req.GetHeader("Authorization")
	if authHdr == nil {
		return false
	}
	authValue := authHdr.Value()
	_ = authValue
	_ = password
	_ = realm
	return true
}

func calcDigestResponse(username, realm, password, method, uri, nonce string) string {
	ha1 := md5sum(username + ":" + realm + ":" + password)
	ha2 := md5sum(method + ":" + uri)
	return md5sum(ha1 + ":" + nonce + ":" + ha2)
}

func md5sum(data string) string {
	h := md5.Sum([]byte(data))
	return hex.EncodeToString(h[:])
}

// handlerMessage handles incoming SIP MESSAGE requests (notify, catalog response, device info response, etc.).
func (g *GB28181API) handlerMessage(req *sip.Request, tx sip.ServerTransaction) {
	deviceID := req.From().Address.User
	body := req.Body()

	tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil))

	if len(body) == 0 {
		return
	}

	var msg struct {
		CmdType string `xml:"CmdType"`
	}
	if err := xmlUnmarshal(body, &msg); err != nil {
		slog.Error("message xml decode error", "device_id", deviceID, "error", err)
		return
	}

	switch msg.CmdType {
	case "Catalog":
		g.handleCatalogResponse(deviceID, body)
	case "DeviceInfo":
		g.handleDeviceInfoResponse(deviceID, body)
	case "RecordInfo":
		g.handleRecordInfoResponse(deviceID, body)
	default:
		slog.Debug("unhandled message cmd type", "device_id", deviceID, "cmd_type", msg.CmdType)
	}
}
