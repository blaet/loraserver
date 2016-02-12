// Package http implements a HTTP application backend.
package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	h "net/http"
	"time"

	"github.com/brocaar/loracontrol"
	"github.com/brocaar/lorawan"
)

// RXPayload is the payload sent to the application backend.
type RXPayload struct {
	TimeReceived time.Time `json:"timeReceived"`
	GatewayCount int       `json:"gatewayCount"`
	Port         int       `json:"port"`
	Payload      []byte    `json:"payload"`
}

// Backend implements a HTTP application backend.
// It expects that the "callbackURL" config string is set to the url
// to which it should send the payload for the application. E.g.
// 		loracontrol.Application{
//			...
//			Config: loracontrol.PropertyBag{
//				String: map[string]string{
//					"callbackURL": "http://example.com/handler",
//				},
//			},
//		}
type Backend struct {
	client       *loracontrol.Client
	txPacketChan chan loracontrol.TXPacket
}

// NewBackend creates a new Backend.
func NewBackend() *Backend {
	return &Backend{
		txPacketChan: make(chan loracontrol.TXPacket),
	}
}

// SetClient sets the loracontrol.Client and is automatically called by
// loracontrol.SetApplicationBackend.
func (b *Backend) SetClient(c *loracontrol.Client) {
	b.client = c
}

// Close closes the application backend.
func (b *Backend) Close() error {
	return nil
}

// Send sends the given packets as one packet to the application handler.
func (b *Backend) Send(appEUI lorawan.EUI64, packets loracontrol.RXPackets) error {
	app, err := b.client.Application().Get(appEUI)
	if err != nil {
		return err
	}
	callbackURL, ok := app.Config.String["callbackURL"]
	if !ok {
		return errors.New("application/http: application config does not contain callbackURL")
	}
	if len(packets) == 0 {
		return errors.New("application/http: packets should have length > 0")
	}

	macPL, ok := packets[0].PHYPayload.MACPayload.(*lorawan.MACPayload)
	if !ok {
		return fmt.Errorf("application/http: expected *lorawan.MACPayload, got %T", packets[0].PHYPayload.MACPayload)
	}

	if len(macPL.FRMPayload) != 1 {
		return errors.New("application/http: expected exactly 1 FRMPayload")
	}

	data, err := macPL.FRMPayload[0].MarshalBinary()
	if err != nil {
		return err
	}

	pl := RXPayload{
		TimeReceived: packets[0].RXInfo.Time,
		GatewayCount: len(packets),
		Port:         int(macPL.FPort),
		Payload:      data,
	}
	data, err = json.Marshal(&pl)
	if err != nil {
		return err
	}

	resp, err := h.Post(callbackURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != h.StatusOK && resp.StatusCode != h.StatusCreated {
		return fmt.Errorf("application/http: expected 200 or 201 response code, got: %d", resp.StatusCode)
	}
	return nil
}

// Receive returns the channel with received packets from the application.
func (b *Backend) Receive() chan loracontrol.TXPacket {
	return b.txPacketChan
}
