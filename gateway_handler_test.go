package loraserver

import (
	"net"
	"testing"

	"github.com/brocaar/lorawan/semtech"
	. "github.com/smartystreets/goconvey/convey"
)

func TestHandleGatewayPacket(t *testing.T) {
	Convey("Given an empty UDPPacket and send channel", t, func() {
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
				So(HandleGatewayPacket(p, sendChan), ShouldBeNil)

				Convey("Then sendChan should contain a PullACK response", func() {
					out := <-sendChan
					So(out.Addr, ShouldEqual, p.Addr)

					var pullACK semtech.PullACKPacket
					So(pullACK.UnmarshalBinary(out.Data), ShouldBeNil)
					So(pullACK.RandomToken, ShouldEqual, 12345)
				})
			})
		})
	})
}
