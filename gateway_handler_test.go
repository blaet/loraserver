package loraserver

import (
	"net"
	"testing"

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
