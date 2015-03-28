/*
 * Minimalist Object Storage, (C) 2014 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package api_test

import (
	"bytes"
	"encoding/xml"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/minio-io/minio/pkg/api"
	"github.com/minio-io/minio/pkg/drivers"
	"github.com/minio-io/minio/pkg/drivers/memory"
	"github.com/minio-io/minio/pkg/drivers/mocks"
	"github.com/stretchr/testify/mock"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type MySuite struct {
	Driver func() drivers.Driver
}

var _ = Suite(&MySuite{
	Driver: func() drivers.Driver {
		return startDriver()
	},
})

var _ = Suite(&MySuite{
	Driver: func() drivers.Driver {
		_, _, driver := memory.Start()
		return driver
	},
})

func (s *MySuite) TestNonExistantObject(c *C) {
	driver := s.Driver()
	switch typedDriver := driver.(type) {
	case *mocks.Driver:
		{
			typedDriver.On("GetObjectMetadata", "bucket", "object", "").Return(drivers.ObjectMetadata{}, drivers.BucketNotFound{Bucket: "bucket"}).Once()
			defer typedDriver.AssertExpectations(c)
		}
	}
	httpHandler := api.HTTPHandler("", driver)
	testServer := httptest.NewServer(httpHandler)
	defer testServer.Close()

	response, err := http.Get(testServer.URL + "/bucket/object")
	c.Assert(err, IsNil)
	c.Log(response.StatusCode)
	c.Assert(response.StatusCode, Equals, http.StatusNotFound)
}

func (s *MySuite) TestEmptyObject(c *C) {
	driver := s.Driver()
	switch typedDriver := driver.(type) {
	case *mocks.Driver:
		{
			metadata := drivers.ObjectMetadata{
				Bucket:      "bucket",
				Key:         "key",
				ContentType: "application/octet-stream",
				Created:     time.Now(),
				Md5:         "d41d8cd98f00b204e9800998ecf8427e",
				Size:        0,
			}
			typedDriver.On("CreateBucket", "bucket").Return(nil).Once()
			typedDriver.On("CreateObject", "bucket", "object", "", "", mock.Anything).Return(nil).Once()
			typedDriver.On("GetObjectMetadata", "bucket", "object", "").Return(metadata, nil).Once()
			typedDriver.On("GetObject", mock.Anything, "bucket", "object").Return(int64(0), nil).Once()
			typedDriver.On("GetObjectMetadata", "bucket", "object", "").Return(metadata, nil).Once()
			defer typedDriver.AssertExpectations(c)
		}
	}
	httpHandler := api.HTTPHandler("", driver)
	testServer := httptest.NewServer(httpHandler)
	defer testServer.Close()

	buffer := bytes.NewBufferString("")
	driver.CreateBucket("bucket")
	driver.CreateObject("bucket", "object", "", "", buffer)

	response, err := http.Get(testServer.URL + "/bucket/object")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)

	responseBody, err := ioutil.ReadAll(response.Body)
	c.Assert(err, IsNil)
	c.Assert(true, Equals, bytes.Equal(responseBody, buffer.Bytes()))

	metadata, err := driver.GetObjectMetadata("bucket", "object", "")
	c.Assert(err, IsNil)
	verifyHeaders(c, response.Header, metadata.Created, 0, "application/octet-stream", metadata.Md5)
}

func (s *MySuite) TestObject(c *C) {
	driver := s.Driver()
	switch typedDriver := driver.(type) {
	case *mocks.Driver:
		{
			metadata := drivers.ObjectMetadata{
				Bucket:      "bucket",
				Key:         "key",
				ContentType: "application/octet-stream",
				Created:     time.Now(),
				Md5:         "5eb63bbbe01eeed093cb22bb8f5acdc3",
				Size:        11,
			}
			typedDriver.On("CreateBucket", "bucket").Return(nil).Once()
			typedDriver.On("CreateObject", "bucket", "object", "", "", mock.Anything).Return(nil).Once()
			typedDriver.On("GetObjectMetadata", "bucket", "object", "").Return(metadata, nil).Twice()
			typedDriver.SetGetObjectWriter("bucket", "object", []byte("hello world"))
			typedDriver.On("GetObject", mock.Anything, "bucket", "object").Return(int64(0), nil).Once()
			defer typedDriver.AssertExpectations(c)
		}
	}
	httpHandler := api.HTTPHandler("", driver)
	testServer := httptest.NewServer(httpHandler)
	defer testServer.Close()

	buffer := bytes.NewBufferString("hello world")
	driver.CreateBucket("bucket")
	driver.CreateObject("bucket", "object", "", "", buffer)

	response, err := http.Get(testServer.URL + "/bucket/object")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)

	responseBody, err := ioutil.ReadAll(response.Body)
	c.Assert(err, IsNil)
	println(string(responseBody))
	c.Assert(responseBody, DeepEquals, []byte("hello world"))

	metadata, err := driver.GetObjectMetadata("bucket", "object", "")
	c.Assert(err, IsNil)
	verifyHeaders(c, response.Header, metadata.Created, len("hello world"), "application/octet-stream", metadata.Md5)
}

func (s *MySuite) TestMultipleObjects(c *C) {
	driver := s.Driver()
	var typedDriver *mocks.Driver
	switch driver := driver.(type) {
	case *mocks.Driver:
		{
			typedDriver = driver

			defer typedDriver.AssertExpectations(c)
		}
	default:
		{
			typedDriver = startDriver()
		}
	}
	metadata1 := drivers.ObjectMetadata{
		Bucket:      "bucket",
		Key:         "object1",
		ContentType: "application/octet-stream",
		Created:     time.Now(),
		// TODO correct md5
		Md5:  "5eb63bbbe01eeed093cb22bb8f5acdc3",
		Size: 9,
	}
	metadata2 := drivers.ObjectMetadata{
		Bucket:      "bucket",
		Key:         "object2",
		ContentType: "application/octet-stream",
		Created:     time.Now(),
		Md5:         "5eb63bbbe01eeed093cb22bb8f5acdc3", // TODO correct md5
		Size:        9,
	}
	metadata3 := drivers.ObjectMetadata{
		Bucket:      "bucket",
		Key:         "object3",
		ContentType: "application/octet-stream",
		Created:     time.Now(),
		Md5:         "5eb63bbbe01eeed093cb22bb8f5acdc3", // TODO correct md5
		Size:        11,
	}
	httpHandler := api.HTTPHandler("", driver)
	testServer := httptest.NewServer(httpHandler)
	defer testServer.Close()

	buffer1 := bytes.NewBufferString("hello one")
	buffer2 := bytes.NewBufferString("hello two")
	buffer3 := bytes.NewBufferString("hello three")

	typedDriver.On("CreateBucket", "bucket").Return(nil).Once()
	driver.CreateBucket("bucket")
	typedDriver.On("CreateObject", "bucket", "object1", "", "", mock.Anything).Return(nil).Once()
	driver.CreateObject("bucket", "object1", "", "", buffer1)
	typedDriver.On("CreateObject", "bucket", "object2", "", "", mock.Anything).Return(nil).Once()
	driver.CreateObject("bucket", "object2", "", "", buffer2)
	typedDriver.On("CreateObject", "bucket", "object3", "", "", mock.Anything).Return(nil).Once()
	driver.CreateObject("bucket", "object3", "", "", buffer3)

	// test non-existant object
	typedDriver.On("GetObjectMetadata", "bucket", "object", "").Return(drivers.ObjectMetadata{}, drivers.ObjectNotFound{}).Once()
	response, err := http.Get(testServer.URL + "/bucket/object")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusNotFound)
	// TODO Test Headers

	//// test object 1

	// get object
	typedDriver.On("GetObjectMetadata", "bucket", "object1", "").Return(metadata1, nil).Once()
	typedDriver.SetGetObjectWriter("bucket", "object1", []byte("hello one"))
	typedDriver.On("GetObject", mock.Anything, "bucket", "object1").Return(int64(0), nil).Once()
	response, err = http.Get(testServer.URL + "/bucket/object1")
	c.Assert(err, IsNil)

	// get metadata
	typedDriver.On("GetObjectMetadata", "bucket", "object1", "").Return(metadata1, nil).Once()
	metadata, err := driver.GetObjectMetadata("bucket", "object1", "")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)

	// verify headers
	verifyHeaders(c, response.Header, metadata.Created, len("hello one"), "application/octet-stream", metadata.Md5)
	c.Assert(err, IsNil)

	// verify response data
	responseBody, err := ioutil.ReadAll(response.Body)
	c.Assert(err, IsNil)
	c.Assert(true, Equals, bytes.Equal(responseBody, []byte("hello one")))

	// test object 2
	// get object
	typedDriver.On("GetObjectMetadata", "bucket", "object2", "").Return(metadata2, nil).Once()
	typedDriver.SetGetObjectWriter("bucket", "object2", []byte("hello two"))
	typedDriver.On("GetObject", mock.Anything, "bucket", "object2").Return(int64(0), nil).Once()
	response, err = http.Get(testServer.URL + "/bucket/object2")
	c.Assert(err, IsNil)

	// get metadata
	typedDriver.On("GetObjectMetadata", "bucket", "object2", "").Return(metadata2, nil).Once()
	metadata, err = driver.GetObjectMetadata("bucket", "object2", "")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)

	// verify headers
	verifyHeaders(c, response.Header, metadata.Created, len("hello two"), "application/octet-stream", metadata.Md5)
	c.Assert(err, IsNil)

	// verify response data
	responseBody, err = ioutil.ReadAll(response.Body)
	c.Assert(err, IsNil)
	c.Assert(true, Equals, bytes.Equal(responseBody, []byte("hello two")))

	// test object 3
	// get object
	typedDriver.On("GetObjectMetadata", "bucket", "object3", "").Return(metadata3, nil).Once()
	typedDriver.SetGetObjectWriter("bucket", "object3", []byte("hello three"))
	typedDriver.On("GetObject", mock.Anything, "bucket", "object3").Return(int64(0), nil).Once()
	response, err = http.Get(testServer.URL + "/bucket/object3")
	c.Assert(err, IsNil)

	// get metadata
	typedDriver.On("GetObjectMetadata", "bucket", "object3", "").Return(metadata3, nil).Once()
	metadata, err = driver.GetObjectMetadata("bucket", "object3", "")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)

	// verify headers
	verifyHeaders(c, response.Header, metadata.Created, len("hello three"), "application/octet-stream", metadata.Md5)
	c.Assert(err, IsNil)

	// verify object
	responseBody, err = ioutil.ReadAll(response.Body)
	c.Assert(err, IsNil)
	c.Assert(true, Equals, bytes.Equal(responseBody, []byte("hello three")))
}

func (s *MySuite) TestNotImplemented(c *C) {
	driver := s.Driver()
	var typedDriver *mocks.Driver
	switch driver := driver.(type) {
	case *mocks.Driver:
		{
			typedDriver = driver
			defer typedDriver.AssertExpectations(c)
		}
	default:
		{
			// we never assert expectations
			typedDriver = startDriver()
		}
	}
	httpHandler := api.HTTPHandler("", driver)
	testServer := httptest.NewServer(httpHandler)
	defer testServer.Close()

	response, err := http.Get(testServer.URL + "/bucket/object?acl")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusNotImplemented)
}

func (s *MySuite) TestHeader(c *C) {
	driver := s.Driver()
	var typedDriver *mocks.Driver
	switch driver := driver.(type) {
	case *mocks.Driver:
		{
			typedDriver = driver
			defer driver.AssertExpectations(c)
		}
	default:
		{
			// we never assert expectations
			typedDriver = startDriver()
		}
	}

	typedDriver.AssertExpectations(c)
	httpHandler := api.HTTPHandler("", driver)
	testServer := httptest.NewServer(httpHandler)
	defer testServer.Close()

	typedDriver.On("GetObjectMetadata", "bucket", "object", "").Return(drivers.ObjectMetadata{}, drivers.ObjectNotFound{}).Once()
	response, err := http.Get(testServer.URL + "/bucket/object")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusNotFound)

	buffer := bytes.NewBufferString("hello world")
	typedDriver.On("CreateBucket", "bucket").Return(nil).Once()
	driver.CreateBucket("bucket")
	typedDriver.On("CreateObject", "bucket", "object", "", "", mock.Anything).Return(nil).Once()
	driver.CreateObject("bucket", "object", "", "", buffer)

	objectMetadata := drivers.ObjectMetadata{
		Bucket:      "bucket",
		Key:         "object",
		ContentType: "application/octet-stream",
		Created:     time.Now(),
		// TODO correct md5
		Md5:  "5eb63bbbe01eeed093cb22bb8f5acdc3",
		Size: 11,
	}

	typedDriver.On("GetObjectMetadata", "bucket", "object", "").Return(objectMetadata, nil).Once()
	typedDriver.SetGetObjectWriter("", "", []byte("hello world"))
	typedDriver.On("GetObject", mock.Anything, "bucket", "object").Return(int64(0), nil).Once()
	response, err = http.Get(testServer.URL + "/bucket/object")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)

	typedDriver.On("GetObjectMetadata", "bucket", "object", "").Return(objectMetadata, nil).Once()
	metadata, err := driver.GetObjectMetadata("bucket", "object", "")
	c.Assert(err, IsNil)
	verifyHeaders(c, response.Header, metadata.Created, len("hello world"), "application/octet-stream", metadata.Md5)
}

func (s *MySuite) TestPutBucket(c *C) {
	driver := s.Driver()
	var typedDriver *mocks.Driver
	switch driver := driver.(type) {
	case *mocks.Driver:
		{
			typedDriver = driver
			defer driver.AssertExpectations(c)
		}
	default:
		{
			// we never assert expectations
			typedDriver = startDriver()
		}
	}

	httpHandler := api.HTTPHandler("", driver)
	testServer := httptest.NewServer(httpHandler)
	defer testServer.Close()

	typedDriver.On("ListBuckets").Return(make([]drivers.BucketMetadata, 0), nil).Once()
	buckets, err := driver.ListBuckets()
	c.Assert(len(buckets), Equals, 0)
	c.Assert(err, IsNil)

	typedDriver.On("CreateBucket", "bucket").Return(nil).Once()
	request, err := http.NewRequest("PUT", testServer.URL+"/bucket", bytes.NewBufferString(""))
	c.Assert(err, IsNil)

	client := http.Client{}
	response, err := client.Do(request)
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)

	// check bucket exists
	typedDriver.On("ListBuckets").Return([]drivers.BucketMetadata{{Name: "bucket"}}, nil).Once()
	buckets, err = driver.ListBuckets()
	c.Assert(len(buckets), Equals, 1)
	c.Assert(err, IsNil)
	c.Assert(buckets[0].Name, Equals, "bucket")
}

func (s *MySuite) TestPutObject(c *C) {
	driver := s.Driver()
	var typedDriver *mocks.Driver
	switch driver := driver.(type) {
	case *mocks.Driver:
		{
			typedDriver = driver
			defer driver.AssertExpectations(c)
		}
	default:
		{
			// we never assert expectations
			typedDriver = startDriver()
		}
	}
	httpHandler := api.HTTPHandler("", driver)
	testServer := httptest.NewServer(httpHandler)
	defer testServer.Close()

	resources := drivers.BucketResourcesMetadata{}

	resources.Maxkeys = 1000
	resources.Prefix = ""

	typedDriver.On("ListObjects", "bucket", mock.Anything).Return([]drivers.ObjectMetadata{}, drivers.BucketResourcesMetadata{}, drivers.BucketNotFound{}).Once()
	objects, resources, err := driver.ListObjects("bucket", resources)
	c.Assert(len(objects), Equals, 0)
	c.Assert(resources.IsTruncated, Equals, false)
	c.Assert(err, Not(IsNil))

	date1 := time.Now()

	// Put Bucket before - Put Object into a bucket
	typedDriver.On("CreateBucket", "bucket").Return(nil).Once()
	request, err := http.NewRequest("PUT", testServer.URL+"/bucket", bytes.NewBufferString(""))
	c.Assert(err, IsNil)

	client := http.Client{}
	response, err := client.Do(request)
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)

	typedDriver.On("CreateObject", "bucket", "two", "", "", mock.Anything).Return(nil).Once()
	request, err = http.NewRequest("PUT", testServer.URL+"/bucket/two", bytes.NewBufferString("hello world"))
	println(err)
	c.Assert(err, IsNil)

	response, err = client.Do(request)
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)

	twoMetadata := drivers.ObjectMetadata{
		Bucket:      "bucket",
		Key:         "two",
		ContentType: "application/octet-stream",
		Created:     time.Now(),
		Md5:         "5eb63bbbe01eeed093cb22bb8f5acdc3",
		Size:        11,
	}

	date2 := time.Now()

	resources.Maxkeys = 1000
	resources.Prefix = ""

	typedDriver.On("ListObjects", "bucket", mock.Anything).Return([]drivers.ObjectMetadata{{}}, drivers.BucketResourcesMetadata{}, nil).Once()
	objects, resources, err = driver.ListObjects("bucket", resources)
	c.Assert(len(objects), Equals, 1)
	c.Assert(resources.IsTruncated, Equals, false)
	c.Assert(err, IsNil)

	var writer bytes.Buffer

	typedDriver.On("GetObjectMetadata", "bucket", "two", "").Return(twoMetadata, nil).Once()
	typedDriver.SetGetObjectWriter("bucket", "two", []byte("hello world"))
	typedDriver.On("GetObject", mock.Anything, "bucket", "two").Return(int64(11), nil).Once()
	driver.GetObject(&writer, "bucket", "two")

	c.Assert(bytes.Equal(writer.Bytes(), []byte("hello world")), Equals, true)

	metadata, err := driver.GetObjectMetadata("bucket", "two", "")
	c.Assert(err, IsNil)
	lastModified := metadata.Created

	c.Assert(date1.Before(lastModified), Equals, true)
	c.Assert(lastModified.Before(date2), Equals, true)
}

func (s *MySuite) TestListBuckets(c *C) {
	driver := s.Driver()
	var typedDriver *mocks.Driver
	switch driver := driver.(type) {
	case *mocks.Driver:
		{
			typedDriver = driver
			defer driver.AssertExpectations(c)
		}
	default:
		{
			// we never assert expectations
			typedDriver = startDriver()
		}
	}
	httpHandler := api.HTTPHandler("", driver)
	testServer := httptest.NewServer(httpHandler)
	defer testServer.Close()

	typedDriver.On("ListBuckets").Return([]drivers.BucketMetadata{}, nil).Once()
	response, err := http.Get(testServer.URL + "/")
	defer response.Body.Close()
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)

	listResponse, err := readListBucket(response.Body)
	c.Assert(err, IsNil)
	c.Assert(len(listResponse.Buckets.Bucket), Equals, 0)

	typedDriver.On("CreateBucket", "foo").Return(nil).Once()
	driver.CreateBucket("foo")

	bucketMetadata := []drivers.BucketMetadata{
		{Name: "foo", Created: time.Now()},
	}
	typedDriver.On("ListBuckets").Return(bucketMetadata, nil).Once()
	response, err = http.Get(testServer.URL + "/")
	defer response.Body.Close()
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)

	listResponse, err = readListBucket(response.Body)
	c.Assert(err, IsNil)
	c.Assert(len(listResponse.Buckets.Bucket), Equals, 1)
	c.Assert(listResponse.Buckets.Bucket[0].Name, Equals, "foo")

	typedDriver.On("CreateBucket", "bar").Return(nil).Once()
	driver.CreateBucket("bar")

	bucketMetadata = []drivers.BucketMetadata{
		{Name: "bar", Created: time.Now()},
		bucketMetadata[0],
	}

	typedDriver.On("ListBuckets").Return(bucketMetadata, nil).Once()
	response, err = http.Get(testServer.URL + "/")
	defer response.Body.Close()
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)

	listResponse, err = readListBucket(response.Body)
	c.Assert(err, IsNil)
	c.Assert(len(listResponse.Buckets.Bucket), Equals, 2)

	c.Assert(listResponse.Buckets.Bucket[0].Name, Equals, "bar")
	c.Assert(listResponse.Buckets.Bucket[1].Name, Equals, "foo")
}

func readListBucket(reader io.Reader) (api.BucketListResponse, error) {
	var results api.BucketListResponse
	decoder := xml.NewDecoder(reader)
	err := decoder.Decode(&results)
	return results, err
}

func (s *MySuite) TestListObjects(c *C) {
	// TODO Implement
}

func (s *MySuite) TestShouldNotBeAbleToCreateObjectInNonexistantBucket(c *C) {
	// TODO Implement
}

func (s *MySuite) TestHeadOnObject(c *C) {
	// TODO
}

func (s *MySuite) TestDateFormat(c *C) {
	// TODO
}

func verifyHeaders(c *C, header http.Header, date time.Time, size int, contentType string, etag string) {
	// Verify date
	c.Assert(header.Get("Last-Modified"), Equals, date.Format(time.RFC1123))

	// verify size
	c.Assert(header.Get("Content-Length"), Equals, strconv.Itoa(size))

	// verify content type
	c.Assert(header.Get("Content-Type"), Equals, contentType)

	// verify etag
	c.Assert(header.Get("Etag"), Equals, etag)
}

func (s *MySuite) TestXMLNameNotInBucketListJson(c *C) {
	driver := s.Driver()
	var typedDriver *mocks.Driver
	switch driver := driver.(type) {
	case *mocks.Driver:
		{
			typedDriver = driver
			defer driver.AssertExpectations(c)
		}
	default:
		{
			// we never assert expectations
			typedDriver = startDriver()
		}
	}
	httpHandler := api.HTTPHandler("", driver)
	testServer := httptest.NewServer(httpHandler)
	defer testServer.Close()

	typedDriver.On("CreateBucket", "foo").Return(nil).Once()
	err := driver.CreateBucket("foo")
	c.Assert(err, IsNil)

	typedDriver.On("ListBuckets").Return([]drivers.BucketMetadata{{Name: "foo", Created: time.Now()}}, nil)
	request, err := http.NewRequest("GET", testServer.URL+"/", bytes.NewBufferString(""))
	c.Assert(err, IsNil)

	request.Header.Add("Accept", "application/json")

	client := http.Client{}
	response, err := client.Do(request)

	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)

	byteResults, err := ioutil.ReadAll(response.Body)
	c.Assert(err, IsNil)
	c.Assert(strings.Contains(string(byteResults), "XML"), Equals, false)
}

func (s *MySuite) TestXMLNameNotInObjectListJson(c *C) {
	driver := s.Driver()
	var typedDriver *mocks.Driver
	switch driver := driver.(type) {
	case *mocks.Driver:
		{
			typedDriver = driver
			defer driver.AssertExpectations(c)
		}
	default:
		{
			//				 we never assert expectations
			typedDriver = startDriver()
		}
	}
	httpHandler := api.HTTPHandler("", driver)
	testServer := httptest.NewServer(httpHandler)
	defer testServer.Close()

	typedDriver.On("CreateBucket", "foo").Return(nil).Once()
	err := driver.CreateBucket("foo")
	c.Assert(err, IsNil)

	typedDriver.On("ListObjects", "foo", mock.Anything).Return([]drivers.ObjectMetadata{}, drivers.BucketResourcesMetadata{}, nil).Once()
	request, err := http.NewRequest("GET", testServer.URL+"/foo", bytes.NewBufferString(""))
	c.Assert(err, IsNil)

	request.Header.Add("Accept", "application/json")

	client := http.Client{}
	response, err := client.Do(request)

	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)

	byteResults, err := ioutil.ReadAll(response.Body)
	c.Assert(err, IsNil)
	c.Assert(strings.Contains(string(byteResults), "XML"), Equals, false)
}

func (s *MySuite) TestContentTypePersists(c *C) {
	driver := s.Driver()
	var typedDriver *mocks.Driver
	switch driver := driver.(type) {
	case *mocks.Driver:
		{
			typedDriver = driver
			defer driver.AssertExpectations(c)
		}
	default:
		{
			//				 we never assert expectations
			typedDriver = startDriver()
		}
	}
	httpHandler := api.HTTPHandler("", driver)
	testServer := httptest.NewServer(httpHandler)
	defer testServer.Close()

	typedDriver.On("CreateBucket", "bucket").Return(nil).Once()
	err := driver.CreateBucket("bucket")
	c.Assert(err, IsNil)

	client := http.Client{}
	typedDriver.On("CreateObject", "bucket", "one", "", "", mock.Anything).Return(nil).Once()
	request, err := http.NewRequest("PUT", testServer.URL+"/bucket/one", bytes.NewBufferString("hello world"))
	delete(request.Header, "Content-Type")
	c.Assert(err, IsNil)
	response, err := client.Do(request)
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)

	// test head
	oneMetadata := drivers.ObjectMetadata{
		Bucket:      "bucket",
		Key:         "one",
		ContentType: "application/octet-stream",
		Created:     time.Now(),
		Md5:         "d41d8cd98f00b204e9800998ecf8427e",
		Size:        0,
	}
	typedDriver.On("GetObjectMetadata", "bucket", "one", "").Return(oneMetadata, nil).Once()
	request, err = http.NewRequest("HEAD", testServer.URL+"/bucket/one", bytes.NewBufferString(""))
	c.Assert(err, IsNil)
	response, err = client.Do(request)
	c.Assert(err, IsNil)
	c.Assert(response.Header.Get("Content-Type"), Equals, "application/octet-stream")

	// test get object
	typedDriver.SetGetObjectWriter("bucket", "once", []byte(""))
	typedDriver.On("GetObjectMetadata", "bucket", "one", "").Return(oneMetadata, nil).Once()
	typedDriver.On("GetObject", mock.Anything, "bucket", "one").Return(int64(0), nil).Once()
	response, err = http.Get(testServer.URL + "/bucket/one")
	c.Assert(response.Header.Get("Content-Type"), Equals, "application/octet-stream")

	typedDriver.On("CreateObject", "bucket", "two", "", "", mock.Anything).Return(nil).Once()
	request, err = http.NewRequest("PUT", testServer.URL+"/bucket/two", bytes.NewBufferString("hello world"))
	delete(request.Header, "Content-Type")
	request.Header.Add("Content-Type", "application/json")
	c.Assert(err, IsNil)
	response, err = client.Do(request)
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)

	twoMetadata := drivers.ObjectMetadata{
		Bucket:      "bucket",
		Key:         "one",
		ContentType: "application/octet-stream",
		Created:     time.Now(),
		// Fix MD5
		Md5:  "d41d8cd98f00b204e9800998ecf8427e",
		Size: 0,
	}
	typedDriver.On("GetObjectMetadata", "bucket", "two", "").Return(twoMetadata, nil).Once()
	request, err = http.NewRequest("HEAD", testServer.URL+"/bucket/two", bytes.NewBufferString(""))
	c.Assert(err, IsNil)
	response, err = client.Do(request)
	c.Assert(err, IsNil)
	c.Assert(response.Header.Get("Content-Type"), Equals, "application/octet-stream")

	// test get object
	typedDriver.On("GetObjectMetadata", "bucket", "two", "").Return(twoMetadata, nil).Once()
	typedDriver.On("GetObject", mock.Anything, "bucket", "two").Return(int64(0), nil).Once()
	response, err = http.Get(testServer.URL + "/bucket/two")
	c.Assert(response.Header.Get("Content-Type"), Equals, "application/octet-stream")
}

func startDriver() *mocks.Driver {
	return &mocks.Driver{
		ObjectWriterData: make(map[string][]byte),
	}
}