package main

import (
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/docopt/docopt-go"
)

var (
	version = "[manual build]"
	usage   = "dard " + version + `

Usage:
  dard [options]
  dard -h | --help
  dard --version

Options:
  -d --dir <path>       Path to storage directory. [default: /srv/dar/]
  -s --max-size <size>  Max file size. [default: 100mb]
  -t --token <length>   Length of token. [default: 10]
  -l --listen <addr>    Listen specified address. [default: 127.0.0.1:8080]
  -h --help             Show this screen.
  --version             Show version.
`
)

func main() {
	rand.Seed(time.Now().UnixNano())

	args, err := docopt.Parse(usage, nil, true, version, false)
	if err != nil {
		panic(err)
	}

	var size datasize.ByteSize
	err = size.UnmarshalText([]byte(args["--max-size"].(string)))
	if err != nil {
		log.Fatalln(err)
	}

	length, err := strconv.Atoi(args["--token"].(string))
	if err != nil {
		log.Fatalln(err)
	}

	handler := &Handler{
		dir:    args["--dir"].(string),
		size:   int64(size.Bytes()),
		length: length,
	}

	err = http.ListenAndServe(args["--listen"].(string), handler)
	if err != nil {
		log.Fatalln(err)
	}
}
