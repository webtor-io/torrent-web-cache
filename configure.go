package main

import (
	"net"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"github.com/urfave/cli"
	cs "github.com/webtor-io/common-services"
	s "github.com/webtor-io/torrent-web-cache/services"
)

func configure(app *cli.App) {
	app.Flags = []cli.Flag{}
	cs.RegisterProbeFlags(app)
	cs.RegisterS3ClientFlags(app)
	s.RegisterS3StorageFlags(app)
	s.RegisterWebFlags(app)
	app.Action = run
}

func run(c *cli.Context) error {
	// Setting ballast
	// _ = make([]byte, 100<<20)

	// Setting HTTP Client
	myTransport := &http.Transport{
		MaxIdleConns:        500,
		MaxIdleConnsPerHost: 500,
		MaxConnsPerHost:     500,
		IdleConnTimeout:     90 * time.Second,
		Dial: (&net.Dialer{
			Timeout:   1 * time.Minute,
			KeepAlive: 1 * time.Minute,
		}).Dial,
	}
	cl := &http.Client{
		Timeout:   1 * time.Minute,
		Transport: myTransport,
	}

	// Setting S3 Session
	s3cl := cs.NewS3Client(c, cl)

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

	// Setting HTTP Piece Pool
	httppp := s.NewHTTPPiecePool(cl)

	// Setting Piece Pool
	pp := s.NewPiecePool(cpp, s3pp, httppp)

	// Setting Leaky Buffer
	lb := s.NewLeakyBuffer(1000, 32*1024)

	// Setting Preload Piece Pool
	ppp, err := s.NewPreloadPiecePool(c, pp, lb)
	if err != nil {
		return errors.Wrap(err, "Failed to setup Preload Piece Pool")
	}
	defer ppp.Close()

	// Setting Preload Queue Pool
	pqp := s.NewPreloadQueuePool(ppp)

	// Setting Reader Pool
	rp := s.NewReaderPool(pp, mip, ttp, lb, ppp, pqp)

	// Setting ProbeService
	probe := cs.NewProbe(c)
	defer probe.Close()

	// Setting WebService
	web := s.NewWeb(c, rp, cpp, lb)
	defer web.Close()

	// Setting ServeService
	serve := cs.NewServe(probe, web)

	// And SERVE!
	return serve.Serve()
}
