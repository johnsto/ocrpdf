package internal

// #cgo LDFLAGS: -ltesseract
// #include "tesseract/capi.h"
// #include <stdlib.h>
import "C"
import (
	"errors"
	"runtime"
	"unsafe"
)

type Word struct {
	Text   string
	Left   int
	Right  int
	Top    int
	Bottom int
	Width  int
	Height int
}

func NewTess(datapath string, language string) (*Tess, error) {
	api := C.TessBaseAPICreate()

	var cDatapath *C.char
	if datapath != "" {
		cDatapath = C.CString(datapath)
	}
	defer C.free(unsafe.Pointer(cDatapath))

	var cLanguage *C.char
	if language != "" {
		cLanguage = C.CString(language)
	}
	defer C.free(unsafe.Pointer(cLanguage))

	res := C.TessBaseAPIInit3(api, cDatapath, cLanguage)
	if res != 0 {
		return nil, errors.New("could not initiate new Tess instance")
	}

	tess := &Tess{
		api: api,
	}

	runtime.SetFinalizer(tess, (*Tess).delete)

	return tess, nil
}

type Tess struct {
	api *C.TessBaseAPI
}

func (t *Tess) delete() {
	if t.api != nil {
		C.TessBaseAPIEnd(t.api)
		C.TessBaseAPIDelete(t.api)
	}
}

// SetImagePix sets the image to perform recognition on
func (t *Tess) SetImagePix(pix *C.struct_Pix) {
	C.TessBaseAPISetImage2(t.api, pix)
}

// Words analyses the document and returns a list of recognised words.
func (t *Tess) Words() []Word {
	var words []Word

	C.TessBaseAPIRecognize(t.api, nil)

	ri := C.TessBaseAPIGetIterator(t.api)
	defer C.TessResultIteratorDelete(ri)
	pi := C.TessResultIteratorGetPageIterator(ri)

	if ri != nil {
		for {
			cWord := C.TessResultIteratorGetUTF8Text(ri, C.RIL_WORD)
			var cLeft, cTop, cRight, cBottom C.int
			C.TessPageIteratorBoundingBox(pi, C.RIL_WORD,
				&cLeft, &cTop, &cRight, &cBottom)

			word := Word{
				Text:   C.GoString(cWord),
				Left:   int(cLeft),
				Right:  int(cRight),
				Top:    int(cTop),
				Bottom: int(cBottom),
				Width:  int(cRight - cLeft),
				Height: int(cBottom - cTop),
			}

			words = append(words, word)
			if C.TessPageIteratorNext(pi, C.RIL_WORD) == C.int(0) {
				break
			}
		}
	}

	return words
}
