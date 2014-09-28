package main

import (
	"flag"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"log"
	"net/http"

	"code.google.com/p/freetype-go/freetype/truetype"
)

var (
	fontFile = flag.String("font", "/usr/share/fonts/truetype/msttcorefonts/impact.ttf", "font to use")
)

func main() {
	flag.Parse()
	ttf, err := ioutil.ReadFile(*fontFile)
	if err != nil {
		log.Fatal(err)
	}
	font, err := truetype.Parse(ttf)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("font loaded:", *fontFile)
}

// Request handles incoming requests
func request(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(1 << 13); err != nil {
		log.Println("unable to ParseMultipartForm")
		log.Println(r)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	switch r.Header.Get("Content-Type") {
	default:
		thing := r.Form.Get("thing")
	}
	sz := image.Rect(0, 0, 500, 500)
}
