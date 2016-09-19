package plugin

import (
	"fmt"
)

type xrror struct {
	base string
	vals []interface{}
}

func (x *xrror) Error() string {
	return fmt.Sprintf("%s", fmt.Sprintf(x.base, x.vals...))
}

func (x *xrror) Out(vals ...interface{}) *xrror {
	x.vals = vals
	return x
}

func Xrror(base string) *xrror {
	return &xrror{base: base}
}
