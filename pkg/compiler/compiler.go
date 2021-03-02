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

package compiler

import (
	"fmt"
	"github.com/nooga/let-go/pkg/vm"
	"io"
	"strings"
)

type Context struct {
	ns         *vm.Namespace
	parent     *Context
	consts     *[]vm.Value
	chunk      *vm.CodeChunk
	formalArgs map[vm.Symbol]int
	source     string
}

// FIXME this is unacceptable hax
var globalConsts *[]vm.Value

func init() {
	globalConsts = &[]vm.Value{}
}

func NewCompiler(ns *vm.Namespace) *Context {
	return &Context{
		ns:     ns,
		consts: globalConsts,
		source: "<default>",
	}
}

func (c *Context) SetSource(source string) *Context {
	c.source = source
	return c
}

func (c *Context) CurrentNS() *vm.Namespace {
	return c.ns
}

func (c *Context) Compile(s string) (*vm.CodeChunk, error) {
	r := NewLispReader(strings.NewReader(s), c.source)
	o, err := r.Read()
	if err != nil {
		return nil, err
	}

	c.chunk = vm.NewCodeChunk(c.consts)
	err = c.compileForm(o)
	if err != nil {
		return nil, err
	}
	c.Emit(vm.OPRET)
	return c.chunk, nil
}

func (c *Context) CompileMultiple(reader io.Reader) (*vm.CodeChunk, vm.Value, error) {
	r := NewLispReader(reader, c.source)
	chunk := vm.NewCodeChunk(c.consts)
	var result vm.Value = vm.NIL
	for {
		o, err := r.Read()
		if err != nil {
			if isErrorEOF(err) {
				break
			}
			return nil, result, err
		}
		formchunk := vm.NewCodeChunk(c.consts)
		c.chunk = formchunk
		err = c.compileForm(o)
		if err != nil {
			return nil, result, err
		}
		chunk.AppendChunk(formchunk)
		formchunk.Append(vm.OPRET)
		f := vm.NewFrame(formchunk, nil)
		result, err = f.Run()
		if err != nil {
			return nil, result, err
		}
	}

	c.chunk = chunk

	c.Emit(vm.OPRET)
	return c.chunk, result, nil
}

func (c *Context) Emit(op uint8) {
	c.chunk.Append(op)
}

func (c *Context) EmitWithArg(op uint8, arg int) {
	c.chunk.Append(op)
	c.chunk.Append32(arg)
}

func (c *Context) Constant(v vm.Value) int {
	for i := range *c.consts {
		if (*c.consts)[i] == v {
			return i
		}
	}
	*c.consts = append(*c.consts, v)
	return len(*c.consts) - 1
}

func (c *Context) Arg(v vm.Symbol) int {
	n, ok := c.formalArgs[v]
	if !ok {
		return -1
	}
	return n
}

func (c *Context) EnterFn(args []vm.Value) (*Context, error) {
	fchunk := vm.NewCodeChunk(c.consts)

	fc := &Context{
		ns:         c.ns,
		parent:     c,
		consts:     c.consts,
		chunk:      fchunk,
		formalArgs: make(map[vm.Symbol]int),
	}

	for i := range args {
		a := args[i]
		s, ok := a.(vm.Symbol)
		if !ok {
			return nil, NewCompileError("all fn formal arguments must be symbols")
		}
		fc.formalArgs[s] = i
	}
	return fc, nil
}

func (c *Context) LeaveFn(ctx *Context) {
	fnchunk := ctx.chunk

	f := vm.MakeFunc(len(ctx.formalArgs), false, fnchunk)

	n := c.Constant(f)
	c.EmitWithArg(vm.OPLDC, n)
}

func (c *Context) compileForm(o vm.Value) error {
	switch o.Type() {
	case vm.IntType, vm.StringType, vm.NilType, vm.BooleanType, vm.KeywordType, vm.CharType, vm.VoidType:
		n := c.Constant(o)
		c.EmitWithArg(vm.OPLDC, n)
	case vm.SymbolType:
		argn := c.Arg(o.(vm.Symbol))
		if argn >= 0 {
			c.EmitWithArg(vm.OPLDA, argn)
			return nil
		}
		varn := c.Constant(c.ns.LookupOrAdd(o.(vm.Symbol)))
		c.EmitWithArg(vm.OPLDC, varn)
		c.Emit(vm.OPLDV)
	case vm.ArrayVectorType:
		v := o.(vm.ArrayVector)
		// FIXME detect const vectors and push them like this
		if len(v) == 0 {
			n := c.Constant(v)
			c.EmitWithArg(vm.OPLDC, n)
			return nil
		}
		vector := c.Constant(c.ns.LookupOrAdd("vector"))
		c.EmitWithArg(vm.OPLDC, vector)
		for i := range v {
			err := c.compileForm(v[i])
			if err != nil {
				return NewCompileError("compiling vector elements").Wrap(err)
			}
		}
		c.EmitWithArg(vm.OPINV, len(v))
	case vm.ListType:
		fn := o.(*vm.List).First()
		// check if we're looking at a special form
		if fn.Type() == vm.SymbolType {
			formCompiler, ok := specialForms[fn.(vm.Symbol)]
			if ok {
				return formCompiler(c, o)
			}

			fvar, ok := c.ns.Lookup(fn.(vm.Symbol)).(*vm.Var)
			if ok && fvar.IsMacro() {
				argvec := o.(*vm.List).Next().(*vm.List).Unbox().([]vm.Value)
				newform := fvar.Invoke(argvec)
				return c.compileForm(newform)
			}
		}

		// treat as function invocation if this is not a special form
		err := c.compileForm(fn)
		if err != nil {
			return NewCompileError("compiling function position").Wrap(err)
		}

		args := o.(*vm.List).Next()
		argc := args.(vm.Collection).Count().Unbox().(int)
		for args != vm.EmptyList {
			err := c.compileForm(args.First())
			if err != nil {
				return NewCompileError("compiling arguments").Wrap(err)
			}
			args = args.Next()
		}

		c.EmitWithArg(vm.OPINV, argc)
	}
	return nil
}

func (c *Context) EmitWithArgPlaceholder(inst uint8) int {
	placeholder := c.CurrentAddress()
	c.EmitWithArg(inst, 0)
	return placeholder
}

func (c *Context) CurrentAddress() int {
	return c.chunk.Length()
}

func (c *Context) UpdatePlaceholderArg(placeholder int, arg int) {
	c.chunk.Update32(placeholder+1, arg)
}

type formCompilerFunc func(*Context, vm.Value) error

var specialForms map[vm.Symbol]formCompilerFunc

func compilerInit() {
	specialForms = map[vm.Symbol]formCompilerFunc{
		"if":    ifCompiler,
		"do":    doCompiler,
		"def":   defCompiler,
		"fn":    fnCompiler,
		"quote": quoteCompiler,
		"var":   varCompiler,
	}
}

func quoteCompiler(c *Context, form vm.Value) error {
	n := c.Constant(form.(vm.Seq).Next().First())
	c.EmitWithArg(vm.OPLDC, n)
	return nil
}

func fnCompiler(c *Context, form vm.Value) error {
	f := form.(*vm.List).Next()

	args := f.First().(vm.ArrayVector).Unbox().([]vm.Value)

	fc, err := c.EnterFn(args)
	defer c.LeaveFn(fc)

	if err != nil {
		return NewCompileError("compiling fn args").Wrap(err)
	}

	body := f.(*vm.List).Next().Unbox().([]vm.Value)
	l := len(body)
	if l == 0 {
		fc.EmitWithArg(vm.OPLDC, fc.Constant(vm.NIL))
		fc.Emit(vm.OPRET)
		return nil
	}
	for i := range body {
		err := fc.compileForm(body[i])
		if err != nil {
			return NewCompileError("compiling do member").Wrap(err)
		}
		if i < l-1 {
			fc.Emit(vm.OPPOP)
		}
	}
	fc.Emit(vm.OPRET)

	return nil
}

func ifCompiler(c *Context, form vm.Value) error {
	args := form.(*vm.List).Next().Unbox().([]vm.Value)
	l := len(args)
	if l < 2 || l > 3 {
		return NewCompileError(fmt.Sprintf("if: wrong number of forms (%d), need 2 or 3", l))
	}
	// compile condition
	err := c.compileForm(args[0])
	if err != nil {
		return NewCompileError("compiling if condition").Wrap(err)
	}
	elseJumpStart := c.EmitWithArgPlaceholder(vm.OPBRF)
	// compile then branch
	err = c.compileForm(args[1])
	if err != nil {
		return NewCompileError("compiling if then branch").Wrap(err)
	}
	finJumpStart := c.EmitWithArgPlaceholder(vm.OPJMP)
	elseJumpEnd := c.CurrentAddress()
	c.UpdatePlaceholderArg(elseJumpStart, elseJumpEnd-elseJumpStart)
	if l == 3 {
		err = c.compileForm(args[2])
		if err != nil {
			return NewCompileError("compiling if else branch").Wrap(err)
		}
	} else {
		c.EmitWithArg(vm.OPLDC, c.Constant(vm.NIL))
	}
	finJumpEnd := c.CurrentAddress()
	c.UpdatePlaceholderArg(finJumpStart, finJumpEnd-finJumpStart)
	return nil
}

func doCompiler(c *Context, form vm.Value) error {
	args := form.(*vm.List).Next().Unbox().([]vm.Value)
	l := len(args)
	if l == 0 {
		c.EmitWithArg(vm.OPLDC, c.Constant(vm.NIL))
		return nil
	}
	for i := range args {
		err := c.compileForm(args[i])
		if err != nil {
			return NewCompileError("compiling do member").Wrap(err)
		}
		if i < l-1 {
			c.Emit(vm.OPPOP)
		}
	}
	return nil
}

func defCompiler(c *Context, form vm.Value) error {
	args := form.(*vm.List).Next().Unbox().([]vm.Value)
	l := len(args)
	if l != 2 {
		return NewCompileError(fmt.Sprintf("def: wrong number of forms (%d), need 2", l))
	}
	sym := args[0]
	val := args[1]
	if sym.Type() != vm.SymbolType {
		return NewCompileError(fmt.Sprintf("def: first argument must be a symbol, got (%v)", sym))
	}
	varr := c.Constant(c.ns.LookupOrAdd(sym.(vm.Symbol)))
	c.EmitWithArg(vm.OPLDC, varr)

	err := c.compileForm(val)
	if err != nil {
		return NewCompileError("compiling def value").Wrap(err)
	}
	c.Emit(vm.OPSTV)

	return nil
}

func varCompiler(c *Context, form vm.Value) error {
	sym := form.(*vm.List).Next().First().(vm.Symbol)
	varr := c.Constant(c.ns.LookupOrAdd(sym))
	c.EmitWithArg(vm.OPLDC, varr)
	return nil
}
