package node

import "github.com/c2stack/c2g/meta"

type Request struct {
	Selection          Selection
	Target             PathSlice
	Constraints        *Constraints
	ConstraintsHandler *ConstraintHandler
}

type NotifyCloser func() error

type NotifyStream interface {
	Notify(*meta.Notification, *Path, Node)
}

type NotifyRequest struct {
	Request
	Meta   *meta.Notification
	Stream NotifyStream
}

type ActionRequest struct {
	Request
	Meta  *meta.Rpc
	Input Selection
}

type ContainerRequest struct {
	Request
	From Selection
	New  bool
	Meta meta.MetaList
}

type ListRequest struct {
	Request
	From       Selection
	New        bool
	StartRow   int
	Row        int
	StartRow64 int64
	Row64      int64
	First      bool
	Meta       *meta.List
	Key        []*Value
}

func (self *ListRequest) SetStartRow(row int64) {
	self.StartRow64 = row
	self.StartRow = int(row)
}

func (self *ListRequest) SetRow(row int64) {
	self.Row64 = row
	self.Row = int(row)
}

func (self *ListRequest) IncrementRow() {
	self.Row64++
	self.Row++
}

type FieldRequest struct {
	Request
	Meta  meta.HasDataType
	Write bool
}
