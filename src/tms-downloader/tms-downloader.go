package main

import (
  "flag"
  "fmt"
  "log"
  "os"
  "time"

  "tms-downloader/mercantile"
  "tms-downloader/tiles"
)

var usageText = `Usage:
    tms-downloader [OPTIONS]
    Download tiles from specific source and save them on hard drive.
Options:
    --url         TMS server url.                              REQUIRED
    --zooms       Comma-separated list of zooms to download.   REQUIRED
    --bbox        Comma-separated list of bbox coordinates.    REQUIRED
    --wait        Wait time (ms) between tile downloads.       DEFAULT:1000
Help Options:
    --help    Help. Prints usage in the stdout.
`

var options = tiles.Options{}

// Tie command-line flags to the variables and
// set default variables and usage messages.
func init() {
	flag.StringVar(&options.URL, "url", "", "")
	flag.Var(&options.Zooms, "zooms", "")
	flag.Var(&options.Bbox, "bbox", "")
	flag.IntVar(&options.WaitTime, "wait", 1000, "")
	flag.BoolVar(&options.Help, "help", false, "")
	flag.Usage = func() {
		fmt.Fprintf(os.Stdout, usageText)
	}
}

func main() {
  flag.Parse()
  if err := options.ValidateOptions(); err != nil {
	   log.Fatal(err)
	}

  tilesIds := mercantile.Tiles(
		options.Bbox.Left,
		options.Bbox.Bottom,
		options.Bbox.Right,
		options.Bbox.Top,
		options.Zooms,
	)

  jobs := tiles.JobStats{Start: time.Now(), All: 0, Succeeded: 0, Failed: 0}

  jobs.All = len(tilesIds)

  for _, tileID := range tilesIds {
    jobs.ShowCurrentState()

    tilesTileID := tiles.GetTileID(tileID.X, tileID.Y, tileID.Z)

    tile, err := tiles.Get(tilesTileID, options)
    if err != nil {
      jobs.Failed++
    } else {
      err := tiles.Save(tile)
      if err != nil {
        jobs.Failed++
      } else {
        jobs.Succeeded++
      }
    }

    time.Sleep(time.Duration(options.WaitTime) * time.Millisecond)
  }

  fmt.Sprintf("\n")
}
