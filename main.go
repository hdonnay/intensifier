package main

import (
	"flag"
	"fmt"
	"hash/fnv"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"code.google.com/p/freetype-go/freetype"
	"code.google.com/p/freetype-go/freetype/raster"
	"code.google.com/p/freetype-go/freetype/truetype"
)

var (
	fontFile   = flag.String("font", "/usr/share/fonts/truetype/msttcorefonts/impact.ttf", "font to use")
	listenPort = flag.Int("port", 8080, "port to listen on")
	expire     = flag.Duration("expire", 14*24*time.Hour, "time to expire images after")
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	flag.Parse()
	workDir := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d", os.Args[0], *listenPort))
	if err := os.Mkdir(workDir, 0700); err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}
	log.Println("made work directory:", workDir)
	ttf, err := ioutil.ReadFile(*fontFile)
	if err != nil {
		log.Fatal(err)
	}
	font, err := truetype.Parse(ttf)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("font loaded:", *fontFile)
	go func() {
		for _ = range time.Tick(30 * time.Minute) {
			e := time.Now().Add(-*expire)
			d, err := os.Open(workDir)
			if err != nil {
				log.Fatal("reaper: unable to open workDir:", err)
			}
			fi, err := d.Readdir(10)
			for ; err != nil; fi, err = d.Readdir(10) {
				for _, i := range fi {
					if i.ModTime().Before(e) {
						if err := os.Remove(i.Name()); err != nil {
							log.Println("reaper:", err)
						}
					}
				}
			}
			if err != io.EOF {
				log.Println("reaper:", err)
			}
		}
	}()
	m := &Memer{
		Font:   font,
		OutDir: workDir,
		Frames: 10,
		Shake:  20,
		Delay:  5,
		FontSz: 20.0,
	}
	log.Println("image reaper started")
	http.HandleFunc("/", index)
	http.Handle("/img/", http.StripPrefix("/img/", http.FileServer(http.Dir(workDir))))
	http.Handle("/create", m)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(*listenPort), nil))
}

type Memer struct {
	Font   *truetype.Font
	OutDir string
	Frames int
	Shake  int
	Delay  int
	FontSz float64
}

// Request handles incoming requests
func (m *Memer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := freetype.NewContext()
	ctx.SetSrc(image.White)
	ctx.SetFont(m.Font)
	ctx.SetFontSize(m.FontSz)
	ctx.SetHinting(freetype.FullHinting)
	ctx.SetDPI(96.0)
	h := fnv.New64a()
	g := gif.GIF{
		Image:     make([]*image.Paletted, m.Frames),
		Delay:     make([]int, m.Frames),
		LoopCount: -1,
	}
	if err := r.ParseMultipartForm(1 << 13); err != nil {
		log.Println(err)
		log.Println(r)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	meme := fmt.Sprintf("[%s intensifies]", r.FormValue("noun"))
	f, _, err := r.FormFile("file")
	if err != nil {
		log.Println(err)
		log.Println(r)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	t := io.TeeReader(f, h)
	img, _, err := image.Decode(t)
	if err != nil {
		log.Println(err)
		log.Println(r)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	id := h.Sum64()
	filename := fmt.Sprintf("%d-%s.gif", id, r.FormValue("noun"))
	localFile := filepath.Join(m.OutDir, filename)
	if _, err := os.Stat(localFile); os.IsExist(err) {
		http.Redirect(w, r, fmt.Sprintf("/img/%s", filename), http.StatusSeeOther)
		return
	}
	rng := rand.New(rand.NewSource(int64(id)))
	xSz, ySz := img.Bounds().Dx(), img.Bounds().Dy()
	xMin, yMin := xSz-m.Shake, ySz-m.Shake
	midPt := xMin / 2
	debug("x", xMin, "y", yMin, "mid", midPt)
	rect := image.Rect(0, 0, xMin, yMin)
	// create new image
	txt := image.NewAlpha(image.Rect(0, 0, int(float64(xMin)*0.8), int(float64(yMin)*0.15)))
	// fill
	draw.Draw(txt, txt.Bounds(), image.Transparent, image.ZP, draw.Src)
	ctx.SetClip(txt.Bounds())
	ctx.SetDst(txt)
	dist, err := ctx.DrawString(meme, raster.Point{0, ctx.PointToFix32(m.FontSz)})
	if err != nil {
		log.Println(err)
	}
	dbg, _ := os.Create(localFile + ".mask")
	debug(gif.Encode(dbg, txt, &gif.Options{NumColors: 256}))
	debug(dist, int(dist.X>>8))
	tOff := midPt - (int(dist.X>>8) / 2)
	debug(tOff)
	tr := txt.Bounds().Add(image.Point{tOff, int(float64(yMin) * 0.8)})
	// generate 10 frames
	for i := 0; i < m.Frames; i++ {
		sp := image.Point{rng.Intn(m.Shake), rng.Intn(m.Shake)}
		g.Delay[i] = m.Delay
		g.Image[i] = image.NewPaletted(rect, palette.WebSafe)
		draw.Draw(g.Image[i], rect, img, sp, draw.Src)
		draw.DrawMask(g.Image[i], tr, image.White, image.ZP, txt, image.ZP, draw.Over)
	}
	// write image
	out, err := os.Create(localFile)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := gif.EncodeAll(out, &g); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/img/%s", filename), http.StatusSeeOther)
	out.Close()
	return
}

func index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `<html><head></head><body>
<form enctype="multipart/form-data" method="post" action="/create">
noun:<input type="text" name="noun"> <input type="file" name="file" accept="image/*" size="40"><br/>
<input type="submit"></form></body></html>`)
	return
}
