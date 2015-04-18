package ocrpdf

import "github.com/jung-kurt/gofpdf"

// Orientation defines page orientations
type Orientation string

// Different orientation modes.
const (
	// AutoOrientation chooses orientation based on longest edge
	AutoOrientation      Orientation = "auto"
	PortraitOrientation              = "portrait"
	LandscapeOrientation             = "landscape"
)

// TextScaling defines text scaling modes
type TextScaling string

const (
	// NoTextScaling specifies that no text scaling will be performed.
	NoTextScaling TextScaling = "off"
	// ContainTextScaling fits the text to the detected word boundary,
	// whilst maintaining the correct aspect ratio for the font.
	ContainTextScaling = "contain"
	// MatchTextScaling fits the text to the detected word boundary exactly,
	// scaling the font if required.
	MatchTextScaling = "match"
)

// Document is a wrapped version of gofpdf.Fpd which adds additional methods
// for constructing documents with OCR-generated text.
type Document struct {
	*gofpdf.Fpdf
	ocrLayerID  int
	scanLayerID int
	debug       bool
	orientation Orientation
	textScaling TextScaling
}

// NewDocument returns a new Document of the specified size.
func NewDocument(size string) *Document {
	pdf := gofpdf.New("P", "mm", size, "")
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetCellMargin(0)
	ocrLayerID := pdf.AddLayer("OCR", true)
	scanLayerID := pdf.AddLayer("Scan", true)
	return &Document{
		Fpdf:        pdf,
		ocrLayerID:  ocrLayerID,
		scanLayerID: scanLayerID,
	}
}

// SetTextScaling enables the scaling of embedded text such that it matches
// the same area that the original text was detected.
func (d *Document) SetTextScaling(mode TextScaling) {
	d.textScaling = mode
}

// SetOrientation sets the orientation of new pages
func (d *Document) SetOrientation(orientation Orientation) {
	d.orientation = orientation
}

// SetDebug enables debug mode, in which detected words are outlined, and the
// text layer is arranged on top of the image (scan) layer.
func (d *Document) SetDebug(enabled bool) {
	d.debug = enabled
}

// AddImageLayer adds the specified image to the page, embedding it using
// the given format, and appear at the specified size (in page units).
func (d *Document) AddImageLayer(image Image, imagename string,
	format string, w, h float64) {
	pdf := d.Fpdf

	pdf.BeginLayer(d.scanLayerID)

	// Register image
	reader, imageFormat, err := image.Reader(format)
	if err != nil {
		pdf.SetError(err)
		return
	}
	pdf.RegisterImageReader(imagename, imageFormat, reader)

	if d.debug {
		// Make scan semi-transparent in debug mode so it's easier to see text
		pdf.SetAlpha(0.5, "Normal")
		defer pdf.SetAlpha(1.0, "Normal")
	}

	pdf.SetXY(0, 0)
	pdf.Image(imagename, 0, 0, w, h, false, imageFormat, 0, "")

	pdf.EndLayer()
}

// AddWords adds the specified words to the page.
func (d *Document) AddWords(words []Word) {
	pdf := d.Fpdf
	for _, word := range words {
		x, y := float64(word.Left), float64(word.Top)
		w, h := float64(word.Width), float64(word.Height)

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

		pdf.Cell(sw, sh, word.Text)
		pdf.TransformEnd()
	}
}

// GetPageConfiguration returns a suitable page size and orientation to
// contain an image of the specified dimensions.
func (d *Document) GetPageConfiguration(iw, ih float64) (
	w, h float64, orientation Orientation) {

	w, h = d.GetPageSize()

	// Add page with correct orientation
	orientation = d.orientation
	if orientation == AutoOrientation {
		if iw > ih {
			w, h = h, w
			orientation = LandscapeOrientation
		} else {
			orientation = PortraitOrientation
		}
	}

	if iw*h < ih*w {
		w = h * iw / ih
	} else {
		h = w * ih / iw
	}

	return w, h, orientation
}

// AddPage appends the given image to the document, annotating the document
// with the detected words. Ensure `name` is unique for each distinct image.
func (d *Document) AddPage(image Image, imagename string,
	words []Word, format string) error {
	iw, ih, _ := image.Dimensions()
	w, h, orientation := d.GetPageConfiguration(float64(iw), float64(ih))

	d.AddPageFormat(string(orientation), gofpdf.SizeType{Wd: w, Ht: h})

	addImageLayer := func() {
		d.AddImageLayer(image, imagename, format, w, h)
	}

	addWordsLayer := func() {
		mx, my := w/float64(iw), h/float64(ih)
		d.BeginLayer(d.ocrLayerID)
		d.TransformBegin()
		d.TransformScale(100*mx, 100*my, 0, 0)
		d.AddWords(words)
		d.TransformEnd()
		d.EndLayer()
	}

	if d.debug {
		// Draw text on top of image
		addImageLayer()
		addWordsLayer()
	} else {
		// Hide text below image
		addWordsLayer()
		addImageLayer()
	}

	if err := d.Error(); err != nil {
		return err
	}

	return nil
}
