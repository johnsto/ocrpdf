package ocrpdf

import "code.google.com/p/gofpdf"

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

// NewDocument returns a new Document of the specified size.
func NewDocument(size string) *Document {
	pdf := gofpdf.New("P", "mm", size, "")
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetCellMargin(0)
	ocrLayerId := pdf.AddLayer("OCR", true)
	scanLayerId := pdf.AddLayer("Scan", true)
	return &Document{
		Fpdf:        pdf,
		ocrLayerId:  ocrLayerId,
		scanLayerId: scanLayerId,
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

// addImageLayer adds the specified image to the page, embedding it using
// the given format.
func (d *Document) addImageLayer(image Image, name string, format string, w, h float64) error {
	pdf := d.Fpdf

	// Add image as top layer
	pdf.BeginLayer(d.scanLayerId)
	reader, imageFormat, err := image.Reader(format)
	if err != nil {
		return err
	}
	pdf.SetXY(0, 0)
	pdf.RegisterImageReader(name, imageFormat, reader)
	if d.debug {
		pdf.SetAlpha(0.5, "Normal")
		defer pdf.SetAlpha(1.0, "Normal")
	}
	pdf.Image(name, 0, 0, w, h, false, imageFormat, 0, "")
	pdf.EndLayer()
	return nil
}

// addTextLayer adds the specified words to the page, scaling the X/Y
// coordinates accordingly.
func (d *Document) addTextLayer(words []Word, mx, my float64) {
	pdf := d.Fpdf
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

// AddPage appends the given image to the document, annotating the document
// with the detected words. Ensure `name` is unique for each distinct image.
func (d *Document) AddPage(name string, image Image, words []Word, format string) error {
	pdf := d.Fpdf

	imageWidth, imageHeight, _ := image.Dimensions()
	w, h := pdf.GetPageSize()

	// Add page with correct orientation
	if d.orientation == AutoOrientation {
		if imageWidth > imageHeight {
			pdf.AddPageFormat(LandscapeOrientation, gofpdf.SizeType{w, h})
			w, h = h, w
		} else {
			pdf.AddPageFormat(PortraitOrientation, gofpdf.SizeType{w, h})
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

	if d.debug {
		// Draw text on top of image
		if err := d.addImageLayer(image, name, format, w, h); err != nil {
			return err
		}
		d.addTextLayer(words, mx, my)
	} else {
		// Hide text below image
		d.addTextLayer(words, mx, my)
		if err := d.addImageLayer(image, name, format, w, h); err != nil {
			return err
		}
	}

	if err := pdf.Error(); err != nil {
		return err
	}

	return nil
}
