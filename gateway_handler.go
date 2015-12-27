package loraserver

import (
	"net"

	log "github.com/Sirupsen/logrus"
	"github.com/brocaar/lorawan/semtech"
)

type UDPPacket struct {
	Data []byte
	Addr *net.UDPAddr
}

// ReadGatewayPackets reads incoming gateway packets from the given UDPConn.
func ReadGatewayPackets(c *net.UDPConn) error {
	buf := make([]byte, 65507) // max udp data size
	for {
		i, addr, err := c.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		data := make([]byte, i)
		copy(data, buf[0:i])
		p := UDPPacket{Addr: addr, Data: data}
		go func() {
			if err := HandleGatewayPacket(p); err != nil {
				log.WithFields(log.Fields{
					"data": p.Data,
					"addr": p.Addr,
				}).Errorf("could not handle packet: %s", err)
			}
		}()
	}
}

// HandleGatewayPacket handles a single gateway packet.
func HandleGatewayPacket(p UDPPacket) error {
	pt, err := semtech.GetPacketType(p.Data)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"addr": p.Addr,
		"data": p.Data,
		"type": pt.String(),
	}).Debug("incoming gateway packet")

	return nil
}
