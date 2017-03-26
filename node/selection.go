package node

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"context"

	"github.com/c2stack/c2g/c2"
	"github.com/c2stack/c2g/meta"
)

// Selection is a link between a data node and a model definition.  It also has a path
// that represents where in the tree or data nodes this selection is located. A Selection
// can be used to operate on data or find other selection.
type Selection struct {
	Browser *Browser
	Parent  *Selection
	Node    Node
	Path    *Path

	// Useful when navigating lists, True if this selector is List node, False if
	// this is for an item in List node.
	InsideList bool

	// Constraints hold list of things to check when walking or editing a node.
	Constraints *Constraints

	// Handler let's you alter what happens when a contraints finds an error
	Handler *ConstraintHandler

	LastErr error
}

func (self Selection) Meta() meta.Meta {
	return self.Path.meta
}

// This selection points nowhere and must have been returned from a function that didn't find
// another selection
func (self Selection) IsNil() bool {
	return self.Path == nil
}

// Create a new independant selection with a different browser from this point in the tree based on a whole
// new data node
func (self Selection) Split(node Node) Selection {
	fork := self
	fork.Parent = nil
	fork.Browser = NewBrowser(self.Path.meta.(meta.MetaList), node)
	fork.Constraints = &Constraints{}
	fork.Node = node
	return fork
}

// If this is a selection in a list, this is the key value of that list item.
func (self Selection) Key() []*Value {
	return self.Path.key
}

func (self Selection) String() string {
	return fmt.Sprint(self.Node.String(), ":", self.Path.String())
}

func (self Selection) Select(r *ChildRequest) Selection {
	// check pre-constraints
	if self.Constraints != nil {
		r.Constraints = self.Constraints
		r.ConstraintsHandler = self.Handler
		if proceed, constraintErr := self.Constraints.CheckContainerPreConstraints(r); !proceed || constraintErr != nil {
			return Selection{LastErr: constraintErr}
		}
	}

	// select node
	var child Selection
	childNode, err := self.Node.Child(*r)
	if err != nil {
		child = Selection{LastErr: err}
	} else if childNode == nil {
		child = Selection{}
	} else {
		child = Selection{
			Browser:     self.Browser,
			Parent:      &self,
			Path:        &Path{parent: self.Path, meta: r.Meta},
			Node:        childNode,
			Constraints: self.Constraints,
			Handler:     self.Handler,
		}
	}

	// check post-constraints
	if self.Constraints != nil {
		if proceed, constraintErr := self.Constraints.CheckContainerPostConstraints(*r, child); !proceed || constraintErr != nil {
			return Selection{LastErr: constraintErr}
		}
	}

	return child
}

func (self Selection) SelectListItem(r *ListRequest) (Selection, []*Value) {
	// check pre-constraints
	if self.Constraints != nil {
		r.Constraints = self.Constraints
		r.ConstraintsHandler = self.Handler
		if proceed, constraintErr := self.Constraints.CheckListPreConstraints(r); !proceed || constraintErr != nil {
			return Selection{LastErr: constraintErr}, nil
		}
	}

	// select node
	var child Selection
	childNode, key, err := self.Node.Next(*r)
	if err != nil {
		child = Selection{LastErr: err}
	} else if childNode == nil {
		child = Selection{}
	} else {
		var parentPath *Path
		if self.Parent != nil {
			parentPath = self.Parent.Path
		}
		child = Selection{
			Browser: self.Browser,
			Parent:  &self,
			Node:    childNode,
			// NOTE: Path.parent is lists parentPath, not self.path
			Path:        &Path{parent: parentPath, meta: self.Path.meta, key: key},
			InsideList:  true,
			Constraints: self.Constraints,
			Handler:     self.Handler,
		}
	}

	// check post-constraints
	if self.Constraints != nil {
		if proceed, constraintErr := self.Constraints.CheckListPostConstraints(*r, child, r.Selection.Path.key); !proceed || constraintErr != nil {
			return Selection{LastErr: constraintErr}, nil
		}
	}

	return child, key
}

func (self Selection) Peek() interface{} {
	return self.Node.Peek(self)
}

func isFwdSlash(r rune) bool {
	return r == '/'
}

func (self Selection) IsConfig(m meta.Meta) bool {
	if hasDetails, ok := m.(meta.HasDetails); ok {
		return hasDetails.Details().Config(self.Path)
	}
	return true
}

// Find navigates to another selector automatically applying constraints to returned selector.
// This supports paths that start with any number of "../" where FindUrl does not.
func (self Selection) Find(path string) Selection {
	p := path
	s := self
	for strings.HasPrefix(p, "../") {
		if s.Parent == nil {
			s.LastErr = c2.NewErrC("No parent path to resolve "+p, 404)
			return s
		} else {
			s = *s.Parent
			p = p[3:]
		}
	}
	var u *url.URL
	u, s.LastErr = url.Parse(p)
	if s.LastErr != nil {
		return s
	}
	return s.FindUrl(u)
}

// FindUrl navigates to another selection with possible constraints as url parameters.  Constraints
// are added to any existing contraints.  Original selector and constraints will remain unaltered
func (self Selection) FindUrl(url *url.URL) Selection {
	if self.LastErr != nil || url == nil {
		return self
	}
	var targetSlice PathSlice
	targetSlice, self.LastErr = ParseUrlPath(url, self.Meta())
	if self.LastErr != nil {
		return self
	}
	if len(url.Query()) > 0 {
		buildConstraints(&self, url.Query())
		if self.LastErr != nil {
			return self
		}
	}
	return self.FindSlice(targetSlice)
}

// Apply constraints in the form of url parameters.
// Original selector and constraints will remain unaltered
// Example:
//     sel2 = sel.Constrain("content=config&depth=4")
//  sel will not have content or depth constraints applies, but sel 2 will
func (self Selection) Constrain(params string) Selection {
	if self.LastErr != nil {
		return self
	}
	if dummy, err := url.Parse("bogus?" + params); err != nil {
		self.LastErr = err
		return self
	} else {
		buildConstraints(&self, dummy.Query())
	}
	return self
}

func buildConstraints(self *Selection, params map[string][]string) {
	constraints := NewConstraints(self.Constraints)
	maxDepth := MaxDepth{MaxDepth: 32}
	if n, found := findIntParam(params, "depth"); found {
		maxDepth.MaxDepth = n
	}
	constraints.AddConstraint("depth", 10, 50, maxDepth)
	if p, found := params["c2-range"]; found {
		if listSelector, selectorErr := NewListRange(p[0]); selectorErr != nil {
			self.LastErr = selectorErr
			return
		} else {
			constraints.AddConstraint("c2-range", 20, 50, listSelector)
		}
	}
	if p, found := params["fields"]; found {
		if listSelector, selectorErr := NewFieldsMatcher(p[0]); selectorErr != nil {
			self.LastErr = selectorErr
			return
		} else {
			constraints.AddConstraint("fields", 10, 50, listSelector)
		}
	}
	if p, found := params["c2-xfields"]; found {
		if listSelector, selectorErr := NewExcludeFieldsMatcher(p[0]); selectorErr != nil {
			self.LastErr = selectorErr
			return
		} else {
			constraints.AddConstraint("c2-xfields", 10, 50, listSelector)
		}
	}
	maxNode := MaxNode{Max: 10000}
	if n, found := findIntParam(params, "c2-max-node-count"); found {
		maxNode.Max = n
	}
	constraints.AddConstraint("c2-max-node-count", 10, 60, maxNode)

	if p, found := params["content"]; found {
		if c, err := NewContentConstraint(self.Path, p[0]); err != nil {
			self.LastErr = err
		} else {
			constraints.AddConstraint("content", 10, 70, c)
		}
	}

	if p, found := params["with-defaults"]; found {
		if c, err := NewWithDefaultsConstraint(p[0]); err != nil {
			self.LastErr = err
		} else {
			constraints.AddConstraint("with-defaults", 50, 70, c)
		}
	}

	self.Constraints = constraints
}

func (self Selection) beginEdit(r NodeRequest, bubble bool) error {
	r.Selection = self
	var triggered bool
	for {
		if err := r.Selection.Node.BeginEdit(r); err != nil {
			return err
		}
		if !triggered {
			if err := self.Browser.Triggers.beginEdit(r); err != nil {
				return err
			}
			triggered = true
		}
		if r.Selection.Parent == nil || !bubble {
			break
		}
		r.Selection = *r.Selection.Parent
		r.EditRoot = false
	}
	return nil
}

func (self Selection) endEdit(r NodeRequest, bubble bool) error {
	r.Selection = self
	var triggered bool
	for {
		if err := r.Selection.Node.EndEdit(r); err != nil {
			return err
		}
		if !triggered {
			if err := self.Browser.Triggers.endEdit(r); err != nil {
				return err
			}
			triggered = true
		}
		if r.Selection.Parent == nil || !bubble {
			break
		}
		r.Selection = *r.Selection.Parent
		r.EditRoot = false
	}
	return nil
}

func (self Selection) Delete() (err error) {
	if self.Node.Delete(NodeRequest{Selection: self, Source: self}); err != nil {
		return err
	}
	if err := self.beginEdit(NodeRequest{Source: self}, true); err != nil {
		return err
	}

	if self.InsideList {
		r := ListRequest{
			Request: Request{
				Selection: *self.Parent,
			},
			Meta:   self.Meta().(*meta.List),
			Delete: true,
			Key:    self.Key(),
		}
		if _, _, err := r.Selection.Node.Next(r); err != nil {
			return err
		}
	} else {
		r := ChildRequest{
			Request: Request{
				Selection: *self.Parent,
			},
			Meta:   self.Meta().(meta.MetaList),
			Delete: true,
		}
		if _, err := r.Selection.Node.Child(r); err != nil {
			return err
		}
	}

	if err := self.endEdit(NodeRequest{Source: self}, true); err != nil {
		return err
	}
	return
}

func findIntParam(params map[string][]string, param string) (int, bool) {
	if v, found := params[param]; found {
		if n, err := strconv.Atoi(v[0]); err == nil {
			return n, true
		}
	}
	return 0, false
}

// InsertInto Copy current node into given node.  If there are any existing containers of list
// items then this will fail by design.
func (self Selection) InsertInto(toNode Node) Selection {
	return self.InsertIntoCntx(context.Background(), toNode)
}

// InsertIntoCntx is like InsertInto but with context control and value statue
func (self Selection) InsertIntoCntx(c context.Context, toNode Node) Selection {
	if self.LastErr == nil {
		self.LastErr = editor{basePath: self.Path}.edit(c, self, self.Split(toNode), editInsert)
	}
	return self
}

// InsertFrom Copy given node into current node.  If there are any existing containers of list
// items then this will fail by design.
func (self Selection) InsertFrom(fromNode Node) Selection {
	return self.InsertFromCntx(context.Background(), fromNode)
}

// InsertFromCntx is like InsertFrom but with context control and value statue
func (self Selection) InsertFromCntx(c context.Context, fromNode Node) Selection {
	if self.LastErr == nil {
		self.LastErr = editor{basePath: self.Path}.edit(c, self.Split(fromNode), self, editInsert)
	}
	return self
}

// UpsertInto Merge current node into given node.  If there are any existing containers of list
// items then data will be merged.
func (self Selection) UpsertInto(toNode Node) Selection {
	return self.UpsertIntoCntx(context.Background(), toNode)
}

// UpsertIntoCntx is like UpsertInto but with context control and value statue
func (self Selection) UpsertIntoCntx(c context.Context, toNode Node) Selection {
	if self.LastErr == nil {
		self.LastErr = editor{basePath: self.Path}.edit(c, self, self.Split(toNode), editUpsert)
	}
	return self
}

// Merge given node into current node.  If there are any existing containers of list
// items then data will be merged.
func (self Selection) UpsertFrom(fromNode Node) Selection {
	return self.UpsertFromCntx(context.Background(), fromNode)
}

// UpsertIntoCntx is like UpsertInto but with context control and value statue
func (self Selection) UpsertFromCntx(c context.Context, fromNode Node) Selection {
	if self.LastErr == nil {
		self.LastErr = editor{basePath: self.Path}.edit(c, self.Split(fromNode), self, editUpsert)
	}
	return self
}

// Copy current node into given node.  There must be matching containers of list
// items or this will fail by design.
func (self Selection) UpdateInto(toNode Node) Selection {
	return self.UpdateIntoCntx(context.Background(), toNode)
}

// UpdateIntoCntx is like UpdateInto but with context control and value statue
func (self Selection) UpdateIntoCntx(c context.Context, toNode Node) Selection {
	if self.LastErr == nil {
		self.LastErr = editor{basePath: self.Path}.edit(c, self, self.Split(toNode), editUpdate)
	}
	return self
}

// Copy given node into current node.  There must be matching containers of list
// items or this will fail by design.
func (self Selection) UpdateFrom(fromNode Node) Selection {
	return self.UpdateFromCntx(context.Background(), fromNode)
}

// UpdateFromCntx is like UpdateFrom but with context control and value statue
func (self Selection) UpdateFromCntx(c context.Context, fromNode Node) Selection {
	if self.LastErr == nil {
		self.LastErr = editor{basePath: self.Path}.edit(c, self.Split(fromNode), self, editUpdate)
	}
	return self
}

// Notifications let's caller subscribe to a node.  Node must be a 'notification' node.
func (self Selection) Notifications(stream NotifyStream) (NotifyCloser, error) {
	return self.NotificationsCntx(context.Background(), stream)
}

// NotificationsCntx is like NotificationsCntx but with context control and value statue
func (self Selection) NotificationsCntx(c context.Context, stream NotifyStream) (NotifyCloser, error) {
	if self.LastErr != nil {
		return nil, self.LastErr
	}
	r := NotifyRequest{
		Request: Request{
			Context:   c,
			Selection: self,
		},
		Meta:   self.Meta().(*meta.Notification),
		Stream: stream,
	}
	return self.Node.Notify(r)
}

// Action let's to call a procedure potentially passing on data and potentially recieving
// data back.
func (self Selection) Action(input Node) Selection {
	return self.ActionCntx(context.Background(), input)
}

// ActionCntx is like Action but with context control and value state
func (self Selection) ActionCntx(c context.Context, input Node) Selection {
	if self.LastErr != nil {
		return self
	}
	r := ActionRequest{
		Request: Request{
			Context:   c,
			Selection: self,
		},
		Meta: self.Meta().(*meta.Rpc),
	}

	if input != nil {
		r.Input = Selection{
			Browser:     self.Browser,
			Parent:      &self,
			Path:        &Path{parent: self.Path, meta: r.Meta.Input},
			Node:        input,
			Constraints: self.Constraints,
			Handler:     self.Handler,
		}
	}

	if self.Constraints != nil {
		r.Constraints = self.Constraints
		r.ConstraintsHandler = self.Handler
		if proceed, constraintErr := self.Constraints.CheckActionPreConstraints(&r); !proceed || constraintErr != nil {
			self.LastErr = constraintErr
			return self
		}
	}

	rpcOutput, rerr := self.Node.Action(r)
	if rerr != nil {
		self.LastErr = rerr
		return self
	}

	var output Selection
	if rpcOutput != nil {
		output = Selection{
			Browser:     self.Browser,
			Parent:      &self,
			Path:        &Path{parent: self.Path, meta: r.Meta.Output},
			Node:        rpcOutput,
			Constraints: self.Constraints,
			Handler:     self.Handler,
		}
	}

	if self.Constraints != nil {
		r.Constraints = self.Constraints
		r.ConstraintsHandler = self.Handler
		if proceed, constraintErr := self.Constraints.CheckActionPostConstraints(r); !proceed || constraintErr != nil {
			self.LastErr = constraintErr
			return self
		}
	}

	return output
}

// Set let's you set a leaf value on a container or list item.
func (self Selection) Set(ident string, value interface{}) error {
	if self.LastErr != nil {
		return self.LastErr
	}
	pos := meta.FindByIdent2(self.Path.meta, ident)
	if pos == nil {
		return c2.NewErrC("property not found "+ident, 404)
	}
	m := pos.(meta.HasDataType)
	v, e := SetValue(m.GetDataType(), value)
	if e != nil {
		return e
	}
	r := FieldRequest{
		Request: Request{
			Selection: self,
		},
		Write: true,
		Meta:  m,
	}
	return self.SetValueHnd(&r, &ValueHandle{Val: v})
}

func (self Selection) SetValueHnd(r *FieldRequest, hnd *ValueHandle) error {
	hnd.Val.Type = r.Meta.GetDataType()
	r.Write = true

	if self.Constraints != nil {
		r.Constraints = self.Constraints
		r.ConstraintsHandler = self.Handler
		if proceed, constraintErr := self.Constraints.CheckFieldPreConstraints(r, hnd); !proceed || constraintErr != nil {
			return constraintErr
		}
	}

	if err := self.Node.Field(*r, hnd); err != nil {
		return err
	}

	if self.Constraints != nil {
		if proceed, constraintErr := self.Constraints.CheckFieldPostConstraints(*r, hnd); !proceed || constraintErr != nil {
			return constraintErr
		}
	}

	return nil
}

// Get let's you get a leaf value from a container or list item
func (self Selection) Get(ident string) (interface{}, error) {
	if self.LastErr != nil {
		return nil, self.LastErr
	}
	v, e := self.GetValue(ident)
	if e != nil {
		return nil, e
	}
	return v.Value(), nil
}

// GetValue let's you get the leaf value as a Value instance.  Returns null if value is null
func (self Selection) GetValue(ident string) (*Value, error) {

	if self.LastErr != nil {
		return nil, self.LastErr
	}
	pos := meta.FindByIdent2(self.Path.meta, ident)
	if pos == nil {
		return nil, c2.NewErrC("property not found "+ident, 404)
	}
	if !meta.IsLeaf(pos) {
		return nil, c2.NewErrC("property is not a leaf "+ident, 400)
	}
	r := FieldRequest{
		Request: Request{
			Selection: self,
		},
		Meta: pos.(meta.HasDataType),
	}

	r.Write = false
	var hnd ValueHandle
	err := self.GetValueHnd(&r, &hnd, true)

	return hnd.Val, err
}

func (self Selection) GetValueHnd(r *FieldRequest, hnd *ValueHandle, useDefault bool) (err error) {

	if self.Constraints != nil {
		r.Constraints = self.Constraints
		r.ConstraintsHandler = self.Handler
		if proceed, constraintErr := self.Constraints.CheckFieldPreConstraints(r, hnd); !proceed || constraintErr != nil {
			return constraintErr
		}
	}

	if err = self.Node.Field(*r, hnd); err != nil {
		return err
	}
	if hnd.Val != nil {
		hnd.Val.Type = r.Meta.GetDataType()
	} else if useDefault && r.Meta.GetDataType().HasDefault() {
		hnd.Val = &Value{Type: r.Meta.GetDataType()}
		hnd.Val.CoerseStrValue(r.Meta.GetDataType().Default())
	}

	if self.Constraints != nil {
		if proceed, constraintErr := self.Constraints.CheckFieldPostConstraints(*r, hnd); !proceed || constraintErr != nil {
			return constraintErr
		}
	}

	return nil
}
