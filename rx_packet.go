package loraserver

import (
	"errors"
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/brocaar/loracontrol"
	"github.com/brocaar/lorawan"
)

// HandleGatewayPackets handles the the packets received by the gateway.
func HandleGatewayPackets(rxPacketChan chan loracontrol.RXPacket, c *loracontrol.Client) {
	for rxPacket := range rxPacketChan {
		go func(rxPacket loracontrol.RXPacket) {
			err := c.Packet().CollectAndCallOnce(&rxPacket, func(packets loracontrol.RXPackets) error {
				return handleCollectedPackets(packets, c)
			})
			if err != nil {
				log.Errorf("error processing packet: %s", err)
			}
		}(rxPacket)
	}
}

func handleCollectedPackets(rxPackets loracontrol.RXPackets, c *loracontrol.Client) error {
	if len(rxPackets) == 0 {
		return errors.New("packet collector returned 0 packets")
	}

	var macs []string
	for _, p := range rxPackets {
		macs = append(macs, p.RXInfo.MAC.String())
	}

	log.WithFields(log.Fields{
		"gw_count": len(rxPackets),
		"gw_macs":  strings.Join(macs, ", "),
		"mtype":    rxPackets[0].PHYPayload.MHDR.MType,
	}).Info("packet(s) collected")

	switch rxPackets[0].PHYPayload.MHDR.MType {
	case lorawan.JoinRequest:
		log.Debug("join request")
	case lorawan.UnconfirmedDataUp:
		return handleRXDataPacket(rxPackets, c)
	case lorawan.ConfirmedDataUp:
		log.Debug("confirmed data up")
	default:
		log.WithField("mtype", rxPackets[0].PHYPayload.MHDR.MType).Warning("unknown MType received")
		return errors.New("unknown MType")
	}
	return nil
}

func handleRXDataPacket(rxPackets loracontrol.RXPackets, client *loracontrol.Client) error {
	if len(rxPackets) == 0 {
		return errors.New("at least 1 RXPacket should be given")
	}
	rxPacket := rxPackets[0]

	// the MACPayload should be of type *lorawan.MACPayload
	macPL, ok := rxPacket.PHYPayload.MACPayload.(*lorawan.MACPayload)
	if !ok {
		return fmt.Errorf("expected *lorawan.MACPayload, got %T", rxPacket.PHYPayload.MACPayload)
	}

	// get the node data from the database
	node, err := client.Node().Get(macPL.FHDR.DevAddr)
	if err != nil {
		if err == loracontrol.ErrObjectDoesNotExist {
			return errors.New("could not find node in the database")
		}
		return err
	}

	// validate the FCnt
	if !node.ValidateAndSetFCntUP(macPL.FHDR.FCnt) {
		log.WithFields(log.Fields{
			"packet_fcnt": macPL.FHDR.FCnt,
			"server_fcnt": node.FCntUp,
		}).Warning("invalid FCnt")
		return errors.New("invalid FCnt or too many dropped frames")
	}

	// validate MIC
	micOK, err := rxPacket.PHYPayload.ValidateMIC(node.NwkSKey)
	if err != nil {
		return err
	}
	if !micOK {
		return errors.New("invalid MIC")
	}

	// decrypt FRMPayload with NwkSKey when FPort == 0
	if macPL.FPort == 0 {
		if err := macPL.DecryptFRMPayload(node.NwkSKey); err != nil {
			return err
		}

		// TODO: handle MAC commands
		log.Warning("TODO: implement MAC commands in FRMPayload")
		return nil
	}

	// send the data to the application
	err = client.Application().Send(node.AppEUI, rxPackets)
	if err == loracontrol.ErrObjectDoesNotExist {
		return errors.New("AppEUI does not exist")
	}
	return err
}
