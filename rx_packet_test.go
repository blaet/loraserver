package loraserver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/brocaar/loracontrol"
	"github.com/brocaar/lorawan"
	. "github.com/smartystreets/goconvey/convey"
)

type testApplicationHandler struct {
	callCount int
}

func (h *testApplicationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.callCount = h.callCount + 1
	w.WriteHeader(http.StatusOK)
}

func TestHandleRXPacketsData(t *testing.T) {
	config := getConfig()

	Convey("Given a packet with data 'abc123' and a Client connected to Redis", t, func() {
		client, err := loracontrol.NewClient(
			loracontrol.SetRedisBackend(config.RedisServer, config.RedisPassword),
			loracontrol.SetHTTPApplicationBackend(),
		)
		So(err, ShouldBeNil)
		So(client.Storage().FlushAll(), ShouldBeNil)

		nwkSKey := [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
		appSKey := [16]byte{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}
		devAddr := [4]byte{1, 1, 1, 1}

		macPl := lorawan.NewMACPayload(true)
		macPl.FHDR = lorawan.FHDR{
			DevAddr: lorawan.DevAddr(devAddr),
			FCtrl:   lorawan.FCtrl{},
			FCnt:    10,
		}
		macPl.FPort = 1
		macPl.FRMPayload = []lorawan.Payload{
			&lorawan.DataPayload{Bytes: []byte("abc123")},
		}
		So(macPl.EncryptFRMPayload(appSKey), ShouldBeNil)

		phy := lorawan.NewPHYPayload(true)
		phy.MHDR = lorawan.MHDR{
			MType: lorawan.UnconfirmedDataUp,
			Major: lorawan.LoRaWANR1,
		}
		phy.MACPayload = macPl
		So(phy.SetMIC(nwkSKey), ShouldBeNil)

		packets := loracontrol.RXPackets{
			{
				RXInfo: loracontrol.RXInfo{
					MAC:        [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
					Time:       time.Now().UTC(),
					Timestamp:  708016819,
					Frequency:  868.5,
					Channel:    2,
					RFChain:    1,
					CRCStatus:  1,
					Modulation: "LORA",
					DataRate:   loracontrol.DataRate{LoRa: "SF7BW125"},
					CodingRate: "4/5",
					RSSI:       -51,
					LoRaSNR:    7,
					Size:       16,
				},
				PHYPayload: phy,
			},
		}

		Convey("When calling handleRXPackets", func() {
			err := handleRXPackets(packets, client)
			Convey("Then it returns an error about missing node", func() {
				So(err, ShouldResemble, errors.New("could not find node in database"))
			})

			Convey("Given a node in the database with correct FCntUp and correct NwkSKey", func() {
				node := &loracontrol.Node{
					DevAddr: [4]byte{1, 1, 1, 1},
					NwkSKey: nwkSKey,
					FCntUp:  9,
					AppEUI:  [8]byte{2, 2, 2, 2, 2, 2, 2, 2},
				}
				So(client.Node().Create(node), ShouldBeNil)

				Convey("Given an application handler is registered", func() {
					handler := &testApplicationHandler{}
					s := httptest.NewServer(handler)
					app := &loracontrol.Application{
						AppEUI:      node.AppEUI,
						CallbackURL: s.URL,
					}
					So(client.Application().Create(app), ShouldBeNil)
					So(handler.callCount, ShouldEqual, 0)

					Convey("Then handleRXPackets does not return an error", func() {
						So(handleRXPackets(packets, client), ShouldBeNil)

						Convey("Then the application handler is called once", func() {
							So(handler.callCount, ShouldEqual, 1)
						})
					})
				})
			})

			Convey("Given a node in the database with incorrect FCntUp and correct NwkSkey", func() {
				node := &loracontrol.Node{
					DevAddr: [4]byte{1, 1, 1, 1},
					NwkSKey: nwkSKey,
					FCntUp:  10,
				}
				So(client.Node().Create(node), ShouldBeNil)

				Convey("Then handleRXPackets returns an invalid FCnt error", func() {
					err := handleRXPackets(packets, client)
					So(err, ShouldResemble, errors.New("the FCnt is invalid or too many dropped frames"))
				})
			})

			Convey("Given a node in the database with correct FCntUP and incorrect NwkSkey", func() {
				node := &loracontrol.Node{
					DevAddr: [4]byte{1, 1, 1, 1},
					NwkSKey: [16]byte{1, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
					FCntUp:  9,
				}
				So(client.Node().Create(node), ShouldBeNil)

				Convey("Then handleRXPackets returns an MIC invalid error", func() {
					err := handleRXPackets(packets, client)
					So(err, ShouldResemble, errors.New("MIC is invalid"))
				})
			})
		})
	})
}
