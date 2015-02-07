package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"code.google.com/p/gofpdf"

	"bitbucket.org/johnsto/ocrpdf/internal"
)

type Document struct {
	*gofpdf.Fpdf
	ocrLayerId  int
	scanLayerId int
	orientation string
	debug       bool
	fitText     bool
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

func (d *Document) SetTextFitting(enabled bool) {
	d.fitText = enabled
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
		pdf.AddPageFormat(d.orientation, gofpdf.SizeType{w, h})
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
		reader, err := image.Reader(format)
		if err != nil {
			return err
		}
		pdf.SetXY(0, 0)
		pdf.RegisterImageReader(imagename, format, reader)
		if d.debug {
			pdf.SetAlpha(0.5, "Normal")
			defer pdf.SetAlpha(1.0, "Normal")
		}
		pdf.Image(imagename, 0, 0, w, h, false, format, 0, "")
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

			if d.fitText {
				if sw == 0 {
					sw = w
				}

				// Calculate scaling factor
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

	tessData := flag.String("tess-data", "", "Tesseract data directory")
	tessLang := flag.String("tess-lang", "", "Tesseract language")

	docSize := flag.String("size", "a4",
		"document size, e.g. A4 or 210x297mm")
	docTitle := flag.String("title", "", "document title")
	docKeywords := flag.String("keywords", "",
		"document keywords (space separated)")
	docAuthor := flag.String("author", "", "document author")
	docOrientation := flag.String("orientation", "auto",
		"document orientation (auto/portrait/landscape)")

	compress := flag.Bool("compress", true, "compress document")

	fontName := flag.String("font-name", "Arial", "OCR layer font")
	fontStyle := flag.String("font-style", "",
		"OCR layer font style, either 'B', 'I' or 'U' (or a combination)")
	fontSize := flag.Float64("font-size", 10, "OCR layer font size")

	textFitting := flag.Bool("fit-text", true, "Scale text to match OCR")

	force := flag.Bool("force", false, "overwrite output file if necessary")

	imgContrast := flag.Float64("contrast", 0.5,
		"automatic contrast amount (0: none, 1: max)")
	imgFormat := flag.String("format", "jpg",
		"format to use when storing images in PDF (jpg|png)")

	debug := flag.Bool("debug", false, "debug mode")

	flag.Parse()

	tess, err := internal.NewTess(*tessData, *tessLang)
	if err != nil {
		fmt.Printf("Could not initialise Tesseract: %s\n", err)
		os.Exit(1)
	}

	doc := NewDocument(*docSize)
	doc.debug = *debug
	doc.SetFont(*fontName, *fontStyle, *fontSize)
	doc.SetTextFitting(*textFitting)
	doc.SetTitle(*docTitle, true)
	doc.SetKeywords(*docKeywords, true)
	doc.SetAuthor(*docAuthor, true)
	doc.SetCompression(*compress)
	doc.SetOrientation(*docOrientation)

	files := flag.Args()

	if len(files) == 0 {
		fmt.Printf("No file(s) specified!\n")
		flag.Usage()
		os.Exit(1)
	}

	outfn := files[0]
	if len(files) == 1 {
		ext := filepath.Ext(outfn)
		outfn = strings.TrimRight(outfn, ext) + ".pdf"
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

	for _, fn := range files {
		img := internal.NewImageFromFile(fn)
		img = img.Adjust(float32(*imgContrast))
		tess.SetImagePix(img.CPIX())
		words := tess.Words()
		err = doc.AddPage(fn, *img, words, *imgFormat)
		if err != nil {
			log.Fatalln(err)
		}
	}

	doc.OutputAndClose(outfile)
}
