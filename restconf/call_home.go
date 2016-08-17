package restconf

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/dhubler/c2g/browse"
	"github.com/dhubler/c2g/c2"
	"github.com/dhubler/c2g/meta"
	"github.com/dhubler/c2g/node"
)

// Implements RFC Draft in spirit-only
//   https://tools.ietf.org/html/draft-ietf-netconf-call-home-17
//
// Draft calls for server-initiated registration and this implementation is client-initiated
// which may or may-not be part of the final draft.  Client-initiated registration at first
// glance appears to be more useful, but this may prove to be a wrong assumption on my part.
//
type CallHome struct {
	Module             *meta.Module
	ControllerAddress  string
	EndpointAddress    string
	EndpointId         string
	Registration       *Registration
	ClientSource       browse.ClientSource
	RegistrationRateMs int
	registerTimer      *time.Ticker
}

type Registration struct {
	Id string
}

func (self *CallHome) Manage() node.Node {
	return &node.Extend{
		Node: node.MarshalContainer(self),
		OnSelect: func(p node.Node, r node.ContainerRequest) (node.Node, error) {
			switch r.Meta.GetIdent() {
			case "registration":
				if self.Registration != nil {
					return node.MarshalContainer(self.Registration), nil
				}
			}
			return nil, nil
		},
		OnEvent: func(p node.Node, sel *node.Selection, e node.Event) error {
			switch e.Type {
			case node.LEAVE_EDIT:
				// We wait for 1 second because on initial configuration load the
				// callback url isn't valid until the web server is also configured.
				time.AfterFunc(1*time.Second, func() {
					if err := self.StartRegistration(); err != nil {
						c2.Err.Printf("Initial registration failed %s", err)
					}
				})
			}
			return p.Event(sel, e)
		},
	}
}

func (self *CallHome) StartRegistration() error {
	firstRegistrationErr := self.Call()
	if self.registerTimer != nil {
		self.registerTimer.Stop()
	}
	if self.RegistrationRateMs > 0 {
		// Even if we fail to register, keep trying
		self.registerTimer = time.NewTicker(time.Duration(self.RegistrationRateMs) * time.Millisecond)
		go func() {
			for range self.registerTimer.C {
				if err := self.Call(); err != nil {
					c2.Err.Printf("Error trying to register %s", err)
				}
			}
		}()
	}
	return firstRegistrationErr
}

func (self *CallHome) Call() (err error) {
	var req *http.Request
	c2.Info.Printf("Registering controller %s", self.ControllerAddress)
	if req, err = http.NewRequest("POST", self.ControllerAddress, nil); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	payload := fmt.Sprintf(`{"module":"%s","id":"%s","endpointAddress":"%s"}`, self.Module.GetIdent(),
		self.EndpointId, self.EndpointAddress)
	req.Body = ioutil.NopCloser(strings.NewReader(payload))
	client := self.ClientSource.GetHttpClient()
	resp, getErr := client.Do(req)
	if getErr != nil {
		return getErr
	}
	defer resp.Body.Close()
	respBytes, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return c2.NewErrC(string(respBytes), resp.StatusCode)
	}
	var rc map[string]interface{}
	if err = json.Unmarshal(respBytes, &rc); err != nil {
		return err
	}
	self.Registration = &Registration{
		Id: rc["id"].(string),
	}
	return nil
}
