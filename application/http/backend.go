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
	client *loracontrol.Client
}

// RXPayload is the payload sent to the application backend.
type RXPayload struct {
	Time         time.Time `json:"time"`
	GatewayCount int       `json:"gatewayCount"`
	PHYPayload   []byte    `json:"phyPayload"`
}

// SetClient sets the loracontrol.Client and is automatically called by
// loracontrol.SetApplicationBackend.
func (b *Backend) SetClient(c *loracontrol.Client) {
	b.client = c
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
	data, err := packets[0].PHYPayload.MarshalBinary()
	if err != nil {
		return err
	}

	pl := RXPayload{
		Time:         packets[0].RXInfo.Time,
		GatewayCount: len(packets),
		PHYPayload:   data,
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
