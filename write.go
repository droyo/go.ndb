package ndb

import (
	"bytes"
	"fmt"
	"reflect"
	"unicode"
	"unicode/utf8"
)

func (e *Encoder) encodeSlice(val reflect.Value) error {
	for i := 0; i < val.Len(); i++ {
		e.Encode(val.Index(i).Interface())
	}
	return nil
}

func (e *Encoder) encodeStruct(val reflect.Value) error {
	typ := val.Type()
	for i := 0; i < typ.NumField(); i++ {
		ft := typ.Field(i)
		attr := ft.Name
		if tag := ft.Tag.Get("ndb"); tag != "" {
			attr = tag
		}
		err := e.writeTuple(attr, val.Field(i))
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *Encoder) encodeMap(val reflect.Value) error {
	for _, k := range val.MapKeys() {
		v := val.MapIndex(k)

		if err := e.writeTuple(k.Interface(), v); err != nil {
			return err
		}
	}
	return nil
}

func (e *Encoder) writeTuple(k interface{}, v reflect.Value) error {
	var values reflect.Value
	var attrBuf, valBuf bytes.Buffer
	fmt.Fprint(&attrBuf, k)

	attr := attrBuf.Bytes()

	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		sliceType := reflect.SliceOf(v.Type())
		pv := reflect.New(sliceType)
		pv.Elem().Set(reflect.MakeSlice(sliceType, 0, 1))
		pv.Elem().Set(reflect.Append(pv.Elem(), v))
		values = pv.Elem()
	} else {
		values = v
	}

	for i := 0; i < values.Len(); i++ {
		fmt.Fprint(&valBuf, values.Index(i).Interface())
		val := valBuf.Bytes()
		if e.start {
			if _, err := e.out.Write([]byte{' '}); err != nil {
				return err
			}
		} else {
			e.start = true
		}

		if !validAttr(attr) {
			return &SyntaxError{nil, 0, fmt.Sprintf("Invalid attribute %s", attr)}
		}
		if !validVal(val) {
			return &SyntaxError{nil, 0, fmt.Sprintf("Invalid value %s", val)}
		}
		if bytes.IndexByte(val, '\'') != -1 {
			val = bytes.Replace(val, []byte{'\''}, []byte{'\'', '\''}, -1)
		}
		if _, err := e.out.Write(attr); err != nil {
			return err
		}
		if _, err := e.out.Write([]byte{'='}); err != nil {
			return err
		}
		x := bytes.IndexFunc(val, func(r rune) bool {
			return unicode.IsSpace(r)
		})
		if x != -1 {
			if _, err := e.out.Write([]byte{'\''}); err != nil {
				return err
			}
		}
		if _, err := e.out.Write(val); err != nil {
			return err
		}
		if x != -1 {
			if _, err := e.out.Write([]byte{'\''}); err != nil {
				return err
			}
		}
		valBuf.Reset()
	}
	return nil
}

func validAttr(attr []byte) bool {
	if !utf8.Valid(attr) {
		return false
	}
	x := bytes.IndexFunc(attr, func(r rune) bool {
		switch {
		case r == '\'':
			return true
		case unicode.IsSpace(r):
			return true
		}
		return !unicode.IsLetter(r) &&
			!unicode.IsNumber(r) &&
			r != '-'
	})
	return x == -1
}

func validVal(val []byte) bool {
	if !utf8.Valid(val) {
		return false
	}
	return bytes.IndexByte(val, '\n') == -1
}
