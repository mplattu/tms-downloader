package tiles

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Luqqk/wms-tiles-downloader/pkg/mercantile"
)

// Options struct stores all available flags
// and their values set by user.
type Options struct {
	URL         string
	Zooms       Zooms
	Bbox        Bbox
	WaitTime    int
	Help        bool
	// If all options are correct,
	// build base URL for all tiles
	// requests.
	BaseURL string
}

// ValidateOptions validates options supplied by user.
// Downloading will start only, if all required options
// have been passed in correct format.
func (options *Options) ValidateOptions() error {
	switch {
	case options.Help:
		flag.Usage()
		os.Exit(0)
		return nil
	case options.URL == "":
		return errors.New("Wms server url is required")
	case options.Zooms == nil:
		return errors.New("Zooms are required")
	case options.Bbox == Bbox{}:
		return errors.New("Bbox is required")
	default:
		return nil
	}
}

// Zooms stores zoom levels, for which
// tiles should be downloaded.
type Zooms []int

// String is the method to format the flag's value, part of the flag.Value interface.
// The String method's output will be used in diagnostics.
func (zooms *Zooms) String() string {
	return fmt.Sprint(*zooms)
}

// Set is the method to set the flag value, part of the flag.Value interface.
// Converts comma-separated values (string in "int,int,int,(...)" format)
// to Zooms type.
func (zooms *Zooms) Set(value string) error {
	for _, val := range strings.Split(value, ",") {
		zoom, err := strconv.Atoi(val)
		if err != nil {
			return err
		}
		*zooms = append(*zooms, zoom)
	}
	return nil
}

// Bbox stores a web mercator bounding box, for which
// tiles should be downloaded.
type Bbox struct {
	Left   float64
	Bottom float64
	Right  float64
	Top    float64
}

// String is the method to format the flag's value, part of the flag.Value interface.
// The String method's output will be used in diagnostics.
func (bbox *Bbox) String() string {
	return fmt.Sprint(*bbox)
}

// Set is the method to set the flag value, part of the flag.Value interface.
// Converts comma-separated values (string in "left,bottom,right,top" format)
// to Bbox struct.
func (bbox *Bbox) Set(value string) error {
	bboxSlice := strings.Split(value, ",")
	left, _ := strconv.ParseFloat(bboxSlice[0], 64)
	bottom, _ := strconv.ParseFloat(bboxSlice[1], 64)
	right, _ := strconv.ParseFloat(bboxSlice[2], 64)
	top, _ := strconv.ParseFloat(bboxSlice[3], 64)
	*bbox = Bbox{Left: left, Bottom: bottom, Right: right, Top: top}
	return nil
}

// Create a Client for control over HTTP client settings.
// Client is safe for concurrent use by multiple goroutines
// and for efficiency should only be created once and re-used.
var client = &http.Client{
	Timeout: time.Second * 30,
}

// Tile contains content received from WMS server
// and other metadata about tile itself. For example
// tile's path in z/x tree, name under which the tile
// will be saved (y.png).
type Tile struct {
	Content []byte
	Path    string
	Name    string
}

func GetTileID(x int, y int, z int) mercantile.TileID {
	var tileID mercantile.TileID

	tileID.X = x
	tileID.Y = y
	tileID.Z = z

	return tileID
}

func getUrlWithCoordinates(url string, tileID mercantile.TileID) string {
	reX := regexp.MustCompile(`{x}`)
	reY := regexp.MustCompile(`{y}`)
	reZ := regexp.MustCompile(`{z}`)

	urlWithCoordinates := reX.ReplaceAllString(url, fmt.Sprintf("%d", tileID.X))
	urlWithCoordinates = reY.ReplaceAllString(urlWithCoordinates, fmt.Sprintf("%d", tileID.Y))
	urlWithCoordinates = reZ.ReplaceAllString(urlWithCoordinates, fmt.Sprintf("%d", tileID.Z))

	return urlWithCoordinates
}

// Get sends http.Get request to WMS Server
// and returns response content.
func Get(tileID mercantile.TileID, options Options) (*Tile, error) {
	// Parse base url and format it
	// with the bbox of the tile.
	// Bbox is calculated by using
	// current tile's id (z/x/y).
	urlWithCoordinates := getUrlWithCoordinates(options.URL, tileID)

	url, err := url.Parse(urlWithCoordinates)
	if err != nil {
		return &Tile{}, err
	}
	q := url.Query()
	url.RawQuery = q.Encode()
	// Request tile using defined client,
	// read response body.
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return &Tile{}, err
	}

	req.Header.Set("User-Agent", "tms-downloader")

	resp, err := client.Do(req)
	if err != nil {
		return &Tile{}, err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return &Tile{}, err
	}
	// Create Tile struct,
	// return pointer.
	tile := &Tile{
		Content: body,
		Path:    fmt.Sprintf("%v/%v", tileID.Z, tileID.X),
		// TODO: File extension (".png" part) should be parsed
		// dynamically, based on --format parameter supplied by
		// the user. 'image/png' is default.
		Name: fmt.Sprintf("%v.png", tileID.Y),
	}
	resp.Body.Close()
	return tile, nil
}

// Save saves the tile passed in
// argument on hard drive.
func Save(tile *Tile) error {
	err := os.MkdirAll(tile.Path, os.ModePerm)
	filepath := path.Join(tile.Path, tile.Name)
	err = ioutil.WriteFile(filepath, tile.Content, os.ModePerm)
	return err
}

// FormatTileBbox converts tile (x, y, z) to bbox string (l,b,r,t)
func FormatTileBbox(tileID mercantile.TileID) string {
	bbox := mercantile.XyBounds(tileID)
	formattedBbox := fmt.Sprintf("%.9f,%.9f,%.9f,%.9f", bbox.Left, bbox.Bottom, bbox.Right, bbox.Top)
	return formattedBbox
}

// JobStats stores number of jobs, that will
// be executed, jobs which have been resolved
// successfully or failed and Start timestamp.
type JobStats struct {
	Start     time.Time
	All       int
	Succeeded int
	Failed    int
}

// ShowCurrentState prints current state of jobs.
func (jobs *JobStats) ShowCurrentState() {
	fmt.Printf("Downloading...%v/%v Succeeded: %v Failed: %v\r",
		jobs.Succeeded+jobs.Failed,
		jobs.All, jobs.Succeeded,
		jobs.Failed,
	)
}

// ShowSummary prints summary along with
// execution time after all jobs have been
// processed.
func (jobs *JobStats) ShowSummary() {
	fmt.Printf("Done: %v/%v Succeeded: %v Failed: %v Execution Time: %v\n",
		jobs.Succeeded+jobs.Failed,
		jobs.All, jobs.Succeeded,
		jobs.Failed,
		time.Since(jobs.Start).Round(time.Millisecond),
	)
}
