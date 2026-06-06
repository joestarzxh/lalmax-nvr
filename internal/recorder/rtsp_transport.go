package recorder

import "github.com/bluenviron/gortsplib/v5"

func rtspTransportProtocol(transport string) *gortsplib.Protocol {
	protocol := gortsplib.ProtocolTCP
	if transport == "udp" {
		protocol = gortsplib.ProtocolUDP
	}
	return &protocol
}
