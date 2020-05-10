package main

import (
	"net"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	cs "github.com/webtor-io/common-services"
	s "github.com/webtor-io/torrent-web-cache/services"
)

func configure(app *cli.App) {
	app.Flags = []cli.Flag{}
	cs.RegisterProbeFlags(app)
	s.RegisterS3ClientFlags(app)
	s.RegisterS3StorageFlags(app)
	s.RegisterWebFlags(app)
	app.Action = run
}

func run(c *cli.Context) error {
	// Setting S3 Session
	s3cl := s.NewS3Client(c)

	// Setting S3 Storage
	s3st := s.NewS3Storage(c, s3cl)

	// Setting MetaInfo Pool
	mip := s.NewMetaInfoPool(s3st)

	// Setting CompletedPieces Pool
	cpp := s.NewCompletedPiecesPool(s3st)

	// Setting S3 Piece Pool
	s3pp := s.NewS3PiecePool(s3st)

	// Setting Torrent Touch Pool
	ttp := s.NewTorrentTouchPool(s3st)

	// Setting HTTP Client
	myTransport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Minute,
		}).Dial,
	}
	cl := &http.Client{
		Timeout:   5 * time.Minute,
		Transport: myTransport,
	}
	// Setting HTTP Piece Pool
	httppp := s.NewHTTPPiecePool(cl)

	// Setting Piece Pool
	pp := s.NewPiecePool(cpp, s3pp, httppp)

	lb := s.NewLeakyBuffer(100, 32*1024)

	// Setting Reader Pool
	rp := s.NewReaderPool(pp, mip, ttp, lb)

	// Setting ProbeService
	probe := cs.NewProbe(c)
	defer probe.Close()

	// Setting WebService
	web := s.NewWeb(c, rp)
	defer web.Close()

	// Setting ServeService
	serve := cs.NewServe(probe, web)

	// And SERVE!
	err := serve.Serve()
	if err != nil {
		log.WithError(err).Error("Got server error")
	}
	return nil
}
