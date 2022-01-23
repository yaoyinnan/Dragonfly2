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

package gzip

import (
	"bytes"
	"compress/gzip"
	"io"
	"sync"

	"github.com/pkg/errors"

	"d7y.io/dragonfly/v2/pkg/compress"
)

const (
	// SpeedAndRatioLB Set the compression speed and compression ratio equalization level
	SpeedAndRatioLB = 4
	Algorithm       = "gzip"
)

type gzipBuilder struct {
}

func (gb *gzipBuilder) Build() compress.Compressor {
	var bufPool = &sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
	return &gzipCompress{
		bufPool: bufPool,
	}
}

func (gb *gzipBuilder) Algorithm() compress.Algorithm {
	return Algorithm
}

func newBuilder() compress.Builder {
	return &gzipBuilder{}
}

func init() {
	compress.Register(newBuilder())
}

type gzipCompress struct {
	bufPool *sync.Pool
}

func (gc *gzipCompress) Decompress(reader io.Reader) (io.ReadCloser, error) {
	r, err := gzip.NewReader(reader)
	return r, err
}

func (gc *gzipCompress) Compress(writer io.Writer) (io.WriteCloser, error) {
	w, err := gzip.NewWriterLevel(writer, SpeedAndRatioLB)
	return w, err
}

func (gc *gzipCompress) CompressRatio(data []byte) (ratio float32, err error) {
	var bb = gc.bufPool.Get().(*bytes.Buffer)
	bb.Reset()
	w, err := gzip.NewWriterLevel(bb, SpeedAndRatioLB)
	if err != nil {
		return -1, errors.Wrap(err, "compression: set gzip compress level error")
	}
	written, err := w.Write(data)
	if err != nil {
		return -1, errors.Wrap(err, "compression: gzip write data error")
	}
	err = w.Close()
	if err != nil {
		return -1, errors.Wrap(err, "compression: gzip write close error")
	}
	return float32(float64(written) / float64(bb.Len())), nil
}
