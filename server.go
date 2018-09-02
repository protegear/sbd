package sbd

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"

	"github.com/inconshreveable/log15"
)

type SBDHandler interface {
	Handle(data *InformationBucket) error
}

type SBDHandlerFunc func(data *InformationBucket) error

func (f SBDHandlerFunc) Handle(data *InformationBucket) error {
	return f(data)
}

func Logger(log log15.Logger, next SBDHandler) SBDHandler {
	return SBDHandlerFunc(func(data *InformationBucket) error {
		js, err := json.Marshal(data)
		if err != nil {
			return err
		}
		log.Info("new data", "elements", string(js))
		return next.Handle(data)
	})
}

type result struct {
	Header
	MOConfirmationMessage
}

func createResult(status byte) *result {
	return &result{Header: Header{ID: moConfirmationID, ElementLength: 1}, MOConfirmationMessage: MOConfirmationMessage{Status: status}}
}
func NewService(log log15.Logger, address string, h SBDHandler) error {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("cannot open listening address %q: %v", address, err)
	}
	defer l.Close()
	for {
		// Wait for a connection.
		conn, err := l.Accept()
		if err != nil {
			log.Crit("cannot accept", "error", err)
			// let it crash! it's up to the caller of the prog to restart it
			panic(err)
		}

		go func(c net.Conn) {
			// directip connects, sends message and closes connection, so no whilte loop is needed
			// to read more than one message from the connection
			defer c.Close()
			log.Info("new connection")
			el, err := GetElements(c)
			res := createResult(0)
			if err != nil {
				log.Error("cannot get elements from connection", "error", err)
				binary.Write(c, binary.BigEndian, res)
				return
			}
			log.Info("received data", "elements", el)
			err = h.Handle(el)
			if err != nil {
				log.Error("error handling message", "error", err)
			} else {
				res.Status = 1
			}
			log.Info("write response", "result", res)
			binary.Write(c, binary.BigEndian, res)
		}(conn)
	}
}
