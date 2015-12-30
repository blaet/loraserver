package loraserver

import (
	"errors"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/brocaar/loracontrol"
	"github.com/brocaar/lorawan"
)

// handleRXPackets handles the given slice of packets. Note that all all
// packets in the slice are the same, but are received by different gateways.
func handleRXPackets(packets loracontrol.RXPackets, client *loracontrol.Client) error {
	// there is not much to do when there are no packets
	if len(packets) == 0 {
		return errors.New("packet collector returned 0 packets")
	}

	var macs [][8]byte
	for _, p := range packets {
		macs = append(macs, p.RXInfo.MAC)
	}
	log.WithFields(log.Fields{
		"gw_count": len(packets),
		"type":     packets[0].PHYPayload.MHDR.MType,
	}).Info("packet(s) collected")

	// the received slice of packets is sorted by CollectAndCallOnce.
	switch packets[0].PHYPayload.MHDR.MType {
	case lorawan.JoinRequest:
		log.Debug("join request")
	case lorawan.UnconfirmedDataUp:
		return handleRXDataPacket(packets, false, client)
	case lorawan.ConfirmedDataUp:
		log.Debug("confirmed data up")
	default:
		log.WithFields(log.Fields{
			"mtype": packets[0].PHYPayload.MHDR.MType,
		}).Warning("unknown MType received")
		return errors.New("unknown MType")
	}

	return nil
}

func handleRXDataPacket(packets loracontrol.RXPackets, confirmed bool, client *loracontrol.Client) error {
	// the MACPayload should be of type *lorawan.MACPayload
	macPL, ok := packets[0].PHYPayload.MACPayload.(*lorawan.MACPayload)
	if !ok {
		return fmt.Errorf("expected *lorawan.MACPayload, got %T instead", packets[0].PHYPayload.MACPayload)
	}

	// get the node data from the database
	node, err := client.Node().Get([4]byte(macPL.FHDR.DevAddr))
	if err != nil {
		if err == loracontrol.ErrObjectDoesNotExist {
			return errors.New("could not find node in database")
		}
		return err
	}

	// validate the FCnt
	if !node.ValidateAndSetFCntUP(macPL.FHDR.FCnt) {
		return errors.New("the FCnt is invalid or too many dropped frames")
	}

	// decrypt the FRMPayload with the NwkSKey when FPort != 0
	if macPL.FPort == 0 {
		if err := macPL.DecryptFRMPayload(node.NwkSKey[:]); err != nil {
			return err
		}
	}

	return nil
}
