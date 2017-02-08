/*
   Copyright 2015-2017 Red Hat, Inc. and/or its affiliates
   and other contributors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package metrics

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
)

func integrationClient() (*Client, error) {
	t, err := randomString()
	if err != nil {
		return nil, err
	}

	p := Parameters{Tenant: t, Url: "http://localhost:8080", AdminToken: "secret"}
	// p := Parameters{Tenant: t, Host: "localhost:8180"}
	// p := Parameters{Tenant: t, Url: "http://192.168.1.105:8080"}
	// p := Parameters{Tenant: t, Host: "209.132.178.218:18080"}
	return NewHawkularClient(p)
}

func randomString() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%X", b[:]), nil
}

func TestTenant(t *testing.T) {
	c, err := integrationClient()
	assert.NoError(t, err)

	// Create simple Tenant
	id, _ := randomString()
	tenant := TenantDefinition{ID: id}
	created, err := c.CreateTenant(tenant)
	assert.NoError(t, err)
	assert.True(t, created)

	// Create tenant with retention settings
	idr, _ := randomString()
	// var typ MetricType
	typ := Gauge

	retentions := make(map[MetricType]int)
	retentions[typ] = 5
	tenant = TenantDefinition{ID: idr, Retentions: retentions}
	created, err = c.CreateTenant(tenant)
	assert.NoError(t, err)
	assert.True(t, created)

	// Fetch Tenants
	tenants, err := c.Tenants()
	assert.NoError(t, err)
	assert.True(t, len(tenants) > 0)

	var tenantFinder = func(tenantId string, tenantList []*TenantDefinition) *TenantDefinition {
		for _, v := range tenantList {
			if v.ID == tenantId {
				return v
			}
		}
		return nil
	}

	ft := tenantFinder(id, tenants)
	assert.NotNil(t, ft)
	assert.Equal(t, id, ft.ID)

	ft = tenantFinder(idr, tenants)
	assert.NotNil(t, ft)
	assert.Equal(t, idr, ft.ID)

	assert.Equal(t, ft.Retentions[typ], 5)
}

func TestTenantModifier(t *testing.T) {
	c, err := integrationClient()
	assert.Nil(t, err)

	ot, _ := randomString()

	// Create for another tenant
	id := "test.metric.create.numeric.tenant.1"
	md := MetricDefinition{ID: id, Type: Gauge}

	ok, err := c.Create(md, Tenant(ot))
	assert.Nil(t, err)
	assert.True(t, ok, "MetricDefinition should have been created")

	// Try to fetch from default tenant - should fail
	mds, err := c.Definitions(Filters(TypeFilter(Gauge)))
	assert.Nil(t, err)
	assert.Nil(t, mds)

	// Try to fetch from the given tenant - should succeed
	mds, err = c.Definitions(Filters(TypeFilter(Gauge)), Tenant(ot))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(mds))
}

func TestCreate(t *testing.T) {
	c, err := integrationClient()
	assert.Nil(t, err)

	id := "test.metric.create.numeric.1"
	md := MetricDefinition{ID: id, Type: Gauge}
	ok, err := c.Create(md)
	assert.Nil(t, err)
	assert.True(t, ok, "MetricDefinition should have been created")

	// Try to recreate the same..
	ok, err = c.Create(md)
	assert.False(t, ok, "Should have received false when recreating them same metric")
	assert.Nil(t, err)

	// Use tags and dataRetention
	tags := make(map[string]string)
	tags["units"] = "bytes"
	tags["env"] = "unittest"
	mdTags := MetricDefinition{ID: "test.metric.create.numeric.2", Tags: tags, Type: Gauge}

	ok, err = c.Create(mdTags)
	assert.True(t, ok, "MetricDefinition should have been created")
	assert.Nil(t, err)

	mdReten := MetricDefinition{ID: "test/metric/create/availability/1", RetentionTime: 12, Type: Availability}
	ok, err = c.Create(mdReten)
	assert.Nil(t, err)
	assert.True(t, ok, "MetricDefinition should have been created")

	// Test one with whitespace
	mdSpace := MetricDefinition{ID: "test metric whitespace 1", Type: Gauge}
	ok, err = c.Create(mdSpace)
	assert.Nil(t, err)
	assert.True(t, ok, "MetricDefinition should have been created")

	// Fetch all the previously created metrics and test equalities..
	mdq, err := c.Definitions(Filters(TypeFilter(Gauge)))
	assert.Nil(t, err)
	assert.Equal(t, 3, len(mdq), "Size of the returned gauge metrics does not match 3")

	mdm := make(map[string]MetricDefinition)
	for _, v := range mdq {
		mdm[v.ID] = *v
	}

	assert.Equal(t, md.ID, mdm[id].ID)
	assert.Equal(t, mdSpace.ID, mdm[mdSpace.ID].ID)
	assert.True(t, reflect.DeepEqual(tags, mdm["test.metric.create.numeric.2"].Tags))

	mda, err := c.Definitions(Filters(TypeFilter(Availability)))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(mda))
	assert.Equal(t, "test/metric/create/availability/1", mda[0].ID)
	assert.Equal(t, 12, mda[0].RetentionTime)

	if mda[0].Type != Availability {
		assert.FailNow(t, "Type did not match Availability", mda[0].Type)
	}
}

func TestTagsModification(t *testing.T) {
	c, err := integrationClient()
	assert.Nil(t, err)
	id := "test/tags/modify/1"
	// Create metric without tags
	md := MetricDefinition{ID: id, Type: Gauge}
	ok, err := c.Create(md)
	assert.Nil(t, err)
	assert.True(t, ok, "MetricDefinition should have been created")

	// Add tags
	tags := make(map[string]string)
	tags["ab"] = "ac"
	tags["host value"] = "test whitespace"
	tags["plus+is+valid+too"] = "plus+value"
	err = c.UpdateTags(Gauge, id, tags)
	assert.Nil(t, err)

	// Fetch metric tags - check for equality
	mdTags, err := c.Tags(Gauge, id)
	assert.Nil(t, err)

	assert.True(t, reflect.DeepEqual(tags, mdTags), "Tags did not match the updated ones")

	tagNamesList := make([]string, 0, len(tags))
	for k, _ := range tags {
		tagNamesList = append(tagNamesList, k)
	}

	// Delete some metric tags
	err = c.DeleteTags(Gauge, id, tagNamesList)
	assert.Nil(t, err)

	// Fetch metric - check that tags were deleted
	mdTags, err = c.Tags(Gauge, id)
	assert.Nil(t, err)
	assert.False(t, len(mdTags) > 0, "Received deleted tags")
}

func checkDatapoints(t *testing.T, c *Client, orig *MetricHeader) {
	metric, err := c.ReadRaw(orig.Type, orig.ID)
	assert.NoError(t, err)
	assert.Equal(t, len(orig.Data), len(metric), "Amount of datapoints does not match expected value")

	for i, d := range metric {
		assert.True(t, orig.Data[i].Timestamp.Equal(d.Timestamp))
		switch orig.Type {
		case Counter:
			origV, _ := orig.Data[i].Value.(int64)
			recvV, _ := d.Value.(int64)
			assert.Equal(t, int64(origV), int64(recvV))
		case Gauge:
			origV, _ := ConvertToFloat64(orig.Data[i].Value)
			recvV, _ := ConvertToFloat64(d.Value)
			assert.Equal(t, origV, recvV)
		case String:
			origV, _ := orig.Data[i].Value.(string)
			recvV, _ := d.Value.(string)
			assert.Equal(t, string(origV), string(recvV))
		}

		assert.True(t, d.Timestamp.Unix() > 0)
	}
}

func TestAddMixedMulti(t *testing.T) {

	// Modify to send both Availability as well as Gauge metrics at the same time
	c, err := integrationClient()
	assert.NoError(t, err)

	startTime := time.Now().Truncate(time.Millisecond)

	mone := Datapoint{Value: 2, Timestamp: startTime}
	hOne := MetricHeader{
		ID:   "test.multi.numeric.1",
		Data: []Datapoint{mone},
		Type: Counter,
	}

	mTwoOne := Datapoint{Value: 1.45, Timestamp: startTime}

	mTwoTwoT := startTime.Add(-1 * time.Second)

	mTwoTwo := Datapoint{Value: float64(4.56), Timestamp: mTwoTwoT}
	hTwo := MetricHeader{
		ID:   "test.multi.numeric.2",
		Data: []Datapoint{mTwoOne, mTwoTwo},
		Type: Gauge,
	}

	mThree := Datapoint{Value: "stringType_test", Timestamp: mTwoTwoT}
	hThree := MetricHeader{
		ID:   "test.multi.string.3",
		Data: []Datapoint{mThree},
		Type: String,
	}

	h := []MetricHeader{hOne, hTwo, hThree}

	err = c.Write(h)
	assert.NoError(t, err)

	checkDatapoints(t, c, &hOne)
	checkDatapoints(t, c, &hTwo)
	checkDatapoints(t, c, &hThree)
}

func TestCheckErrors(t *testing.T) {
	c, err := integrationClient()
	assert.Nil(t, err)

	mH := MetricHeader{
		ID:   "test.number.as.string",
		Data: []Datapoint{Datapoint{Value: "notFloat"}},
		Type: Gauge,
	}

	err = c.Write([]MetricHeader{mH})
	assert.NotNil(t, err, "Invalid non-float value should not be accepted")
	_, err = c.ReadRaw(mH.Type, mH.ID)
	assert.Nil(t, err, "Querying empty metric should not generate an error")
}

func TestTokenAuthenticationWithSSL(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Authorization", r.Header.Get("Authorization"))
	}))
	defer s.Close()

	tenant, err := randomString()
	assert.NoError(t, err)

	tC := &tls.Config{InsecureSkipVerify: true}

	p := Parameters{
		Tenant:    tenant,
		Url:       s.URL,
		Token:     "62590bf9827213afadea8b5077a5bdc0",
		TLSConfig: tC,
	}

	c, err := NewHawkularClient(p)
	assert.NoError(t, err)

	r, err := c.Send(c.URL("GET"))
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("Bearer %s", p.Token), r.Header.Get("X-Authorization"))
}

func TestBasicAuthenticationWithSSL(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Authorization", r.Header.Get("Authorization"))
	}))
	defer s.Close()

	tC := &tls.Config{InsecureSkipVerify: true}

	p := Parameters{
		Tenant:    "some tenant",
		Url:       s.URL,
		Username:  "user",
		Password:  "pass",
		TLSConfig: tC,
	}

	c, err := NewHawkularClient(p)
	assert.NoError(t, err)

	r, err := c.Send(c.URL("GET"))
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%v:%v", p.Username, p.Password)))), r.Header.Get("X-Authorization"))
}

func TestInvalidBasicAuthentication(t *testing.T) {
	tC := &tls.Config{InsecureSkipVerify: true}

	p := Parameters{
		Tenant:    "some tenant",
		Url:       "http://localhost:8080",
		Username:  "user",
		Password:  "pass",
		Token:     "token",
		TLSConfig: tC,
	}

	_, err := NewHawkularClient(p)
	assert.Error(t, err, "Should not be able to specify both credentials and token")

	p = Parameters{
		Tenant:    "some tenant",
		Url:       "http://localhost:8080",
		Username:  "user",
		TLSConfig: tC,
	}

	_, err = NewHawkularClient(p)
	assert.Error(t, err, "Should not be able to specify just username")

	p = Parameters{
		Tenant:    "some tenant",
		Url:       "http://localhost:8080",
		Password:  "pass",
		TLSConfig: tC,
	}

	_, err = NewHawkularClient(p)
	assert.Error(t, err, "Should not be able to specify just password")
}

func TestBuckets(t *testing.T) {
	c, err := integrationClient()
	assert.NoError(t, err)

	tags := make(map[string]string)
	tags["units"] = "bytes"
	tags["env"] = "unittest"
	// Needs https://issues.jboss.org/browse/HWKMETRICS-530 to be fixed first
	// tags["whitespace plus+empty"] = "space+causes problems"
	mdTags := MetricDefinition{ID: "test.buckets.1", Tags: tags, Type: Gauge}

	ok, err := c.Create(mdTags)
	assert.NoError(t, err)
	assert.True(t, ok)

	hone := MetricHeader{
		ID: "test.buckets.1",
		// Data: []Datapoint{mone},
		Type: Gauge,
	}

	data := make([]Datapoint, 0, 10)
	ts := time.Now()

	for i := 0; i < 10; i++ {
		ts = ts.Add(-1 * time.Second)
		val := 1.45 * float64(i)
		data = append(data, Datapoint{
			Value:     val,
			Timestamp: ts,
		})
	}

	hone.Data = data

	err = c.Write([]MetricHeader{hone})
	assert.NoError(t, err)

	bp, err := c.ReadBuckets(Gauge, Filters(TagsFilter(tags), BucketsFilter(1), PercentilesFilter([]float64{90.0, 99.0})))
	assert.NoError(t, err)
	assert.NotNil(t, bp)

	assert.Equal(t, 1, len(bp), "Only one bucket was requested")
	assert.Equal(t, uint64(10), bp[0].Samples, "Sampling should be based on 10 values")
	assert.Equal(t, 2, len(bp[0].Percentiles), "Two percentiles were requested")

	assert.Equal(t, 90.0, bp[0].Percentiles[0].Quantile)
	assert.Equal(t, 99.0, bp[0].Percentiles[1].Quantile)
	assert.True(t, bp[0].Start.Unix() > 0, "Start time should be higher than 0")
	assert.True(t, bp[0].End.Unix() > 0, "End time should be higher than 0")

	bp, err = c.ReadBuckets(Gauge, Filters(TagsFilter(tags), BucketsDurationFilter(time.Second), StartTimeFilter(ts)))
	assert.NoError(t, err)
	assert.NotNil(t, bp)

	assert.Equal(t, 11, len(bp), "One second buckets were requested for the whole time")
}

func TestTagQueries(t *testing.T) {
	c, err := integrationClient()
	assert.NoError(t, err)

	tags := make(map[string]string)

	// Create definitions
	for i := 1; i < 10; i++ {
		hostname := fmt.Sprintf("host%d", i)
		metricID := fmt.Sprintf("test.tags.host.%d", i)
		tags["hostname"] = hostname // No need to worry about using the same map
		md := MetricDefinition{ID: metricID, Tags: tags, Type: Gauge}

		ok, err := c.Create(md)
		assert.NoError(t, err)
		assert.True(t, ok)
	}

	tags["hostname"] = "host[123]"

	// Now query
	mds, err := c.Definitions(Filters(TagsFilter(tags)))
	assert.NoError(t, err)
	assert.Equal(t, 3, len(mds))

	// Now query the available hostnames
	values, err := c.TagValues(tags)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(values))
	assert.Equal(t, 3, len(values["hostname"]))
}

func TestEscapingDatapoints(t *testing.T) {
	c, err := integrationClient()
	assert.NoError(t, err)

	startTime := time.Now().Truncate(time.Millisecond)

	mone := Datapoint{Value: 2, Timestamp: startTime}
	hOne := MetricHeader{
		ID:   "test whitespace counter 1",
		Data: []Datapoint{mone},
		Type: Counter,
	}

	hTwo := MetricHeader{
		ID:   "test+plus+counter+1",
		Data: []Datapoint{mone},
		Type: Counter,
	}

	err = c.Write([]MetricHeader{hOne, hTwo})
	assert.NoError(t, err)

	checkDatapoints(t, c, &hOne)
	checkDatapoints(t, c, &hTwo)
}

func TestQueryEscape(t *testing.T) {
	plusses := "1+2+3"
	whitespace := "test whitespace JEE"
	slashes := "test/my/mind"
	combination := "test/with whitespace+plusses"

	escapedPlusses := URLEscape(plusses)
	escapedWhitespace := URLEscape(whitespace)
	escapedSlashes := URLEscape(slashes)
	escapedCombination := URLEscape(combination)
	assert.Equal(t, escapedPlusses, "1%2B2%2B3")
	assert.Equal(t, escapedWhitespace, "test%20whitespace%20JEE")
	assert.Equal(t, escapedSlashes, "test%2Fmy%2Fmind")
	assert.Equal(t, escapedCombination, "test%2Fwith%20whitespace%2Bplusses")
}

func getMetrics(prefix int) []MetricHeader {
	points := 10
	metrics := 100000

	ts := time.Now()
	m := make([]MetricHeader, 0, metrics)

	for j := 0; j < (points * metrics); j += points {
		id := fmt.Sprintf("bench.metric.%d.%d", prefix, j)
		data := make([]Datapoint, 0, points)
		for k := 0; k < points; k++ {
			ts = ts.Add(1 * time.Millisecond)
			data = append(data, Datapoint{
				Timestamp: ts,
				Value:     float64(k),
			})
		}
		m = append(m, MetricHeader{
			ID:   id,
			Type: Gauge,
			Data: data,
		})
	}

	return m
}

func toBatches(m []MetricHeader, batchSize int) chan []MetricHeader {
	if batchSize == 0 {
		c := make(chan []MetricHeader, 1)
		c <- m
		return c
	}

	size := int(math.Ceil(float64(len(m)) / float64(batchSize)))
	c := make(chan []MetricHeader, size)

	for i := 0; i < len(m); i += batchSize {
		n := i + batchSize
		if len(m) < n {
			n = len(m)
		}
		part := m[i:n]
		c <- part
	}

	return c
}

func BenchmarkHawkular(b *testing.B) {
	for i := 0; i < b.N; i++ {
		m := getMetrics(i)
		t, _ := randomString()
		p := Parameters{Tenant: t, Url: "http://localhost:8080", Concurrency: 32}
		c, _ := NewHawkularClient(p)

		b.ResetTimer()

		wg := &sync.WaitGroup{}
		parts := toBatches(m, 100)
		close(parts)

		for p := range parts {
			wg.Add(1)
			go func(mh []MetricHeader) {
				c.Write(mh)
				wg.Done()
			}(p)
		}

		wg.Wait()
	}
}
