package main

import (
	"os"

	joonix "github.com/joonix/log"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {

	log.SetFormatter(joonix.NewFormatter())
	log.SetLevel(log.DebugLevel)
	app := cli.NewApp()
	app.Name = "torrent-web-cache"
	app.Usage = "Serves cached torrent data"
	app.Version = "0.0.1"
	configure(app)
	err := app.Run(os.Args)
	if err != nil {
		log.WithError(err).Fatal("Failed to serve application")
	}
}
