package services

import (
	"bytes"
	"context"
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

func (s *S3Storage) TouchTorrent(ctx context.Context, h string) (err error) {
	key := "touch/" + h
	log.Debugf("Touching torrent key=%v bucket=%v", key, s.bucket)
	_, err = s.cl.Get().PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(fmt.Sprintf("%v", time.Now().Unix()))),
	})
	if err != nil {
		return errors.Wrapf(err, "Failed to touch torrent key=%v", key)
	}
	return
}

func (s *S3Storage) GetTorrent(ctx context.Context, h string) (io.ReadCloser, error) {
	key := h + ".torrent"
	log.Debugf("Fetching torrent key=%v bucket=%v", key, s.bucket)
	r, err := s.cl.Get().GetObjectWithContext(ctx, &s3.GetObjectInput{
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

func (s *S3Storage) GetPiece(ctx context.Context, h string, p string, start int64, end int64) (io.ReadCloser, error) {
	key := h + "/" + p
	ra := fmt.Sprintf("bytes=%v-%v", start, end)
	log.Debugf("Fetching piece key=%v bucket=%v range=%v", key, s.bucket, ra)
	r, err := s.cl.Get().GetObjectWithContext(ctx, &s3.GetObjectInput{
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
	// buf := make([]byte, end-start+1000)
	// n, err := io.ReadFull(r.Body, buf)
	// log.Info(n)
	return r.Body, nil
}

func (s *S3Storage) GetCompletedPieces(ctx context.Context, h string) (io.ReadCloser, error) {
	key := h + "/completed_pieces"
	log.Debugf("Fetching completed pieces key=%v bucket=%v", key, s.bucket)
	r, err := s.cl.Get().GetObjectWithContext(ctx, &s3.GetObjectInput{
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
