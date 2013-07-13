package ndb

import (
	"reflect"
	"net/textproto"
	"unicode"
	"bytes"
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
func errNewline(line []byte, offset int64) error {
	return &SyntaxError { line, offset, "Values may not contain new lines" }
}

func (d *Decoder) getPairs() ([]pair, error) {
	var tuples [][]byte
	d.pairbuf = d.pairbuf[0:0]
	line, err := d.src.ReadContinuedLineBytes()
	if err != nil {
		return nil,err
	}
	tuples,err = lex(line)
	if err != nil {
		return nil,err
	} else {
		for _,t := range tuples {
			d.pairbuf = append(d.pairbuf, parseTuple(t))
		}
	}
	return d.pairbuf, nil
}

func (d *Decoder) saveData(p []pair, val reflect.Value) error {
	return nil
}

func parseTuple(tuple []byte) pair {
	var p pair
	fmt.Printf("Split %s\n", string(tuple))
	s := bytes.SplitN(tuple, []byte("="), 2)
	p.attr = s[0]
	if len(s) > 1 {
		if len(s[1]) > 1 {
			if s[1][0] == '\'' && len(s[1]) > 2 && s[1][len(s[1])-1] == '\'' {
				s[1] = s[1][1:len(s[1])-1]
			}
		}
		p.val = bytes.Replace(s[1], []byte("''"), []byte("'"), -1)
	}
	fmt.Println("Made ", p)
	return p
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
	scanQuoteString
)

func lex(line []byte) ([][]byte, error) {
	var offset int64
	state := make(scanState, 0, 3)
	tuples := make([][]byte, 0, 10)
	buf := bytes.NewReader(line)
	var beg int64
	
	for r,sz,err := buf.ReadRune(); err == nil; r,sz,err = buf.ReadRune() {
		fmt.Printf("(%d,%c) %s|%s\n", state.top(), r, line[:offset], line[offset:])
		if r == 0xFFFD && sz == 1 {
			return nil, errBadUnicode(line, offset)
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
				state.pop()
				tuples = append(tuples, line[beg:offset])
				fmt.Println("Save", string(line[beg:offset]))
			} else if r == '=' {
				state.pop()
				state.push(scanValueStart)
			} else if !(unicode.IsLetter(r) || unicode.IsNumber(r))  {
				return nil,errBadAttr(line, offset)
			}
		case scanValueStart:
			if unicode.IsSpace(r) {
				state.pop()
				tuples = append(tuples, line[beg:offset])
				fmt.Println("Save", string(line[beg:offset]))
			} else if r == '\'' {
				state.push(scanQuoteStart)
			} else {
				state.pop()
				state.push(scanValue)
			}
		case scanValue:
			if unicode.IsSpace(r) {
				state.pop()
				tuples = append(tuples, line[beg:offset])
				fmt.Println("Save", string(line[beg:offset]))
			}
		case scanQuoteStart:
			if r == '\'' {
				state.pop()
			} else {
				state.pop()
				state.push(scanQuoteString)
			}
		case scanQuoteString:
			if r == '\'' {
				state.pop()
			} else if r == '\n' {
				return nil,errNewline(line, offset)
			}
		}
		offset += int64(sz)
	}
	switch state.top() {
	case scanQuoteString, scanQuoteStart:
		return nil,errUnterminated(line, offset)
	case scanNone:
	default:
		tuples = append(tuples, line[beg:offset])
		fmt.Println("Save", string(line[beg:offset]))
	}
	return tuples,nil
}
