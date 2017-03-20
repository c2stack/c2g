package node

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"context"

	"github.com/c2stack/c2g/meta"
	"github.com/c2stack/c2g/meta/yang"
)

// Example:
//  DataValue(data, "foo.10.bar.blah.0")
func MapValue(container map[string]interface{}, path string) interface{} {
	segments := strings.Split(path, ".")
	var v interface{}
	v = container
	for _, seg := range segments {
		switch x := v.(type) {
		case []map[string]interface{}:
			n, _ := strconv.Atoi(seg)
			v = x[n]
		case []interface{}:
			n, _ := strconv.Atoi(seg)
			v = x[n]
		case map[string]interface{}:
			v = x[seg]
		default:
			panic(fmt.Sprintf("Bad type %s on %s", reflect.TypeOf(v), seg))
		}
		if v == nil {
			return nil
		}
	}
	return v
}

func Singleton(name string, f ChildFunc) Node {
	return &MyNode{
		OnChild: func(r ChildRequest) (Node, error) {
			if r.Meta.GetIdent() == name {
				return f(r)
			}
			return nil, nil
		},
	}
}

func RenameMeta(m meta.Meta, rename string) {
	switch m := m.(type) {
	case *meta.Container:
		m.Ident = rename
	case *meta.Module:
		m.Ident = rename
	case *meta.List:
		m.Ident = rename
	case *meta.Leaf:
		m.Ident = rename
	case *meta.LeafList:
		m.Ident = rename
	case *meta.Choice:
		m.Ident = rename
	case *meta.ChoiceCase:
		m.Ident = rename
	case *meta.Any:
		m.Ident = rename
	default:
		panic("rename not supported on " + reflect.TypeOf(m).Name())
	}
}

// Copys meta while expanding all groups and typedefs.  This has the potentional
// to dramatically increase the size of your meta and more dangerously, go into infinite
// recursion on recursive metas
func DecoupledMetaCopy(yangPath meta.StreamSource, src meta.MetaList) meta.MetaList {
	yangModule := yang.RequireModule(yangPath, "yang")
	var copy meta.MetaList
	m := meta.FindByPath(yangModule, "module/definitions")
	if meta.IsList(src) {
		m = meta.FindByIdentExpandChoices(m, "list")
		copy = &meta.List{}
	} else {
		m = meta.FindByIdentExpandChoices(m, "container")
		copy = &meta.Container{}
	}
	srcNode := SchemaData{true}.MetaList(src)
	destNode := SchemaData{true}.MetaList(copy)
	NewBrowser(m.(meta.MetaList), srcNode).Root().InsertInto(context.Background(), destNode)
	return copy
}
