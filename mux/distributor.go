// Package mux provides a service to split incoming directip messages to backend HTTP
// services. The mux stores a list of targets and each target has a pattern for an IMEI.
// If the IMEI of the incoming message matches with the given regurlar expression, the mux
// will send an HTTP request with a JSON message to the configured backend.
//
// Every target service will receive a sbd.InformationElements as a JSON representation in its
// POST body. Please take into account that this service and package does not parse the payload
// which is of type []byte. Many devices use the payload to transfer specific types of data. Your
// backend service has to know how to handle these types.
package mux

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"

	"github.com/inconshreveable/log15"
	"github.com/protegear/sbd"
)

// A Target stores the configuration of a backend service where the SBD data should be pushed.
type Target struct {
	ID          string            `yaml:"id,omitempty"`
	IMEIPattern string            `yaml:"imeipattern"`
	Backend     string            `yaml:"backend"`
	SkipTLS     bool              `yaml:"skiptls,omitempty"`
	Header      map[string]string `yaml:"header"`
	imeipattern *regexp.Regexp
	client      *http.Client
}

// Targets is a list of Target's
type Targets []Target

// A Distributer can handle the SBD data and dispatches them to the targets. When
// the targets are reconfigured, the can be set vith WithTargets.
type Distributer interface {
	WithTargets(targets Targets) error
	Targets() Targets
	Handle(data *sbd.InformationBucket) error
	Close()
}

type distributer struct {
	log15.Logger
	targets       []Target
	sbdChannel    chan *sbdMessage
	configChannel chan Targets
}

type sbdMessage struct {
	data          sbd.InformationBucket
	returnedError chan error
}

// New creates a new Distributor with the given number of workers
func New(numworkers int, log log15.Logger) Distributer {
	sc := make(chan *sbdMessage)
	cc := make(chan Targets)
	s := &distributer{
		sbdChannel:    sc,
		configChannel: cc,
		Logger:        log,
	}
	for i := 0; i < numworkers; i++ {
		go s.run(i)
	}
	return s
}

func (f *distributer) Targets() Targets {
	return f.targets
}

func (f *distributer) WithTargets(targets Targets) error {
	var ar Targets
	for _, t := range targets {
		p, err := regexp.Compile(t.IMEIPattern)
		if err != nil {
			return fmt.Errorf("cannot compile patter: %q: %v", t.IMEIPattern, err)
		}
		t.imeipattern = p
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: t.SkipTLS,
			},
		}
		t.client = &http.Client{Transport: tr}

		ar = append(ar, t)
	}
	f.configChannel <- ar
	return nil
}

func (f *distributer) Handle(data *sbd.InformationBucket) error {
	return f.distribute(data)
}

func (f *distributer) distribute(data *sbd.InformationBucket) error {
	msg := &sbdMessage{data: *data, returnedError: make(chan error)}
	f.sbdChannel <- msg
	rerr := <-msg.returnedError
	close(msg.returnedError)
	return rerr
}

func (f *distributer) Close() {
	f.Info("close distributor")
	close(f.configChannel)
	close(f.sbdChannel)
}

func (f *distributer) run(worker int) {
	f.Info("start distributor service", "worker", worker)
	for {
		select {
		case cfg, more := <-f.configChannel:
			if !more {
				return
			}
			f.Info("set config", "config", cfg, "worker", worker)
			f.targets = cfg
		case msg := <-f.sbdChannel:
			go f.handle(msg)
		}
	}
}

func (f *distributer) handle(m *sbdMessage) {
	js, err := json.Marshal(m.data)
	if err != nil {
		m.returnedError <- err
		return
	}
	imei := m.data.Header.GetIMEI()
	for _, t := range f.targets {
		if t.imeipattern.MatchString(imei) {
			rq, err := http.NewRequest(http.MethodPost, t.Backend, bytes.NewBuffer(js))
			if err != nil {
				f.Error("cannot create request", "error", err, "target", t.Backend)
				m.returnedError <- err
				return
			}
			rq.Header.Add("Content-Type", "application/json")
			for k, v := range t.Header {
				rq.Header.Add(k, v)
			}
			rsp, err := t.client.Do(rq)
			if err != nil {
				f.Error("cannot call webhook", "target", t.Backend, "error", err)
				m.returnedError <- err
				return
			}
			defer rsp.Body.Close()
			content, _ := ioutil.ReadAll(rsp.Body)
			if rsp.StatusCode/100 == 2 {
				f.Info("data transmitted", "target", t.Backend, "status", rsp.Status, "content", string(content))
			} else {
				f.Error("data not transmitted", "target", t.Backend, "status", rsp.Status, "content", string(content))
				m.returnedError <- err
				return
			}
		}
	}
	m.returnedError <- nil
}
