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

type theKeywordType struct {
	zero Keyword
}

func (t *theKeywordType) String() string     { return t.Name() }
func (t *theKeywordType) Type() ValueType    { return TypeType }
func (t *theKeywordType) Unbox() interface{} { return reflect.TypeOf(t) }

func (t *theKeywordType) Name() string { return "let-go.lang.Keyword" }

func (t *theKeywordType) Box(bare interface{}) (Value, error) {
	raw, ok := bare.(fmt.Stringer)
	if !ok {
		return BooleanType.zero, NewTypeError(bare, "can't be boxed as", t)
	}
	return Keyword(raw.String()), nil
}

// KeywordType is the type of KeywordValues
var KeywordType *theKeywordType

func init() {
	KeywordType = &theKeywordType{zero: "????BADKeyword????"}
}

// Keyword is boxed int
type Keyword string

// Type implements Value
func (l Keyword) Type() ValueType { return KeywordType }

// Unbox implements Unbox
func (l Keyword) Unbox() interface{} {
	return string(l)
}

func (l Keyword) String() string {
	return fmt.Sprintf(":%s", string(l))
}

func (l Keyword) Arity() int {
	return -1
}

func (l Keyword) Invoke(pargs []Value) Value {
	vl := len(pargs)
	if vl < 1 || vl > 2 {
		// FIXME return error
		return NIL
	}
	as, ok := pargs[0].(Lookup)
	if !ok {
		// FIXME return error
		return NIL
	}
	if vl == 1 {
		return as.ValueAt(l)
	}
	return as.ValueAtOr(l, pargs[1])
}
