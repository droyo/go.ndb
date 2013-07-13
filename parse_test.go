package ndb

import (
	"testing"
	"bytes"
)

var parseTests = []struct {
	in []byte
	out []pair
}{
	{
		in: []byte("key1=val1 key2=val2 key3=val3"),
		out: []pair {
			{[]byte("key1"),[]byte("val1")},
			{[]byte("key2"),[]byte("val2")},
			{[]byte("key3"),[]byte("val3")}},
	},
	{
		in: []byte("title='Some value with spaces' width=340 height=200"),
		out: []pair {
			{[]byte("title"),[]byte("Some value with spaces")},
			{[]byte("width"),[]byte("340")},
			{[]byte("height"),[]byte("200")}},
	},
	{
		in: []byte("title='Dave''s pasta' sq=Davis cost=$$"),
		out: []pair {
			{[]byte("title"),[]byte("Dave's pasta")},
			{[]byte("sq"),[]byte("Davis")},
			{[]byte("cost"),[]byte("$$")}},
	},
	{
		in: []byte("action=''bradley key=jay mod=ctrl+alt+shift"),
		out: []pair {
			{[]byte("action"),[]byte("'bradley")},
			{[]byte("key"),[]byte("jay")},
			{[]byte("mod"),[]byte("ctrl+alt+shift")}},
	},
	{
		in: []byte("action=reload key='' mod=ctrl+alt+shift"),
		out: []pair {
			{[]byte("action"),[]byte("reload")},
			{[]byte("key"),[]byte("'")},
			{[]byte("mod"),[]byte("ctrl+alt+shift")}},
	},
}

func Test_parsing(t *testing.T) {
	for i,tt := range parseTests {
		d := NewDecoder(bytes.NewReader(tt.in))
		p,err := d.getPairs()
		if err != nil {
			t.Error(err)
			t.FailNow()
		} else {
			for j := range tt.out {
				if j > len(p) || !match(p[j],tt.out[j]) {
					t.Errorf("%d: getPairs %s => %v, want %v",i, tt.in, p, tt.out)
					t.FailNow()
				}
			}
		}
	}
}

func match(p1, p2 pair) bool {
	return (bytes.Compare(p1.attr, p2.attr) == 0) && (bytes.Compare(p1.val, p2.val) == 0)
}
