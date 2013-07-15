// Package ndb decodes and encodes simple strings of attribute=value pairs.
// The accepted format is based on Plan 9's ndb(6) format found at
// http://plan9.bell-labs.com/magic/man2html/6/ndb, with additional
// rules for quoting values containing white space.
//
// Attributes are UTF-8 encoded strings of any printable non-whitespace
// character, except for the equals sign ('='). Value strings may contain
// any printable character except for a new line. Values containing white
// space must be enclosed in single quotes. Single quotes can be escaped
// by doubling them, like so:
//
// 	* {"example1": "Let's go shopping"} is encoded as
// 	  example1='Let''s go shopping'
// 	* {"example2": "Escape ' marks by doubling like this: ''"}
// 	  example2='Escape '' marks by doubling like this: '''''
// 	* {"example3": "can't"}
// 	  example3=can''t
//
// Tuples must be separated by at least one whitespace character. The same
// attribute may appear multiple times in an ndb string. When decoding an
// ndb string with repeated attributes, the destination type must be a slice.
package ndb

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/textproto"
	"reflect"
	"unicode/utf8"
)

// A SyntaxError occurs when malformed input, such as an unterminated
// quoted string, is received. It contains the UTF-8 encoded line that
// was being read and the position of the first byte that caused the
// syntax error. Data may only be valid until the next call to the
// Decode() method
type SyntaxError struct {
	Data    []byte
	Offset  int64
	Message string
}

// A TypeError occurs when a Go value is incompatible with the ndb
// string it must store or create.
type TypeError struct {
	Type reflect.Type
}

func (e *TypeError) Error() string {
	return fmt.Sprintf("Invalid type %s or nil pointer", e.Type.String())
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func (e *SyntaxError) Error() string {
	start := e.Offset
	end := min(e.Offset+10, int64(len(e.Data)))

	if e.Data != nil {
		// Make sure we're on utf8 boundaries
		for !utf8.RuneStart(e.Data[start]) && start > 0 {
			start--
		}
		for !utf8.Valid(e.Data[start:end]) && end < int64(len(e.Data)) {
			end++
		}
		return fmt.Sprintf("%s\n\tat `%s'", e.Message, e.Data[start:end])
	}
	return e.Message
}

// An Encoder wraps an io.Writer and serializes Go values
// into ndb strings. Successive calls to the Encode() method
// append lines to the io.Writer.
type Encoder struct {
	start bool
	out   io.Writer
}

// A decoder wraps an io.Reader and decodes successive ndb strings
// into Go values using the Decode() function.
type Decoder struct {
	src       *textproto.Reader
	pairbuf   []pair
	finfo     map[string][]int
	havemulti bool
	attrs     map[string]struct{}
	multi     map[string]struct{}
}

// The Unmarshal function reads an entire ndb string and unmarshals it
// into the Go value v. Value v must be a pointer. Unmarshal will behave
// differently depending on the type of value v points to.
//
// If v is a slice, Unmarshal will decode all lines from the ndb input
// into slice elements. Otherwise, Unmarshal will decode only the first
// line.
//
// If v is a map, Unmarshal will populate v with key/value pairs, where
// value is decoded according to the concrete types of the map.
//
// If v is a struct, Unmarshal will populate struct fields whose names
// match the ndb attribute. Struct fields may be annotated with a tag
// of the form `ndb:"name"`, where name matches the attribute string
// in the ndb input.
//
// Struct fields or map keys that do not match the ndb input are left
// unmodified. Ndb attributes that do not match any struct fields are
// silently dropped. If an ndb string cannot be converted to the
// destination value or a syntax error occurs, an error is returned
// and v is left unmodified. Unmarshal can only store to exported (capitalized)
// fields of a struct.
func Unmarshal(data []byte, v interface{}) error {
	d := NewDecoder(bytes.NewReader(data))
	return d.Decode(v)
}

// NewDecoder returns a Decoder with its input pulled from an io.Reader
func NewDecoder(r io.Reader) *Decoder {
	d := new(Decoder)
	d.src = textproto.NewReader(bufio.NewReader(r))
	d.attrs = make(map[string]struct{}, 8)
	d.multi = make(map[string]struct{}, 8)
	d.finfo = make(map[string][]int, 8)
	return d
}

// The Decode method follows the same parsing rules as Unmarshal(), but
// reads its input from the Decoder's input stream.
func (d *Decoder) Decode(v interface{}) error {
	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	if typ.Kind() != reflect.Ptr {
		return &TypeError{typ}
	}

	if typ.Elem().Kind() == reflect.Slice {
		return d.decodeSlice(val)
	}
	p, err := d.getPairs()
	if err != nil {
		return err
	}

	switch typ.Elem().Kind() {
	default:
		return &TypeError{val.Type()}
	case reflect.Map:
		if val.Elem().IsNil() {
			val.Elem().Set(reflect.MakeMap(typ.Elem()))
		}
		return d.saveMap(p, val.Elem())
	case reflect.Struct:
		if val.IsNil() {
			return &TypeError{nil}
		}
		return d.saveStruct(p, val.Elem())
	}
	return nil
}

// Marshal encodes a value into an ndb string. Marshal will use the String
// method of each struct field or map entry to produce ndb output.
// If v is a slice or array, multiple ndb lines will be output, one
// for each element. For structs, attribute names will be the name of
// the struct field, or the fields ndb annotation if it exists.
// Ndb attributes may not contain white space. Ndb values may contain
// white space but may not contain new lines. If Marshal cannot produce
// valid ndb strings, an error is returned. No guarantee is made about
// the order of the tuples.
func Marshal(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	e := NewEncoder(&buf)
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// The Encode method will write the ndb encoding of the Go value v
// to its backend io.Writer. Unlike Decode(), slice or array values
// are valid, and will cause multiple ndb lines to be written.
// If the value cannot be fully encoded, an error is returned and
// no data will be written to the io.Writer.
func (e *Encoder) Encode(v interface{}) error {
	val := reflect.ValueOf(v)
	// Drill down to the concrete value
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return &TypeError{nil}
		} else {
			val = val.Elem()
		}
	}
	defer func() {
		e.start = false
	}()
	switch val.Kind() {
	case reflect.Slice:
		return e.encodeSlice(val)
	case reflect.Struct:
		return e.encodeStruct(val)
	case reflect.Map:
		return e.encodeMap(val)
	default:
		return &TypeError{val.Type()}
	}
}

// NewEncoder returns an Encoder that writes ndb output to an
// io.Writer
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{out: w}
}
