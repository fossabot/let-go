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

type theFuncType struct{}

func (t *theFuncType) String() string     { return t.Name() }
func (t *theFuncType) Type() ValueType    { return TypeType }
func (t *theFuncType) Unbox() interface{} { return reflect.TypeOf(t) }

func (t *theFuncType) Name() string { return "let-go.lang.Fn" }
func (t *theFuncType) Box(fn interface{}) (Value, error) {
	return NIL, NewTypeError(fn, "can't be boxed as", t)
}

var FuncType *theFuncType

func init() {
	FuncType = &theFuncType{}
}

type Func struct {
	arity       int
	isVariadric bool
	chunk       *CodeChunk
}

func MakeFunc(arity int, variadric bool, c *CodeChunk) *Func {
	return &Func{
		arity:       arity,
		isVariadric: variadric,
		chunk:       c,
	}
}

func (l *Func) Type() ValueType { return FuncType }

type FuncInterface func(interface{})

// Unbox implements Unbox
func (l *Func) Unbox() interface{} {
	proxy := func(in []reflect.Value) []reflect.Value {
		args := make([]Value, len(in))
		for i := range in {
			a, _ := BoxValue(in[i]) // FIXME handle error
			args[i] = a
		}
		f := NewFrame(l.chunk, args)
		out, _ := f.Run() // FIXME handle error
		return []reflect.Value{reflect.ValueOf(out.Unbox())}
	}
	return func(fptr interface{}) {
		fn := reflect.ValueOf(fptr).Elem()
		v := reflect.MakeFunc(fn.Type(), proxy)
		fn.Set(v)
	}
}

func (l *Func) Arity() int {
	return l.arity
}

func (l *Func) Invoke(pargs []Value) Value {
	args := pargs
	if l.isVariadric {
		// pretty sure variadric should guarantee arity >= 1
		sargs := args[0 : l.arity-1]
		rest := args[l.arity-1:]
		// FIXME don't swallow the error, make invoke return an error
		restlist, _ := ListType.Box(rest)
		args = append(sargs, restlist)
	}
	f := NewFrame(l.chunk, args)
	// FIXME don't swallow the error, make invoke return an error
	v, _ := f.Run()
	return v
}

func (l *Func) String() string {
	return fmt.Sprintf("<fn %p>", l)
}

func (l *Func) MakeClosure() Fn {
	return &Closure{
		closedOvers: nil,
		fn:          l,
	}
}

type Closure struct {
	closedOvers []Value
	fn          *Func
}

func (l *Closure) Type() ValueType { return FuncType }

// Unbox implements Unbox
func (l *Closure) Unbox() interface{} {
	proxy := func(in []reflect.Value) []reflect.Value {
		args := make([]Value, len(in))
		for i := range in {
			a, _ := BoxValue(in[i]) // FIXME handle error
			args[i] = a
		}
		f := NewFrame(l.fn.chunk, args)
		f.closedOvers = l.closedOvers
		out, _ := f.Run() // FIXME handle error
		return []reflect.Value{reflect.ValueOf(out.Unbox())}
	}
	return func(fptr interface{}) {
		fn := reflect.ValueOf(fptr).Elem()
		v := reflect.MakeFunc(fn.Type(), proxy)
		fn.Set(v)
	}
}

func (l *Closure) Arity() int {
	return l.fn.arity
}

func (l *Closure) Invoke(pargs []Value) Value {
	args := pargs
	if l.fn.isVariadric {
		// pretty sure variadric should guarantee arity >= 1
		sargs := args[0 : l.fn.arity-1]
		rest := args[l.fn.arity-1:]
		// FIXME don't swallow the error, make invoke return an error
		restlist, _ := ListType.Box(rest)
		args = append(sargs, restlist)
	}
	f := NewFrame(l.fn.chunk, args)
	f.closedOvers = l.closedOvers
	// FIXME don't swallow the error, make invoke return an error
	v, _ := f.Run()
	return v
}

func (l *Closure) String() string {
	return l.fn.String()
}
