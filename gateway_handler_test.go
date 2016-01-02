package loraserver

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/brocaar/loracontrol"
	"github.com/brocaar/lorawan/semtech"
	. "github.com/smartystreets/goconvey/convey"
)

func TestHandleGatewayPacket(t *testing.T) {
	Convey("Given an empty Client, UDPPacket and send channel", t, func() {
		client := &loracontrol.Client{}
		p := UDPPacket{
			Addr: &net.UDPAddr{},
		}
		sendChan := make(chan UDPPacket, 1)

		Convey("When the UDPPacket is of type PullData", func() {
			pullData := semtech.PullDataPacket{
				ProtocolVersion: 1,
				RandomToken:     12345,
				GatewayMAC:      [...]byte{1, 2, 3, 4, 5, 6, 7, 8},
			}
			bytes, err := pullData.MarshalBinary()
			So(err, ShouldBeNil)
			p.Data = bytes

			Convey("And calling HandleGatewayPacket", func() {
				So(HandleGatewayPacket(p, sendChan, client), ShouldBeNil)

				Convey("Then sendChan contains a PullACK response", func() {
					out := <-sendChan
					So(out.Addr, ShouldEqual, p.Addr)

					var pullACK semtech.PullACKPacket
					So(pullACK.UnmarshalBinary(out.Data), ShouldBeNil)
					So(pullACK.RandomToken, ShouldEqual, 12345)
				})
			})
		})

		Convey("When the UDPPacket is of type PushData", func() {
			pushData := semtech.PushDataPacket{
				ProtocolVersion: 1,
				RandomToken:     12345,
				GatewayMAC:      [...]byte{1, 2, 3, 4, 5, 6, 7, 8},
				// we don't care about the payload in this test
			}
			bytes, err := pushData.MarshalBinary()
			So(err, ShouldBeNil)
			p.Data = bytes

			Convey("And calling HandleGatewayPacket", func() {
				So(HandleGatewayPacket(p, sendChan, client), ShouldBeNil)

				Convey("Then sendChan contains a PushACK response", func() {
					out := <-sendChan
					So(out.Addr, ShouldEqual, p.Addr)

					var pushACK semtech.PushACKPacket
					So(pushACK.UnmarshalBinary(out.Data), ShouldBeNil)
					So(pushACK.RandomToken, ShouldEqual, 12345)
				})
			})
		})
	})
}

func TestHandleGatewayPushData(t *testing.T) {
	c := getConfig()

	Convey("Given a Client, UDPPacket and send channel", t, func() {
		client, err := loracontrol.NewClient(
			loracontrol.SetRedisBackend(c.RedisServer, c.RedisPassword),
			loracontrol.SetHTTPApplicationBackend(),
		)
		So(err, ShouldBeNil)
		So(client.Storage().FlushAll(), ShouldBeNil)
		addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:1234")
		So(err, ShouldBeNil)
		p := UDPPacket{
			Addr: addr,
		}
		sendChan := make(chan UDPPacket, 1)

		Convey("Given the PushData contains gateway stats", func() {
			packet := semtech.PushDataPacket{
				ProtocolVersion: 1,
				RandomToken:     12345,
				GatewayMAC:      [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
				Payload: semtech.PushDataPayload{
					Stat: &semtech.Stat{
						Time: semtech.ExpandedTime(time.Time{}.UTC()),
						Lati: 1.234,
						Long: 2.123,
						Alti: 123,
						RXNb: 1,
						RXOK: 2,
						RXFW: 3,
						ACKR: 33.3,
						DWNb: 4,
					},
				},
			}

			bytes, err := packet.MarshalBinary()
			So(err, ShouldBeNil)
			p.Data = bytes

			Convey("When calling HandleGatewayPushData", func() {
				err := HandleGatewayPushData(p, sendChan, client)
				So(err, ShouldBeNil)

				Convey("Then getting the gateway from the storage returns the expected data", func() {
					gw, err := client.Gateway().Get([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
					So(err, ShouldBeNil)

					So(gw, ShouldResemble, loracontrol.NewGatewayFromSemtech(addr, [8]byte{1, 2, 3, 4, 5, 6, 7, 8}, packet.Payload.Stat))
				})
			})
		})

		Convey("Given the PushData contains RX data packets", func() {
			packet := semtech.PushDataPacket{
				ProtocolVersion: 1,
				RandomToken:     12345,
				GatewayMAC:      [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
				Payload: semtech.PushDataPayload{
					RXPK: []semtech.RXPK{
						{
							Time: semtech.CompactTime(time.Now().UTC()),
							Tmst: 708016819,
							Freq: 868.5,
							Chan: 2,
							RFCh: 1,
							Stat: 1,
							Modu: "LORA",
							DatR: semtech.DatR{LoRa: "SF7BW125"},
							CodR: "4/5",
							RSSI: -51,
							LSNR: 7,
							Size: 16,
							Data: "QAEBAQGAAAABVfdjR6YrSw==",
						},
					},
				},
			}

			bytes, err := packet.MarshalBinary()
			So(err, ShouldBeNil)
			p.Data = bytes

			Convey("When calling HandleGatewayPushData", func() {
				err := HandleGatewayPushData(p, sendChan, client)
				// to process the packet the server will lookup the node by its DevAddr
				// which doesn't exists in this test. the processing behaviour is tested
				// in a different testcase
				Convey("Then it returns an error that the node could not be found", func() {
					So(err, ShouldResemble, errors.New("could not find node in database"))
				})
				Convey("Then there should be one collected packet in the database", func() {
					var packets []loracontrol.RXPacket
					err = client.Storage().GetAll("packet.R6YrSw==", &packets)
					So(err, ShouldBeNil)
					So(packets, ShouldHaveLength, 1)
				})
			})
		})
	})
}
