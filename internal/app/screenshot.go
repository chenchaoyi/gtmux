package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/screenshot"
)

var screenshotNative screenshot.NativeCall = screenshot.Native

const screenshotHelp = `gtmux screenshot <operation> [--json]
  capabilities                         Query the running Mac host; never opens permissions
  sources --app <bundle-id> | --displays List explicit capture sources (30-second references)
  capture --source-ref REF [--rect x,y,w,h --unit pt] --output FILE.png
  import --input FILE --output FILE.png Convert an existing image without live provenance
  annotate --capture FILE.capture.json --spec MARKS.json --output FILE.png
  edit --document FILE.capture.json    Explicitly open the existing native editor
  target --pane %N [--session-id SID]  Resolve an exact supported live conversation
  send --image FILE --target-ref REF --request-id ID [--message-file FILE]
  status ID                           Read the last durable receipt, not agent completion

Native operations require a compatible running macOS App. No MCP is required.
Capture/annotation never send implicitly. Outputs stay in your chosen task directory.
Send refuses drafts, approvals, changed targets and unknown delivery retries.
Target references currently require a verified managed Codex or DSH native binding;
other agents keep the existing explicit gtmux send --attach interface.
Every operation emits versioned JSON; --timeout defaults to 10s (maximum 10s).
`

func screenshotOutput(result any, err error) int {
	reply := struct {
		Version int               `json:"schema_version"`
		OK      bool              `json:"ok"`
		Result  any               `json:"result,omitempty"`
		Error   *screenshot.Error `json:"error,omitempty"`
	}{Version: screenshot.Version, OK: err == nil, Result: result}
	if err != nil {
		if receipt, ok := result.(screenshot.Receipt); !ok || receipt.ID == "" {
			reply.Result = nil
		}
		reply.Error = screenshot.AsError(err)
	}
	_ = json.NewEncoder(os.Stdout).Encode(reply)
	if err != nil {
		return 1
	}
	return 0
}

func cmdScreenshot(args []string) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(screenshotHelp)
		return 0
	}
	op := args[0]
	fs := flag.NewFlagSet("screenshot "+op, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	_ = fs.Bool("json", true, "machine output")
	timeout := fs.Duration("timeout", 10*time.Second, "overall request timeout")
	var app, ref, rect, unit, output, input, document, specPath, pane, session, target, requestID, noteFile string
	var displays bool
	switch op {
	case "capabilities":
	case "sources":
		fs.StringVar(&app, "app", "", "app bundle identifier")
		fs.BoolVar(&displays, "displays", false, "explicitly list displays")
	case "capture":
		fs.StringVar(&ref, "source-ref", "", "source reference")
		fs.StringVar(&rect, "rect", "", "source-local region")
		fs.StringVar(&unit, "unit", "pt", "region units")
		fs.StringVar(&output, "output", "", "output PNG")
	case "import":
		fs.StringVar(&input, "input", "", "input image")
		fs.StringVar(&output, "output", "", "output PNG")
	case "annotate":
		fs.StringVar(&document, "capture", "", "capture document")
		fs.StringVar(&specPath, "spec", "", "annotation JSON")
		fs.StringVar(&output, "output", "", "output PNG")
	case "edit":
		fs.StringVar(&document, "document", "", "editable document")
	case "target":
		fs.StringVar(&pane, "pane", "", "exact pane")
		fs.StringVar(&session, "session-id", "", "expected session")
	case "send":
		fs.StringVar(&input, "image", "", "image")
		fs.StringVar(&target, "target-ref", "", "exact target")
		fs.StringVar(&requestID, "request-id", "", "stable request ID")
		fs.StringVar(&noteFile, "message-file", "", "note file")
	case "status":
	default:
		return screenshotOutput(nil, screenshot.Fail("unknown-operation", "Use gtmux screenshot --help"))
	}
	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			fmt.Print(screenshotHelp)
			return 0
		}
		return screenshotOutput(nil, screenshot.Fail("invalid-options", "Check this operation's flags"))
	}
	if *timeout <= 0 || *timeout > 10*time.Second {
		return screenshotOutput(nil, screenshot.Fail("invalid-options", "Use 0 < timeout <= 10s"))
	}
	if fs.NArg() != 0 && op != "status" {
		return screenshotOutput(nil, screenshot.Fail("invalid-options", "Unexpected positional arguments"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if op == "target" {
		result, err := screenshotResolveTarget(ctx, pane, session)
		return screenshotOutput(result, err)
	}
	if op == "send" {
		result, err := screenshotSend(ctx, input, target, requestID, noteFile)
		return screenshotOutput(result, err)
	}
	if op == "status" {
		if fs.NArg() != 1 {
			return screenshotOutput(nil, screenshot.Fail("request-required", "Provide the request ID"))
		}
		r, err := screenshot.ReadReceipt(fs.Arg(0))
		return screenshotOutput(r, err)
	}
	req := screenshot.Request{Operation: op, App: app, SourceRef: ref}
	if op == "sources" {
		if (app == "") == (!displays) {
			return screenshotOutput(nil, screenshot.Fail("source-required", "Choose --app or --displays"))
		}
		if displays {
			req.SourceKind = "display"
		}
	}
	if op == "capture" {
		if ref == "" || unit != "pt" {
			return screenshotOutput(nil, screenshot.Fail("source-required", "Choose a source reference and pt coordinates"))
		}
		if rect != "" {
			for _, v := range strings.Split(rect, ",") {
				n, err := strconv.ParseFloat(v, 64)
				if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
					return screenshotOutput(nil, screenshot.Fail("invalid-region", "Use x,y,width,height in source-local points"))
				}
				req.Rect = append(req.Rect, n)
			}
			if len(req.Rect) != 4 || req.Rect[0] < 0 || req.Rect[1] < 0 || req.Rect[2] <= 0 || req.Rect[3] <= 0 {
				return screenshotOutput(nil, screenshot.Fail("invalid-region", "Expected a positive source-local region"))
			}
		}
	}
	if op == "import" && input == "" {
		return screenshotOutput(nil, screenshot.Fail("image-required", "Choose an input image"))
	}
	if op == "annotate" || op == "edit" {
		doc, err := screenshot.ReadDocument(document)
		if err != nil {
			return screenshotOutput(nil, err)
		}
		var spec screenshot.Spec
		if op == "edit" && doc.BaseDocument != nil && doc.Annotations != nil {
			spec = *doc.Annotations
			doc = *doc.BaseDocument
		} else if op == "edit" {
			spec = screenshot.Spec{Version: 1, CaptureID: doc.CaptureID, ImageSHA: doc.ImageSHA, Space: "image_px", Marks: []screenshot.Mark{}}
		} else {
			spec, err = screenshot.ReadSpec(specPath)
			if err != nil {
				return screenshotOutput(nil, err)
			}
		}
		if err = spec.Validate(doc); err != nil {
			return screenshotOutput(nil, err)
		}
		if spec.Space == "preview_px" {
			preview, err := screenshot.OpenImage(doc.PreviewPath, doc.PreviewSHA)
			if err != nil {
				return screenshotOutput(nil, err)
			}
			preview.Close()
		}
		req.Document, req.Spec, req.ImageSHA = &doc, &spec, doc.ImageSHA
		input = doc.ImagePath
		if op == "edit" {
			f, err := screenshot.OpenImage(input, doc.ImageSHA)
			if err != nil {
				return screenshotOutput(nil, err)
			}
			defer f.Close()
			reply, err := screenshotNative(ctx, req, []*os.File{f})
			return screenshotOutput(reply.Result, err)
		}
	}
	if op == "capabilities" || op == "sources" {
		reply, err := screenshotNative(ctx, req, nil)
		if op == "capabilities" && err != nil {
			return screenshotOutput(map[string]any{"capture_available": false, "annotation_available": false, "reason": screenshot.AsError(err).Code, "mcp_required": false}, nil)
		}
		return screenshotOutput(reply.Result, err)
	}
	doc, err := screenshot.Produce(ctx, req, input, output, screenshotNative)
	if err != nil {
		return screenshotOutput(nil, err)
	}
	meta := strings.TrimSuffix(doc.ImagePath, filepath.Ext(doc.ImagePath)) + ".capture.json"
	return screenshotOutput(map[string]any{"document_path": meta, "document": doc}, nil)
}
