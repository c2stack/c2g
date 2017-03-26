package restconf

import (
	"bytes"
	"io"
	"testing"

	"strings"

	"io/ioutil"

	"github.com/c2stack/c2g/c2"
	"github.com/c2stack/c2g/meta/yang"
	"github.com/c2stack/c2g/node"
)

func Test_DriverFlow(t *testing.T) {
	m := yang.RequireModuleFromString(nil, `module x {namespace ""; prefix ""; revision 0;
		container car {			
			container mileage {
				leaf odometer {
					type int32;
				}
				leaf trip {
					type int32;
				}
			}		
			container make {
				leaf model {
					type string;
				}
			}	
		}
}`)
	support := &testDriverFlowSupport{
		t: t,
	}
	expected := `{"mileage":{"odometer":1000}}`
	support.get = map[string]string{
		"car": expected,
	}
	d := &driver{support: support}
	b := node.NewBrowser(m, d.node())
	var actual bytes.Buffer
	if err := b.Root().Find("car").InsertInto(node.NewJsonWriter(&actual).Node()).LastErr; err != nil {
		t.Error(err)
	}
	if err := c2.CheckEqual(expected, actual.String()); err != nil {
		t.Error(err)
	}

	support.get = map[string]string{
		"car": `{}`,
	}
	expectedEdit := `{"mileage":{"odometer":1001}}`
	edit := node.ReadJson(expectedEdit)
	if err := b.Root().Find("car").UpsertFrom(edit).LastErr; err != nil {
		t.Error(err)
	}
	if err := c2.CheckEqual(expectedEdit, support.put["car"]); err != nil {
		t.Error(err)
	}
}

type testDriverFlowSupport struct {
	t    *testing.T
	get  map[string]string
	put  map[string]string
	post map[string]string
}

func (self *testDriverFlowSupport) driverSubs() map[string]*driverSub {
	panic("not implemented")
}

func (self *testDriverFlowSupport) driverDo(method string, params string, p *node.Path, payload io.Reader) (node.Node, error) {
	path := p.StringNoModule()
	self.t.Log(method, path, params)
	switch method {
	case "GET":
		in, found := self.get[path]
		if !found {
			return node.ErrorNode{Err: c2.NewErr("no response for " + path)}, nil
		}
		return node.NewJsonReader(strings.NewReader(in)).Node(), nil
	case "PUT":
		body, _ := ioutil.ReadAll(payload)
		self.put = map[string]string{
			path: string(body),
		}
	case "POST":
		body, _ := ioutil.ReadAll(payload)
		self.post = map[string]string{
			path: string(body),
		}
	}
	return nil, nil
}

func (self *testDriverFlowSupport) driverWebsocket() (io.Writer, error) {
	panic("not implemented")
}
