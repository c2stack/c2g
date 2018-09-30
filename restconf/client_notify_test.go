package restconf

import (
	"strings"
	"testing"
	"time"

	"github.com/freeconf/gconf/device"
	"github.com/freeconf/gconf/meta"
	"github.com/freeconf/gconf/meta/yang"
	"github.com/freeconf/gconf/node"
	"github.com/freeconf/gconf/nodes"
)

func TestClientNotif(t *testing.T) {
	// t.Skip("Fails until we figure out how to get WS connections to autoconnect")
	ypath := meta.PathStreamSource("./testdata:../yang")
	m := yang.RequireModule(ypath, "x")
	var msgs chan string
	var s *Server
	msgs = make(chan string, 1)
	msgs <- "original session"
	connect := func() {
		n := &nodes.Basic{
			OnNotify: func(r node.NotifyRequest) (node.NotifyCloser, error) {
				go func() {
					for s := range msgs {
						r.Send(nodes.ReflectChild(map[string]interface{}{
							"z": s,
						}))
					}
				}()
				return func() error {
					return nil
				}, nil
			},
		}
		b := node.NewBrowser(m, n)
		d := device.New(ypath)
		d.AddBrowser(b)
		s = NewServer(d)
		err := d.ApplyStartupConfig(strings.NewReader(`
		{
			"restconf" : {
				"web": {
					"port" : ":9080"
				},
				"debug" : true
			}
		}`))
		if err != nil {
			t.Fatal(err)
		}
	}
	connect()
	<-time.After(2 * time.Second)
	factory := Client{YangPath: ypath}
	c, err := factory.NewDevice("http://localhost:9080/restconf")
	if err != nil {
		t.Fatal(err)
	}
	b, err := c.Browser("x")
	if err != nil {
		t.Fatal(err)
	}
	wait := make(chan string)
	sub, err := b.Root().Find("y").Notifications(func(sel node.Selection) {
		actual, err := nodes.WriteJSON(sel)
		if err != nil {
			t.Fatal(err)
		}
		wait <- actual
	})
	if err != nil {
		t.Fatal(err)
	}
	msg := <-wait
	if msg != `{"z":"original session"}` {
		t.Error(msg)
	}
	sub()
	s.Close()
}
