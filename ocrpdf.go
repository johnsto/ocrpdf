package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kingpin"

	"code.google.com/p/gofpdf"

	"bitbucket.org/johnsto/ocrpdf/internal"
)

var (
	app = kingpin.New("ocrpdf", "OCR and PDF")

	output = app.Flag("output", "output file name").Short('o').String()

	tessData = app.Flag("tess-data", "Tesseract data directory").String()
	tessLang = app.Flag("tess-lang", "Tesseract language").String()

	docSize = app.Flag("size", "document size").
		Short('s').Default("a4").String()
	docTitle    = app.Flag("title", "document title").Short('t').String()
	docKeywords = app.Flag("keywords", "space-separated document keywords").
			Short('t').String()
	docAuthor      = app.Flag("author", "document author").Short('o').String()
	docOrientation = app.Flag("orientation", "document orientation").
			Default("auto").Short('r').Enum("auto", "portrait", "landscape")

	compress = app.Flag("compress", "compress document").
			Default("true").Short('c').Bool()

	fontName = app.Flag("font-name", "text font").
			Default("Arial").String()
	fontStyle = app.Flag("font-style", "font style, [B]old, [I]talic, [U]nderline").
			PlaceHolder(" ").
			Enum("B", "I", "U", "BI", "BU", "IU", "BIU")
	fontSize = app.Flag("font-size", "OCR layer font size").
			Default("10").Float()
	textScaling = app.Flag("scaling", "Scale text to match word boundaries").
			Default("match").Enum("off", "contain", "match")

	force = app.Flag("force", "overwrite output file").Short('f').Bool()

	imgContrast = app.Flag("contrast", "automatic contrast amount").
			Default("0.5").Float()
	imgFormat = app.Flag("format", "format to use when storing images in PDF").
			Default("auto").Enum("auto", "jpg", "png")

	debug   = app.Flag("debug", "enable debug mode").Short('d').Bool()
	verbose = app.Flag("verbose", "enable verbose mode").Short('v').Bool()

	input = app.Arg("input", "input image").Required().Strings()
)

type Orientation string
type TextScaling string

const (
	AutoOrientation      Orientation = "auto"
	PortraitOrientation              = "portrait"
	LandscapeOrientation             = "landscape"
	NoTextScaling        TextScaling = "off"
	ContainTextScaling               = "contain"
	MatchTextScaling                 = "match"
)

type Document struct {
	*gofpdf.Fpdf
	ocrLayerId  int
	scanLayerId int
	debug       bool
	orientation Orientation
	textScaling TextScaling
}

func NewDocument(size string) *Document {
	pdf := gofpdf.New("P", "mm", size, "")
	pdf.SetAutoPageBreak(false, 0)
	ocrLayerId := pdf.AddLayer("OCR", true)
	scanLayerId := pdf.AddLayer("Scan", true)
	return &Document{
		Fpdf:        pdf,
		ocrLayerId:  ocrLayerId,
		scanLayerId: scanLayerId,
	}
}

func (d *Document) SetTextScaling(mode TextScaling) {
	d.textScaling = mode
}

func (d *Document) SetOrientation(orientation string) error {
	o := strings.ToLower(orientation)
	if o == "p" || o == "portrait" {
		d.orientation = "P"
		return nil
	} else if o == "l" || o == "landscape" {
		d.orientation = "L"
		return nil
	} else if o == "a" || o == "auto" {
		d.orientation = "A"
		return nil
	} else {
		return fmt.Errorf("Unknown orientation '%s'", orientation)
	}
}

func (d *Document) AddPage(imagename string, image internal.Image, words []internal.Word, format string) error {
	pdf := d.Fpdf

	imageWidth, imageHeight, _ := image.Dimensions()
	w, h := pdf.GetPageSize()

	// Add page with correct orientation
	if d.orientation == "A" {
		if imageWidth > imageHeight {
			pdf.AddPageFormat("L", gofpdf.SizeType{w, h})
			w, h = h, w
		} else {
			pdf.AddPageFormat("P", gofpdf.SizeType{w, h})
		}
	} else {
		pdf.AddPageFormat(string(d.orientation), gofpdf.SizeType{w, h})
	}

	// Determine image scaling factor
	iw, ih := float64(imageWidth), float64(imageHeight)
	mx, my := 1.0, 1.0

	if iw*h < ih*w {
		w = h * iw / ih
	} else {
		h = w * ih / iw
	}
	mx = w / iw
	my = h / ih

	addImageLayer := func() error {
		// Add image as top layer
		pdf.BeginLayer(d.scanLayerId)
		reader, imageFormat, err := image.Reader(format)
		if err != nil {
			return err
		}
		pdf.SetXY(0, 0)
		pdf.RegisterImageReader(imagename, imageFormat, reader)
		if d.debug {
			pdf.SetAlpha(0.5, "Normal")
			defer pdf.SetAlpha(1.0, "Normal")
		}
		pdf.Image(imagename, 0, 0, w, h, false, imageFormat, 0, "")
		pdf.EndLayer()
		return nil
	}

	addTextLayer := func() {
		// Add words acquired from OCR as bottom layer
		pdf.SetCellMargin(0)
		pdf.BeginLayer(d.ocrLayerId)
		for _, word := range words {
			x := float64(word.Left) * mx
			y := float64(word.Top) * my
			w := float64(word.Width) * mx
			h := float64(word.Height) * my

			// Scaling factors
			sx, sy := 1.0, 1.0

			// Get word dimensions at current font size
			sw := pdf.GetStringWidth(word.Text)
			_, sh := pdf.GetFontSize()

			switch d.textScaling {
			case ContainTextScaling:
				// Text expands linearly until contained by word boundary
				if sw == 0 {
					sw = w
				}
				if sw*h > sh*w {
					sx = w / sw
					sy = sx
				} else {
					sx = h / sh
					sy = sx
				}
			case MatchTextScaling:
				// Text has exactly same shape as word boundary
				if sw == 0 {
					sw = w
				}
				sx = w / sw
				sy = h / sh
			}

			if d.debug {
				// Outline detected word area
				pdf.SetDrawColor(255, 0, 0)
				pdf.Rect(x, y, w, h, "D")
			}

			// Print word in area of original box
			pdf.SetXY(x, y)
			pdf.TransformBegin()
			pdf.TransformScale(100*sx, 100*sy, x, y)
			if d.debug {
				// Highlight target area in green
				pdf.SetAlpha(0.5, "Multiply")
				pdf.SetFillColor(0, 255, 0)
				pdf.Rect(x, y, sw, sh, "F")
				pdf.SetAlpha(1.0, "Normal")
			}
			pdf.Write(sh, word.Text)
			pdf.TransformEnd()
		}
		pdf.EndLayer()
	}

	if d.debug {
		// Draw text on top of image
		if err := addImageLayer(); err != nil {
			return err
		}
		addTextLayer()
	} else {
		// Hide text below image
		addTextLayer()
		if err := addImageLayer(); err != nil {
			return err
		}
	}

	if err := pdf.Error(); err != nil {
		return err
	}

	return nil
}

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))

	if *verbose {
		fmt.Println("Initialising Tesseract...")
	}
	tess, err := internal.NewTess(*tessData, *tessLang)
	if err != nil {
		fmt.Printf("Could not initialise Tesseract: %s\n", err)
		os.Exit(1)
	}

	doc := NewDocument(*docSize)
	doc.debug = *debug
	doc.SetFont(*fontName, *fontStyle, *fontSize)
	doc.SetTextScaling(TextScaling(*textScaling))
	doc.SetTitle(*docTitle, true)
	doc.SetKeywords(*docKeywords, true)
	doc.SetAuthor(*docAuthor, true)
	doc.SetCompression(*compress)
	doc.SetOrientation(*docOrientation)

	files := *input

	if len(files) == 0 {
		fmt.Printf("No file(s) specified!\n")
		flag.Usage()
		os.Exit(1)
	}

	// When only one file is specified, output to a PDF of the same name
	outfn := *output
	if outfn == "" {
		outfn = files[0]
		ext := filepath.Ext(outfn)
		outfn = strings.TrimRight(outfn, ext) + ".pdf"
	}

	if *verbose {
		fmt.Printf("Using '%s' as output file.\n", outfn)
	}

	openFlags := os.O_RDWR | os.O_CREATE
	if *force {
		openFlags |= os.O_TRUNC
	} else {
		openFlags |= os.O_EXCL
	}

	outfile, err := os.OpenFile(outfn, openFlags, 0666)

	if os.IsExist(err) {
		fmt.Printf("Output file '%s' already exists. Use -force to overwrite.",
			outfn)
		os.Exit(1)
	} else if err != nil {
		fmt.Printf("Couldn't create output file '%s': %s", outfn, err)
		os.Exit(1)
	}

	// Iterate through each filename specified, adding a page for each
	for i, fn := range files {
		no := i + 1
		if *verbose {
			fmt.Printf("[P%d] Reading '%s'...\n", no, fn)
		}
		img := internal.NewImageFromFile(fn)
		img = img.Adjust(float32(*imgContrast))
		tess.SetImagePix(img.CPIX())
		if *verbose {
			fmt.Printf("[P%d] Recognising...", no)
		}
		words := tess.Words()
		if *verbose {
			fmt.Printf(" %d words found.\n", len(words))
			fmt.Printf("[P%d] Adding page\n", no)
		}
		err = doc.AddPage(fn, *img, words, *imgFormat)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	if *verbose {
		fmt.Printf("Writing output to '%s'...\n", outfn)
	}
	doc.OutputAndClose(outfile)
}
