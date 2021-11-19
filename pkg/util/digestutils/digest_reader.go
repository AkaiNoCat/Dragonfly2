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

package digestutils

import (
	"crypto/md5"
	"encoding/hex"
	"hash"
	"io"

	"github.com/pkg/errors"

	logger "d7y.io/dragonfly/v2/internal/dflog"
)

var (
	ErrDigestNotMatch = errors.New("digest not match")
)

// digestReader reads stream with RateLimiter.
type digestReader struct {
	r      io.Reader
	hash   hash.Hash
	digest string
	*logger.SugaredLoggerOnWith
}

type DigestReader interface {
	io.Reader
	Digest() string
}

// TODO add AF_ALG digest https://github.com/golang/sys/commit/e24f485414aeafb646f6fca458b0bf869c0880a1

func NewDigestReader(log *logger.SugaredLoggerOnWith, reader io.Reader, digest ...string) io.Reader {
	var d string
	if len(digest) > 0 {
		d = digest[0]
	}
	return &digestReader{
		SugaredLoggerOnWith: log,
		digest:              d,
		// TODO support more digest method like sha1, sha256
		hash: md5.New(),
		r:    reader,
	}
}

func (dr *digestReader) Read(p []byte) (int, error) {
	n, err := dr.r.Read(p)
	if err != nil && err != io.EOF {
		return n, err
	}
	if n > 0 {
		dr.hash.Write(p[:n])
	}
	if err == io.EOF && dr.digest != "" {
		digest := dr.Digest()
		if digest != dr.digest {
			dr.Warnf("digest not match, desired: %s, actual: %s", dr.digest, digest)
			return n, ErrDigestNotMatch
		}
		dr.Debugf("digest match: %s", digest)
	}
	return n, err
}

// Digest returns the digest of contents.
func (dr *digestReader) Digest() string {
	return hex.EncodeToString(dr.hash.Sum(nil)[:16])
}
