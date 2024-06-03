package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Decoder struct {
	raw          string
	decoderFuncs []DecoderFunc
}

type DecoderFunc func(string) (any, int, bool)

func NewDecoder(raw string) *Decoder {
	d := &Decoder{raw: raw}
	return d.WithDecoderFunc(d.decodeSlice).WithDecoderFunc(d.decodeString).WithDecoderFunc(d.decodeInteger)
}

func (d *Decoder) WithDecoderFunc(f DecoderFunc) *Decoder {
	d.decoderFuncs = append(d.decoderFuncs, f)
	return d
}

func (d *Decoder) Decode() (any, error) {
	for _, f := range d.decoderFuncs {
		if r, _, ok := f(d.raw); ok {
			return r, nil
		}
	}

	return nil, fmt.Errorf("failed to decode value: %s", d.raw)
}

func (d *Decoder) decodeString(chunk string) (any, int, bool) {
	if !unicode.IsDigit(rune(chunk[0])) {
		return "", 0, false
	}

	var firstColonIndex int

	for i := 0; i < len(chunk); i++ {
		if chunk[i] == ':' {
			firstColonIndex = i
			break
		}
	}

	lengthStr := chunk[:firstColonIndex]

	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return "", 0, false
	}

	val := chunk[firstColonIndex+1 : firstColonIndex+1+length]

	return val, length + 1 + len(lengthStr), true
}

func (d *Decoder) decodeInteger(chunk string) (any, int, bool) {
	if rune(chunk[0]) != 'i' {
		return 0, 0, false
	}

	num, err := strconv.Atoi(chunk[1:strings.IndexByte(chunk, 'e')])
	if err != nil {
		return 0, 0, false
	}

	return num, len(strconv.Itoa(num)) + 2, true
}

func (d *Decoder) decodeSlice(chunk string) (any, int, bool) {
	if rune(chunk[0]) != 'l' {
		return nil, 0, false
	}

	if rune(chunk[1]) == 'e' {
		return []any{}, 2, true
	}

	slice := make([]any, 0, 1024)
	offset := 1

	for rune(chunk[offset]) != 'e' {
		for _, f := range d.decoderFuncs {
			if v, l, ok := f(chunk[offset:]); ok {
				slice = append(slice, v)
				offset += l
			}
		}
	}

	return slice, offset + 1, true
}
