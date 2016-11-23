package node

import (
	"bytes"
	"strings"
	"testing"

	"github.com/c2stack/c2g/meta/yang"
)

var mstr = `
module m {
	namespace "";
	prefix "";
	revision 0;
	container a {
		container b {
			leaf x {
				type string;
			}
		}
	}
	list p {
		key "k";
		leaf k {
			type string;
		}
		container q {
			leaf s {
				type string;
			}
		}
		list r {
			leaf z {
				type int32;
			}
		}
	}
}
`

func TestCollectionWriteNoKey(t *testing.T) {
	m, err := yang.LoadModuleCustomImport(mstr, nil)
	if err != nil {
		t.Fatal(err)
	}
	sel := NewBrowser2(m, MapNode(make(map[string]interface{}))).Root()
	data := `{"p":[{"q":{"s":"hi"}}]}`
	err = sel.InsertFrom(NewJsonReader(strings.NewReader(data)).Node()).LastErr
	if err == nil {
		t.Error("should be rejected")
	} else if !strings.HasPrefix(err.Error(), "no key") {
		t.Log("wrong expected error ", err)
	}
}

func TestCollectionWrite(t *testing.T) {
	m, err := yang.LoadModuleCustomImport(mstr, nil)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		data string
		path string
	}{
		{
			`{"a":{"b":{"x":"waldo"}}}`,
			"a.b.x",
		},
		{
			`{"p":[{"k":"walter"},{"k":"waldo"},{"k":"weirdo"}]}`,
			"p.1.k",
		},
	}
	for _, test := range tests {
		root := make(map[string]interface{})
		bd := MapNode(root)
		sel := NewBrowser2(m, bd).Root()
		if err = sel.InsertFrom(NewJsonReader(strings.NewReader(test.data)).Node()).LastErr; err != nil {
			t.Error(err)
		}
		actual := MapValue(root, test.path)
		if actual != "waldo" {
			t.Error(actual)
		}
	}
}

func TestCollectionRead(t *testing.T) {
	m := YangFromString(mstr)
	tests := []struct {
		root     map[string]interface{}
		expected string
	}{
		{
			map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"x": "waldo",
					},
				},
			},
			`{"a":{"b":{"x":"waldo"}}}`,
		},
		{
			map[string]interface{}{
				"p": []interface{}{
					map[string]interface{}{"k": "walter"},
					map[string]interface{}{"k": "waldo"},
					map[string]interface{}{"k": "weirdo"},
				},
			},
			`{"p":[{"k":"walter"},{"k":"waldo"},{"k":"weirdo"}]}`,
		},
	}
	for _, test := range tests {
		bd := MapNode(test.root)
		var buff bytes.Buffer
		sel := NewBrowser2(m, bd).Root()
		if err := sel.InsertInto(NewJsonWriter(&buff).Node()).LastErr; err != nil {
			t.Error(err)
		}
		actual := buff.String()
		if actual != test.expected {
			t.Errorf("\nExpected:%s\n  Actual:%s", test.expected, actual)
		}
	}
}

func TestCollectionDelete(t *testing.T) {
	m := YangFromString(mstr)
	tests := []struct {
		root     map[string]interface{}
		path     string
		expected string
	}{
		{
			map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"x": "waldo",
					},
				},
			},
			"a/b",
			`{"a":{}}`,
		},
		{
			map[string]interface{}{
				"p": []interface{}{
					map[string]interface{}{"k": "walter"},
					map[string]interface{}{"k": "waldo"},
					map[string]interface{}{"k": "weirdo"},
				},
			},
			"p=walter",
			`{"p":[{"k":"waldo"},{"k":"weirdo"}]}`,
		},
	}
	for _, test := range tests {
		bd := MapNode(test.root)
		sel := NewBrowser2(m, bd).Root()

		if err := sel.Find(test.path).Delete(); err != nil {
			t.Error(err)
		}

		var buff bytes.Buffer
		if err := sel.InsertInto(NewJsonWriter(&buff).Node()).LastErr; err != nil {
			t.Error(err)
		}
		actual := buff.String()

		if actual != test.expected {
			t.Errorf("\nExpected:%s\n  Actual:%s", test.expected, actual)
		}
	}
}
