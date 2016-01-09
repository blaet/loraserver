package loraserver

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"

	log "github.com/Sirupsen/logrus"
	"github.com/brocaar/loracontrol"
	"github.com/brocaar/lorawan/semtech"
)

// UDPPacket contains the UDP packet data and addr.
type UDPPacket struct {
	Data []byte
	Addr *net.UDPAddr
}

// SendGatewayPackets sends packets from the sendChan to the given UDP connection.
func SendGatewayPackets(c *net.UDPConn, sendChan chan UDPPacket) error {
	for p := range sendChan {
		pt, err := semtech.GetPacketType(p.Data)
		if err != nil {
			return err
		}

		log.WithFields(log.Fields{
			"addr": p.Addr,
			"type": pt.String(),
		}).Info("outgoing gateway packet")

		_, err = c.WriteToUDP(p.Data, p.Addr)
		if err != nil {
			return err
		}
	}
	return nil
}

// ReadGatewayPackets reads incoming gateway packets from the given UDPConn.
// Only UDP related errors are returned. Errors returned by processing the
// packet are logged as errors.
func ReadGatewayPackets(c *net.UDPConn, sendChan chan UDPPacket, client *loracontrol.Client) error {
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
			if err := HandleGatewayPacket(p, sendChan, client); err != nil {
				log.WithFields(log.Fields{
					"udp_data_base64": base64.StdEncoding.EncodeToString(p.Data),
					"addr":            p.Addr,
				}).Errorf("could not handle packet: %s", err)
			}
		}()
	}
}

// HandleGatewayPacket handles a single gateway packet.
func HandleGatewayPacket(p UDPPacket, sendChan chan UDPPacket, client *loracontrol.Client) error {
	pt, err := semtech.GetPacketType(p.Data)
	if err != nil {
		log.WithFields(log.Fields{
			"addr": p.Addr,
			"data": p.Data,
		}).Errorf("could get packet type: %s", err)
		return err
	}

	switch pt {
	case semtech.PushData:
		log.WithFields(log.Fields{
			"addr": p.Addr,
			"type": pt.String(),
		}).Info("incoming gateway packet")
		return HandleGatewayPushData(p, sendChan, client)
	case semtech.PullData:
		log.WithFields(log.Fields{
			"addr": p.Addr,
			"type": pt.String(),
		}).Info("incoming gateway packet")
		return HandleGatewayPullData(p, sendChan)
	default:
		return fmt.Errorf("unknown packet type: %s", pt)
	}
}

// HandleGatewayPullData handles PullData packets and returns them with
// a PullACK response.
func HandleGatewayPullData(p UDPPacket, sendChan chan UDPPacket) error {
	packet := semtech.PullDataPacket{}
	if err := packet.UnmarshalBinary(p.Data); err != nil {
		return err
	}

	ack := semtech.PullACKPacket{
		ProtocolVersion: 1,
		RandomToken:     packet.RandomToken,
	}
	bytes, err := ack.MarshalBinary()
	if err != nil {
		return err
	}
	sendChan <- UDPPacket{
		Addr: p.Addr,
		Data: bytes,
	}
	return nil
}

// HandleGatewayPushData handles PushData packets (node packets and / or
// gateway stats) and returns them with a PushACK response.
func HandleGatewayPushData(p UDPPacket, sendChan chan UDPPacket, client *loracontrol.Client) error {
	packet := semtech.PushDataPacket{}
	if err := packet.UnmarshalBinary(p.Data); err != nil {
		return err
	}

	// ack the packet
	ack := semtech.PushACKPacket{
		ProtocolVersion: 1,
		RandomToken:     packet.RandomToken,
	}
	bytes, err := ack.MarshalBinary()
	if err != nil {
		return err
	}
	sendChan <- UDPPacket{
		Addr: p.Addr,
		Data: bytes,
	}

	// store the gateway stats
	if packet.Payload.Stat != nil {
		if err := updateGatewayStat(p.Addr, packet.GatewayMAC, packet.Payload.Stat, client); err != nil {
			return err
		}
	}

	// collect received packets
	for _, rxpk := range packet.Payload.RXPK {
		if err := collectGatewayRXPacket(p.Addr, packet.GatewayMAC, &rxpk, client); err != nil {
			return err
		}
	}

	return nil
}

// updateGatewayStat updates the gateway stats.
func updateGatewayStat(addr *net.UDPAddr, mac [8]byte, stat *semtech.Stat, client *loracontrol.Client) error {
	log.WithFields(log.Fields{
		"addr": addr,
		"mac":  mac,
	}).Info("storing gateway stats")
	gw := loracontrol.NewGatewayFromSemtech(addr, mac, stat)
	return client.Gateway().Upsert(gw)
}

// collectGatewayRXPacket collects the incoming semtech.RXPK packet.
func collectGatewayRXPacket(addr *net.UDPAddr, mac [8]byte, rxpk *semtech.RXPK, client *loracontrol.Client) error {
	logFields := log.Fields{
		"addr": addr,
		"mac":  mac,
		"data": rxpk.Data,
	}
	log.WithFields(logFields).Info("handling received node data")

	// decode packet
	rxPacket, err := loracontrol.NewRXPacketFromSemtech(mac, rxpk)
	if err != nil {
		return err
	}

	// check CRC
	if rxPacket.RXInfo.CRCStatus != 1 {
		log.WithFields(logFields).Warningf("packet CRC is invalid: %d", rxPacket.RXInfo.CRCStatus)
		return errors.New("invalid CRC")
	}

	return client.Packet().CollectAndCallOnce(rxPacket, func(packets loracontrol.RXPackets) error {
		return handleRXPackets(packets, client)
	})
}
