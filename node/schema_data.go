package node

import (
	"fmt"

	"github.com/c2stack/c2g/meta"
	"github.com/c2stack/c2g/meta/yang"
)

/**
 * This is used to encode YANG models. In order to navigate the YANG model it needs a model
 * which is the YANG YANG model.  Note: It can be confusing which is the data and which is the
 * goober.
 */
type SchemaData struct {
	// resolve all uses, groups and typedefs.  if this is false, then depth must be
	// used to avoid infinite recursion
	Resolve bool
}

func SelectModule(m *meta.Module, resolve bool) *Browser {
	return NewBrowser(yangModule(), SchemaData{Resolve: resolve}.Yang(m))
}

type MetaListSelector func(m meta.Meta) (Node, error)

func (self SchemaData) Yang(module *meta.Module) Node {
	s := &MyNode{}
	s.OnChild = func(r ChildRequest) (Node, error) {
		switch r.Meta.GetIdent() {
		case "module":
			return self.Module(module), nil
		}
		return nil, nil
	}
	return s
}

func (self SchemaData) Module(module *meta.Module) Node {
	return &Extend{
		Label: "Module",
		Node:  self.MetaList(module),
		OnChild: func(parent Node, r ChildRequest) (child Node, err error) {
			switch r.Meta.GetIdent() {
			case "revision":
				if r.New {
					module.Revision = &meta.Revision{}
				}
				if module.Revision != nil {
					return self.Revision(module.Revision), nil
				}
				return nil, nil
			}
			return parent.Child(r)
		},
	}
}

func (self SchemaData) Revision(rev *meta.Revision) Node {
	return &MyNode{
		OnField: func(r FieldRequest, hnd *ValueHandle) (err error) {
			switch r.Meta.GetIdent() {
			case "rev-date":
				if r.Write {
					rev.Ident = hnd.Val.Str
				} else {
					hnd.Val = &Value{Str: rev.Ident, Type: r.Meta.GetDataType()}
				}
			default:
				if r.Write {
					err = WriteField(r.Meta, rev, hnd.Val)
				} else {
					hnd.Val, err = ReadField(r.Meta, rev)
				}
			}
			return nil
		},
	}
}

func (self SchemaData) Type(typeData *meta.DataType) Node {
	return &MyNode{
		OnField: func(r FieldRequest, hnd *ValueHandle) (err error) {
			switch r.Meta.GetIdent() {
			case "ident":
				if r.Write {
					typeData.Ident = hnd.Val.Str
					typeData.SetFormat(meta.DataTypeImplicitFormat(hnd.Val.Str))
				} else {
					hnd.Val, err = SetValue(r.Meta.GetDataType(), typeData.Ident)
				}
			case "minLength":
				if r.Write {
					typeData.SetMinLength(hnd.Val.Int)
				} else {
					if self.Resolve || typeData.MinLengthPtr != nil {
						hnd.Val, err = SetValue(r.Meta.GetDataType(), typeData.MinLength())
					}
				}
			case "maxLength":
				if r.Write {
					typeData.SetMaxLength(hnd.Val.Int)
				} else {
					if self.Resolve || typeData.MaxLengthPtr != nil {
						hnd.Val, err = SetValue(r.Meta.GetDataType(), typeData.MaxLength())
					}
				}
			case "path":
				if r.Write {
					typeData.SetPath(hnd.Val.Str)
				} else {
					if self.Resolve || typeData.PathPtr != nil {
						hnd.Val, err = SetValue(r.Meta.GetDataType(), typeData.Path())
					}
				}
			case "enumeration":
				if r.Write {
					typeData.SetEnumeration(hnd.Val.Strlist)
				} else {
					if self.Resolve || len(typeData.EnumerationRef) > 0 {
						hnd.Val, err = SetValue(r.Meta.GetDataType(), typeData.Enumeration())
					}
				}
			}
			return
		},
	}
}

func (self SchemaData) Groupings(groupings meta.MetaList) Node {
	s := &MyNode{}
	i := listIterator{dataList: groupings, resolve: self.Resolve}
	s.OnNext = func(r ListRequest) (Node, []*Value, error) {
		var key = r.Key
		var group *meta.Grouping
		if r.New {
			group = &meta.Grouping{Ident: r.Key[0].Str}
			groupings.AddMeta(group)
		} else {
			if i.iterate(r.Selection, r.Meta, r.Key, r.First, r.Row) {
				group = i.data.(*meta.Grouping)
				if len(key) == 0 {
					key = SetValues(r.Meta.KeyMeta(), group.Ident)
				}
			}
		}
		if group != nil {
			return self.MetaList(group), key, nil
		}
		return nil, nil, nil
	}
	return s
}

func (self SchemaData) RpcIO(i *meta.RpcInput, o *meta.RpcOutput) Node {
	var io meta.MetaList
	if i != nil {
		io = i
	} else {
		io = o
	}
	return self.MetaList(io)
}

func (self SchemaData) createGroupingsTypedefsDefinitions(parent meta.MetaList, childMeta meta.Meta) meta.Meta {
	var child meta.Meta
	switch childMeta.GetIdent() {
	case "leaf":
		child = &meta.Leaf{}
	case "anyxml":
		child = &meta.Any{}
	case "leaf-list":
		child = &meta.LeafList{}
	case "container":
		child = &meta.Container{}
	case "list":
		child = &meta.List{}
	case "uses":
		child = &meta.Uses{}
	case "grouping":
		child = &meta.Grouping{}
	case "typedef":
		child = &meta.Typedef{}
	case "rpc", "action":
		child = &meta.Rpc{}
	case "notification":
		child = &meta.Notification{}
	case "choice":
		child = &meta.Choice{}
	case "case":
		child = &meta.ChoiceCase{}
	default:
		panic("Unknown type:" + childMeta.GetIdent())
	}
	parent.AddMeta(child)
	return child
}

func (self SchemaData) Rpc(rpc *meta.Rpc) Node {
	return &Extend{
		Label: "rpc",
		Node:  ReflectNode(rpc),
		OnChild: func(parent Node, r ChildRequest) (Node, error) {
			switch r.Meta.GetIdent() {
			case "input":
				if r.New {
					rpc.AddMeta(&meta.RpcInput{})
				}
				if rpc.Input != nil {
					return self.RpcIO(rpc.Input, nil), nil
				}
				return nil, nil
			case "output":
				if r.New {
					rpc.AddMeta(&meta.RpcOutput{})
				}
				if rpc.Output != nil {
					return self.RpcIO(nil, rpc.Output), nil
				}
				return nil, nil
			}
			return parent.Child(r)
		},
	}
}

func (self SchemaData) Typedefs(typedefs meta.MetaList) Node {
	s := &MyNode{}
	i := listIterator{dataList: typedefs, resolve: self.Resolve}
	s.OnNext = func(r ListRequest) (Node, []*Value, error) {
		var key = r.Key
		var typedef *meta.Typedef
		if r.New {
			typedef = &meta.Typedef{Ident: r.Key[0].Str}
			typedefs.AddMeta(typedef)
		} else {
			if i.iterate(r.Selection, r.Meta, r.Key, r.First, r.Row) {
				typedef = i.data.(*meta.Typedef)
				if len(key) == 0 {
					key = SetValues(r.Meta.KeyMeta(), typedef.Ident)
				}
			}
		}
		if typedef != nil {
			return self.Typedef(typedef), key, nil
		}
		return nil, nil, nil
	}
	return s
}

func (self SchemaData) Typedef(typedef *meta.Typedef) Node {
	return &Extend{
		Label: "Typedef",
		Node:  ReflectNode(typedef),
		OnChild: func(parent Node, r ChildRequest) (Node, error) {
			switch r.Meta.GetIdent() {
			case "type":
				if r.New {
					typedef.SetDataType(&meta.DataType{Parent: typedef})
				}
				if typedef.DataType != nil {
					return self.Type(typedef.DataType), nil
				}
			}
			return nil, nil
		},
	}
}

func (self SchemaData) MetaList(data meta.MetaList) Node {
	var details *meta.Details
	if hasDetails, ok := data.(meta.HasDetails); ok {
		details = hasDetails.Details()
	}
	return &Extend{
		Label: "MetaList",
		Node:  ReflectNode(data),
		OnChild: func(parent Node, r ChildRequest) (Node, error) {
			hasGroupings, implementsHasGroupings := data.(meta.HasGroupings)
			hasTypedefs, implementsHasTypedefs := data.(meta.HasTypedefs)
			switch r.Meta.GetIdent() {
			case "groupings":
				if !self.Resolve && implementsHasGroupings {
					groupings := hasGroupings.GetGroupings()
					if r.New || !meta.ListEmpty(groupings) {
						return self.Groupings(groupings), nil
					}
				}
				return nil, nil
			case "typedefs":
				if !self.Resolve && implementsHasTypedefs {
					typedefs := hasTypedefs.GetTypedefs()
					if r.New || !meta.ListEmpty(typedefs) {
						return self.Typedefs(typedefs), nil
					}
				}
				return nil, nil
			case "definitions":
				defs := data.(meta.MetaList)
				if r.New || !meta.ListEmpty(defs) {
					return self.Definitions(defs), nil
				}
				return nil, nil
			}
			return parent.Child(r)
		},
		OnField: func(p Node, r FieldRequest, hnd *ValueHandle) (err error) {
			switch r.Meta.GetIdent() {
			case "config":
				if r.Write {
					details.SetConfig(hnd.Val.Bool)
				} else {
					if self.Resolve || details.ConfigPtr != nil {
						hnd.Val = &Value{Bool: details.Config(r.Selection.Path), Type: r.Meta.GetDataType()}
					}
				}
			case "mandatory":
				if r.Write {
					details.SetMandatory(hnd.Val.Bool)
				} else {
					if self.Resolve || details.MandatoryPtr != nil {
						hnd.Val = &Value{Bool: details.Mandatory(), Type: r.Meta.GetDataType()}
					}
				}
			default:
				if r.Write {
					err = WriteField(r.Meta, data, hnd.Val)
				} else {
					hnd.Val, err = ReadField(r.Meta, data)
				}
			}
			return
		},
	}
}

func (self SchemaData) Leaf(leaf *meta.Leaf, leafList *meta.LeafList, any *meta.Any) Node {
	var leafy meta.HasDataType
	if leaf != nil {
		leafy = leaf
	} else if leafList != nil {
		leafy = leafList
	} else {
		leafy = any
	}
	s := &MyNode{
		Peekable: leafy,
	}
	details := leafy.(meta.HasDetails).Details()
	s.OnChild = func(r ChildRequest) (Node, error) {
		switch r.Meta.GetIdent() {
		case "type":
			if r.New {
				leafy.SetDataType(&meta.DataType{Parent: leafy})
			}
			if leafy.GetDataType() != nil {
				return self.Type(leafy.GetDataType()), nil
			}
		}
		return nil, nil
	}
	s.OnField = func(r FieldRequest, hnd *ValueHandle) (err error) {
		switch r.Meta.GetIdent() {
		case "config":
			if r.Write {
				details.SetConfig(hnd.Val.Bool)
			} else {
				if self.Resolve || details.ConfigPtr != nil {
					hnd.Val = &Value{Bool: details.Config(r.Selection.Path), Type: r.Meta.GetDataType()}
				}
			}
		case "mandatory":
			if r.Write {
				details.SetMandatory(hnd.Val.Bool)
			} else {
				if self.Resolve || details.MandatoryPtr != nil {
					hnd.Val = &Value{Bool: details.Mandatory(), Type: r.Meta.GetDataType()}
				}
			}
		default:
			if r.Write {
				WriteField(r.Meta, leafy, hnd.Val)
			} else {
				hnd.Val, err = ReadField(r.Meta, leafy)
			}
		}
		return

	}
	return s
}

func (self SchemaData) Uses(data *meta.Uses) Node {
	// TODO: uses has refine container(s)
	return ReflectNode(data)
}

func (self SchemaData) Cases(choice *meta.Choice) Node {
	s := &MyNode{
		Peekable: choice,
	}
	i := listIterator{dataList: choice, resolve: self.Resolve}
	s.OnNext = func(r ListRequest) (Node, []*Value, error) {
		key := r.Key
		var choiceCase *meta.ChoiceCase
		if r.New {
			choiceCase = &meta.ChoiceCase{}
			choice.AddMeta(choiceCase)
		} else {
			if i.iterate(r.Selection, r.Meta, key, r.First, r.Row) {
				choiceCase = i.data.(*meta.ChoiceCase)
				key = SetValues(r.Meta.KeyMeta(), choiceCase.Ident)
			}
		}
		if choiceCase != nil {
			return self.MetaList(choiceCase), key, nil
		}
		return nil, nil, nil
	}
	return s
}

func (self SchemaData) Choice(data *meta.Choice) Node {
	return &Extend{
		Label: "Choice",
		Node:  ReflectNode(data),
		OnChild: func(parent Node, r ChildRequest) (Node, error) {
			switch r.Meta.GetIdent() {
			case "cases":
				// TODO: Not sure how to do create w/o what type to create
				return self.Cases(data), nil
			}
			return nil, nil
		},
	}
}

type listIterator struct {
	data     meta.Meta
	dataList meta.MetaList
	iterator meta.MetaIterator
	resolve  bool
	temp     int
}

func (i *listIterator) iterate(sel Selection, m *meta.List, key []*Value, first bool, row int) bool {
	i.data = nil
	if i.dataList == nil {
		return false
	}
	if len(key) > 0 {
		sel.Path.key = key
		if first {
			i.data = meta.FindByIdent2(i.dataList, key[0].Str)
		}
	} else {
		if first {
			i.iterator = meta.NewMetaListIterator(i.dataList, i.resolve)
			for j := 0; j < row && i.iterator.HasNextMeta(); j++ {
			}
		}
		if i.iterator.HasNextMeta() {
			i.data = i.iterator.NextMeta()
			if i.data == nil {
				panic(fmt.Sprintf("Bad iterator at %s, item number %d", sel.String(), i.temp))
			}
			sel.Path.key = SetValues(m.KeyMeta(), i.data.GetIdent())
		}
	}
	return i.data != nil
}

func (self SchemaData) Definition(parent meta.MetaList, data meta.Meta) Node {
	s := &MyNode{
		Peekable: data,
	}
	s.OnChoose = func(state Selection, choice *meta.Choice) (m *meta.ChoiceCase, err error) {
		caseType := self.DefinitionType(data)
		return choice.GetCase(caseType), nil
	}
	s.OnChild = func(r ChildRequest) (Node, error) {
		if r.New {
			data = self.createGroupingsTypedefsDefinitions(parent, r.Meta)
		}
		if data == nil {
			return nil, nil
		}
		switch r.Meta.GetIdent() {
		case "anyxml":
			return self.Leaf(nil, nil, data.(*meta.Any)), nil
		case "leaf":
			return self.Leaf(data.(*meta.Leaf), nil, nil), nil
		case "leaf-list":
			return self.Leaf(nil, data.(*meta.LeafList), nil), nil
		case "uses":
			return self.Uses(data.(*meta.Uses)), nil
		case "choice":
			return self.Choice(data.(*meta.Choice)), nil
		case "rpc", "action":
			return self.Rpc(data.(*meta.Rpc)), nil
		default:
			return self.MetaList(data.(meta.MetaList)), nil
		}
		return nil, nil
	}
	s.OnField = func(r FieldRequest, hnd *ValueHandle) (err error) {
		if r.Write {
			if data != nil {
				err = WriteField(r.Meta, data, hnd.Val)
			}
		} else {
			hnd.Val, err = ReadField(r.Meta, data)
		}
		return
	}
	return s
}

func (self SchemaData) Definitions(dataList meta.MetaList) Node {
	s := &MyNode{
		Peekable: dataList,
	}
	i := listIterator{dataList: dataList, resolve: self.Resolve}
	s.OnNext = func(r ListRequest) (Node, []*Value, error) {
		key := r.Key
		if r.New {
			return self.Definition(dataList, nil), key, nil
		} else {
			if i.iterate(r.Selection, r.Meta, r.Key, r.First, r.Row) {
				if len(key) == 0 {
					key = SetValues(r.Meta.KeyMeta(), i.data.GetIdent())
				}
				return self.Definition(dataList, i.data), key, nil
			}
		}
		return nil, nil, nil
	}
	return s
}

func (self SchemaData) DefinitionType(data meta.Meta) string {
	switch data.(type) {
	case *meta.List:
		return "list"
	case *meta.Uses:
		return "uses"
	case *meta.Choice:
		return "choice"
	case *meta.Any:
		return "anyxml"
	case *meta.Notification:
		return "notification"
	case *meta.Rpc:
		return "action"
	case *meta.Leaf:
		return "leaf"
	case *meta.LeafList:
		return "leaf-list"
	default:
		return "container"
	}
}

var yangYang *meta.Module

func yangModule() *meta.Module {
	if yangYang == nil {
		var err error
		if yangYang, err = yang.LoadModuleCustomImport(yangYangStr, nil); err != nil {
			panic(err)
		}
	}
	return yangYang
}

var yangYangStr = `
module yang {
    namespace "http://schema.org/yang";
    prefix "schema";
    description "Yang definition of yang";
    revision 0 {
        description "Yang 1.0 with some 1.1 features";
    }

	uses module;	

    grouping def-header {
        leaf ident {
            type string;
        }
        leaf description {
            type string;
        }
    }

    grouping type {
        container type {
            leaf ident {
                type string;
            }
            leaf range {
                type string;
            }
            leaf-list enumeration {
                type string;
            }
            leaf path {
                type string;
            }
            leaf minLength {
                type int32;
            }
            leaf maxLength {
                type int32;
            }
        }
    }

    grouping groupings-typedefs {
        list groupings {
            key "ident";
            uses def-header;

            /*
              !! CIRCULAR
            */
            uses groupings-typedefs;
            uses containers-lists-leafs-uses-choice;
        }
        list typedefs {
            key "ident";
            uses def-header;
            uses type;
        }
    }

    grouping has-details {
	leaf config {
	    type boolean;
	}
	leaf mandatory {
	    type boolean;
	}
    }

    grouping containers-lists-leafs-uses-choice {
        list definitions {
            key "ident";
            leaf ident {
            	type string;
            }
            choice body-stmt {
                case container {
                    container container {
                        uses def-header;
                        uses has-details;
                        uses groupings-typedefs;
                        uses containers-lists-leafs-uses-choice;
                        /*uses notifications; */
                    }
                }
                case list {
                    container list {
                        leaf-list key {
                            type string;
                        }
                        uses def-header;
                        uses has-details;
                        uses groupings-typedefs;
                        uses containers-lists-leafs-uses-choice;
                        /* uses notifications; */
                    }
                }
                case leaf {
                    container leaf {
                        uses def-header;
                        uses has-details;
                        uses type;
                    }
                }
                case anyxml {
                    container anyxml {
                        uses def-header;
                        uses has-details;
                        uses type;
                    }
                }
                case leaf-list {
                    container leaf-list {
                        uses def-header;
                        uses has-details;
                        uses type;
                    }
                }
                case uses {
                    container uses {
                        uses def-header;
                        /* need to expand this to use refine */
                    }
                }
                case choice {
                    container choice {
                        uses def-header;
                        list cases {
                            key "ident";
                            leaf ident {
                                type string;
                            }
                            /*
                             !! CIRCULAR
                            */
                            uses containers-lists-leafs-uses-choice;
                        }
                    }
                }
                case notification {
                    container notification {
			    uses def-header;
			    uses groupings-typedefs;
			    uses containers-lists-leafs-uses-choice;
                    }
                }
                case action {
                    container action {
			    uses def-header;
			    uses def-header;
			    container input {
				uses groupings-typedefs;
				uses containers-lists-leafs-uses-choice;
			    }
			    container output {
				uses groupings-typedefs;
				uses containers-lists-leafs-uses-choice;
			    }
                    }
                }
            }
        }
    }

    grouping module {
    	container module {
			uses def-header;
			leaf namespace {
				type string;
			}
			leaf prefix {
				type string;
			}
			container revision {
				leaf rev-date {
					type string;
				}
				leaf description {
					type string;
				}
			}
			uses groupings-typedefs;
			uses containers-lists-leafs-uses-choice;
		}
	}
}`
