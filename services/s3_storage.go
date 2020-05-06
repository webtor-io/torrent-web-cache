package services

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

type S3Storage struct {
	bucket string
	cl     *S3Client
}

const (
	AWS_BUCKET = "aws-bucket"
)

func RegisterS3StorageFlags(c *cli.App) {
	c.Flags = append(c.Flags, cli.StringFlag{
		Name:   AWS_BUCKET,
		Usage:  "AWS Bucket",
		Value:  "",
		EnvVar: "AWS_BUCKET",
	})
}

func NewS3Storage(c *cli.Context, cl *S3Client) *S3Storage {
	return &S3Storage{
		bucket: c.String(AWS_BUCKET),
		cl:     cl,
	}
}

func (s *S3Storage) TouchTorrent(h string) (err error) {
	key := "touch/" + h
	log.Infof("Touching torrent key=%v bucket=%v", key, s.bucket)
	_, err = s.cl.Get().PutObject(&s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(fmt.Sprintf("%v", time.Now().Unix()))),
	})
	if err != nil {
		return errors.Wrapf(err, "Failed to touch torrent key=%v", key)
	}
	return
}

func (s *S3Storage) GetTorrent(h string) (io.ReadCloser, error) {
	key := h + ".torrent"
	log.Infof("Fetching torrent key=%v bucket=%v", key, s.bucket)
	r, err := s.cl.Get().GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == s3.ErrCodeNoSuchKey {
			return nil, nil
		}
		return nil, errors.Wrap(err, "Failed to fetch torrent")
	}
	return r.Body, nil
}

func (s *S3Storage) GetPiece(h string, p string, start int64, end int64) (io.ReadCloser, error) {
	key := h + "/" + p
	ra := fmt.Sprintf("bytes=%v-%v", start, end)
	log.Infof("Fetching piece key=%v bucket=%v range=%v", key, s.bucket, ra)
	r, err := s.cl.Get().GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Range:  aws.String(ra),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == s3.ErrCodeNoSuchKey {
			return nil, nil
		}
		return nil, errors.Wrap(err, "Failed to fetch piece")
	}
	return r.Body, nil
}

func (s *S3Storage) GetCompletedPieces(h string) (io.ReadCloser, error) {
	key := h + "/completed_pieces"
	log.Infof("Fetching completed pieces key=%v bucket=%v", key, s.bucket)
	r, err := s.cl.Get().GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == s3.ErrCodeNoSuchKey {
			return nil, nil
		}
		return nil, errors.Wrap(err, "Failed to fetch completed pieces")
	}
	return r.Body, nil
}
