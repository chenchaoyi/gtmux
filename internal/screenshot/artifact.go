package screenshot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type NativeCall func(context.Context, Request, []*os.File) (Reply, error)

func ReadDocument(path string) (Document, error) {
	var d Document
	b, err := readBounded(path, MaxJSON)
	if err != nil {
		return d, err
	}
	if err = Decode(b, &d); err != nil {
		return d, err
	}
	return d, d.Validate()
}
func ReadSpec(path string) (Spec, error) {
	var s Spec
	b, err := readBounded(path, MaxJSON)
	if err == nil {
		err = Decode(b, &s)
	}
	return s, err
}
func readBounded(path string, limit int64) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, Fail("file-unavailable", "The input must be an accessible regular file")
	}
	defer f.Close()
	i, err := f.Stat()
	if err != nil || !i.Mode().IsRegular() || i.Size() > limit {
		return nil, Fail("file-too-large", "Input exceeds its size budget")
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if len(b) > int(limit) {
		return nil, Fail("file-too-large", "Input exceeds its size budget")
	}
	return b, err
}

func OpenImage(path, expected string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, Fail("image-unavailable", "The input image is not accessible")
	}
	valid := false
	defer func() {
		if !valid {
			f.Close()
		}
	}()
	i, err := f.Stat()
	if err != nil || !i.Mode().IsRegular() || i.Size() <= 0 || i.Size() > MaxImage {
		return nil, Fail("invalid-image", "Image must be a regular file no larger than 30 MiB")
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, MaxImage+1))
	if err != nil || n > MaxImage {
		return nil, Fail("invalid-image", "Image read exceeded its budget")
	}
	got := hex.EncodeToString(h.Sum(nil))
	if expected != "" && got != expected {
		return nil, Fail("image-changed", "Image bytes no longer match the document")
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	valid = true
	return f, nil
}

// Produce stages all output in the caller's directory. The metadata file is the
// commit marker and is published last. Existing files are never overwritten.
func Produce(ctx context.Context, request Request, input, output string, call NativeCall) (Document, error) {
	var zero Document
	if request.Operation == "annotate" && request.Document != nil && request.Document.BaseDocument != nil {
		return zero, Fail("editable-document", "Use edit to change existing marks, or import the derived image as a new base")
	}
	if !strings.EqualFold(filepath.Ext(output), ".png") {
		return zero, Fail("invalid-output", "Choose an explicit .png output in your task directory")
	}
	output, err := filepath.Abs(output)
	if err != nil {
		return zero, err
	}
	parent := filepath.Dir(output)
	info, err := os.Stat(parent)
	if err != nil || !info.IsDir() {
		return zero, Fail("output-directory-unavailable", "Create the selected task output directory first")
	}
	stem := strings.TrimSuffix(output, filepath.Ext(output))
	names := []string{output, stem + ".preview.png", stem + ".capture.json"}
	for _, p := range names {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			return zero, Fail("output-exists", "An output already exists; choose another filename")
		}
	}
	var files []*os.File
	var temporary []string
	defer func() {
		for _, f := range files {
			f.Close()
		}
		for _, p := range temporary {
			_ = os.Remove(p)
		}
	}()
	for range 2 {
		f, err := os.CreateTemp(parent, ".gtmux-shot-")
		if err != nil {
			return zero, err
		}
		files = append(files, f)
		temporary = append(temporary, f.Name())
	}
	var descriptors []*os.File
	if input != "" {
		f, err := OpenImage(input, request.ImageSHA)
		if err != nil {
			return zero, err
		}
		defer f.Close()
		descriptors = append(descriptors, f)
	}
	if input != "" && request.ImageSHA == "" {
		f := descriptors[0]
		h := sha256.New()
		if _, err = io.Copy(h, io.LimitReader(f, MaxImage+1)); err != nil {
			return zero, err
		}
		request.ImageSHA = hex.EncodeToString(h.Sum(nil))
		if _, err = f.Seek(0, io.SeekStart); err != nil {
			return zero, err
		}
	}
	descriptors = append(descriptors, files...)
	reply, err := call(ctx, request, descriptors)
	if err != nil {
		return zero, err
	}
	if ctx.Err() != nil {
		return zero, ctx.Err()
	}
	var doc Document
	if err = Decode(reply.Result, &doc); err != nil {
		return zero, err
	}
	if err = doc.Validate(); err != nil {
		return zero, err
	}
	var immutable []string
	for i, f := range files {
		info, err := f.Stat()
		if err != nil || info.Size() <= 0 || info.Size() > MaxImage {
			return zero, Fail("invalid-output", "Native output is missing or exceeds its budget")
		}
		if err = f.Sync(); err != nil {
			return zero, err
		}
		if _, err = f.Seek(0, io.SeekStart); err != nil {
			return zero, err
		}
		// Publish a CLI-owned copy, never the inode handed to a native producer.
		// A late native writer therefore cannot mutate a published artifact.
		copy, err := os.CreateTemp(parent, ".gtmux-shot-final-")
		if err != nil {
			return zero, err
		}
		temporary = append(temporary, copy.Name())
		immutable = append(immutable, copy.Name())
		h := sha256.New()
		n, err := io.Copy(io.MultiWriter(copy, h), io.LimitReader(f, MaxImage+1))
		if err == nil {
			err = copy.Sync()
		}
		closeErr := copy.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil || n > MaxImage {
			return zero, Fail("invalid-output", "Native output changed or exceeded its byte budget")
		}
		check, err := os.Open(copy.Name())
		if err != nil {
			return zero, err
		}
		config, format, decodeErr := image.DecodeConfig(check)
		check.Close()
		w, hgt := doc.PixelWidth, doc.PixelHeight
		if i == 1 {
			w, hgt = doc.PreviewWidth, doc.PreviewHeight
		}
		if decodeErr != nil || format != "png" || config.Width != w || config.Height != hgt {
			return zero, Fail("invalid-output", "Native PNG dimensions do not match the image metadata")
		}

		got := hex.EncodeToString(h.Sum(nil))
		if i == 0 {
			if got != doc.ImageSHA {
				return zero, Fail("image-changed", "Native output hash does not match its receipt")
			}
		} else if got != doc.PreviewSHA {
			return zero, Fail("preview-changed", "Native preview hash does not match its receipt")
		}
	}
	doc.ImagePath, doc.PreviewPath = names[0], names[1]
	if request.Operation == "annotate" && request.Document != nil && request.Spec != nil {
		base := *request.Document
		doc.BasePath, doc.BaseSHA = base.ImagePath, base.ImageSHA
		doc.BaseDocument = &base
		doc.Annotations = request.Spec
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return zero, err
	}
	if len(data)+1 > MaxJSON {
		return zero, Fail("json-too-large", "Editable metadata exceeds its size budget")
	}
	meta, err := os.CreateTemp(parent, ".gtmux-shot-")
	if err != nil {
		return zero, err
	}
	defer meta.Close()
	temporary = append(temporary, meta.Name())
	if _, err = meta.Write(append(data, '\n')); err != nil {
		return zero, err
	}
	if err = meta.Sync(); err != nil {
		return zero, err
	}
	immutable = append(immutable, meta.Name())
	var published []string
	rollback := func() {
		for i, p := range published {
			a, e1 := os.Stat(immutable[i])
			b, e2 := os.Lstat(p)
			if e1 == nil && e2 == nil && os.SameFile(a, b) {
				_ = os.Remove(p)
			}
		}
	}
	for i, p := range names {
		if ctx.Err() != nil {
			rollback()
			return zero, ctx.Err()
		}
		if err = os.Link(immutable[i], p); err != nil {
			rollback()
			return zero, Fail("output-publication-failed", "Could not publish outputs without overwriting existing files")
		}
		published = append(published, p)
	}
	if ctx.Err() != nil {
		rollback()
		return zero, ctx.Err()
	}
	return doc, nil
}
