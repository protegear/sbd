package sbd

import (
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestEmptyPayload(t *testing.T) {
	ts, _ := NewDIPServer("127.0.0.1:0")
	go ts.Start()
	defer ts.Close()
	Convey("with only an imei given", t, func() {
		rq := NewRequest().With(
			IMEI("123"),
		)
		Convey("the encoded data should be correct", func(c C) {
			ts.Handle = func(mg *MessageHeader, dih *DirectIPHeader, payload []byte, priority *int) Confirmation {
				c.So(mg.ProtocolRevision, ShouldEqual, 1)
				c.So(mg.MessageLength, ShouldEqual, 24)
				imei := strings.Trim(string(dih.IMEI[:]), "\x00")
				c.So(imei, ShouldEqual, "123")
				return Confirmation{}
			}
		})
		rq.Do(ts.address)
	})
}

func TestWithPayload(t *testing.T) {
	ts, _ := NewDIPServer("127.0.0.1:0")
	go ts.Start()
	defer ts.Close()
	Convey("with imei and payload given", t, func() {
		rq := NewRequest().With(
			IMEI("123"),
			Payload([]byte("my dummy payload")),
		)
		Convey("the message length should be correct", func(c C) {
			ts.Handle = func(mg *MessageHeader, dih *DirectIPHeader, payload []byte, priority *int) Confirmation {
				c.So(mg.MessageLength, ShouldEqual, 43)
				pl := string(payload)
				c.So(pl, ShouldEqual, "my dummy payload")
				return Confirmation{}
			}
		})
		rq.Do(ts.address)
	})

}
