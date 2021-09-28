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

package errors

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/suite"
)

func TestErrorSuite(t *testing.T) {
	suite.Run(t, new(ErrorTestSuite))
}

type ErrorTestSuite struct {
	suite.Suite
}

func (s *ErrorTestSuite) TestIsConvertFailed() {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "equal",
			args: args{
				err: ErrConvertFailed,
			},
			want: true,
		}, {
			name: "wrap",
			args: args{
				err: errors.Wrapf(ErrConvertFailed, "wrap err"),
			},
			want: true,
		}, {
			name: "notEqual",
			args: args{
				err: errors.Wrapf(ErrInvalidValue, "invaid"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, IsConvertFailed(tt.args.err))
		})
	}
}

func (s *ErrorTestSuite) TestIsDataNotFound() {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "equal",
			args: args{
				err: ErrDataNotFound,
			},
			want: true,
		}, {
			name: "wrap",
			args: args{
				err: errors.Wrapf(ErrDataNotFound, "wrap err"),
			},
			want: true,
		}, {
			name: "notEqual",
			args: args{
				err: errors.Wrapf(ErrInvalidValue, "invaid"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, IsDataNotFound(tt.args.err))
		})
	}
}

func (s *ErrorTestSuite) TestIsDownloadFail() {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "equal",
			args: args{
				err: ErrTaskDownloadFail,
			},
			want: true,
		}, {
			name: "wrap",
			args: args{
				err: errors.Wrapf(errors.Wrapf(ErrTaskDownloadFail, "wrap err"), "wapp err"),
			},
			want: true,
		}, {
			name: "notEqual",
			args: args{
				err: errors.Wrapf(ErrInvalidValue, "invaid"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, IsDownloadFail(tt.args.err))
		})
	}
}

func (s *ErrorTestSuite) TestIsFileNotExist() {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "equal",
			args: args{
				err: ErrFileNotExist{},
			},
			want: true,
		}, {
			name: "wrap",
			args: args{
				err: errors.Wrapf(errors.Wrapf(ErrFileNotExist{}, "wrap err"), "wapp err"),
			},
			want: true,
		}, {
			name: "notEqual",
			args: args{
				err: errors.Wrapf(ErrInvalidValue, "invaid"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, IsFileNotExist(tt.args.err))
		})
	}
}

func (s *ErrorTestSuite) TestIsInvalidValue() {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "equal",
			args: args{
				err: ErrInvalidValue,
			},
			want: true,
		}, {
			name: "wrap",
			args: args{
				err: errors.Wrapf(errors.Wrapf(ErrInvalidValue, "wrap err"), "wapp err"),
			},
			want: true,
		}, {
			name: "notEqual",
			args: args{
				err: errors.Wrapf(ErrInvalidValue, "invaid"),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, IsInvalidValue(tt.args.err))
		})
	}
}

func (s *ErrorTestSuite) TestIsInconsistentValues() {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "equal",
			args: args{
				err: ErrInconsistentValues{},
			},
			want: true,
		}, {
			name: "wrap",
			args: args{
				err: errors.Wrapf(errors.Wrapf(ErrInconsistentValues{}, "wrap err"), "wapp err"),
			},
			want: true,
		}, {
			name: "notEqual",
			args: args{
				err: errors.Wrapf(ErrInvalidValue, "invaid"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, IsInconsistentValues(tt.args.err))
		})
	}
}

func (s *ErrorTestSuite) TestIsResourceExpired() {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "equal",
			args: args{
				err: ErrResourceExpired{},
			},
			want: true,
		}, {
			name: "wrap",
			args: args{
				err: errors.Wrapf(errors.Wrapf(ErrResourceExpired{}, "wrap err"), "wapp err"),
			},
			want: true,
		}, {
			name: "notEqual",
			args: args{
				err: errors.Wrapf(ErrInvalidValue, "invaid"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, IsResourceExpired(tt.args.err))
		})
	}
}

func (s *ErrorTestSuite) TestIsResourceNotSupportRangeRequest() {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "equal",
			args: args{
				err: ErrResourceNotSupportRangeRequest{},
			},
			want: true,
		}, {
			name: "wrap",
			args: args{
				err: errors.Wrapf(errors.Wrapf(ErrResourceNotSupportRangeRequest{}, "wrap err"), "wapp err"),
			},
			want: true,
		}, {
			name: "notEqual",
			args: args{
				err: errors.Wrapf(ErrInvalidValue, "invaid"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, IsResourceNotSupportRangeRequest(tt.args.err))
		})
	}
}

func (s *ErrorTestSuite) TestIsSystemError() {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "equal",
			args: args{
				err: ErrSystemError,
			},
			want: true,
		}, {
			name: "wrap",
			args: args{
				err: errors.Wrapf(errors.Wrapf(ErrSystemError, "wrap err"), "wapp err"),
			},
			want: true,
		}, {
			name: "notEqual",
			args: args{
				err: errors.Wrapf(ErrInvalidValue, "invaid"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, IsSystemError(tt.args.err))
		})
	}
}

func (s *ErrorTestSuite) TestIsTaskIDDuplicate() {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "equal",
			args: args{
				err: ErrTaskIDDuplicate{
					TaskID: "test",
					Cause:  fmt.Errorf("test"),
				},
			},
			want: true,
		}, {
			name: "wrap",
			args: args{
				err: errors.Wrapf(ErrTaskIDDuplicate{
					TaskID: "test",
					Cause:  fmt.Errorf("test")}, "wapp err"),
			},
			want: true,
		}, {
			name: "notEqual",
			args: args{
				err: errors.Wrapf(ErrInvalidValue, "invaid"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, IsTaskIDDuplicate(tt.args.err))
		})
	}
}

func (s *ErrorTestSuite) TestIsURLNotReachable() {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "equal",
			args: args{
				err: ErrURLNotReachable{
					URL: "test",
				},
			},
			want: true,
		}, {
			name: "wrap",
			args: args{
				err: errors.Wrapf(ErrURLNotReachable{
					URL: "test",
				}, "wapp err"),
			},
			want: true,
		}, {
			name: "notEqual",
			args: args{
				err: errors.Wrapf(ErrInvalidValue, "invaid"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, IsURLNotReachable(tt.args.err))
		})
	}
}
