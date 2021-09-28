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

package searcher

import (
	"testing"

	"d7y.io/dragonfly/v2/manager/model"
	"github.com/stretchr/testify/assert"
)

func TestSchedulerCluster(t *testing.T) {
	tests := []struct {
		name              string
		schedulerClusters []model.SchedulerCluster
		conditions        map[string]string
		expect            func(t *testing.T, data model.SchedulerCluster, ok bool)
	}{
		{
			name:              "conditions is empty",
			schedulerClusters: []model.SchedulerCluster{{Name: "foo"}},
			conditions:        map[string]string{},
			expect: func(t *testing.T, data model.SchedulerCluster, ok bool) {
				assert := assert.New(t)
				assert.Equal(ok, false)
			},
		},
		{
			name:              "scheduler clusters is empty",
			schedulerClusters: []model.SchedulerCluster{},
			conditions:        map[string]string{"location": "foo"},
			expect: func(t *testing.T, data model.SchedulerCluster, ok bool) {
				assert := assert.New(t)
				assert.Equal(ok, false)
			},
		},
		{
			name: "match according to security_domain condition",
			schedulerClusters: []model.SchedulerCluster{
				{
					Name: "foo",
					SecurityGroup: model.SecurityGroup{
						Domain: "domain-1",
					},
				},
				{
					Name: "bar",
				},
			},
			conditions: map[string]string{"security_domain": "domain-1"},
			expect: func(t *testing.T, data model.SchedulerCluster, ok bool) {
				assert := assert.New(t)
				assert.Equal(data.Name, "foo")
				assert.Equal(ok, true)
			},
		},
		{
			name: "match according to location condition",
			schedulerClusters: []model.SchedulerCluster{
				{
					Name: "foo",
					Scopes: map[string]interface{}{
						"location": []string{"location-1"},
					},
				},
				{
					Name: "bar",
				},
			},
			conditions: map[string]string{"location": "location-1"},
			expect: func(t *testing.T, data model.SchedulerCluster, ok bool) {
				assert := assert.New(t)
				assert.Equal(data.Name, "foo")
				assert.Equal(ok, true)
			},
		},
		{
			name: "match according to idc condition",
			schedulerClusters: []model.SchedulerCluster{
				{
					Name: "foo",
					Scopes: map[string]interface{}{
						"idc": []string{"idc-1"},
					},
				},
				{
					Name: "bar",
				},
			},
			conditions: map[string]string{"idc": "idc-1"},
			expect: func(t *testing.T, data model.SchedulerCluster, ok bool) {
				assert := assert.New(t)
				assert.Equal(data.Name, "foo")
				assert.Equal(ok, true)
			},
		},
		{
			name: "match according to location and idc condition",
			schedulerClusters: []model.SchedulerCluster{
				{
					Name: "foo",
					Scopes: map[string]interface{}{
						"location": []string{"location-1"},
						"idc":      []string{"idc-1"},
					},
				},
				{
					Name: "bar",
				},
			},
			conditions: map[string]string{
				"location": "location-1",
				"idc":      "idc-1",
			},
			expect: func(t *testing.T, data model.SchedulerCluster, ok bool) {
				assert := assert.New(t)
				assert.Equal(data.Name, "foo")
				assert.Equal(ok, true)
			},
		},
		{
			name: "match according to security_domain and location conditions",
			schedulerClusters: []model.SchedulerCluster{
				{
					Name: "foo",
					Scopes: map[string]interface{}{
						"location": []string{"location-1"},
					},
					SecurityGroup: model.SecurityGroup{
						Domain: "domain-1",
					},
				},
				{
					Name: "bar",
				},
			},
			conditions: map[string]string{
				"security_domain": "domain-1",
				"location":        "location-1",
			},
			expect: func(t *testing.T, data model.SchedulerCluster, ok bool) {
				assert := assert.New(t)
				assert.Equal(data.Name, "foo")
				assert.Equal(ok, true)
			},
		},
		{
			name: "match according to security_domain and idc conditions",
			schedulerClusters: []model.SchedulerCluster{
				{
					Name: "foo",
					Scopes: map[string]interface{}{
						"idc": []string{"idc-1"},
					},
					SecurityGroup: model.SecurityGroup{
						Domain: "domain-1",
					},
				},
				{
					Name: "bar",
				},
			},
			conditions: map[string]string{
				"security_domain": "domain-1",
				"idc":             "idc-1",
			},
			expect: func(t *testing.T, data model.SchedulerCluster, ok bool) {
				assert := assert.New(t)
				assert.Equal(data.Name, "foo")
				assert.Equal(ok, true)
			},
		},
		{
			name: "match according to all conditions",
			schedulerClusters: []model.SchedulerCluster{
				{
					Name: "foo",
					Scopes: map[string]interface{}{
						"idc":      []string{"idc-1"},
						"location": []string{"location-1"},
					},
					SecurityGroup: model.SecurityGroup{
						Domain: "domain-1",
					},
				},
				{
					Name: "bar",
				},
			},
			conditions: map[string]string{
				"security_domain": "domain-1",
				"idc":             "idc-1",
				"location":        "location-1",
			},
			expect: func(t *testing.T, data model.SchedulerCluster, ok bool) {
				assert := assert.New(t)
				assert.Equal(data.Name, "foo")
				assert.Equal(ok, true)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			searcher := New()
			clusters, ok := searcher.FindSchedulerCluster(tc.schedulerClusters, tc.conditions)
			tc.expect(t, clusters, ok)
		})
	}
}

func TestCalculateSchedulerClusterMean(t *testing.T) {
	tests := []struct {
		name       string
		conditions map[string]string
		rawScopes  map[string]interface{}
		expect     func(t *testing.T, mean float64)
	}{
		{
			name:       "conditions and rawScopes is empty",
			conditions: map[string]string{},
			rawScopes:  map[string]interface{}{},
			expect: func(t *testing.T, mean float64) {
				assert := assert.New(t)
				assert.Equal(mean, float64(0))
			},
		},
		{
			name: "missed matches",
			conditions: map[string]string{
				"location": "location-1",
			},
			rawScopes: map[string]interface{}{
				"idc": []string{"idc-1"},
			},
			expect: func(t *testing.T, mean float64) {
				assert := assert.New(t)
				assert.Equal(mean, float64(0))
			},
		},
		{
			name: "match according to location",
			conditions: map[string]string{
				"location": "location-1",
			},
			rawScopes: map[string]interface{}{
				"location": []string{"location-1"},
			},
			expect: func(t *testing.T, mean float64) {
				assert := assert.New(t)
				assert.Equal(mean, float64(conditionLocationWeight))
			},
		},
		{
			name: "match according to idc",
			conditions: map[string]string{
				"idc": "idc-1",
			},
			rawScopes: map[string]interface{}{
				"idc": []string{"idc-1"},
			},
			expect: func(t *testing.T, mean float64) {
				assert := assert.New(t)
				assert.Equal(mean, float64(conditionIDCWeight))
			},
		},
		{
			name: "match according to location and idc",
			conditions: map[string]string{
				"location": "location-1",
				"idc":      "idc-1",
			},
			rawScopes: map[string]interface{}{
				"location": []string{"location-1"},
				"idc":      []string{"idc-1"},
			},
			expect: func(t *testing.T, mean float64) {
				assert := assert.New(t)
				assert.Equal(mean, float64(conditionLocationWeight+conditionIDCWeight))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mean := calculateSchedulerClusterMean(tc.conditions, tc.rawScopes)
			tc.expect(t, mean)
		})
	}
}

func TestCalculateConditionScore(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		scope  []string
		expect func(t *testing.T, score float64)
	}{
		{
			name:  "value is empty",
			value: "",
			scope: []string{"foo"},
			expect: func(t *testing.T, score float64) {
				assert := assert.New(t)
				assert.Equal(score, float64(0))
			},
		},
		{
			name:  "scope is empty",
			value: "foo",
			scope: []string{},
			expect: func(t *testing.T, score float64) {
				assert := assert.New(t)
				assert.Equal(score, float64(0))
			},
		},
		{
			name:  "match according to value",
			value: "foo",
			scope: []string{"foo"},
			expect: func(t *testing.T, score float64) {
				assert := assert.New(t)
				assert.Equal(score, float64(1))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score := calculateConditionScore(tc.value, tc.scope)
			tc.expect(t, score)
		})
	}
}
