/*
 * Copyright (c) 2021 Marcin Gasperowicz <xnooga@gmail.com>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated
 * documentation files (the "Software"), to deal in the Software without restriction, including without limitation the
 * rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit
 * persons to whom the Software is furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all copies or substantial portions of the
 * Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE
 * WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR
 * OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

package vm

import (
	"fmt"
	"reflect"
)

type theNamespaceType struct{}

func (t *theNamespaceType) String() string     { return t.Name() }
func (t *theNamespaceType) Type() ValueType    { return TypeType }
func (t *theNamespaceType) Unbox() interface{} { return reflect.TypeOf(t) }

func (t *theNamespaceType) Name() string { return "let-go.lang.Namespace" }
func (t *theNamespaceType) Box(fn interface{}) (Value, error) {
	return NIL, NewTypeError(fn, "can't be boxed as", t)
}

var NamespaceType *theNamespaceType

func init() {
	NamespaceType = &theNamespaceType{}
}

type Refer struct {
	ns  *Namespace
	all bool
}

type Namespace struct {
	name     string
	registry map[Symbol]*Var
	refers   map[Symbol]*Refer
}

func (n *Namespace) Type() ValueType { return NamespaceType }

// Unbox implements Unbox
func (n *Namespace) Unbox() interface{} {
	return nil
}

func NewNamespace(name string) *Namespace {
	return &Namespace{
		name:     name,
		registry: map[Symbol]*Var{},
		refers:   map[Symbol]*Refer{},
	}
}

func (n *Namespace) Def(name string, val Value) *Var {
	s := Symbol(name)
	va := NewVar(n, n.name, name)
	va.SetRoot(val)
	n.registry[s] = va
	return va
}

func (n *Namespace) LookupOrAdd(symbol Symbol) Value {
	val, ok := n.registry[symbol]
	if !ok {
		return n.Def(string(symbol), NIL)
	}
	return val
}

func (n *Namespace) Lookup(symbol Symbol) Value {
	sns, sym := symbol.Namespaced()
	if sns == NIL {
		v := n.registry[sym.(Symbol)]
		if v == nil {
			for _, ref := range n.refers {
				v = ref.ns.registry[sym.(Symbol)]
				if v != nil {
					return v
				}
			}
		}
		if v == nil {
			return NIL
		}
		return v
	}
	refer := n.refers[sns.(Symbol)]
	if refer == nil {
		return NIL
	}
	return refer.ns.registry[sym.(Symbol)]
}

func (n *Namespace) Refer(ns *Namespace, alias string, all bool) {
	nom := ns.Name()
	if alias != "" {
		nom = alias
	}
	n.refers[Symbol(nom)] = &Refer{
		all: all,
		ns:  ns,
	}
}

func (n *Namespace) Name() string {
	return n.name
}

func (n *Namespace) String() string {
	return fmt.Sprintf("<ns %s>", n.Name())
}
