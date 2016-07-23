package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnsto/ocrpdf"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	MM_TO_INCH float64 = 0.039
)

var (
	debug   = false
	verbose = false

	app = kingpin.New("ocrpdf", "Converts scanned documents into searchable PDFs")

	files  = app.Arg("files", "filename(s)").Required().Strings()
	output = app.Flag("output", "output filename").Short('o').String()
	force  = app.Flag("force", "overwrite output file").Short('f').Bool()

	// Tesseract configuration
	tessData = app.Flag("tess-data", "Tesseract data directory").String()
	tessLang = app.Flag("tess-lang", "Tesseract language").String()

	// Document configuration
	docSize = app.Flag("size", "document size").
		Short('s').Default("a4").String()
	docOrientation = app.Flag("orientation", "document orientation").
			Default("auto").Short('r').Enum("auto", "portrait", "landscape")
	docCompress = app.Flag("compress", "compress document").
			Default("true").Short('c').Bool()
	docDPI = app.Flag("dpi", "resize image to DPI").Default("0").Int()

	// Document metadata
	docTitle    = app.Flag("title", "document title").Short('t').String()
	docSubject  = app.Flag("subject", "document subject").Short('j').String()
	docKeywords = app.Flag("keywords", "space-separated document keywords").
			Short('k').String()
	docAuthor  = app.Flag("author", "document author").Short('a').String()
	docCreator = app.Flag("creator", "document creator").
			Default("ocrpdf").String()

	// Font settings
	fontName = app.Flag("font-name", "text font").
			Default("Arial").String()
	fontStyle = app.Flag("font-style", "font style, [B]old, [I]talic, [U]nderline").
			PlaceHolder(" ").Enum("B", "I", "U", "BI", "BU", "IU", "BIU")
	fontSize = app.Flag("font-size", "OCR layer font size").
			Default("10").Float()

	// Text settings
	textScaling = app.Flag("scaling", "Scale text to match word boundaries").
			Default("match").Enum("off", "contain", "match")

	// Image settings
	imgContrast = app.Flag("contrast", "automatic contrast amount").
			Default("0.5").Float()
	imgFormat = app.Flag("format", "format to use when storing images in PDF").
			Default("auto").Enum("auto", "jpg", "png")
)

func init() {
	app.Flag("debug", "enable debug mode").Short('d').BoolVar(&debug)
	app.Flag("verbose", "enable verbose mode").Short('v').BoolVar(&verbose)
}

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))

	logv("Initialising Tesseract...")
	tess, err := ocrpdf.NewTess(*tessData, *tessLang)

	if err != nil {
		logef("could not initialise Tesseract: %s\n", err)
		os.Exit(1)
	}

	doc := ocrpdf.NewDocument(*docSize)
	doc.SetDebug(debug)
	doc.SetFont(*fontName, *fontStyle, *fontSize)
	doc.SetTextScaling(ocrpdf.TextScaling(*textScaling))
	doc.SetTitle(*docTitle, true)
	doc.SetSubject(*docSubject, true)
	doc.SetKeywords(*docKeywords, true)
	doc.SetAuthor(*docAuthor, true)
	doc.SetCompression(*docCompress)
	doc.SetOrientation(ocrpdf.Orientation(*docOrientation))

	outfn := *output
	infns := *files
	if outfn == "" {
		// Search input files for a .pdf file
		pos := -1
		for i, fn := range infns {
			ext := strings.ToLower(filepath.Ext(fn))
			if ext == ".pdf" {
				if pos >= 0 {
					// two output files specified?
					logef("Multiple .pdf output files specified. " +
						"Use -o to specify output file explicitly.\n")
					os.Exit(1)
				}
				pos = i
				outfn = fn
			}
		}

		if pos >= 0 {
			// Remove output file from list of input files
			infns = append(infns[:pos], infns[pos+1:]...)
		} else {
			// No .pdf file on command line, so use name of first input instead
			outfn = infns[0]
			ext := filepath.Ext(outfn)
			outfn = strings.TrimRight(outfn, ext) + ".pdf"
		}
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
		logef("Output file '%s' already exists. Use -force to overwrite.\n",
			outfn)
		os.Exit(1)
	} else if err != nil {
		logef("Couldn't create output file '%s': %s\n", outfn, err)
		os.Exit(1)
	}

	// Iterate through each filename specified, adding a page for each
	for i, fn := range infns {
		pageno := i + 1

		// Read image file
		logvf("[P%d] Reading '%s'...\n", pageno, fn)
		img, err := ocrpdf.NewImageFromFile(fn)
		if err != nil {
			logef("Unable to read image from file '%s'\n", fn)
			os.Exit(1)
		}

		w, h, d := img.Dimensions()
		logvf("[P%d] Read '%s' (%dx%d@%d)\n", pageno, fn, w, h, d)

		if *docDPI != 0 {
			// Resize image to requested d/in (rather, d/mm)
			dpmm := float64(*docDPI) * MM_TO_INCH
			pw, ph := doc.GetPageSize()
			w, h := int32(pw*dpmm), int32(ph*dpmm)
			logvf("[P%d] Scaling down to (%d,%d) @ %ddpi\n",
				pageno, w, h, *docDPI)
			img = img.ScaleDown(w, h)
		}

		// Increase contrast
		img = img.Adjust(float32(*imgContrast))
		tess.SetImagePix(img.CPIX())

		// Extract words
		logvf("[P%d] Recognising...", pageno)
		words := tess.Words()
		logvf(" %d words found.\n", len(words))

		// Add to PDF
		logvf("[P%d] Adding page\n", pageno)
		err = doc.AddPage(*img, fn, words, *imgFormat)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	logvf("Writing output to '%s'...\n", outfn)

	doc.OutputAndClose(outfile)
}
