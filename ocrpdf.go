package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kingpin"

	"bitbucket.org/johnsto/ocrpdf/internal"
)

var (
	debug   bool = false
	verbose bool = false

	app = kingpin.New("ocrpdf", "Converts scanned documents into searchable PDFs")

	input  = app.Arg("input", "input image filename(s)").Required().Strings()
	output = app.Flag("output", "output filename").Short('o').String()
	force  = app.Flag("force", "overwrite output file").Short('f').Bool()

	tessData = app.Flag("tess-data", "Tesseract data directory").String()
	tessLang = app.Flag("tess-lang", "Tesseract language").String()

	docSize        = app.Flag("size", "document size").Short('s').Default("a4").String()
	docTitle       = app.Flag("title", "document title").Short('t').String()
	docSubject     = app.Flag("subject", "document subject").Short('j').String()
	docKeywords    = app.Flag("keywords", "space-separated document keywords").Short('t').String()
	docAuthor      = app.Flag("author", "document author").Short('a').String()
	docCreator     = app.Flag("creator", "document creator").Default("ocrpdf").String()
	docOrientation = app.Flag("orientation", "document orientation").
			Default("auto").Short('r').Enum("auto", "portrait", "landscape")
	docCompress = app.Flag("compress", "compress document").
			Default("true").Short('c').Bool()

	fontName  = app.Flag("font-name", "text font").Default("Arial").String()
	fontStyle = app.Flag("font-style", "font style, [B]old, [I]talic, [U]nderline").
			PlaceHolder(" ").Enum("B", "I", "U", "BI", "BU", "IU", "BIU")
	fontSize = app.Flag("font-size", "OCR layer font size").Default("10").Float()

	textScaling = app.Flag("scaling", "Scale text to match word boundaries").
			Default("match").Enum("off", "contain", "match")

	imgContrast = app.Flag("contrast", "automatic contrast amount").Default("0.5").Float()
	imgFormat   = app.Flag("format", "format to use when storing images in PDF").
			Default("auto").Enum("auto", "jpg", "png")
)

func init() {
	app.Flag("debug", "enable debug mode").Short('d').BoolVar(&debug)
	app.Flag("verbose", "enable verbose mode").Short('v').BoolVar(&verbose)
}

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))

	logv("Initialising Tesseract...")
	tess, err := internal.NewTess(*tessData, *tessLang)

	if err != nil {
		fmt.Errorf("Could not initialise Tesseract: %s\n", err)
		os.Exit(1)
	}

	doc := NewDocument(*docSize)
	doc.debug = debug
	doc.SetFont(*fontName, *fontStyle, *fontSize)
	doc.SetTextScaling(TextScaling(*textScaling))
	doc.SetTitle(*docTitle, true)
	doc.SetSubject(*docSubject, true)
	doc.SetKeywords(*docKeywords, true)
	doc.SetAuthor(*docAuthor, true)
	doc.SetCompression(*docCompress)
	doc.SetOrientation(*docOrientation)

	files := *input

	// When only one file is specified, output to a PDF of the same name
	outfn := *output
	if outfn == "" {
		outfn = files[0]
		ext := filepath.Ext(outfn)
		outfn = strings.TrimRight(outfn, ext) + ".pdf"
	}

	logvf("Using '%s' as output file.\n", outfn)

	openFlags := os.O_RDWR | os.O_CREATE
	if *force {
		openFlags |= os.O_TRUNC
	} else {
		openFlags |= os.O_EXCL
	}

	outfile, err := os.OpenFile(outfn, openFlags, 0666)

	if os.IsExist(err) {
		fmt.Printf("Output file '%s' already exists. Use -force to overwrite.\n", outfn)
		os.Exit(1)
	} else if err != nil {
		fmt.Printf("Couldn't create output file '%s': %s\n", outfn, err)
		os.Exit(1)
	}

	// Iterate through each filename specified, adding a page for each
	for i, fn := range files {
		pageno := i + 1

		logvf("[P%d] Reading '%s'...\n", pageno, fn)
		img := internal.NewImageFromFile(fn)
		img = img.Adjust(float32(*imgContrast))
		tess.SetImagePix(img.CPIX())

		logvf("[P%d] Recognising...", pageno)
		words := tess.Words()
		logvf(" %d words found.\n", len(words))

		logvf("[P%d] Adding page\n", pageno)
		err = doc.AddPage(fn, *img, words, *imgFormat)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	logvf("Writing output to '%s'...\n", outfn)

	doc.OutputAndClose(outfile)
}
