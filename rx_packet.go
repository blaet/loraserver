package loraserver

import (
	"errors"
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/brocaar/loracontrol"
	"github.com/brocaar/lorawan"
)

// HandleGatewayPackets handles the the packets received by the gateway, each
// in a separate goroutine. Errors are logged.
func HandleGatewayPackets(c *loracontrol.Client) {
	for rxPacket := range c.Gateway().Receive() {
		go func(rxPacket loracontrol.RXPacket) {
			if err := handleGatewayPacket(rxPacket, c); err != nil {
				log.Errorf("error processing packet: %s", err)
			}
		}(rxPacket)
	}
}

// handleGatewayPacket first validates the correctness of the packet (FCnt, MIC),
// it will decrypt the payload and it will call CollectAndCallOnce (to collect
// packets received by other gateways before proceeding).
func handleGatewayPacket(rxPacket loracontrol.RXPacket, client *loracontrol.Client) error {
	switch rxPacket.PHYPayload.MHDR.MType {
	case lorawan.JoinRequest:
		return errors.New("TODO: implement JoinRequest packet handling")
	case lorawan.UnconfirmedDataUp, lorawan.ConfirmedDataUp:
		// MACPayload must be of type *lorawan.MACPayload
		macPL, ok := rxPacket.PHYPayload.MACPayload.(*lorawan.MACPayload)
		if !ok {
			return fmt.Errorf("expected *lorawan.MACPayload, got %T", rxPacket.PHYPayload.MACPayload)
		}

		// get the node-session data
		nodeSession, err := client.NodeSession().Get(macPL.FHDR.DevAddr)
		if err != nil {
			if err == loracontrol.ErrObjectDoesNotExist {
				return errors.New("node-session does not exist")
			}
			return err
		}

		// validate and get the full int32 FCnt
		fullFCnt, ok := nodeSession.ValidateAndGetFullFCntUp(macPL.FHDR.FCnt)
		if !ok {
			log.WithFields(log.Fields{
				"packet_fcnt": macPL.FHDR.FCnt,
				"server_fcnt": nodeSession.FCntUp,
			}).Warning("invalid FCnt")
			return errors.New("invalid FCnt or too many dropped frames")
		}
		macPL.FHDR.FCnt = fullFCnt

		// validate MIC
		micOK, err := rxPacket.PHYPayload.ValidateMIC(nodeSession.NwkSKey)
		if err != nil {
			return err
		}
		if !micOK {
			return errors.New("invalid MIC")
		}

		if macPL.FPort == 0 {
			// decrypt FRMPayload with NwkSKey when FPort == 0
			if err := macPL.DecryptFRMPayload(nodeSession.NwkSKey); err != nil {
				return err
			}
		} else {
			// decrypt FRMPayload with AppSKey
			if err := macPL.DecryptFRMPayload(nodeSession.AppSKey); err != nil {
				return err
			}
			rxPacket.PHYPayload.MACPayload = macPL
		}
	default:
		log.WithField("mtype", rxPacket.PHYPayload.MHDR.MType).Warning("unknown MType received")
		return errors.New("unknown MType")
	}

	return client.Packet().CollectAndCallOnce(rxPacket, func(packets loracontrol.RXPackets) error {
		return handleCollectedPackets(packets, client)
	})
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
		log.Info("=====  UNCONFIRMED DATA UP =====")
		return handleRXDataPacket(rxPackets, c)
	case lorawan.ConfirmedDataUp:
		log.Info("=====  CONFIRMED DATA UP =====")
		return handleRXDataPacket(rxPackets, c)
	default:
		log.WithField("mtype", rxPackets[0].PHYPayload.MHDR.MType).Warning("unknown MType received")
		return errors.New("unknown MType")
	}
	return nil
}

func handleRXDataPacket(rxPackets loracontrol.RXPackets, client *loracontrol.Client) error {
	if len(rxPackets) == 0 {
		return errors.New("at least 1 RXPacket must be given")
	}
	rxPacket := rxPackets[0]

	// the MACPayload should be of type *lorawan.MACPayload
	macPL, ok := rxPacket.PHYPayload.MACPayload.(*lorawan.MACPayload)
	if !ok {
		return fmt.Errorf("expected *lorawan.MACPayload, got %T", rxPacket.PHYPayload.MACPayload)
	}

	// get the node-session data from the database
	nodeSession, err := client.NodeSession().Get(macPL.FHDR.DevAddr)
	if err != nil {
		if err == loracontrol.ErrObjectDoesNotExist {
			log.Error("Nodesession not found for DevAddr: ", macPL.FHDR.DevAddr)
		}
		return err
	}

	log.Info("= NodeSession found for devAddr: ", macPL.FHDR.DevAddr)

	// get the node data from the database
	node, err := client.Node().Get(nodeSession.DevEUI)
	if err != nil {
		if err == loracontrol.ErrObjectDoesNotExist {
			log.Error("Node not found for devEUI: ", nodeSession.DevEUI)
		}
		return err
	}

	log.Info("== Node found for devEUI: ", nodeSession.DevEUI)

	// decrypt FRMPayload with NwkSKey when FPort == 0
	if macPL.FPort == 0 {
		// TODO: handle MAC commands
		log.Warning("TODO: implement MAC commands in FRMPayload")
		return nil
	}

	// send the data to the application
	if err := client.Application().Send(node.AppEUI, rxPackets); err != nil {
		if err == loracontrol.ErrObjectDoesNotExist {
			return errors.New("AppEUI does not exist")
		}
		return err
	}

	log.Info("=== Data has been sent to application")

	// increment counter
	nodeSession.FCntUp = nodeSession.FCntUp + 1
	if err := client.NodeSession().UpdateExpire(nodeSession); err != nil {
		if err == loracontrol.ErrObjectDoesNotExist {
			return errors.New("node-session does not exist")
		}
		return err
	}

	log.Info("==== FCntUp counter incremented")

	return nil
}
