package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// encodeLiveCapture stores the real screenshot crop losslessly without spending
// test time compressing it. Deterministic bytes preserve the capture comparisons.
func encodeLiveCapture(crop *image.NRGBA) ([]byte, error) {
	var result bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.NoCompression}
	if err := encoder.Encode(&result, crop); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

func TestEncodeLiveCapture(t *testing.T) {
	for _, tc := range []struct {
		name   string
		pixels []color.NRGBA
	}{
		{"opaque", []color.NRGBA{
			{0, 0, 0, 255}, {255, 255, 255, 255}, {1, 2, 3, 255},
			{254, 128, 64, 255}, {17, 33, 65, 255}, {255, 0, 127, 255},
		}},
		{"translucent", []color.NRGBA{
			{255, 128, 64, 1}, {3, 250, 129, 64}, {1, 2, 3, 127},
			{254, 128, 64, 128}, {17, 33, 65, 200}, {255, 0, 127, 254},
		}},
		{"transparent", []color.NRGBA{
			{0, 0, 0, 0}, {255, 255, 255, 0}, {1, 2, 3, 0},
			{254, 128, 64, 0}, {17, 33, 65, 0}, {255, 0, 127, 0},
		}},
		{"mixed_alpha", []color.NRGBA{
			{255, 128, 64, 0}, {3, 250, 129, 1}, {1, 2, 3, 127},
			{254, 128, 64, 128}, {17, 33, 65, 254}, {255, 0, 127, 255},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			crop := image.NewNRGBA(image.Rect(0, 0, 3, 2))
			for i, pixel := range tc.pixels {
				crop.SetNRGBA(i%3, i/3, pixel)
			}
			before := bytes.Clone(crop.Pix)
			encoded, err := encodeLiveCapture(crop)
			if err != nil {
				t.Fatal(err)
			}
			checkLiveCapturePixels(t, encoded, crop)
			repeated, err := encodeLiveCapture(crop)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(encoded, repeated) {
				t.Fatal("repeated capture encoding changed bytes")
			}
			identical := image.NewNRGBA(crop.Bounds())
			copy(identical.Pix, crop.Pix)
			same, err := encodeLiveCapture(identical)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(encoded, same) {
				t.Fatal("equal capture pixels produced different bytes")
			}
			for channel := 0; channel < 4; channel++ {
				changed := image.NewNRGBA(crop.Bounds())
				copy(changed.Pix, crop.Pix)
				changed.Pix[4+channel] ^= 1
				different, err := encodeLiveCapture(changed)
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Equal(encoded, different) {
					t.Fatalf("capture channel %d changed without changing bytes", channel)
				}
				checkLiveCapturePixels(t, different, changed)
			}
			if !bytes.Equal(crop.Pix, before) {
				t.Fatal("capture encoding changed source pixels")
			}
		})
	}
}

func checkLiveCapturePixels(t *testing.T, encoded []byte, want *image.NRGBA) {
	t.Helper()
	decoded, err := png.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != want.Bounds() {
		t.Fatalf("capture bounds=%v want=%v", decoded.Bounds(), want.Bounds())
	}
	for y := want.Bounds().Min.Y; y < want.Bounds().Max.Y; y++ {
		for x := want.Bounds().Min.X; x < want.Bounds().Max.X; x++ {
			got := color.NRGBAModel.Convert(decoded.At(x, y)).(color.NRGBA)
			if expected := want.NRGBAAt(x, y); got != expected {
				t.Fatalf("capture pixel (%d,%d)=%v want=%v", x, y, got, expected)
			}
		}
	}
}
