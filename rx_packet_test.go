package loraserver

import (
	"errors"
	"testing"
	"time"

	"github.com/brocaar/loracontrol"
	"github.com/brocaar/lorawan"
	. "github.com/smartystreets/goconvey/convey"
)

type testApplicationBackend struct {
	callCount int
}

func (b *testApplicationBackend) Send(appEUI lorawan.EUI64, rxPackets loracontrol.RXPackets) error {
	b.callCount = b.callCount + 1
	return nil
}

func (b *testApplicationBackend) SetClient(c *loracontrol.Client) {}

func TestHandleGatewayPackets(t *testing.T) {
	config := getConfig()

	Convey("Given a Client connected to a clean Redis database", t, func() {
		appBackend := &testApplicationBackend{}
		client, err := loracontrol.NewClient(
			loracontrol.SetStorageBackend(loracontrol.NewRedisBackend(config.RedisServer, config.RedisPassword)),
			loracontrol.SetApplicationBackend(appBackend),
			loracontrol.SetGatewayBackend(&loracontrol.DummyGatewayBackend{}),
		)
		So(err, ShouldBeNil)
		So(client.Storage().FlushAll(), ShouldBeNil)

		nwkSKey := lorawan.AES128Key{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
		appSKey := lorawan.AES128Key{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}
		devAddr := lorawan.DevAddr{1, 1, 1, 1}
		appEUI := lorawan.EUI64{4, 4, 4, 4, 4, 4, 4, 4}

		Convey("Given a test RXPackets", func() {
			macPL := lorawan.NewMACPayload(true)
			macPL.FHDR = lorawan.FHDR{
				DevAddr: devAddr,
				FCtrl:   lorawan.FCtrl{},
				FCnt:    10,
			}
			macPL.FPort = 1
			macPL.FRMPayload = []lorawan.Payload{
				&lorawan.DataPayload{Bytes: []byte("abc123")},
			}
			So(macPL.EncryptFRMPayload(appSKey), ShouldBeNil)

			phy := lorawan.NewPHYPayload(true)
			phy.MHDR = lorawan.MHDR{
				MType: lorawan.UnconfirmedDataUp,
				Major: lorawan.LoRaWANR1,
			}
			phy.MACPayload = macPL
			So(phy.SetMIC(nwkSKey), ShouldBeNil)

			rxPackets := loracontrol.RXPackets{
				{
					RXInfo: loracontrol.RXInfo{
						MAC:        lorawan.EUI64{1, 2, 3, 4, 5, 6, 7, 8},
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

			Convey("When calling handleCollectedPackets", func() {
				err := handleCollectedPackets(rxPackets, client)
				Convey("Then an error is returned that the node does not exists", func() {
					So(err, ShouldResemble, errors.New("could not find node in the database"))
				})
			})

			Convey("Given the node is in the database", func() {
				node := &loracontrol.Node{
					DevAddr: devAddr,
					NwkSKey: nwkSKey,
					FCntUp:  9,
					AppEUI:  appEUI,
				}
				So(client.Node().Create(node), ShouldBeNil)

				Convey("Given the application is in the database", func() {
					app := &loracontrol.Application{
						AppEUI: node.AppEUI,
					}
					So(client.Application().Create(app), ShouldBeNil)

					Convey("Then handleCollectedPackets does not return an error", func() {
						err := handleCollectedPackets(rxPackets, client)
						So(err, ShouldBeNil)

						Convey("Then the app backend Send was called once", func() {
							So(appBackend.callCount, ShouldEqual, 1)
						})
					})

					Convey("When calling HandleGatewayPackets", func() {
						pkChan := make(chan loracontrol.RXPacket, 1)
						pkChan <- rxPackets[0]
						close(pkChan)

						HandleGatewayPackets(pkChan, client)

						Convey("Then the packet has been sent by the app backend", func() {
							time.Sleep(time.Millisecond * 200)
							So(appBackend.callCount, ShouldEqual, 1)
						})
					})

					Convey("When the FCnt is invalid", func() {
						node.FCntUp = 10
						So(client.Node().Update(node), ShouldBeNil)

						Convey("Then handleCollectedPackets returns an invalid FCnt error", func() {
							err := handleCollectedPackets(rxPackets, client)
							So(err, ShouldResemble, errors.New("invalid FCnt or too many dropped frames"))
						})
					})

					Convey("When the NwkSKey is invalid", func() {
						node.NwkSKey[0] = 0
						So(client.Node().Update(node), ShouldBeNil)

						Convey("Then handleCollectedPackets returns an invalid MIC error", func() {
							err := handleCollectedPackets(rxPackets, client)
							So(err, ShouldResemble, errors.New("invalid MIC"))
						})
					})
				})
			})
		})
	})
}
