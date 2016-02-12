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
	err       error
}

func (b *testApplicationBackend) Send(appEUI lorawan.EUI64, rxPackets loracontrol.RXPackets) error {
	b.callCount = b.callCount + 1
	return b.err
}

func (b *testApplicationBackend) SetClient(c *loracontrol.Client) {}

func (b *testApplicationBackend) Close() error {
	return nil
}

func (b *testApplicationBackend) Receive() chan loracontrol.TXPacket {
	return make(chan loracontrol.TXPacket)
}

type testGatewayBackend struct {
	rxPacketChan chan loracontrol.RXPacket
}

func (b *testGatewayBackend) SetClient(c *loracontrol.Client) {}

func (b *testGatewayBackend) Send(txPacket loracontrol.TXPacket) error {
	return nil
}

func (b *testGatewayBackend) Receive() chan loracontrol.RXPacket {
	return b.rxPacketChan
}

func (b *testGatewayBackend) Close() error {
	return nil
}

func TestHandleGatewayPackets(t *testing.T) {
	config := getConfig()

	Convey("Given a Client connected to a clean Redis database", t, func() {
		appBackend := &testApplicationBackend{}
		gwBackend := &testGatewayBackend{
			rxPacketChan: make(chan loracontrol.RXPacket, 1),
		}
		client, err := loracontrol.NewClient(
			loracontrol.SetStorageBackend(loracontrol.NewRedisBackend(config.RedisServer, config.RedisPassword)),
			loracontrol.SetApplicationBackend(appBackend),
			loracontrol.SetGatewayBackend(gwBackend),
		)
		So(err, ShouldBeNil)
		So(client.Storage().FlushAll(), ShouldBeNil)

		nwkSKey := lorawan.AES128Key{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
		appSKey := lorawan.AES128Key{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}
		devAddr := lorawan.DevAddr{1, 1, 1, 1}

		Convey("Given a test RXPacket", func() {
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

			rxPacket := loracontrol.RXPacket{
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
			}

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

			Convey("When calling handleGatewayPacket", func() {
				err := handleGatewayPacket(rxPacket, client)
				Convey("Then an error is returned that the node-session does not exists", func() {
					So(err, ShouldResemble, errors.New("node-session does not exist"))
				})
			})

			Convey("Given the node-session is in the database", func() {
				nodeSession := loracontrol.NodeSession{
					DevAddr: devAddr,
					DevEUI:  [8]byte{1, 1, 1, 1, 1, 1, 1, 1},
					NwkSKey: nwkSKey,
					FCntUp:  10,
				}
				So(client.NodeSession().CreateExpire(nodeSession), ShouldBeNil)

				Convey("Given the node is in the database", func() {
					node := loracontrol.Node{
						DevEUI: [8]byte{1, 1, 1, 1, 1, 1, 1, 1},
						AppEUI: [8]byte{2, 2, 2, 2, 2, 2, 2, 2},
						AppKey: appSKey,
					}
					So(client.Node().Create(node), ShouldBeNil)

					Convey("Given the application is in the database", func() {
						app := loracontrol.Application{
							AppEUI: [8]byte{2, 2, 2, 2, 2, 2, 2, 2},
						}
						So(client.Application().Create(app), ShouldBeNil)

						Convey("Then handleGatewayPacket does not return an error", func() {
							err := handleGatewayPacket(rxPacket, client)
							So(err, ShouldBeNil)

							Convey("Then the app backend Send was called once", func() {
								So(appBackend.callCount, ShouldEqual, 1)
							})

							Convey("Then FCntUp on the node-session is incremented", func() {
								n, err := client.NodeSession().Get(nodeSession.DevAddr)
								So(err, ShouldBeNil)
								So(n.FCntUp, ShouldEqual, nodeSession.FCntUp+1)
							})
						})

						Convey("When calling HandleGatewayPackets", func() {
							gwBackend.rxPacketChan <- rxPackets[0]
							close(gwBackend.rxPacketChan)
							HandleGatewayPackets(client)

							Convey("Then the packet has been sent by the app backend", func() {
								time.Sleep(time.Millisecond * 200)
								So(appBackend.callCount, ShouldEqual, 1)
							})
						})

						Convey("When the FCnt is invalid", func() {
							nodeSession.FCntUp = 11
							So(client.NodeSession().UpdateExpire(nodeSession), ShouldBeNil)

							Convey("Then handleGatewayPacket returns an invalid FCnt error", func() {
								err := handleGatewayPacket(rxPacket, client)
								So(err, ShouldResemble, errors.New("invalid FCnt or too many dropped frames"))
							})
						})

						Convey("When the NwkSKey is invalid", func() {
							nodeSession.NwkSKey[0] = 0
							So(client.NodeSession().UpdateExpire(nodeSession), ShouldBeNil)

							Convey("Then handleGatewayPacket returns an invalid MIC error", func() {
								err := handleGatewayPacket(rxPacket, client)
								So(err, ShouldResemble, errors.New("invalid MIC"))
							})
						})

						Convey("When the application backend returns an error", func() {
							appBackend.err = errors.New("BOOM!")
							Convey("When calling handleGatewayPacket", func() {
								err := handleGatewayPacket(rxPacket, client)
								So(err, ShouldResemble, errors.New("BOOM!"))

								Convey("Then FCntUp on the node-session is not incremented", func() {
									n, err := client.NodeSession().Get(nodeSession.DevAddr)
									So(err, ShouldBeNil)
									So(n.FCntUp, ShouldEqual, nodeSession.FCntUp)
								})
							})
						})

					})
				})
			})
		})
	})
}
