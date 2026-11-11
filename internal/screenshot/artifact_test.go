package screenshot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fixturePNG(t *testing.T) []byte {
	t.Helper()
	m := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			m.Set(x, y, color.RGBA{255, 80, 20, 255})
		}
	}
	var b bytes.Buffer
	if e := png.Encode(&b, m); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func fixtureDoc(t *testing.T, b []byte) Document {
	return Document{Version: 1, CaptureID: "synthetic-frame", Kind: "imported", ImageSHA: Digest(b), PixelWidth: 4, PixelHeight: 2, PointWidth: 2, PointHeight: 1, PreviewID: "preview-frame", PreviewSHA: Digest(b), PreviewWidth: 4, PreviewHeight: 2, CapturedAt: time.Now().UTC().Format(time.RFC3339)}
}
func fixtureProducer(t *testing.T, b []byte, doc Document) NativeCall {
	return func(_ context.Context, _ Request, fs []*os.File) (Reply, error) {
		for _, f := range fs[len(fs)-2:] {
			if _, e := f.Write(b); e != nil {
				t.Fatal(e)
			}
		}
		v, _ := json.Marshal(doc)
		return Reply{Version: 1, OK: true, Result: v}, nil
	}
}
func TestASC001ArtifactPublicationIsAtomicAndPreservesExistingFiles(t *testing.T) {
	b := fixturePNG(t)
	d := fixtureDoc(t, b)
	dir := t.TempDir()
	out := filepath.Join(dir, "result.png")
	doc, e := Produce(context.Background(), Request{Operation: "capture"}, "", out, fixtureProducer(t, b, d))
	if e != nil {
		t.Fatal(e)
	}
	got, e := ReadDocument(filepath.Join(dir, "result.capture.json"))
	if e != nil || got.ImageSHA != Digest(b) || got.ImagePath != doc.ImagePath {
		t.Fatal(got, e)
	}
	if _, e = Produce(context.Background(), Request{Operation: "capture"}, "", out, fixtureProducer(t, b, d)); e == nil {
		t.Fatal("overwrote output")
	}
	original, _ := os.ReadFile(out)
	if !bytes.Equal(original, b) {
		t.Fatal("existing changed")
	}
	bad := d
	bad.ImageSHA = Digest([]byte("wrong"))
	badout := filepath.Join(dir, "bad.png")
	if _, e = Produce(context.Background(), Request{Operation: "capture"}, "", badout, fixtureProducer(t, b, bad)); e == nil {
		t.Fatal("hash mismatch accepted")
	}
	if _, e = os.Lstat(badout); !os.IsNotExist(e) {
		t.Fatal("partial result published", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	call := fixtureProducer(t, b, d)
	cancelled := func(c context.Context, r Request, f []*os.File) (Reply, error) {
		v, e := call(c, r, f)
		cancel()
		return v, e
	}
	if _, e = Produce(ctx, Request{Operation: "capture"}, "", filepath.Join(dir, "late.png"), cancelled); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(dir, "late.png")); !os.IsNotExist(e) {
		t.Fatal("late result published")
	}
	victim := filepath.Join(dir, "victim")
	_ = os.WriteFile(victim, []byte("keep"), 0600)
	link := filepath.Join(dir, "link.png")
	_ = os.Symlink(victim, link)
	if _, e = Produce(context.Background(), Request{Operation: "capture"}, "", link, call); e == nil {
		t.Fatal("symlink output accepted")
	}
	if _, e = OpenImage(link, ""); e == nil {
		t.Fatal("symlink input accepted")
	}
}
func TestASC003CoordinatesAndEditableImagesStayBoundToExactBytes(t *testing.T) {
	b := fixturePNG(t)
	d := fixtureDoc(t, b)
	s := Spec{Version: 1, CaptureID: d.CaptureID, ImageSHA: d.ImageSHA, Space: "preview_px", PreviewID: d.PreviewID, Marks: []Mark{{Type: "redact", X: 1, Y: 0, Width: 2, Height: 1}}}
	if e := s.Validate(d); e != nil {
		t.Fatal(e)
	}
	bad := s
	bad.PreviewID = "other"
	if bad.Validate(d) == nil {
		t.Fatal("stale preview accepted")
	}
	bad = s
	bad.Marks = []Mark{{Type: "rect", X: 3, Width: 2, Height: 1}}
	if bad.Validate(d) == nil {
		t.Fatal("outside mark accepted")
	}
	dir := t.TempDir()
	base := filepath.Join(dir, "base.png")
	_ = os.WriteFile(base, b, 0600)
	d.ImagePath = base
	var passed *os.File
	call := func(c context.Context, r Request, f []*os.File) (Reply, error) {
		if r.Spec == nil || r.ImageSHA != d.ImageSHA {
			t.Fatal("frame lost")
		}
		dup, e := os.OpenFile(f[1].Name(), os.O_WRONLY, 0)
		if e != nil {
			t.Fatal(e)
		}
		passed = dup
		return fixtureProducer(t, b, d)(c, r, f)
	}
	doc, e := Produce(context.Background(), Request{Operation: "annotate", ImageSHA: d.ImageSHA, Document: &d, Spec: &s}, base, filepath.Join(dir, "marked.png"), call)
	if e != nil {
		t.Fatal(e)
	}
	if doc.BaseDocument == nil || doc.Annotations.PreviewID != d.PreviewID {
		t.Fatal("editable frame lost")
	}
	_, _ = passed.WriteAt([]byte("late writer"), 0)
	passed.Close()
	out, _ := os.ReadFile(doc.ImagePath)
	if !bytes.Equal(out, b) {
		t.Fatal("native writer changed published inode")
	}
	unchanged, _ := os.ReadFile(base)
	if !bytes.Equal(unchanged, b) {
		t.Fatal("base mutated")
	}
	if _, e = Produce(context.Background(), Request{Operation: "annotate", Document: &doc}, base, filepath.Join(dir, "nested.png"), call); e == nil {
		t.Fatal("silently nested editable image")
	}
	badDoc := d
	badDoc.PixelWidth = 3
	if _, e = Produce(context.Background(), Request{Operation: "capture"}, "", filepath.Join(dir, "wrongdims.png"), fixtureProducer(t, b, badDoc)); e == nil {
		t.Fatal("PNG dimensions mismatch accepted")
	}
}
