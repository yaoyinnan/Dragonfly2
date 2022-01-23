/*
 *     Copyright 2020 The Dragonfly Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package compress

import (
	"io"
)

type Algorithm string

func (ca Algorithm) String() string {
	return string(ca)
}

var (
	// m is a map from name to builder.
	m = make(map[Algorithm]Builder)
)

func Register(b Builder) {
	m[b.Algorithm()] = b
}

func Get(ca Algorithm) Builder {
	if b, ok := m[ca]; ok {
		return b
	}
	return nil
}

// Builder creates a compressor that will be used to compress/decompress data
type Builder interface {
	Build() Compressor
	Algorithm() Algorithm
}

type Compressor interface {

	// Decompress decompresses data
	Decompress(reader io.Reader) (io.ReadCloser, error)

	// Compress compresses data
	Compress(writer io.Writer) (io.WriteCloser, error)

	// CompressRatio return compression ratio
	CompressRatio(data []byte) (ratio float32, err error)
}
