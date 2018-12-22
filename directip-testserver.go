package sbd

import (
	"encoding/binary"
	"log"
	"net"
)

type DIPHandler func(mg *MessageHeader, dih *DirectIPHeader, payload []byte, priority *int) Confirmation

func handle(mg *MessageHeader, dih *DirectIPHeader, payload []byte, priority *int) Confirmation {
	log.Printf("MH: %v, DIH: %v, Data: %v, Prio: %v", mg, dih, payload, priority)
	return Confirmation{}
}

func onError(e error) {
	log.Printf("Error: %v", e)
}

// A DIPServer is a server which can listen on an given tcp address
// and parses the incoming directip messages. Set the Handler-Field
// to implement your wanted behaviour. The default is simply logging.
type DIPServer struct {
	address      string
	listener     net.Listener
	confirmation confirmationMessage

	Handle  DIPHandler
	OnError func(error)
}

func (ts *DIPServer) start() {
	go func() {
		for {
			con, err := ts.listener.Accept()
			if err != nil {
				return
			}
			go func(con net.Conn) {
				defer con.Close()
				var res MessageHeader
				if err := binary.Read(con, binary.BigEndian, &res); err != nil {
					ts.OnError(err)
					return
				}
				var dih mtDirectIPHeader
				if err := binary.Read(con, binary.BigEndian, &dih); err != nil {
					ts.OnError(err)
					return
				}
				read := binary.Size(dih)
				var data []byte
				var prio *int
				for read < int(res.MessageLength) {
					var ph Header
					if err := binary.Read(con, binary.BigEndian, &ph); err != nil {
						ts.OnError(err)
						return
					}
					if ph.ID == mtPayloadID {
						data = make([]byte, ph.ElementLength)
						if _, err := con.Read(data); err != nil {
							ts.OnError(err)
							return
						}
						read += int(ph.ElementLength) + binary.Size(ph)
					}
					if ph.ID == mtMessagePriority {
						var mtp mtPriority
						if err := binary.Read(con, binary.BigEndian, &mtp); err != nil {
							ts.OnError(err)
							return
						}
						read += binary.Size(mtp)
						i := int(mtp.Level)
						prio = &i
					}
				}
				conf := ts.Handle(&res, &dih.DirectIPHeader, data, prio)
				confgMsg := confirmationMessage{
					MessageHeader: MessageHeader{
						ProtocolRevision: protocolRevision,
						MessageLength:    25,
					},
					Header: Header{
						ID:            mtConfirmationMsg,
						ElementLength: 25,
					},
					Confirmation: conf,
				}
				binary.Write(con, binary.BigEndian, &confgMsg)
			}(con)
		}
	}()
}

// Close closes the server so it does not accept any more connections.
func (ts *DIPServer) Close() {
	ts.listener.Close()
}

// Start starts listening. This function will block!
func (ts *DIPServer) Start() {
	ts.start()
}

func (ts *DIPServer) reset() {
	ts.Handle = handle
	ts.OnError = onError
}

// NewDIPServer returns a new directip server listening on the given address.
func NewDIPServer(address string) (*DIPServer, error) {
	ls, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	ts := DIPServer{
		address:  ls.Addr().String(),
		listener: ls,
	}
	ts.reset()
	return &ts, nil
}
