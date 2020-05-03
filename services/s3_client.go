package services

import (
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

type S3Client struct {
	accessKeyID     string
	secretAccessKey string
	endpoint        string
	region          string
	s3              *s3.S3
	mux             sync.Mutex
	err             error
	inited          bool
}

const (
	AWS_ACCESS_KEY_ID     = "aws-access-key-id"
	AWS_SECRET_ACCESS_KEY = "aws-secret-access-key"
	AWS_ENDPOINT          = "aws-endpoint"
	AWS_REGION            = "aws-region"
)

func RegisterS3ClientFlags(c *cli.App) {
	c.Flags = append(c.Flags, cli.StringFlag{
		Name:   AWS_ACCESS_KEY_ID,
		Usage:  "AWS Access Key ID",
		Value:  "",
		EnvVar: "AWS_ACCESS_KEY_ID",
	})
	c.Flags = append(c.Flags, cli.StringFlag{
		Name:   AWS_SECRET_ACCESS_KEY,
		Usage:  "AWS Secret Access Key",
		Value:  "",
		EnvVar: "AWS_SECRET_ACCESS_KEY",
	})
	c.Flags = append(c.Flags, cli.StringFlag{
		Name:   AWS_ENDPOINT,
		Usage:  "AWS Endpoint",
		Value:  "",
		EnvVar: "AWS_ENDPOINT",
	})
	c.Flags = append(c.Flags, cli.StringFlag{
		Name:   AWS_REGION,
		Usage:  "AWS Region",
		Value:  "",
		EnvVar: "AWS_REGION",
	})
}

func NewS3Client(c *cli.Context) *S3Client {
	return &S3Client{
		accessKeyID:     c.String(AWS_ACCESS_KEY_ID),
		secretAccessKey: c.String(AWS_SECRET_ACCESS_KEY),
		endpoint:        c.String(AWS_ENDPOINT),
		region:          c.String(AWS_REGION),
		inited:          false,
	}
}

func (s *S3Client) Get() *s3.S3 {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.inited {
		return s.s3
	}
	s.s3 = s.get()
	s.inited = true
	return s.s3
}

func (s *S3Client) get() *s3.S3 {
	log.Info("Initializing S3")
	c := &aws.Config{
		Credentials: credentials.NewStaticCredentials(s.accessKeyID, s.secretAccessKey, ""),
		Endpoint:    aws.String(s.endpoint),
		Region:      aws.String(s.region),
		// DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
	}
	ss := session.New(c)
	s.s3 = s3.New(ss)
	return s.s3
}
