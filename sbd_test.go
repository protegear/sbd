package sbd

import (
	"bytes"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

const (
	sample_msg1 = "\x01\x00E\x01\x00\x1c\x9dL\xce{300230000000000\x00\x15\x9d\x00\x00X\x1b\x19\xe0\x03\x00\x0b\x00\x06\x8c\xda\x8av\xfe\x00\x00\x00\x08\x02\x00\x15\x00!a\xac\x0c\x85\xb2\x8f\xf4\x9f\x08k\x0f\xf0\x00\x00\x00\x00\x07\xff|"
	sample_msg2 = "\x01\x00E\x01\x00\x1c\x9d\x81\x98\x1f300230000000001\x00\x15\xf0\x00\x00X\x1d\xe6N\x03\x00\x0b\x00\x06\xb7a\x8a;\x1e\x00\x00\x00\x04\x02\x00\x15\x00!b\xb8\x0cO\xb1\x0f\xf3\x9e\x04j\x02\x07bV\xc2A\xea\x95|"
	sample_msg3 = "\x01\x008\x01\x00\x1cp\xec\ai300234063904190\x00\x00K\x00\x00U\x9e\xba,\x02\x00\x16test message0123456789"
)

/*
| IMEI: 300230000000000
| Message protocol version: 01
| Total message length: 69
| MO header IEI: 01
| MO header length: 28
| Call Detail Record Reference: 2639056507
| Session status: 00
| MO MSN: 5533
| MT MSN: 0
| Message timestamp: 1478171104
| Payload header IEI: 03
| Payload header len: 11
| Location orientation code: 0
| Location latitude (deg): 6
| Location latitude (min): 36058
| Location longitude (deg): 138
| Location longitude (min): 30462
| CEP radius: 8
| Payload IEI: 02
| Payload length: 21
| Payload (hex): 002161ac0c85b28ff49f086b0ff00000000007ff7c
------------------------------------------------------
| IMEI: 300230000000001
| Message protocol version: 01
| Total message length: 69
| MO header IEI: 01
| MO header length: 28
| Call Detail Record Reference: 2642515999
| Session status: 00
| MO MSN: 5616
| MT MSN: 0
| Message timestamp: 1478354510
| Payload header IEI: 03
| Payload header len: 11
| Location orientation code: 0
| Location latitude (deg): 6
| Location latitude (min): 46945
| Location longitude (deg): 138
| Location longitude (min): 15134
| CEP radius: 4
| Payload IEI: 02
| Payload length: 21
| Payload (hex): 002162b80c4fb10ff39e046a02076256c241ea957c
*/

func TestParseHeader(t *testing.T) {
	msgs := []struct {
		Name string
		Msg  string
		Len  int
	}{
		{Name: "sample1", Msg: sample_msg1, Len: 69},
		{Name: "sample2", Msg: sample_msg2, Len: 69},
		{Name: "sample3", Msg: sample_msg3, Len: 56},
	}
	for _, msg := range msgs {
		Convey("Loading "+msg.Name, t, func() {
			f := bytes.NewBufferString(msg.Msg)

			Convey("When the header is parsed", func() {
				mh, err := parseMessageHeader(f)
				So(err, ShouldBeNil)

				Convey("The revision and length should be ok", func() {
					So(mh.ProtocolRevision, ShouldEqual, 1)
					So(mh.MessageLength, ShouldEqual, msg.Len)
				})
			})
		})
	}
}

func TestMODirectIPHeader(t *testing.T) {
	Convey("Loading sample3", t, func() {
		f := bytes.NewBufferString(sample_msg3)
		Convey("There should be two elements", func() {
			el, err := GetElements(f)
			So(err, ShouldBeNil)
			So(el.Header, ShouldNotBeNil)
			So(el.Location, ShouldBeNil)
			So(el.Payload, ShouldNotBeNil)
			Convey("And the elements should contain correct data", func() {
				So(el.Header.GetTime().Unix(), ShouldEqual, 1436465708)
				So(el.Header.GetIMEI(), ShouldEqual, "300234063904190")
				So(el.Payload, ShouldResemble, []byte("test message0123456789"))
			})
		})
	})
}

func TestLocationInformation(t *testing.T) {
	Convey("Loading sample1", t, func() {
		f := bytes.NewBufferString(sample_msg1)
		Convey("There should be three elements", func() {
			el, err := GetElements(f)
			So(err, ShouldBeNil)
			So(el.Header, ShouldNotBeNil)
			So(el.Location, ShouldNotBeNil)
			So(el.Payload, ShouldNotBeNil)
			Convey("And the location should contain correct data", func() {
				lat, lng := el.Location.GetLatLng()
				So(lat, ShouldAlmostEqual, 6.600967, .00001)
				So(lng, ShouldAlmostEqual, 138.507700, .00001)
				So(el.Location.GetCEPRadius(), ShouldEqual, 8)
			})
		})
	})
}

func TestOrientation(t *testing.T) {
	Convey("given a specific position", t, func() {
		lat := 1.0
		lng := 1.0
		Convey("it should have positive values in NE", func() {
			lt, ln := NE.LatLng(lat, lng)
			So(lt, ShouldAlmostEqual, lat, .00001)
			So(ln, ShouldAlmostEqual, lng, .00001)
		})
		Convey("it should have negative values in SW", func() {
			lt, ln := SW.LatLng(lat, lng)
			So(lt, ShouldAlmostEqual, -1*lat, .00001)
			So(ln, ShouldAlmostEqual, -1*lng, .00001)
		})
		Convey("it should have negative longitude in NE", func() {
			lt, ln := NW.LatLng(lat, lng)
			So(lt, ShouldAlmostEqual, lat, .00001)
			So(ln, ShouldAlmostEqual, -1*lng, .00001)
		})
		Convey("it should have negative latitude in SE", func() {
			lt, ln := SE.LatLng(lat, lng)
			So(lt, ShouldAlmostEqual, -1*lat, .00001)
			So(ln, ShouldAlmostEqual, lng, .00001)
		})
	})
}
