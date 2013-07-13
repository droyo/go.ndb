// Package ndb decodes and encodes simple strings of key=value pairs.
// The accepted format is based on Plan 9's ndb(6) format found at
// http://plan9.bell-labs.com/magic/man2html/6/ndb . Values containing
// white space must be quoted in single quotes. Two single quotes escape
// a literal single quote. Attributes must not contain white space. A 
// value may contain any printable unicode character except for a new line.
package ndb

import (
	"reflect"
	"bytes"
	"bufio"
	"net/textproto"
	"fmt"
	"io"
	"unicode/utf8"
)

// A SyntaxError contains the data that caused an error and the
// offset of the first byte that caused the syntax error. Data may
// only be valid until the next call to the Decode() method
type SyntaxError struct {
	Data []byte
	Offset int64
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

func min(a,b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func (e *SyntaxError) Error() string {
	start := e.Offset
	end := min(e.Offset + 10, int64(len(e.Data)))
	
	// Make sure we're on utf8 boundaries
	for !utf8.RuneStart(e.Data[start]) && start > 0 {
		start--
	}
	for !utf8.Valid(e.Data[start:end]) && end < int64(len(e.Data)) {
		end++
	}
	
	return fmt.Sprintf("%s\n\tat `%s'", e.Message, e.Data[start:end])
}

// An Encoder wraps an io.Writer and serializes Go values
// into ndb strings. Successive calls to the Encode() method
// append lines to the io.Writer.
type Encoder struct {
	out bufio.Writer
}

// A decoder wraps an io.Reader and decodes successive ndb strings
// into Go values using the Decode() function.
type Decoder struct {
	src *textproto.Reader
	pairbuf []pair
}

// The Parse function reads an entire ndb string and unmarshals it
// into the Go value v. Parse will behave differently depending on
// the concrete type of v. Value v must be a reference type, either a
// pointer, map, or slice.
//
// 	* If v is a slice, Parse will decode all lines from the ndb
// 	  input into array elements. Otherwise, Parse will decode only
// 	  the first line.
//
// 	* If v is of the type (map[string] interface{}), Parse will
// 	  populate v with key/value pairs, where value is decoded
// 	  according to the concrete type of the map's value.
//
// 	* If v is a struct, Parse will populate struct fields whose
// 	  names match the ndb attribute. Struct fields may be annotated
// 	  with a tag of the form `ndb: name`, where name matches the
// 	  attribute string in the ndb input.
//
// Struct fields or map keys that do not match the ndb input are left
// unmodified. Ndb attributes that do not match any struct fields are
// silently dropped. If an ndb string cannot be converted to the
// destination value or a syntax error occurs, an error is returned
// and v is left unmodified. Parse can only store to exported (capitalized)
// fields of a struct.
func Parse(data []byte, v interface{}) error {
	d := NewDecoder(bytes.NewReader(data))
	return d.Decode(v)
}

// NewDecoder returns a Decoder with its input pulled from an io.Reader
func NewDecoder(r io.Reader) *Decoder {
	d := new(Decoder)
	d.src = textproto.NewReader(bufio.NewReader(r))
	return d
}

// The Decode method follows the same parsing rules as Parse(), but
// will read at most one ndb string. As such, slices or arrays are
// not valid types for v.
func (d *Decoder) Decode(v interface{}) error {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return &TypeError{val.Type()}
	}
	if p,err := d.getPairs(); err != nil {
		return err
	} else {
		return d.saveData(p, val.Elem())
	}
}

// Emit encodes a value into an ndb string. Emit will use the String
// method of each struct field or map entry to produce ndb output.
// If v is a slice or array, multiple ndb lines will be output, one
// for each element. For structs, attribute names will be the name of
// the struct field, or the fields ndb annotation if it exists.
// Ndb attributes may not contain white space. Ndb values may contain
// white space but may not contain new lines. If Emit cannot produce
// valid ndb strings, an error is returned.
func Emit(v interface{}) ([]byte, error) {
	return nil,nil
}

// The Encode method will write the ndb encoding of the Go value v
// to its backend io.Writer. Unlike Decode(), slice or array values
// are valid, and will cause multiple ndb lines to be written.
// If the value cannot be fully encoded, an error is returned and
// no data will be written to the io.Writer.
func (e *Encoder) Encode(v interface{}) error {
	return nil
}

// NewEncoder returns an Encoder that writes ndb output to an
// io.Writer
func NewEncoder(w io.Writer) *Encoder {
	return nil
}

// Flush forces all outstanding data in an Encoder to be written to
// its backend io.Writer.
func (e *Encoder) Flush() {
}
