// Copyright 2017-Present Pivotal Software, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http:#www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package blobstore_test

import (
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	awss3 "github.com/aws/aws-sdk-go/service/s3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotalservices/goblob/blobstore"
)

var _ = Describe("S3BucketIterator", func() {
	var (
		iterator       blobstore.BucketIterator
		store          blobstore.Blobstore
		bucketName     string
		s3Client       *awss3.S3
		minioAccessKey string
		minioSecretKey string
	)

	const (
		s3Region = "us-east-1"
	)

	BeforeEach(func() {
		var s3Endpoint string

		if os.Getenv("MINIO_ACCESS_KEY") == "" {
			minioAccessKey = "example-access-key"
		} else {
			minioAccessKey = os.Getenv("MINIO_ACCESS_KEY")
		}

		if os.Getenv("MINIO_SECRET_KEY") == "" {
			minioSecretKey = "example-secret-key"
		} else {
			minioSecretKey = os.Getenv("MINIO_SECRET_KEY")
		}

		if os.Getenv("MINIO_PORT_9000_TCP_ADDR") == "" {
			s3Endpoint = "http://127.0.0.1:9000"
		} else {
			s3Endpoint = fmt.Sprintf("http://%s:9000", os.Getenv("MINIO_PORT_9000_TCP_ADDR"))
		}

		session := session.New(&aws.Config{
			Region: aws.String(s3Region),
			Credentials: credentials.NewStaticCredentials(
				minioAccessKey,
				minioSecretKey,
				"example-token",
			),
			Endpoint:         aws.String(s3Endpoint),
			DisableSSL:       aws.Bool(true),
			S3ForcePathStyle: aws.Bool(true),
		})

		s3Client = awss3.New(session)

		bucketName = fmt.Sprintf("some-bucket-%d", GinkgoParallelNode())

		_, err := s3Client.CreateBucket(&awss3.CreateBucketInput{
			Bucket: aws.String(bucketName),
		})
		Expect(err).NotTo(HaveOccurred())

		store = blobstore.NewS3(
			minioAccessKey,
			minioSecretKey,
			s3Region,
			s3Endpoint,
			true,
			true,
			true,
			"some-buildpacks",
			"some-droplets",
			"some-packages",
			"some-resources",
		)

		iterator, err = store.NewBucketIterator(bucketName)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		listObjectsOutput, err := s3Client.ListObjects(&awss3.ListObjectsInput{
			Bucket: aws.String(bucketName),
		})
		Expect(err).NotTo(HaveOccurred())

		for _, item := range listObjectsOutput.Contents {
			_, err := s3Client.DeleteObject(&awss3.DeleteObjectInput{
				Bucket: aws.String(bucketName),
				Key:    item.Key,
			})
			Expect(err).NotTo(HaveOccurred())
		}

		_, err = s3Client.DeleteBucket(&awss3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Next", func() {
		It("returns an error", func() {
			_, err := iterator.Next()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("no more items in iterator"))
		})

		Context("when a blob exists in the bucket", func() {
			var expectedBlob blobstore.Blob

			BeforeEach(func() {
				expectedBlob = blobstore.Blob{
					Path: fmt.Sprintf("%s/some-path/some-file", bucketName),
				}

				_, err := s3Client.PutObject(&awss3.PutObjectInput{
					Body:   strings.NewReader("content"),
					Bucket: aws.String(bucketName),
					Key:    aws.String("some-path/some-file"),
					Metadata: map[string]*string{
						"Checksum": aws.String("some-checksum"),
					},
				})
				Expect(err).NotTo(HaveOccurred())

				iterator, err = store.NewBucketIterator(bucketName)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the blob", func() {
				blob, err := iterator.Next()
				Expect(err).NotTo(HaveOccurred())
				Expect(*blob).To(Equal(expectedBlob))
			})

			It("returns an error when all blobs have been listed", func() {
				_, err := iterator.Next()
				Expect(err).NotTo(HaveOccurred())

				_, err = iterator.Next()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("no more items in iterator"))
			})
		})
	})

	Describe("Done", func() {
		Context("when blobs exist in the bucket", func() {
			BeforeEach(func() {
				_, err := s3Client.PutObject(&awss3.PutObjectInput{
					Body:   strings.NewReader("content"),
					Bucket: aws.String(bucketName),
					Key:    aws.String("some-path/some-file"),
					Metadata: map[string]*string{
						"Checksum": aws.String("some-checksum"),
					},
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = s3Client.PutObject(&awss3.PutObjectInput{
					Body:   strings.NewReader("content"),
					Bucket: aws.String(bucketName),
					Key:    aws.String("some-path/some-other-file"),
					Metadata: map[string]*string{
						"Checksum": aws.String("some-checksum"),
					},
				})
				Expect(err).NotTo(HaveOccurred())

				iterator, err = store.NewBucketIterator(bucketName)
				Expect(err).NotTo(HaveOccurred())
			})

			It("causes Next to return an error", func() {
				iterator.Done()

				_, err := iterator.Next()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("no more items in iterator"))
			})
		})
	})
})
