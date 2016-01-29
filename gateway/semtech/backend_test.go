package semtech

import (
	"encoding/base64"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/brocaar/loracontrol"
	"github.com/brocaar/lorawan"
	. "github.com/smartystreets/goconvey/convey"
)

func init() {
	log.SetLevel(log.ErrorLevel)
}

type config struct {
	RedisServer   string
	RedisPassword string
}

func getConfig() *config {
	c := &config{
		RedisServer:   "localhost:6379",
		RedisPassword: "",
	}

	if v := os.Getenv("TEST_REDIS_SERVER"); v != "" {
		c.RedisServer = v
	}
	if v := os.Getenv("TEST_REDIS_PASSWORD"); v != "" {
		c.RedisPassword = v
	}

	return c
}

func TestBackend(t *testing.T) {
	c := getConfig()

	Convey("Given a Client with Redis backend and an empty database", t, func() {
		addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:8123")
		So(err, ShouldBeNil)

		backend, err := NewBackend(addr.Port)
		So(err, ShouldBeNil)
		defer backend.Close()

		client, err := loracontrol.NewClient(
			loracontrol.SetStorageBackend(loracontrol.NewRedisBackend(c.RedisServer, c.RedisPassword)),
			loracontrol.SetApplicationBackend(&loracontrol.DummyApplicationBackend{}),
			loracontrol.SetGatewayBackend(backend),
		)
		So(err, ShouldBeNil)
		So(client.Storage().FlushAll(), ShouldBeNil)

		Convey("Given a UDP socket", func() {
			gwAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
			So(err, ShouldBeNil)
			conn, err := net.ListenUDP("udp", gwAddr)
			So(err, ShouldBeNil)
			defer conn.Close()
			So(conn.SetDeadline(time.Now().Add(time.Second*1)), ShouldBeNil)

			Convey("When sending a PULL_DATA packet", func() {
				p := PullDataPacket{
					RandomToken: 1234,
					GatewayMAC:  [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
				}
				b, err := p.MarshalBinary()
				So(err, ShouldBeNil)
				_, err = conn.WriteToUDP(b, addr)
				So(err, ShouldBeNil)

				Convey("Then an ACK packet is returned", func() {
					buf := make([]byte, 65507)
					i, _, err := conn.ReadFromUDP(buf)
					So(err, ShouldBeNil)
					ack := PullACKPacket{}
					So(ack.UnmarshalBinary(buf[:i]), ShouldBeNil)
					So(ack.RandomToken, ShouldEqual, p.RandomToken)
				})
			})

			Convey("When sending a PUSH_DATA packet with stats", func() {
				p := PushDataPacket{
					RandomToken: 1234,
					GatewayMAC:  [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
					Payload: PushDataPayload{
						Stat: &Stat{
							Time: ExpandedTime(time.Time{}.UTC()),
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
				b, err := p.MarshalBinary()
				So(err, ShouldBeNil)
				_, err = conn.WriteToUDP(b, addr)
				So(err, ShouldBeNil)

				Convey("Then an ACK packet is returned", func() {
					buf := make([]byte, 65507)
					i, _, err := conn.ReadFromUDP(buf)
					So(err, ShouldBeNil)
					ack := PushACKPacket{}
					So(ack.UnmarshalBinary(buf[:i]), ShouldBeNil)
					So(ack.RandomToken, ShouldEqual, 1234)
				})

				Convey("Then the gateway stats are stored", func() {
					time.Sleep(time.Millisecond * 100)
					_, err := client.Gateway().Get(p.GatewayMAC)
					So(err, ShouldBeNil)
				})
			})

			Convey("When sending a PUSH_DATA packet with RXPK", func() {
				p := PushDataPacket{
					RandomToken: 1234,
					GatewayMAC:  [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
					Payload: PushDataPayload{
						RXPK: []RXPK{
							{
								Time: CompactTime(time.Now().UTC()),
								Tmst: 708016819,
								Freq: 868.5,
								Chan: 2,
								RFCh: 1,
								Stat: 1,
								Modu: "LORA",
								DatR: DatR{LoRa: "SF7BW125"},
								CodR: "4/5",
								RSSI: -51,
								LSNR: 7,
								Size: 16,
								Data: "QAEBAQGAAAABVfdjR6YrSw==",
							},
						},
					},
				}
				b, err := p.MarshalBinary()
				So(err, ShouldBeNil)
				_, err = conn.WriteToUDP(b, addr)
				So(err, ShouldBeNil)

				Convey("Then an ACK packet is returned", func() {
					buf := make([]byte, 65507)
					i, _, err := conn.ReadFromUDP(buf)
					So(err, ShouldBeNil)
					ack := PushACKPacket{}
					So(ack.UnmarshalBinary(buf[:i]), ShouldBeNil)
					So(ack.RandomToken, ShouldEqual, 1234)
				})

				Convey("Then Receive() returns the RXPK", func() {
					time.Sleep(time.Millisecond * 100)
					rxpk := <-backend.Receive()

					rxpk2, err := newRXPacketFromSemtech(p.GatewayMAC, &p.Payload.RXPK[0])
					So(err, ShouldBeNil)

					So(&rxpk, ShouldResemble, rxpk2)
				})
			})

			Convey("Given an TXPacket", func() {
				var nwkSKey lorawan.AES128Key
				macPL := lorawan.NewMACPayload(false)
				phy := lorawan.NewPHYPayload(false)
				phy.MACPayload = macPL
				phy.MHDR = lorawan.MHDR{
					MType: lorawan.UnconfirmedDataDown,
					Major: lorawan.LoRaWANR1,
				}
				So(phy.SetMIC(nwkSKey), ShouldBeNil)

				txPacket := loracontrol.TXPacket{
					TXInfo: loracontrol.TXInfo{
						MAC:         lorawan.EUI64{1, 2, 3, 4, 5, 6, 7, 8},
						Immediately: true,
						Timestamp:   12345,
						Frequency:   123.45,
						RFChain:     1,
						Power:       12,
						DataRate: loracontrol.DataRate{
							LoRa: "SF12BW500",
						},
						CodeRate:           "4/5",
						FrequencyDeviation: 300,
						DisableCRC:         true,
					},
					PHYPayload: phy,
				}

				Convey("When sending the packet to the gateway, and the gateway does not exist", func() {
					err := client.Gateway().Send(txPacket)
					Convey("Then Send returns an gateway not found error", func() {
						So(err, ShouldResemble, errors.New("gateway/semtech: gateway does not exist"))
					})
				})

				Convey("Given a matching gateway in the database", func() {
					gw := &loracontrol.Gateway{
						MAC: lorawan.EUI64{1, 2, 3, 4, 5, 6, 7, 8},
						Config: loracontrol.PropertyBag{
							String: map[string]string{
								"udp_addr": conn.LocalAddr().String(),
							},
						},
					}
					So(client.Gateway().Upsert(gw), ShouldBeNil)

					Convey("When sending the packet to the gateway", func() {
						So(client.Gateway().Send(txPacket), ShouldBeNil)

						Convey("Then the correct data is received by the gateway", func() {
							buf := make([]byte, 65507)
							i, _, err := conn.ReadFromUDP(buf)
							So(i, ShouldBeGreaterThan, 0)
							pullResp := PullRespPacket{}
							So(pullResp.UnmarshalBinary(buf[:i]), ShouldBeNil)

							b, err := phy.MarshalBinary()
							So(err, ShouldBeNil)

							So(pullResp, ShouldResemble, PullRespPacket{
								Payload: PullRespPayload{
									TXPK: TXPK{
										Imme: true,
										Tmst: 12345,
										Freq: 123.45,
										RFCh: 1,
										Powe: 12,
										Modu: "LORA",
										DatR: DatR{
											LoRa: "SF12BW500",
										},
										CodR: "4/5",
										FDev: 300,
										NCRC: true,
										Size: uint16(len(b)),
										Data: base64.StdEncoding.EncodeToString(b),
										IPol: true,
									},
								},
							})
						})
					})
				})
			})
		})
	})
}

func TestNewGatewayFromSemtech(t *testing.T) {
	now := time.Now().UTC()

	Convey("Given a Stat struct, a Gateway MAC and a *net.UDPAddr", t, func() {
		stat := &Stat{
			Time: ExpandedTime(now),
			Lati: 1.234,
			Long: 2.123,
			Alti: 234,
			RXNb: 1,
			RXOK: 2,
			RXFW: 3,
			ACKR: 33.3,
			DWNb: 4,
		}

		mac := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
		addr := &net.UDPAddr{}

		Convey("When calling newGatewayFromSemtech", func() {
			gw := newGatewayFromSemtech(addr, mac, stat)
			Convey("Then all Gateway fields are set correctly", func() {
				So(gw, ShouldResemble, &loracontrol.Gateway{
					UpdatedAt:                   now,
					MAC:                         mac,
					Latitude:                    1.234,
					Longitude:                   2.123,
					Altitude:                    234,
					UpstreamPacketsReceived:     1,
					UpstreamPacketsReceivedOK:   2,
					UpstreamPacketsForwarded:    3,
					UpstreamDatagramsACKRate:    33.3,
					DownstreamDatagramsReceived: 4,
					Config: loracontrol.PropertyBag{
						String: map[string]string{
							"udp_addr": addr.String(),
						},
					},
				})
			})
		})
	})
}

func TestNewRXPacketFromSemtech(t *testing.T) {
	now := time.Now().UTC()

	Convey("Given a RXPKstruct and a Gateway MAC", t, func() {
		rxpk := &RXPK{
			Time: CompactTime(now),
			Tmst: 708016819,
			Freq: 868.5,
			Chan: 2,
			RFCh: 1,
			Stat: 1,
			Modu: "LORA",
			DatR: DatR{LoRa: "SF7BW125"},
			CodR: "4/5",
			RSSI: -51,
			LSNR: 7,
			Size: 16,
			Data: "QAEBAQGAAAABVfdjR6YrSw==",
		}

		mac := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}

		Convey("When calling newRXPacketFromSemtech", func() {
			rxPacket, err := newRXPacketFromSemtech(mac, rxpk)
			So(err, ShouldBeNil)

			Convey("Then all RXInfo fields are set correctly", func() {
				So(rxPacket.RXInfo, ShouldResemble, loracontrol.RXInfo{
					MAC:        mac,
					Time:       now,
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
				})
			})

			Convey("Then the PHYPayload contains the expected data", func() {
				nwkSKey := [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
				appSKey := [16]byte{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}

				// contruct the source message to compare the MIC
				macPayload := lorawan.NewMACPayload(true)
				macPayload.FHDR = lorawan.FHDR{
					DevAddr: lorawan.DevAddr([4]byte{1, 1, 1, 1}),
					FCtrl: lorawan.FCtrl{
						ADR: true,
					},
				}
				macPayload.FPort = 1
				macPayload.FRMPayload = []lorawan.Payload{
					&lorawan.DataPayload{Bytes: []byte{170, 187, 204}}, // AABBCC in hex
				}
				So(macPayload.EncryptFRMPayload(appSKey), ShouldBeNil)

				phy := lorawan.NewPHYPayload(true)
				phy.MHDR = lorawan.MHDR{
					MType: lorawan.UnconfirmedDataUp,
					Major: lorawan.LoRaWANR1,
				}
				phy.MACPayload = macPayload
				So(phy.SetMIC(nwkSKey), ShouldBeNil)

				// both MICs should be equal
				So(rxPacket.PHYPayload.MIC, ShouldResemble, phy.MIC)

				// try to decrypt the payload
				mpl, ok := rxPacket.PHYPayload.MACPayload.(*lorawan.MACPayload)
				So(ok, ShouldBeTrue)
				So(mpl.DecryptFRMPayload(appSKey), ShouldBeNil)
				So(mpl.FRMPayload, ShouldHaveLength, 1)
				dpl, ok := mpl.FRMPayload[0].(*lorawan.DataPayload)
				So(ok, ShouldBeTrue)
				So(dpl.Bytes, ShouldResemble, []byte{170, 187, 204})
			})
		})
	})
}
