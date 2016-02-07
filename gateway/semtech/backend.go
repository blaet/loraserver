package semtech

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/brocaar/loracontrol"
	"github.com/brocaar/lorawan"
)

type udpPacket struct {
	data []byte
	addr *net.UDPAddr
}

// NewBackend creates a new Backend.
func NewBackend(port int) (loracontrol.GatewayBackend, error) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return nil, err
	}
	log.WithField("addr", addr).Info("starting gateway udp listener")
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}

	b := &Backend{
		conn:     conn,
		rxChan:   make(chan loracontrol.RXPacket),
		sendChan: make(chan udpPacket),
	}

	b.wg.Add(2)

	go func() {
		err := b.readPackets()
		if !b.closed {
			log.Fatal(err)
		}
		b.wg.Done()
	}()
	go func() {
		err := b.sendPackets()
		if !b.closed {
			log.Fatal(err)
		}
		b.wg.Done()
	}()

	return b, nil
}

// Backend implements a Semtech backend.
type Backend struct {
	client   *loracontrol.Client
	conn     *net.UDPConn
	rxChan   chan loracontrol.RXPacket
	sendChan chan udpPacket
	closed   bool
	wg       sync.WaitGroup
}

// SetClient sets the loracontrol.Client and is automatically called by
// loracontrol.SetGatewayBackend.
func (b *Backend) SetClient(c *loracontrol.Client) {
	b.client = c
}

// Close closes the backend.
func (b *Backend) Close() error {
	b.closed = true
	close(b.sendChan)
	if err := b.conn.Close(); err != nil {
		return err
	}
	b.wg.Wait()
	return nil
}

// Receive returns the RXPacket channel.
func (b *Backend) Receive() chan loracontrol.RXPacket {
	return b.rxChan
}

// Send sends the given TXPacket to the gateway.
func (b *Backend) Send(txPacket loracontrol.TXPacket) error {
	gw, err := b.client.Gateway().Get(txPacket.TXInfo.MAC)
	if err != nil {
		if err == loracontrol.ErrObjectDoesNotExist {
			return errors.New("gateway/semtech: gateway does not exist")
		}
		return err
	}

	addrStr, ok := gw.Config.String["udp_addr"]
	if !ok {
		return errors.New("gateway/semtech: gateway does not have udp_addr config string")
	}

	addr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		return err
	}

	txpk, err := newTXPKFromTXPacket(txPacket)
	if err != nil {
		return err
	}

	pullResp := PullRespPacket{
		Payload: PullRespPayload{
			TXPK: txpk,
		},
	}

	data, err := pullResp.MarshalBinary()
	if err != nil {
		return err
	}

	b.sendChan <- udpPacket{
		data: data,
		addr: addr,
	}
	return nil
}

func (b *Backend) readPackets() error {
	buf := make([]byte, 65507) // max udp data size
	for {
		i, addr, err := b.conn.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		data := make([]byte, i)
		copy(data, buf[:i])
		go func() {
			if err := b.handlePacket(addr, data); err != nil {
				log.WithFields(log.Fields{
					"udp_data_base64": base64.StdEncoding.EncodeToString(data),
					"addr":            addr,
				}).Errorf("could not handle packet: %s", err)
			}
		}()
	}
}

func (b *Backend) sendPackets() error {
	for p := range b.sendChan {
		pt, err := GetPacketType(p.data)
		if err != nil {
			return err
		}
		log.WithFields(log.Fields{
			"addr": p.addr,
			"type": pt,
		}).Info("outgoing gateway packet")

		if _, err = b.conn.WriteToUDP(p.data, p.addr); err != nil {
			return err
		}
	}
	return nil
}

func (b *Backend) handlePacket(addr *net.UDPAddr, data []byte) error {
	pt, err := GetPacketType(data)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"addr": addr,
		"type": pt,
	}).Info("incoming gateway packet")

	switch pt {
	case PushData:
		return b.handlePushData(addr, data)
	case PullData:
		return b.handlePullData(addr, data)
	default:
		return fmt.Errorf("unknown packet type: %s", pt)
	}
}

func (b *Backend) handlePullData(addr *net.UDPAddr, data []byte) error {
	p := PullDataPacket{}
	if err := p.UnmarshalBinary(data); err != nil {
		return err
	}
	ack := PullACKPacket{
		RandomToken: p.RandomToken,
	}
	bytes, err := ack.MarshalBinary()
	if err != nil {
		return err
	}
	b.sendChan <- udpPacket{
		addr: addr,
		data: bytes,
	}
	return nil
}

func (b *Backend) handlePushData(addr *net.UDPAddr, data []byte) error {
	p := PushDataPacket{}
	if err := p.UnmarshalBinary(data); err != nil {
		return err
	}

	// ack the packet
	ack := PushACKPacket{
		RandomToken: p.RandomToken,
	}
	bytes, err := ack.MarshalBinary()
	if err != nil {
		return err
	}
	b.sendChan <- udpPacket{
		addr: addr,
		data: bytes,
	}

	// store gateway stats
	if p.Payload.Stat != nil {
		if err := b.updateStat(addr, p.GatewayMAC, p.Payload.Stat); err != nil {
			return err
		}
	}

	// collect rx packets
	for _, rxpk := range p.Payload.RXPK {
		if err := b.collectRXPacket(addr, p.GatewayMAC, &rxpk); err != nil {
			return err
		}
	}

	return nil
}

func (b *Backend) updateStat(addr *net.UDPAddr, mac lorawan.EUI64, stat *Stat) error {
	log.WithFields(log.Fields{
		"addr": addr,
		"mac":  mac,
	}).Info("storing gateway stats")
	gw := newGatewayFromSemtech(addr, mac, stat)
	return b.client.Gateway().Upsert(gw)
}

func (b *Backend) collectRXPacket(addr *net.UDPAddr, mac lorawan.EUI64, rxpk *RXPK) error {
	logFields := log.Fields{
		"addr": addr,
		"mac":  mac,
		"data": rxpk.Data,
	}
	log.WithFields(logFields).Info("handling received node data")

	// decode packet
	rxPacket, err := newRXPacketFromSemtech(mac, rxpk)
	if err != nil {
		return err
	}

	// check CRC
	if rxPacket.RXInfo.CRCStatus != 1 {
		log.WithFields(logFields).Warningf("invalid packet CRC: %d", rxPacket.RXInfo.CRCStatus)
		return errors.New("invalid CRC")
	}

	b.rxChan <- *rxPacket
	return nil
}

func newGatewayFromSemtech(addr *net.UDPAddr, mac lorawan.EUI64, stat *Stat) loracontrol.Gateway {
	return loracontrol.Gateway{
		UpdatedAt:                   time.Time(stat.Time),
		MAC:                         mac,
		Latitude:                    stat.Lati,
		Longitude:                   stat.Long,
		Altitude:                    int(stat.Alti),
		UpstreamPacketsReceived:     uint(stat.RXNb),
		UpstreamPacketsReceivedOK:   uint(stat.RXOK),
		UpstreamPacketsForwarded:    uint(stat.RXFW),
		UpstreamDatagramsACKRate:    stat.ACKR,
		DownstreamDatagramsReceived: uint(stat.DWNb),
		Config: loracontrol.PropertyBag{
			String: map[string]string{
				"udp_addr": addr.String(),
			},
		},
	}
}

func newRXPacketFromSemtech(mac lorawan.EUI64, rxpk *RXPK) (*loracontrol.RXPacket, error) {
	// this is always an uplink payload
	phy := lorawan.NewPHYPayload(true)
	bytes, err := base64.StdEncoding.DecodeString(rxpk.Data)
	if err != nil {
		return nil, fmt.Errorf("semtech: could not base64 decode data: %s", err)
	}
	if err := phy.UnmarshalBinary(bytes); err != nil {
		return nil, fmt.Errorf("semtech: could not unmarshal PHYPayload: %s", err)
	}

	rxPacket := &loracontrol.RXPacket{
		PHYPayload: phy,
		RXInfo: loracontrol.RXInfo{
			MAC:        mac,
			Time:       time.Time(rxpk.Time),
			Timestamp:  rxpk.Tmst,
			Frequency:  rxpk.Freq,
			Channel:    uint(rxpk.Chan),
			RFChain:    uint(rxpk.RFCh),
			CRCStatus:  int(rxpk.Stat),
			Modulation: rxpk.Modu,
			DataRate: loracontrol.DataRate{
				LoRa: rxpk.DatR.LoRa,
				FSK:  uint(rxpk.DatR.FSK),
			},
			CodingRate: rxpk.CodR,
			RSSI:       int(rxpk.RSSI),
			LoRaSNR:    rxpk.LSNR,
			Size:       uint(rxpk.Size),
		},
	}
	return rxPacket, nil
}

func newTXPKFromTXPacket(txPacket loracontrol.TXPacket) (TXPK, error) {
	b, err := txPacket.PHYPayload.MarshalBinary()
	if err != nil {
		return TXPK{}, err
	}

	txpk := TXPK{
		Imme: txPacket.TXInfo.Immediately,
		Tmst: txPacket.TXInfo.Timestamp,
		Freq: txPacket.TXInfo.Frequency,
		RFCh: uint8(txPacket.TXInfo.RFChain),
		Powe: uint8(txPacket.TXInfo.Power),
		Modu: txPacket.TXInfo.DataRate.Modulation(),
		DatR: DatR{
			LoRa: txPacket.TXInfo.DataRate.LoRa,
			FSK:  uint32(txPacket.TXInfo.DataRate.FSK),
		},
		CodR: txPacket.TXInfo.CodeRate,
		FDev: uint16(txPacket.TXInfo.FrequencyDeviation),
		Size: uint16(len(b)),
		NCRC: txPacket.TXInfo.DisableCRC,
		Data: base64.RawStdEncoding.EncodeToString(b),
	}

	if txPacket.TXInfo.DataRate.Modulation() == "LORA" {
		txpk.IPol = true
	}

	return txpk, nil
}
