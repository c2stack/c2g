package node

import (
	"github.com/c2g/meta"
	"github.com/c2g/meta/yang"
	"testing"
)

func TestContentConstraintParse(t *testing.T) {
	if c, _ := NewContentConstraint(nil, "config"); c != ContentConfig {
		t.Fail()
	}
}

func TestContentConstraintCheck(t *testing.T) {
	mstr := `
	module m {
		namespace "";
		prefix "";
		revision 0;
		container x {
			leaf a {
				type string;
				config "false";
			}
		}
		container y {
			config "false";
			leaf a {
				type string;
			}
		}
		container z {
			leaf a {
				type string;
			}
		}
	}
	`
	m, _ := yang.LoadModuleFromString(nil, mstr)
	x := meta.FindByIdent2(m, "x").(meta.MetaList)
	y := meta.FindByIdent2(m, "y").(meta.MetaList)
	mSel := Select(m, nil)
	containerTests := []struct {
		sel *Selection
		m meta.MetaList
		expected bool
	} {
		{
			mSel,
			x,
			true,
		},
		{
			mSel,
			y,
			false,
		},
	}
	for i, test := range containerTests {
		r := &ContainerRequest{
			Request: Request{
				Selection: test.sel,
			},
			Meta: test.m,
		}
		pass, _ := ContentConfig.CheckContainerPreConstraints(r, false)
		if pass != test.expected {
			t.Errorf("container test %d failed", i)
		}
	}

	xSel := mSel.SelectChild(x, nil)
	xa := meta.FindByIdent2(x, "a").(meta.HasDataType)
	ySel := mSel.SelectChild(y, nil)
	ya := meta.FindByIdent2(y, "a").(meta.HasDataType)
	z := meta.FindByIdent2(m, "z").(meta.MetaList)
	zSel := mSel.SelectChild(z, nil)
	za := meta.FindByIdent2(z, "a").(meta.HasDataType)
	fieldTests := []struct {
		sel *Selection
		m meta.HasDataType
		expected bool
	} {
		{
			xSel,
			xa,
			false,
		},
		{
			ySel,
			ya,
			false,
		},
		{
			zSel,
			za,
			true,
		},
	}
	for i, test := range fieldTests {
		r := &FieldRequest{
			Request: Request{
				Selection: test.sel,
			},
			Meta: test.m,
		}
		pass, _ := ContentConfig.CheckFieldPreConstraints(r, false)
		if pass != test.expected {
			t.Errorf("field test %d failed", i)
		}
	}
}