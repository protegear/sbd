package sbd

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
)

const (
	flushMTQeue         = 1
	sendRingAlertNoMTM  = 2
	updateSSDLocation   = 8
	highPriorityMessage = 16
	assignMTMSN         = 32

	maxPayload = 1890
)

// A DirectIPRequest encapsulates the data needed for a directip message.
type DirectIPRequest struct {
	dispositionflags uint16
	imei             string
	clientmsgid      string
	payload          []byte
	priorityLevel    *int
}

type DirectIPHeader struct {
	UniqueClientMsgID [4]byte  `json:"uniqueclientmsgid`
	IMEI              [15]byte `json:"imei"`
	DispositionFlags  uint16   `json:"dispositionflags"`
}
type mtDirectIPHeader struct {
	Header
	DirectIPHeader
}

type mtPriority struct {
	Header
	Level uint16 `json:"level"`
}

// A Confirmation is returned when a directip call is invoked.
type Confirmation struct {
	UniqueClientMsgID [4]byte  `json:"uniqueclientmsgid`
	IMEI              [15]byte `json:"imei"`
	AutoIDReference   uint32   `json:"autoidreference"`
	MessageStatus     int16    `json:"messagestatus"`
}

type confirmationMessage struct {
	MessageHeader
	Header
	Confirmation
}

// DirectOption is the type for configuring the request
type DirectOption func(rq *DirectIPRequest)

// NewRequest returns a new request to the given address.
func NewRequest() *DirectIPRequest {
	return &DirectIPRequest{}
}

// With can be used to set some request options.
func (rq *DirectIPRequest) With(opts ...DirectOption) *DirectIPRequest {
	for _, o := range opts {
		o(rq)
	}
	return rq
}

func (rq *DirectIPRequest) Do(serverAddress string) (*Confirmation, error) {
	_, _, err := net.SplitHostPort(serverAddress)
	if err != nil {
		return nil, fmt.Errorf("the server adress must have the form host:port (%q): %v", serverAddress, err)
	}
	mth := mtDirectIPHeader{
		Header: Header{
			ID:            mtHeaderID,
			ElementLength: 21,
		},
		DirectIPHeader: DirectIPHeader{
			DispositionFlags: rq.dispositionflags,
		},
	}
	var buf bytes.Buffer
	copy(mth.UniqueClientMsgID[:], []byte(rq.clientmsgid)[0:4])
	copy(mth.IMEI[:], []byte(rq.imei)[0:15])
	if err := binary.Write(&buf, binary.BigEndian, &mth); err != nil {
		return nil, fmt.Errorf("cannot write MT Header: %v", err)
	}
	if rq.priorityLevel != nil {
		pr := mtPriority{
			Header: Header{
				ID:            mtMessagePriority,
				ElementLength: uint16(2),
			},
			Level: uint16(*rq.priorityLevel),
		}
		if err := binary.Write(&buf, binary.BigEndian, &pr); err != nil {
			return nil, fmt.Errorf("cannot write MT Priority IE: %v", err)
		}

	}
	if rq.payload != nil {
		if len(rq.payload) > maxPayload {
			return nil, fmt.Errorf("the payload is to large (%d)", len(rq.payload))
		}
		h := Header{
			ID:            mtPayloadID,
			ElementLength: uint16(len(rq.payload)),
		}
		if err := binary.Write(&buf, binary.BigEndian, &h); err != nil {
			return nil, fmt.Errorf("cannot write MT Payload IE: %v", err)
		}
		if _, err := buf.Write(rq.payload); err != nil {
			return nil, fmt.Errorf("cannot write payload to buffer: %v", err)
		}
	}
	data := buf.Bytes()
	h := MessageHeader{
		ProtocolRevision: protocolRevision,
		MessageLength:    uint16(len(data)),
	}
	conn, err := net.Dial("tcp", serverAddress)
	if err != nil {
		return nil, fmt.Errorf("cannot dial %q: %v", serverAddress, err)
	}
	defer conn.Close()
	if err := binary.Write(conn, binary.BigEndian, &h); err != nil {
		return nil, fmt.Errorf("cannot write MT Message Header: %v", err)
	}
	_, err = conn.Write(data)
	if err != nil {
		return nil, fmt.Errorf("cannot write data to connection: %v", err)
	}

	var result confirmationMessage
	if err := binary.Read(conn, binary.BigEndian, &result); err != nil {
		return nil, fmt.Errorf("cannot read MT confirmation: %v", err)
	}

	return &result.Confirmation, nil
}

// FlushMTQueue sets the corresponding disposition flag.
func FlushMTQueue(rq *DirectIPRequest) {
	rq.dispositionflags |= flushMTQeue
}

// SendRingAlertNoMTM sets the corresponding disposition flag.
func SendRingAlertNoMTM(rq DirectIPRequest) {
	rq.dispositionflags |= sendRingAlertNoMTM
}

// UpdateSSDLocation sets the corresponding disposition flag.
func UpdateSSDLocation(rq DirectIPRequest) {
	rq.dispositionflags |= updateSSDLocation
}

// HighPriorityMessage sets the corresponding disposition flag.
func HighPriorityMessage(rq DirectIPRequest) {
	rq.dispositionflags |= highPriorityMessage
}

// AssignMTMSN sets the corresponding disposition flag.
func AssignMTMSN(rq DirectIPRequest) {
	rq.dispositionflags |= assignMTMSN
}

// IMEI sets the imei in the header.
func IMEI(imei string) DirectOption {
	return func(rq *DirectIPRequest) {
		rq.imei = imei
	}
}

// Payload sets payload, so a payload information element will be added.
func Payload(payload []byte) DirectOption {
	return func(rq *DirectIPRequest) {
		rq.payload = payload
	}
}

// PriorityLevel sets the prio level so a priority information element will be added.
func PriorityLevel(lvl int) DirectOption {
	return func(rq *DirectIPRequest) {
		rq.priorityLevel = &lvl
	}
}

// ClientMsgID sets the clientmsgi in the header. only the first 4 bytes
// are used!
func ClientMsgID(msg string) DirectOption {
	return func(rq *DirectIPRequest) {
		rq.clientmsgid = msg
	}
}
