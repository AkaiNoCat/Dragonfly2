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

package rangeutils

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	separator = "-"
)

type Range struct {
	StartIndex int64 `json:"start_index"`
	EndIndex   int64 `json:"end_index"`
}

func (r Range) String() string {
	return fmt.Sprintf("%d%s%d", r.StartIndex, separator, r.EndIndex)
}

// GetRange parses Range according to range string.
// rangeStr: "start-end"
func GetRange(rangeStr string) (r *Range, err error) {
	ranges := strings.Split(rangeStr, separator)
	if len(ranges) != 2 {
		return nil, fmt.Errorf("range value(%s) is illegal which should be like 0-45535", rangeStr)
	}

	startIndex, err := strconv.ParseInt(ranges[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("range(%s) start is not a non-negative number", rangeStr)
	}
	endIndex, err := strconv.ParseInt(ranges[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("range(%s) end is not a non-negative number", rangeStr)
	}

	if endIndex < startIndex {
		return nil, fmt.Errorf("range(%s) start is larger than end", rangeStr)
	}

	return &Range{
		StartIndex: startIndex,
		EndIndex:   endIndex,
	}, nil
}

// ParseRange parses the start and the end from rangeStr and returns them.
func ParseRange(rangeStr string, length int64) (*Range, error) {
	if strings.Count(rangeStr, "-") != 1 {
		return nil, errors.Errorf("invalid range: %s, should be like 0-1023", rangeStr)
	}

	// -{length}
	if strings.HasPrefix(rangeStr, "-") {
		rangeStruct, err := handlePrefixRange(rangeStr, length)
		if err != nil {
			return nil, err
		}
		return rangeStruct, nil
	}

	// {startIndex}-
	if strings.HasSuffix(rangeStr, "-") {
		rangeStruct, err := handleSuffixRange(rangeStr, length)
		if err != nil {
			return nil, err
		}
		return rangeStruct, nil
	}

	rangeStruct, err := handlePairRange(rangeStr, length)
	if err != nil {
		return nil, err
	}
	return rangeStruct, nil
}

func handlePrefixRange(rangeStr string, length int64) (*Range, error) {
	downLength, err := strconv.ParseInt(strings.TrimPrefix(rangeStr, "-"), 10, 64)
	if err != nil || downLength < 0 {
		return nil, errors.Errorf("failed to parse range: %s to int: %v", rangeStr, err)
	}

	if downLength > length {
		return nil, errors.Errorf("range: %s, the downLength is larger than length", rangeStr)
	}

	return &Range{
		StartIndex: length - downLength,
		EndIndex:   length - 1,
	}, nil
}

func handleSuffixRange(rangeStr string, length int64) (*Range, error) {
	startIndex, err := strconv.ParseInt(strings.TrimSuffix(rangeStr, "-"), 10, 64)
	if err != nil || startIndex < 0 {
		return nil, errors.Errorf("failed to parse range: %s to int: %v", rangeStr, err)
	}

	if startIndex > length {
		return nil, errors.Errorf("range: %s, the startIndex is larger than length", rangeStr)
	}

	return &Range{
		StartIndex: startIndex,
		EndIndex:   length - 1,
	}, nil
}

func handlePairRange(rangeStr string, length int64) (*Range, error) {
	rangePair := strings.Split(rangeStr, "-")

	startIndex, err := strconv.ParseInt(rangePair[0], 10, 64)
	if err != nil || startIndex < 0 {
		return nil, errors.Errorf("failed to parse range: %s to int: %v", rangeStr, err)
	}
	if startIndex > length {
		return nil, errors.Errorf("range: %s, the startIndex is larger than length", rangeStr)
	}

	endIndex, err := strconv.ParseInt(rangePair[1], 10, 64)
	if err != nil || endIndex < 0 {
		return nil, errors.Errorf("failed to parse range: %s to int: %v", rangeStr, err)
	}
	if endIndex >= length {
		//attention
		endIndex = length - 1
	}

	if endIndex < startIndex {
		return nil, errors.Errorf("range: %s, the start is larger the end", rangeStr)
	}

	return &Range{
		StartIndex: startIndex,
		EndIndex:   endIndex,
	}, nil
}
