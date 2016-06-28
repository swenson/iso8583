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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBCDDecode(t *testing.T) {

	b := []byte("954")
	r := rbcd(b)
	assert.Equal(t, "0954", fmt.Sprintf("%X", r))

	r = lbcd(b)
	assert.Equal(t, "9540", fmt.Sprintf("%X", r))

	b = []byte("31")
	r = lbcd(b)
	assert.Equal(t, "31", fmt.Sprintf("%X", r))
	r = rbcd(b)
	assert.Equal(t, "31", fmt.Sprintf("%X", r))

	b = []byte("123ab4")
	assert.Equal(t, []byte("\x12\x3a\xb4"), bcd(b))
	b = []byte("00")
	assert.Equal(t, []byte("\x00"), bcd(b))

	assert.Panics(t,
		func() {
			bcd([]byte("test"))
		}, "Calling bcd() with invalid hex should panic")

}

func TestBCDEncode(t *testing.T) {
	assert.Equal(t, []byte("12a34f"), bcd2Ascii([]byte("\x12\xa3\x4f")))

	assert.Equal(t, []byte("12345"), bcdl2Ascii([]byte("\x12\x34\x50"), 5))

	assert.Equal(t, []byte("12345"), bcdr2Ascii([]byte("\x01\x23\x45"), 5))
}
