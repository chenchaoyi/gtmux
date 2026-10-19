package docs

import (
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Read actual shipped pixels, not a repeated list of the intended colours. A
// translucent white mark on the light launch ground must fail this regression.
func TestMobileLaunchMarkContrastAndSharedSource(t *testing.T) {
	root := filepath.Join("..", "..", "mobileapp")
	for _, suffix := range []string{"", "@2x", "@3x"} {
		file := filepath.Join(root, "ios", "GtmuxMobile", "Images.xcassets", "LaunchMark.imageset", "launch-mark"+suffix+".png")
		f, err := os.Open(file)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		scale := img.Bounds().Dx() / 112
		if scale < 1 || img.Bounds().Dy() != img.Bounds().Dx() {
			t.Fatal("launch mark stretched", img.Bounds())
		}
		for _, ground := range []color.NRGBA{{242, 242, 247, 255}, {13, 13, 15, 255}} {
			pixel := func(x, y int) color.NRGBA {
				c := color.NRGBAModel.Convert(img.At(x*scale, y*scale)).(color.NRGBA)
				a := float64(c.A) / 255
				return color.NRGBA{uint8(float64(c.R)*a + float64(ground.R)*(1-a)), uint8(float64(c.G)*a + float64(ground.G)*(1-a)), uint8(float64(c.B)*a + float64(ground.B)*(1-a)), 255}
			}
			plate := pixel(56, 12)
			for _, point := range [][2]int{{36, 36}, {76, 36}, {56, 76}} {
				if ratio := launchContrast(pixel(point[0], point[1]), plate); ratio < 3 {
					t.Fatalf("%s pane contrast %.2f on ground %v", suffix, ratio, ground)
				}
			}
			if p := pixel(76, 36); p.R != 6 || p.G != 182 || p.B != 212 {
				t.Fatal("brand cyan changed", p)
			}
		}
	}
	story, err := os.ReadFile(filepath.Join(root, "ios", "GtmuxMobile", "LaunchScreen.storyboard"))
	if err != nil {
		t.Fatal(err)
	}
	component, err := os.ReadFile(filepath.Join(root, "src", "ui", "LaunchMark.tsx"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(story), `image="LaunchMark"`) || !strings.Contains(string(component), "LaunchMark.imageset/launch-mark.png") {
		t.Fatal("native and RN no longer share the shipped artwork")
	}
}

func launchContrast(a, b color.NRGBA) float64 {
	luminance := func(c color.NRGBA) float64 {
		out := 0.0
		for i, v := range []uint8{c.R, c.G, c.B} {
			x := float64(v) / 255
			if x <= 0.04045 {
				x /= 12.92
			} else {
				x = math.Pow((x+0.055)/1.055, 2.4)
			}
			out += x * []float64{0.2126, 0.7152, 0.0722}[i]
		}
		return out
	}
	x, y := luminance(a), luminance(b)
	return (math.Max(x, y) + 0.05) / (math.Min(x, y) + 0.05)
}
