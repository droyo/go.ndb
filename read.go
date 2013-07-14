package ndb

import (
	"io"
	"reflect"
	"net/textproto"
	"unicode"
	"strconv"
	"bytes"
	"strings"
	"fmt"
)

type scanner struct {
	src *textproto.Reader
}

type pair struct {
	attr, val []byte
}

func (p pair) String() string {
	return fmt.Sprintf("%s => %s", string(p.attr), string(p.val))
}

func errBadAttr(line []byte, offset int64) error {
	return &SyntaxError { line, offset, "Invalid attribute name" }
}
func errUnterminated(line []byte, offset int64) error {
	return &SyntaxError { line, offset, "Unterminated quoted string" }
}
func errBadUnicode(line []byte, offset int64) error {
	return &SyntaxError { line, offset, "Invalid UTF8 input" }
}
func errMissingSpace(line []byte, offset int64) error {
	return &SyntaxError { line, offset, "Missing white space between tuples" }
}

func (d *Decoder) getPairs() ([]pair, error) {
	line, err := d.src.ReadContinuedLineBytes()
	if err != nil {
		return nil,err
	}
	d.reset()
	return d.parseLine(line)
}

func (d *Decoder) reset() {
	d.pairbuf = d.pairbuf[0:0]
	for k := range d.finfo {
		delete(d.finfo, k)
	}
	for k := range d.multi {
		delete(d.attrs, k)
		delete(d.multi, k)
	}
	d.havemulti = false
}

func (d *Decoder) decodeSlice(val reflect.Value) error {
	var err error
	
	if val.Kind() != reflect.Ptr {
		return &TypeError{val.Type()}
	}
	if val.Type().Elem().Kind() != reflect.Slice {
		return &TypeError{val.Type()}
	}
	if val.Elem().IsNil() {
		val.Elem().Set(reflect.MakeSlice(val.Type().Elem(), 0, 5))
	}
	add := reflect.New(val.Type().Elem().Elem())
	for err = d.Decode(add.Interface()); err != nil; err = d.Decode(add.Interface()) {
		s := reflect.Append(val.Elem(), add.Elem())
		val.Elem().Set(s)
	}
	if err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return nil
}

func (d *Decoder) saveMap(pairs []pair, val reflect.Value) error {
	kv := reflect.New(val.Type().Key())
	
	if d.havemulti {
		if val.Type().Elem().Kind() != reflect.Slice {
			return &TypeError{val.Type()}
		}
		vv := reflect.New(val.Type().Elem().Elem())
		for _,p := range pairs {
			if err := storeVal(kv, p.attr); err != nil {
				return err
			}
			if err := storeVal(vv, p.val); err != nil {
				return err
			}
			slot := val.MapIndex(kv.Elem())
			if slot.Kind() == reflect.Invalid {
				slot = reflect.MakeSlice(val.Type().Elem(), 0, 4)
			}
			
			slot = reflect.Append(slot, vv.Elem())
			val.SetMapIndex(kv.Elem(), slot)
		}
	} else {
		vv := reflect.New(val.Type().Elem())
		for _,p := range pairs {
			if err := storeVal(kv, p.attr); err != nil {
				return err
			}
			if err := storeVal(vv, p.val); err != nil {
				return err
			}
			val.SetMapIndex(kv.Elem(), vv.Elem())
		}
	}
	return nil
}

func (d *Decoder) saveStruct(pairs []pair, val reflect.Value) error {
	var tag string
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !val.FieldByIndex(field.Index).CanSet() {
			continue
		}
		tag = field.Tag.Get("ndb")
		if tag != "" {
			d.finfo[tag] = field.Index
		} else {
			d.finfo[field.Name] = field.Index
		}
	}
	for _,p := range pairs {
		if id,ok := d.finfo[string(p.attr)]; ok {
			f := val.FieldByIndex(id)
			if _,ok := d.multi[string(p.attr)]; ok {
				if f.Kind() != reflect.Slice {
					return &TypeError{f.Type()}
				}
				add := reflect.New(f.Type().Elem())
				if err := storeVal(add, p.val); err != nil {
					return err
				}
				f.Set(reflect.Append(f, add.Elem()))
			} else if err := storeVal(f, p.val); err != nil {
				return err
			}
		}
	}
	return nil
}

func storeVal(dst reflect.Value, src []byte) error {
	if dst.Kind() == reflect.Ptr {
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		dst = dst.Elem()
	}
	
	switch dst.Kind() {
	default:
		return &TypeError{dst.Type()}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		itmp, err := strconv.ParseInt(string(src), 10, dst.Type().Bits())
		if err != nil {
			return err
		}
		dst.SetInt(itmp)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		utmp, err := strconv.ParseUint(string(src), 10, dst.Type().Bits())
		if err != nil {
			return err
		}
		dst.SetUint(utmp)
	case reflect.Float32, reflect.Float64:
		ftmp, err := strconv.ParseFloat(string(src), dst.Type().Bits())
		if err != nil {
			return err
		}
		dst.SetFloat(ftmp)
	case reflect.Bool:
		value, err := strconv.ParseBool(strings.TrimSpace(string(src)))
		if err != nil {
			return err
		}
		dst.SetBool(value)
	case reflect.String:
		dst.SetString(string(src))
	case reflect.Slice:
		if len(src) == 0 {
			src = []byte{}
		}
		dst.SetBytes(src)
	}
	return nil
}

type scanState []int
func (s *scanState) push(n int) {
	*s = append(*s, n)
}
func (s scanState) top() int {
	if len(s) > 0 {
		return s[len(s)-1]
	}
	return scanNone
}
func (s *scanState) pop() int {
	v := s.top()
	if len(*s) > 0 {
		*s = (*s)[0:len(*s)-1]
	}
	return v
}

const (
	scanNone = iota
	scanAttr
	scanValue
	scanValueStart
	scanQuoteStart
	scanQuoteValue
	scanQuoteClose
)

// This is the main tokenizing function. For now it's a messy state machine.
// It could be cleaned up with better use of structures and methods, or
// by copying Rob Pike's Go lexing talk.
func (d *Decoder) parseLine(line []byte) ([]pair, error) {
	var add pair
	var beg,offset int64
	var esc bool
	
	state := make(scanState, 0, 3)
	buf := bytes.NewReader(line)
	
	for r,sz,err := buf.ReadRune(); err == nil; r,sz,err = buf.ReadRune() {
		if r == 0xFFFD && sz == 1 {
			return nil,errBadUnicode(line, offset)
		}
		switch state.top() {
		case scanNone:
			if unicode.IsSpace(r) {
				// skip
			} else if unicode.IsLetter(r) || unicode.IsNumber(r) {
				state.push(scanAttr)
				beg = offset
			} else {
				return nil,errBadAttr(line, offset)
			}
		case scanAttr:
			if unicode.IsSpace(r) {
				add.attr = line[beg:offset]	
				d.pairbuf = append(d.pairbuf, add)
				if _,ok := d.attrs[string(add.attr)]; ok {
					d.havemulti = true
					d.multi[string(add.attr)] = struct{}{}
				} else {
					d.attrs[string(add.attr)] = struct{}{}
				}
				add.attr,add.val,esc = nil,nil,false
				state.pop()
			} else if r == '=' {
				add.attr = line[beg:offset]
				if _,ok := d.attrs[string(add.attr)]; ok {
					d.havemulti = true
					d.multi[string(add.attr)] = struct{}{}
				} else {
					d.attrs[string(add.attr)] = struct{}{}
				}
				state.pop()
				state.push(scanValueStart)
			} else if !(unicode.IsLetter(r) || unicode.IsNumber(r))  {
				return nil,errBadAttr(line, offset)
			}
		case scanValueStart:
			beg = offset
			state.pop()
			state.push(scanValue)
			
			if r == '\'' {
				state.push(scanQuoteStart)
				break
			}
			fallthrough
		case scanValue:
			if unicode.IsSpace(r) {
				state.pop()
				add.val = line[beg:offset]
				if esc {
					add.val = bytes.Replace(add.val, []byte("''"), []byte("'"), -1)
				}
				d.pairbuf = append(d.pairbuf, add)
				add.attr,add.val = nil,nil
			}
		case scanQuoteClose:
			state.pop()
			if r == '\'' {
				esc = true
				state.push(scanQuoteValue)
			} else if unicode.IsSpace(r) {
				state.pop()
				add.val = line[beg:offset-1]
				if esc {
					add.val = bytes.Replace(add.val, []byte("''"), []byte("'"), -1)
				}
				d.pairbuf = append(d.pairbuf, add)
				add.attr,add.val,esc = nil,nil,false
			} else {
				return nil,errMissingSpace(line, offset)
			}
		case scanQuoteStart:
			state.pop()
			if r != '\'' {
				beg++
				state.pop()
				state.push(scanQuoteValue)
			} else {
				esc = true
			}
		case scanQuoteValue:
			if r == '\'' {
				state.pop()
				state.push(scanQuoteClose)
			} else if r == '\n' {
				return nil,errUnterminated(line, offset)
			}
		}
		offset += int64(sz)
	}
	switch state.top() {
	case scanQuoteValue, scanQuoteStart:
		return nil,errUnterminated(line, offset)
	case scanAttr:
		add.attr = line[beg:offset]
		if _,ok := d.attrs[string(add.attr)]; ok {
			d.havemulti = true
			d.multi[string(add.attr)] = struct{}{}
		} else {
			d.attrs[string(add.attr)] = struct{}{}
		}
		d.pairbuf = append(d.pairbuf, add)
	case scanValueStart:
		beg = offset
		fallthrough
	case scanQuoteClose:
		offset--
		fallthrough
	case scanValue:
		add.val = line[beg:offset]
		if esc {
			add.val = bytes.Replace(add.val, []byte("''"), []byte("'"), -1)
		}
		d.pairbuf = append(d.pairbuf, add)
	}
	return d.pairbuf,nil
}
