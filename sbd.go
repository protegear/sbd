package sbd

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type Orientation byte
type SessionStatus byte

type ElementID byte

const (
	moHeaderID              = ElementID(0x01)
	moPayloadID             = ElementID(0x02)
	moLocationInformationID = ElementID(0x03)
	moConfirmationID        = ElementID(0x05)

	// SBD Session Status
	StCompleted             = SessionStatus(0)
	StMTTooLarge            = SessionStatus(1)
	StLocationUnacceptable  = SessionStatus(2)
	StTimeout               = SessionStatus(10)
	StIMEITooLarge          = SessionStatus(12)
	StRFLinkLoss            = SessionStatus(13)
	StIMEIProtocolAnomaly   = SessionStatus(14)
	StIMEIProhibitedGateway = SessionStatus(15)

	NE = Orientation(0)
	NW = Orientation(1)
	SE = Orientation(2)
	SW = Orientation(3)

	protocolRevision = 1
)

func (o Orientation) LatLng(lat, lng float64) (float64, float64) {
	switch o {
	case NW:
		return lat, -1 * lng
	case SW:
		return -1 * lat, -1 * lng
	case SE:
		return -1 * lat, lng
	}
	// this is NE
	return lat, lng
}

func (eid ElementID) TargetType() interface{} {
	switch eid {
	case moPayloadID:
		return &MOPayload{}
	case moLocationInformationID:
		return &MOLocationInformation{}
	case moConfirmationID:
		return &MOConfirmationMessage{}
	case moHeaderID:
		fallthrough
	default:
		return &MODirectIPHeader{}
	}
}

// A MessageHeader defines the revision and the whole message length.
type MessageHeader struct {
	ProtocolRevision byte
	MessageLength    uint16
}

// An Header is sent before every information element and
// specifies the ID (aka type) and length of the element.
type Header struct {
	ID            ElementID `json:"id"`
	ElementLength uint16    `json:"elementlength"`
}

// An InformationElement contains a header and the data which can have
// different types. You have to inspect the header's ID field to
// get the type.
type InformationElement struct {
	Header `json:"header"`
	Data   interface{} `json:"data"`
}

func (u *InformationElement) UnmarshalJSON(data []byte) error {
	// a little bit inperformant, because we parse the header twice
	// but hey .... who cares?
	h := &struct {
		Header `json:"header"`
	}{
		Header: u.Header,
	}
	if err := json.Unmarshal(data, &h); err != nil {
		return err
	}
	buf := h.ID.TargetType()
	ie := &struct {
		Header `json:"header"`
		Data   interface{} `json:"data"`
	}{
		Header: u.Header,
		Data:   buf,
	}
	if err := json.Unmarshal(data, &ie); err != nil {
		return err
	}
	u.Header = ie.Header
	u.Data = ie.Data
	return nil
}

// InformationElements is a wrapper type for the InformationElement's
// which are in a bundled bucket
type InformationBucket struct {
	Header   *MODirectIPHeader      `json:"header"`
	Payload  []byte                 `json:"payload"`
	Location *MOLocationInformation `json:"location"`
	Position *Location              `json:"position"`
}

// The MODirectIPHeader contains some information about the message
// itself.
type MODirectIPHeader struct {
	CDRReference  uint32        `json:"cdrreference"`
	IMEI          [15]byte      `json:"imei"`
	SessionStatus SessionStatus `json:"sessionstatus"`
	MOMSN         uint16        `json:"momsn"`
	MTMSN         uint16        `json:"mtmsn"`
	TimeOfSession uint32        `json:"timeofsession"`
}

// GetTime returns the time which is specified by the TimeOfSession field
func (dih *MODirectIPHeader) GetTime() time.Time {
	return time.Unix(int64(dih.TimeOfSession), 0)
}

// GetIMEI returns the imei as a string
func (dih *MODirectIPHeader) GetIMEI() string {
	return string(dih.IMEI[:])
}

func (u *MODirectIPHeader) UnmarshalJSON(data []byte) error {
	type Alias MODirectIPHeader
	dat := &struct {
		IMEI string `json:"imei"`
		*Alias
	}{
		Alias: (*Alias)(u),
	}
	if err := json.Unmarshal(data, &dat); err != nil {
		return err
	}
	copy(u.IMEI[:], dat.IMEI)
	return nil
}

func (u *MODirectIPHeader) MarshalJSON() ([]byte, error) {
	type Alias MODirectIPHeader
	return json.Marshal(&struct {
		IMEI string `json:"imei"`
		*Alias
	}{
		Alias: (*Alias)(u),
		IMEI:  string(u.IMEI[:]),
	})
}

// MOPayload is a wrapper around some blob data.
type MOPayload struct {
	Payload []byte `json:"payload"`
}

// A MOConfirmationMessage contains the confirmation status.
type MOConfirmationMessage struct {
	Status byte `json:"status"`
}

// Success checks if the confirmation was successfull.
func (ch *MOConfirmationMessage) Success() bool {
	return ch.Status == 1
}

// MOLocationInformation contains location information and the
// cep radius in km.
type MOLocationInformation struct {
	Position  LocationData `json:"position"`
	CEPRadius uint32       `json:"cepradius"`
}

// LocationData contains an orientation as well as the latitude and
// longitude in degree's and minutes.
type LocationData struct {
	OrientationCode Orientation `json:"orientationcode"`
	LatDegree       byte        `json:"latdegree"`
	LatMinute       uint16      `json:"latminute"`
	LngDegree       byte        `json:"lngdegree"`
	LngMinute       uint16      `json:"lngminute"`
}

type Location struct {
	Latitude  float64
	Longitude float64
}

// GetLatLng converts the location information to latitude/longitude
// values which can be used by other systems. The orientiation is
// used to convert the values to positive or negative vals.
func (loc *MOLocationInformation) GetLatLng() (float64, float64) {
	la := float64(loc.Position.LatDegree) + float64(loc.Position.LatMinute)/1000.0/60.0
	ln := float64(loc.Position.LngDegree) + float64(loc.Position.LngMinute)/1000.0/60.0

	return loc.Position.OrientationCode.LatLng(la, ln)
}

// GetCEPRadius simply returns the radius as an int value
func (loc *MOLocationInformation) GetCEPRadius() int {
	return int(loc.CEPRadius)
}

func parseMessageHeader(in io.Reader) (*MessageHeader, error) {
	var res MessageHeader
	if err := binary.Read(in, binary.BigEndian, &res); err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("cannot read message header: %v", err)
	}
	return &res, nil
}

func parseInformationElement(in io.Reader) (*InformationElement, error) {
	var h Header
	if err := binary.Read(in, binary.BigEndian, &h); err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("cannot read informationelement header: %v", err)
	}
	el, err := parseElementByType(&h, in)
	if err != nil {
		return nil, err
	}

	return &InformationElement{Header: h, Data: el}, nil
}

// GetElements parses the given stream and returns an array of found
// elements.
func GetElements(in io.Reader) (*InformationBucket, error) {
	mh, err := parseMessageHeader(in)
	if err != nil {
		return nil, err
	}
	if mh.ProtocolRevision != protocolRevision {
		return nil, fmt.Errorf("wrong protocol version: %d", mh.ProtocolRevision)
	}
	bbuf := make([]byte, mh.MessageLength)
	_, err = io.ReadFull(in, bbuf)
	if err != nil {
		return nil, fmt.Errorf("cannot read bytes from message: %v", err)
	}
	buffer := bytes.NewBuffer(bbuf)
	buck := new(InformationBucket)
	for {
		ie, err := parseInformationElement(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch ie.ID {
		case moPayloadID:
			buck.Payload = ie.Data.(*MOPayload).Payload
		case moHeaderID:
			buck.Header = ie.Data.(*MODirectIPHeader)
		case moLocationInformationID:
			buck.Location = ie.Data.(*MOLocationInformation)
			lat, lng := buck.Location.GetLatLng()
			buck.Position = &Location{Latitude: lat, Longitude: lng}
		}
	}

	return buck, nil
}

func parseElementByType(h *Header, in io.Reader) (interface{}, error) {
	buf := h.ID.TargetType()

	// we cannot read the payload struct with binary.Read because it has
	// a byte-slice as field. so we must do it the ugly way here.
	if h.ID == moPayloadID {
		buf = make([]byte, h.ElementLength)
	}
	if err := binary.Read(in, binary.BigEndian, buf); err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("cannot read informationelement content: %v", err)
	}
	if h.ID == moPayloadID {
		return &MOPayload{Payload: buf.([]byte)}, nil
	}

	return buf, nil
}

// NewPayload returns an element which contains the given bytes as payload.
func NewPayload(b []byte) *InformationElement {
	return &InformationElement{
		Header: Header{
			ID:            moPayloadID,
			ElementLength: uint16(len(b)),
		},
		Data: &MOPayload{
			Payload: b,
		},
	}
}
