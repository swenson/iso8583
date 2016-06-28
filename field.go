// Copyright 2015-2016 御弟
// Copyright 2016 Capital One
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 		http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package iso8583

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	// ASCII is ASCII encoding
	ASCII = iota
	// BCD is "left-aligned" BCD
	BCD
	// rBCD is "right-aligned" BCD with odd length (for ex. "643" as [6 67] == "0643"), only for Numeric, Llnumeric and Lllnumeric fields
	rBCD
)

// Errors
const (
	ErrInvalidEncoder       string = "invalid encoder"
	ErrInvalidLengthEncoder string = "invalid length encoder"
	ErrInvalidLengthHead    string = "invalid length head"
	ErrMissingLength        string = "missing length"
	ErrValueTooLong         string = "length of value is longer than definition; type=%s, def_len=%d, len=%d"
	ErrBadRaw               string = "bad raw data"
	ErrParseLengthFailed    string = "parse length head failed"
)

// Type interface for ISO 8583 fields
type Type interface {
	// Byte representation of current field.
	Bytes(encoder, lenEncoder, length int) ([]byte, error)

	// Load unmarshal byte value into Iso8583Type according to the
	// specific arguments. It returns the number of bytes actually read.
	Load(raw []byte, encoder, lenEncoder, length int) (int, error)

	// IsEmpty check is field empty
	IsEmpty() bool
}

// A Numeric contains numeric value only in fix length. It holds numeric
// value as a string. Supportted encoder are ascii, bcd and rbcd. Length is
// required for marshalling and unmarshalling.
type Numeric string

// NewNumeric create new Numeric field
func NewNumeric(val string) *Numeric {
	n := Numeric(val)
	return &n
}

// IsEmpty check Numeric field for empty value
func (n *Numeric) IsEmpty() bool {
	return len(string(*n)) == 0
}

// Bytes encode Numeric field to bytes
func (n *Numeric) Bytes(encoder, lenEncoder, length int) ([]byte, error) {
	val := []byte(*n)
	if length == -1 {
		return nil, errors.New(ErrMissingLength)
	}
	// if encoder == rBCD then length can be, for example, 3,
	// but value can be, for example, "0631" (after decode from rBCD, because BCD use 1 byte for 2 digits),
	// and we can encode it only if first digit == 0
	if (encoder == rBCD) &&
		len(val) == (length+1) &&
		(string(val[0:1]) == "0") {
		// Cut value to length
		val = val[1:len(val)]
	}

	if len(val) > length {
		return nil, fmt.Errorf(ErrValueTooLong, "Numeric", length, len(val))
	}
	if len(val) < length {
		val = append([]byte(strings.Repeat("0", length-len(val))), val...)
	}
	switch encoder {
	case BCD:
		return lbcd(val), nil
	case rBCD:
		return rbcd(val), nil
	case ASCII:
		return val, nil
	default:
		return nil, errors.New(ErrInvalidEncoder)
	}
}

// Load decode Numeric field from bytes
func (n *Numeric) Load(raw []byte, encoder, lenEncoder, length int) (int, error) {
	if length == -1 {
		return 0, errors.New(ErrMissingLength)
	}
	switch encoder {
	case BCD:
		l := (length + 1) / 2
		if len(raw) < l {
			return 0, errors.New(ErrBadRaw)
		}
		*n = Numeric(string(bcdl2Ascii(raw[:l], length)))
		return l, nil
	case rBCD:
		l := (length + 1) / 2
		if len(raw) < l {
			return 0, errors.New(ErrBadRaw)
		}
		*n = Numeric(string(bcdr2Ascii(raw[0:l], length)))
		return l, nil
	case ASCII:
		if len(raw) < length {
			return 0, errors.New(ErrBadRaw)
		}
		*n = Numeric(string(raw[:length]))
		return length, nil
	default:
		return 0, errors.New(ErrInvalidEncoder)
	}
}

// An Alphanumeric contains alphanumeric value in fix length. The only
// supportted encoder is ascii. Length is required for marshalling and
// unmarshalling.
type Alphanumeric string

// NewAlphanumeric create new Alphanumeric field
func NewAlphanumeric(val string) *Alphanumeric {
	a := Alphanumeric(val)
	return &a
}

// IsEmpty check Alphanumeric field for empty value
func (a *Alphanumeric) IsEmpty() bool {
	return len(*a) == 0
}

// Bytes encode Alphanumeric field to bytes
func (a *Alphanumeric) Bytes(encoder, lenEncoder, length int) ([]byte, error) {
	val := []byte(*a)
	if length == -1 {
		return nil, errors.New(ErrMissingLength)
	}
	if len(val) > length {
		return nil, fmt.Errorf(ErrValueTooLong, "Alphanumeric", length, len(val))
	}
	if len(val) < length {
		val = append([]byte(strings.Repeat(" ", length-len(val))), val...)
	}
	return val, nil
}

// Load decode Alphanumeric field from bytes
func (a *Alphanumeric) Load(raw []byte, encoder, lenEncoder, length int) (int, error) {
	if length == -1 {
		return 0, errors.New(ErrMissingLength)
	}
	if len(raw) < length {
		return 0, errors.New(ErrBadRaw)
	}
	*a = Alphanumeric(string(raw[:length]))
	return length, nil
}

// Binary contains binary value
type Binary struct {
	Value  []byte
	FixLen int
}

// NewBinary create new Binary field
func NewBinary(d []byte) *Binary {
	return &Binary{d, -1}
}

// IsEmpty check Binary field for empty value
func (b *Binary) IsEmpty() bool {
	return len(b.Value) == 0
}

// Bytes encode Binary field to bytes
func (b *Binary) Bytes(encoder, lenEncoder, l int) ([]byte, error) {
	length := l
	if b.FixLen != -1 {
		length = b.FixLen
	}
	if length == -1 {
		return nil, errors.New(ErrMissingLength)
	}
	if len(b.Value) > length {
		return nil, fmt.Errorf(ErrValueTooLong, "Binary", length, len(b.Value))
	}
	if len(b.Value) < length {
		return append(b.Value, make([]byte, length-len(b.Value))...), nil
	}
	return b.Value, nil
}

// MarshalJSON is used for formatting the JSON representation of this field.
func (b *Binary) MarshalJSON() ([]byte, error) {
	// TODO: use encoder, length
	if b.Value == nil {
		return []byte(`""`), nil
	}
	return []byte(`"` + base64.URLEncoding.EncodeToString(b.Value) + `"`), nil
}

// Load decode Binary field from bytes
func (b *Binary) Load(raw []byte, encoder, lenEncoder, length int) (int, error) {
	if length == -1 {
		return 0, errors.New(ErrMissingLength)
	}
	if len(raw) < length {
		return 0, errors.New(ErrBadRaw)
	}
	b.Value = raw[:length]
	b.FixLen = length
	return length, nil
}

// Llvar contains bytes in non-fixed length field, first 2 symbols of field contains length
type Llvar struct {
	Value []byte
}

// NewLlvar create new Llvar field
func NewLlvar(val []byte) *Llvar {
	return &Llvar{val}
}

// IsEmpty check Llvar field for empty value
func (l *Llvar) IsEmpty() bool {
	return len(l.Value) == 0
}

// Bytes encode Llvar field to bytes
func (l *Llvar) Bytes(encoder, lenEncoder, length int) ([]byte, error) {
	if length != -1 && len(l.Value) > length {
		return nil, fmt.Errorf(ErrValueTooLong, "Llvar", length, len(l.Value))
	}
	if encoder != ASCII {
		return nil, errors.New(ErrInvalidEncoder)
	}

	lenStr := fmt.Sprintf("%02d", len(l.Value))
	contentLen := []byte(lenStr)
	var lenVal []byte
	switch lenEncoder {
	case ASCII:
		lenVal = contentLen
		if len(lenVal) > 2 {
			return nil, errors.New(ErrInvalidLengthHead)
		}
	case rBCD:
		fallthrough
	case BCD:
		lenVal = rbcd(contentLen)
		if len(lenVal) > 1 {
			return nil, errors.New(ErrInvalidLengthHead)
		}
	default:
		return nil, errors.New(ErrInvalidLengthEncoder)
	}
	return append(lenVal, l.Value...), nil
}

// MarshalJSON is used for formatting the JSON representation of this field.
func (l *Llvar) MarshalJSON() ([]byte, error) {
	// TODO: use encoder, length
	if l.Value == nil {
		return []byte(`""`), nil
	}
	return []byte(`"` + base64.URLEncoding.EncodeToString(l.Value[2:]) + `"`), nil
}

// Load decode Llvar field from bytes
func (l *Llvar) Load(raw []byte, encoder, lenEncoder, length int) (read int, err error) {
	// parse length head:
	var contentLen int
	switch lenEncoder {
	case ASCII:
		read = 2
		contentLen, err = strconv.Atoi(string(raw[:read]))
		if err != nil {
			return 0, errors.New(ErrParseLengthFailed + ": " + string(raw[:2]))
		}
	case rBCD:
		fallthrough
	case BCD:
		read = 1
		contentLen, err = strconv.Atoi(string(bcdr2Ascii(raw[:read], 2)))
		if err != nil {
			return 0, errors.New(ErrParseLengthFailed + ": " + string(raw[0]))
		}
	default:
		return 0, errors.New(ErrInvalidLengthEncoder)
	}
	if len(raw) < (read + contentLen) {
		return 0, errors.New(ErrBadRaw)
	}
	// parse body:
	l.Value = raw[read : read+contentLen]
	read += contentLen
	if encoder != ASCII {
		return 0, errors.New(ErrInvalidEncoder)
	}

	return read, nil
}

// A Llnumeric contains numeric value only in non-fix length, contains length in first 2 symbols. It holds numeric
// value as a string. Supportted encoder are ascii, bcd and rbcd. Length is
// required for marshalling and unmarshalling.
type Llnumeric string

// NewLlnumeric create new Llnumeric field
func NewLlnumeric(val string) *Llnumeric {
	l := Llnumeric(val)
	return &l
}

// IsEmpty check Llnumeric field for empty value
func (l *Llnumeric) IsEmpty() bool {
	return len(*l) == 0
}

// Bytes encode Llnumeric field to bytes
func (l *Llnumeric) Bytes(encoder, lenEncoder, length int) ([]byte, error) {
	raw := []byte(*l)
	if length != -1 && len(raw) > length {
		return nil, fmt.Errorf(ErrValueTooLong, "Llnumeric", length, len(raw))
	}

	val := raw
	switch encoder {
	case ASCII:
	case BCD:
		val = lbcd(raw)
	case rBCD:
		val = rbcd(raw)
	default:
		return nil, errors.New(ErrInvalidEncoder)
	}

	lenStr := fmt.Sprintf("%02d", len(raw)) // length of digital characters
	contentLen := []byte(lenStr)
	var lenVal []byte
	switch lenEncoder {
	case ASCII:
		lenVal = contentLen
		if len(lenVal) > 2 {
			return nil, errors.New(ErrInvalidLengthHead)
		}
	case rBCD:
		fallthrough
	case BCD:
		lenVal = rbcd(contentLen)
		if len(lenVal) > 1 || len(contentLen) > 3 {
			return nil, errors.New(ErrInvalidLengthHead)
		}
	default:
		return nil, errors.New(ErrInvalidLengthEncoder)
	}
	return append(lenVal, val...), nil
}

// Load decode Llnumeric field from bytes
func (l *Llnumeric) Load(raw []byte, encoder, lenEncoder, length int) (read int, err error) {
	// parse length head:
	var contentLen int
	switch lenEncoder {
	case ASCII:
		read = 2
		contentLen, err = strconv.Atoi(string(raw[:read]))
		if err != nil {
			return 0, errors.New(ErrParseLengthFailed + ": " + string(raw[:2]))
		}
	case rBCD:
		fallthrough
	case BCD:
		read = 1
		contentLen, err = strconv.Atoi(string(bcdr2Ascii(raw[:read], 2)))
		if err != nil {
			return 0, errors.New(ErrParseLengthFailed + ": " + string(raw[0]))
		}
	default:
		return 0, errors.New(ErrInvalidLengthEncoder)
	}

	// parse body:
	switch encoder {
	case ASCII:
		if len(raw) < (read + contentLen) {
			return 0, errors.New(ErrBadRaw)
		}
		*l = Llnumeric(string(raw[read : read+contentLen]))
		read += contentLen
	case rBCD:
		fallthrough
	case BCD:
		bcdLen := (contentLen + 1) / 2
		if len(raw) < (read + bcdLen) {
			return 0, errors.New(ErrBadRaw)
		}
		*l = Llnumeric(string(bcdl2Ascii(raw[read:read+bcdLen], contentLen)))
		read += bcdLen
	default:
		return 0, errors.New(ErrInvalidEncoder)
	}
	return read, nil
}

// Lllvar contains bytes in non-fixed length field, first 3 symbols of field contains length
type Lllvar []byte

// NewLllvar create new Lllvar field
func NewLllvar(val []byte) *Lllvar {
	l := Lllvar(val)
	return &l
}

// IsEmpty check Lllvar field for empty value
func (l *Lllvar) IsEmpty() bool {
	return len(*l) == 0
}

// Bytes encode Lllvar field to bytes
func (l *Lllvar) Bytes(encoder, lenEncoder, length int) ([]byte, error) {
	if length != -1 && len(*l) > length {
		return nil, fmt.Errorf(ErrValueTooLong, "Lllvar", length, len(*l))
	}
	if encoder != ASCII {
		return nil, errors.New(ErrInvalidEncoder)
	}

	lenStr := fmt.Sprintf("%03d", len(*l))
	contentLen := []byte(lenStr)
	var lenVal []byte
	switch lenEncoder {
	case ASCII:
		lenVal = contentLen
		if len(lenVal) > 3 {
			return nil, errors.New(ErrInvalidLengthHead)
		}
	case rBCD:
		fallthrough
	case BCD:
		lenVal = rbcd(contentLen)
		if len(lenVal) > 2 || len(contentLen) > 3 {
			return nil, errors.New(ErrInvalidLengthHead)
		}
	default:
		return nil, errors.New(ErrInvalidLengthEncoder)
	}
	return append(lenVal, (*l)...), nil
}

// Load decode Lllvar field from bytes
func (l *Lllvar) Load(raw []byte, encoder, lenEncoder, length int) (read int, err error) {
	// parse length head:
	var contentLen int
	switch lenEncoder {
	case ASCII:
		read = 3
		contentLen, err = strconv.Atoi(string(raw[:read]))
		if err != nil {
			return 0, errors.New(ErrParseLengthFailed + ": " + string(raw[:3]))
		}
	case rBCD:
		fallthrough
	case BCD:
		read = 2
		contentLen, err = strconv.Atoi(string(bcdr2Ascii(raw[:read], 3)))
		if err != nil {
			return 0, errors.New(ErrParseLengthFailed + ": " + string(raw[:2]))
		}
	default:
		return 0, errors.New(ErrInvalidLengthEncoder)
	}
	if len(raw) < (read + contentLen) {
		return 0, errors.New(ErrBadRaw)
	}
	// parse body:
	*l = Lllvar(raw[read : read+contentLen])
	read += contentLen
	if encoder != ASCII {
		return 0, errors.New(ErrInvalidEncoder)
	}

	return read, nil
}

// A Lllnumeric contains numeric value only in non-fix length, contains length in first 3 symbols. It holds numeric
// value as a string. Supportted encoder are ascii, bcd and rbcd. Length is
// required for marshalling and unmarshalling.
type Lllnumeric string

// NewLllnumeric create new Lllnumeric field
func NewLllnumeric(val string) *Lllnumeric {
	l := Lllnumeric(val)
	return &l
}

// IsEmpty check Lllnumeric field for empty value
func (l *Lllnumeric) IsEmpty() bool {
	return len(*l) == 0
}

// Bytes encode Lllnumeric field to bytes
func (l *Lllnumeric) Bytes(encoder, lenEncoder, length int) ([]byte, error) {
	raw := []byte(*l)
	if length != -1 && len(raw) > length {
		return nil, fmt.Errorf(ErrValueTooLong, "Lllnumeric", length, len(raw))
	}

	val := raw
	switch encoder {
	case ASCII:
	case BCD:
		val = lbcd(raw)
	case rBCD:
		val = rbcd(raw)
	default:
		return nil, errors.New(ErrInvalidEncoder)
	}

	lenStr := fmt.Sprintf("%03d", len(raw)) // length of digital characters
	contentLen := []byte(lenStr)
	var lenVal []byte
	switch lenEncoder {
	case ASCII:
		lenVal = contentLen
		if len(lenVal) > 3 {
			return nil, errors.New(ErrInvalidLengthHead)
		}
	case rBCD:
		fallthrough
	case BCD:
		lenVal = rbcd(contentLen)
		if len(lenVal) > 2 || len(contentLen) > 3 {
			return nil, errors.New(ErrInvalidLengthHead)
		}
	default:
		return nil, errors.New(ErrInvalidLengthEncoder)
	}
	return append(lenVal, val...), nil
}

// Load decode Lllnumeric field from bytes
func (l *Lllnumeric) Load(raw []byte, encoder, lenEncoder, length int) (read int, err error) {
	// parse length head:
	var contentLen int
	switch lenEncoder {
	case ASCII:
		read = 3
		contentLen, err = strconv.Atoi(string(raw[:read]))
		if err != nil {
			return 0, errors.New(ErrParseLengthFailed + ": " + string(raw[:3]))
		}
	case rBCD:
		fallthrough
	case BCD:
		read = 2
		contentLen, err = strconv.Atoi(string(bcdr2Ascii(raw[:read], 2)))
		if err != nil {
			return 0, errors.New(ErrParseLengthFailed + ": " + string(raw[:2]))
		}
	default:
		return 0, errors.New(ErrInvalidLengthEncoder)
	}

	// parse body:
	switch encoder {
	case ASCII:
		if len(raw) < (read + contentLen) {
			return 0, errors.New(ErrBadRaw)
		}
		*l = Lllnumeric(string(raw[read : read+contentLen]))
		read += contentLen
	case rBCD:
		fallthrough
	case BCD:
		bcdLen := (contentLen + 1) / 2
		if len(raw) < (read + bcdLen) {
			return 0, errors.New(ErrBadRaw)
		}
		*l = Lllnumeric(string(bcdl2Ascii(raw[read:read+bcdLen], contentLen)))
		read += bcdLen
	default:
		return 0, errors.New(ErrInvalidEncoder)
	}
	return read, nil
}
