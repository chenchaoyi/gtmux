// Package screenshot defines the CLI/native screenshot contract. Image bytes stay
// in caller-owned files; runtime state contains transport and receipt metadata only.
package screenshot

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
)

const Version = 1
const MaxJSON = 256 << 10
const MaxImage = 30 << 20
const MaxPixels = 32_000_000

type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

func (e *Error) Error() string        { return e.Code + ": " + e.Message }
func Fail(code, message string) error { return &Error{Code: code, Message: message} }

type Reply struct {
	Version int             `json:"schema_version"`
	ID      string          `json:"request_id,omitempty"`
	OK      bool            `json:"ok"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Mark struct {
	Type       string      `json:"type"`
	X          float64     `json:"x,omitempty"`
	Y          float64     `json:"y,omitempty"`
	Width      float64     `json:"width,omitempty"`
	Height     float64     `json:"height,omitempty"`
	From       []float64   `json:"from,omitempty"`
	To         []float64   `json:"to,omitempty"`
	Points     [][]float64 `json:"points,omitempty"`
	Text       string      `json:"text,omitempty"`
	Color      string      `json:"color,omitempty"`
	WidthStyle string      `json:"width_style,omitempty"`
}
type Spec struct {
	Version   int    `json:"schema_version"`
	CaptureID string `json:"capture_id"`
	ImageSHA  string `json:"image_sha256"`
	Space     string `json:"coordinate_space"`
	PreviewID string `json:"preview_id,omitempty"`
	Marks     []Mark `json:"annotations"`
}

// Request contains no arbitrary native file paths or shell commands. The input
// and two output files, when required, travel as opened SCM_RIGHTS descriptors.
type Request struct {
	Version    int       `json:"schema_version"`
	ID         string    `json:"request_id"`
	Nonce      string    `json:"host_nonce"`
	Operation  string    `json:"operation"`
	DeadlineMS int64     `json:"deadline_ms"`
	App        string    `json:"app,omitempty"`
	SourceKind string    `json:"source_kind,omitempty"`
	SourceRef  string    `json:"source_ref,omitempty"`
	Rect       []float64 `json:"rect,omitempty"`
	ImageSHA   string    `json:"image_sha256,omitempty"`
	Document   *Document `json:"document,omitempty"`
	Spec       *Spec     `json:"spec,omitempty"`
}

type Document struct {
	Version       int       `json:"schema_version"`
	CaptureID     string    `json:"capture_id"`
	Kind          string    `json:"source_kind"`
	ImagePath     string    `json:"image_path"`
	ImageSHA      string    `json:"image_sha256"`
	PixelWidth    int       `json:"pixel_width"`
	PixelHeight   int       `json:"pixel_height"`
	PointWidth    float64   `json:"point_width"`
	PointHeight   float64   `json:"point_height"`
	PreviewID     string    `json:"preview_id"`
	PreviewPath   string    `json:"preview_path"`
	PreviewSHA    string    `json:"preview_sha256"`
	PreviewWidth  int       `json:"preview_width"`
	PreviewHeight int       `json:"preview_height"`
	SourceRect    []float64 `json:"source_rect_pt,omitempty"`
	SourceToImage []float64 `json:"source_to_image,omitempty"`
	CapturedAt    string    `json:"captured_at"`
	BasePath      string    `json:"base_image_path,omitempty"`
	BaseSHA       string    `json:"base_image_sha256,omitempty"`
	BaseDocument  *Document `json:"base_document,omitempty"`
	Annotations   *Spec     `json:"annotation_spec,omitempty"`
}

func RandomID() (string, error) {
	var b [16]byte
	_, err := rand.Read(b[:])
	return hex.EncodeToString(b[:]), err
}
func Decode(data []byte, value any) error {
	if len(data) > MaxJSON {
		return Fail("json-too-large", "JSON exceeds the request budget")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return Fail("invalid-json", "Invalid or unsupported JSON fields")
	}
	if d.Decode(new(any)) != io.EOF {
		return Fail("invalid-json", "Expected exactly one JSON value")
	}
	return nil
}
func (d Document) Validate() error {
	if d.Version != Version || d.CaptureID == "" || len(d.ImageSHA) != 64 || len(d.PreviewSHA) != 64 || d.PixelWidth <= 0 || d.PixelHeight <= 0 || d.PixelWidth > MaxPixels || d.PixelHeight > MaxPixels || int64(d.PixelWidth)*int64(d.PixelHeight) > MaxPixels || d.PreviewWidth <= 0 || d.PreviewHeight <= 0 || d.PreviewID == "" || d.PreviewWidth > 1600 || d.PreviewHeight > 1600 {
		return Fail("invalid-document", "Invalid image or preview metadata")
	}
	if _, err := hex.DecodeString(d.ImageSHA); err != nil {
		return Fail("invalid-document", "Invalid image hash")
	}
	if _, err := hex.DecodeString(d.PreviewSHA); err != nil {
		return Fail("invalid-document", "Invalid preview hash")
	}
	if !finite(d.PointWidth) || !finite(d.PointHeight) || d.PointWidth <= 0 || d.PointHeight <= 0 || math.Abs(float64(d.PixelWidth)/d.PointWidth-float64(d.PixelHeight)/d.PointHeight) > 0.02 {
		return Fail("invalid-document", "Invalid point/pixel transform")
	}
	if d.BaseDocument != nil && d.BaseDocument.BaseDocument != nil {
		return Fail("invalid-document", "Nested editable documents are unsupported")
	}
	return nil
}
func (s Spec) Validate(d Document) error {
	if err := d.Validate(); err != nil {
		return err
	}
	if s.Version != Version || s.CaptureID != d.CaptureID || s.ImageSHA != d.ImageSHA || len(s.Marks) > 500 || (s.Space != "image_px" && s.Space != "preview_px") || (s.Space == "preview_px" && s.PreviewID != d.PreviewID) {
		return Fail("annotation-frame-mismatch", "Annotations must address this exact image and coordinate frame")
	}
	w, h := float64(d.PixelWidth), float64(d.PixelHeight)
	if s.Space == "preview_px" {
		w, h = float64(d.PreviewWidth), float64(d.PreviewHeight)
	}
	point := func(p []float64) bool {
		return len(p) == 2 && finite(p[0]) && finite(p[1]) && p[0] >= 0 && p[1] >= 0 && p[0] <= w && p[1] <= h
	}
	count := 0
	for _, m := range s.Marks {
		if m.Color != "" && m.Color != "red" && m.Color != "blue" && m.Color != "yellow" {
			return Fail("invalid-annotation", "Unsupported marking color")
		}
		if m.WidthStyle != "" && m.WidthStyle != "thin" && m.WidthStyle != "medium" && m.WidthStyle != "thick" {
			return Fail("invalid-annotation", "Unsupported stroke style")
		}
		valid := false
		switch m.Type {
		case "rect", "ellipse", "mosaic", "redact":
			valid = point([]float64{m.X, m.Y}) && finite(m.Width) && finite(m.Height) && m.Width > 0 && m.Height > 0 && m.X+m.Width <= w && m.Y+m.Height <= h
		case "arrow":
			valid = point(m.From) && point(m.To)
		case "line":
			valid = len(m.Points) >= 2
			count += len(m.Points)
			for _, p := range m.Points {
				valid = valid && point(p)
			}
		case "text":
			valid = point([]float64{m.X, m.Y}) && len(m.Text) > 0 && len(m.Text) <= 2048
		}
		if !valid || count > 10000 {
			return Fail("invalid-annotation", "Annotation is invalid or outside the selected image")
		}
	}
	return nil
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func AsError(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Code: "deadline-exceeded", Message: "The screenshot request deadline expired"}
	}
	if errors.Is(err, context.Canceled) {
		return &Error{Code: "cancelled", Message: "The screenshot request was cancelled"}
	}
	return &Error{Code: "operation-failed", Message: "The operation could not be completed"}
}
