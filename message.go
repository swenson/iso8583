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
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

const (
	// TagField specifies which ISO 8583 field the struct member maps to.
	TagField string = "field"
	// TagEncode changes the encoding of an ISO 8583 field.
	TagEncode string = "encode"
	// TagLength specifies the length of an ISO 8583 field.
	TagLength string = "length"
	// TagIso is the tag for a plain string encoding the ISO8583 encoding information
	TagIso string = "iso8583"
)

type fieldInfo struct {
	Kind         string
	Value        string
	ByteValue    []byte
	Index        int
	Encode       int
	LenEncode    int
	Length       int
	LengthFormat string
	Field        Type
}

// Message is structure for ISO 8583 message encode and decode
type Message struct {
	Mti          string
	MtiEncode    int
	SecondBitmap bool
	Data         interface{}
}

// NewMessage creates new Message structure
func NewMessage(mti string, data interface{}) *Message {
	return &Message{mti, ASCII, false, data}
}

// Bytes marshall Message to bytes
func (m *Message) Bytes() (ret []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("Critical error:" + fmt.Sprint(r))
			ret = nil
		}
	}()

	ret = make([]byte, 0)

	// generate MTI:
	mtiBytes, err := m.encodeMti()
	if err != nil {
		return nil, err
	}
	ret = append(ret, mtiBytes...)

	// generate bitmap and fields:
	fields := parseFields(m.Data)

	byteNum := 8
	if m.SecondBitmap {
		byteNum = 16
	}
	bitmap := make([]byte, byteNum)
	data := make([]byte, 0, 512)

	for byteIndex := 0; byteIndex < byteNum; byteIndex++ {
		for bitIndex := 0; bitIndex < 8; bitIndex++ {

			i := byteIndex*8 + bitIndex + 1

			// if we need second bitmap (additional 8 bytes) - set first bit in first bitmap
			if m.SecondBitmap && i == 1 {
				step := uint(7 - bitIndex)
				bitmap[byteIndex] |= (0x01 << step)
			}

			if info, ok := fields[i]; ok {

				if info.Kind != "" {
					switch info.Kind {
					case "numeric":
						if info.LengthFormat == "" {
							info.Field = NewNumeric(info.Value)
						} else if info.LengthFormat == "ll" {
							info.Field = NewLlnumeric(info.Value)
						} else if info.LengthFormat == "lll" {
							info.Field = NewLllnumeric(info.Value)
						} else {
							panic("Unknown length format: %s" + info.LengthFormat)
						}
					case "alphanum":
						info.Field = NewAlphanumeric(info.Value)
					case "binary":
						if info.LengthFormat == "" {
							info.Field = NewBinary(info.ByteValue)
						} else if info.LengthFormat == "ll" {
							info.Field = NewLllvar(info.ByteValue)
						} else if info.LengthFormat == "lll" {
							info.Field = NewLllvar(info.ByteValue)
						} else {
							panic("Unknown length format: %s" + info.LengthFormat)
						}
					default:
						panic("Unknown kind: " + info.Kind)
					}
				}

				// if field is empty, then we can't add it to bitmap
				if info.Field.IsEmpty() {
					continue
				}

				// mark 1 in bitmap:
				step := uint(7 - bitIndex)
				bitmap[byteIndex] |= (0x01 << step)
				// append data:
				d, err := info.Field.Bytes(info.Encode, info.LenEncode, info.Length)
				if err != nil {
					return nil, err
				}
				data = append(data, d...)
			}
		}
	}
	ret = append(ret, bitmap...)
	ret = append(ret, data...)

	return ret, nil
}

func (m *Message) encodeMti() ([]byte, error) {
	if m.Mti == "" {
		return nil, errors.New("MTI is required")
	}
	if len(m.Mti) != 4 {
		return nil, errors.New("MTI is invalid")
	}

	// check MTI, it must contain only digits
	if _, err := strconv.Atoi(m.Mti); err != nil {
		return nil, errors.New("MTI is invalid")
	}

	switch m.MtiEncode {
	case BCD:
		return bcd([]byte(m.Mti)), nil
	default:
		return []byte(m.Mti), nil
	}
}

func parseFields(msg interface{}) map[int]*fieldInfo {
	fields := make(map[int]*fieldInfo)

	v := reflect.Indirect(reflect.ValueOf(msg))
	if v.Kind() != reflect.Struct {
		panic("data must be a struct")
	}
	for i := 0; i < v.NumField(); i++ {
		if isPtrOrInterface(v.Field(i).Kind()) && v.Field(i).IsNil() {
			continue
		}

		sf := v.Type().Field(i)

		if sf.Tag == "" {
			continue
		}
		_, isString := v.Field(i).Interface().(string)
		_, isByteArray := v.Field(i).Interface().([]byte)
		if sf.Tag.Get(TagIso) != "" && (isString || isByteArray) {
			// process new tag
			info := sf.Tag.Get(TagIso)
			parts := strings.Split(info, ",")
			if len(parts) < 2 {
				panic("iso8583 tag must have at least two parts")
			}
			kind := parts[0]
			field := 0
			length := -1
			lengthFormat := ""
			for _, part := range parts[1:] {
				sides := strings.Split(part, "=")
				if len(sides) != 2 {
					panic("iso8583 tag parts must have be of the form a=b")
				}
				lhs, rhs := sides[0], sides[1]
				var err error
				switch lhs {
				case "field":
					field, err = strconv.Atoi(rhs)
					if err != nil {
						panic(err)
					}
				case "length":
					length, err = strconv.Atoi(rhs)
					if err != nil {
						panic(err)
					}
				case "length_format":
					lengthFormat = rhs
				default:
					panic("Unknown field: " + lhs)
				}
			}

			fields[field] = &fieldInfo{
				Kind:         kind,
				Index:        field,
				Length:       length,
				LengthFormat: lengthFormat,
			}
			if kind == "binary" {
				fields[field].ByteValue = v.Field(i).Bytes()
			} else {
				fields[field].Value = v.Field(i).String()
			}
			continue
		}

		if sf.Tag.Get(TagField) == "" {
			continue
		}

		index, err := strconv.Atoi(sf.Tag.Get(TagField))
		if err != nil {
			panic("value of field must be numeric")
		}

		encode := 0
		lenEncode := 0
		if raw := sf.Tag.Get(TagEncode); raw != "" {
			enc := strings.Split(raw, ",")
			if len(enc) == 2 {
				lenEncode = parseEncodeStr(enc[0])
				encode = parseEncodeStr(enc[1])
			} else {
				encode = parseEncodeStr(enc[0])
			}
		}

		length := -1
		if l := sf.Tag.Get(TagLength); l != "" {
			length, err = strconv.Atoi(l)
			if err != nil {
				panic("value of length must be numeric")
			}
		}

		field, ok := v.Field(i).Interface().(Type)
		if !ok {
			panic("field must be Iso8583Type")
		}
		fields[index] = &fieldInfo{
			Index:     index,
			Encode:    encode,
			LenEncode: lenEncode,
			Length:    length,
			Field:     field,
		}
	}
	return fields
}

func isPtrOrInterface(k reflect.Kind) bool {
	return k == reflect.Interface || k == reflect.Ptr
}

func parseEncodeStr(str string) int {
	switch str {
	case "ascii":
		return ASCII
	case "lbcd":
		fallthrough
	case "bcd":
		return BCD
	case "rbcd":
		return rBCD
	}
	return -1
}

// Load unmarshall Message from bytes
func (m *Message) Load(raw []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("Critical error:" + fmt.Sprint(r))
		}
	}()

	if m.Mti == "" {
		m.Mti, err = decodeMti(raw, m.MtiEncode)
		if err != nil {
			return err
		}
	}
	start := 4
	if m.MtiEncode == BCD {
		start = 2
	}

	fields := parseFields(m.Data)

	byteNum := 8
	if raw[start]&0x80 == 0x80 {
		// 1st bit == 1
		m.SecondBitmap = true
		byteNum = 16
	}
	bitByte := raw[start : start+byteNum]
	start += byteNum

	for byteIndex := 0; byteIndex < byteNum; byteIndex++ {
		for bitIndex := 0; bitIndex < 8; bitIndex++ {
			step := uint(7 - bitIndex)
			if (bitByte[byteIndex] & (0x01 << step)) == 0 {
				continue
			}

			i := byteIndex*8 + bitIndex + 1
			if i == 1 {
				// field 1 is the second bitmap
				continue
			}
			f, ok := fields[i]
			if !ok {
				return fmt.Errorf("field %d not defined", i)
			}
			l, err := f.Field.Load(raw[start:], f.Encode, f.LenEncode, f.Length)
			if err != nil {
				return fmt.Errorf("field %d: %s", i, err)
			}
			start += l
		}
	}
	return nil
}
