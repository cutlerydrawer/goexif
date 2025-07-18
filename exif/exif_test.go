package exif

//go:generate go run regen_regress.go -- regress_expected_test.go
//go:generate go fmt regress_expected_test.go

import (
	"bytes"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/simpicapp/goexif/tiff"
)

var dataDir = flag.String("test_data_dir", ".", "Directory where the data files for testing are located")

func TestDecode(t *testing.T) {
	fpath := filepath.Join(*dataDir, "samples")
	f, err := os.Open(fpath)
	if err != nil {
		t.Fatalf("Could not open sample directory '%s': %v", fpath, err)
	}

	names, err := f.Readdirnames(0)
	if err != nil {
		t.Fatalf("Could not read sample directory '%s': %v", fpath, err)
	}

	cnt := 0
	for _, name := range names {
		t.Logf("testing file %v", name)
		f, err := os.Open(filepath.Join(fpath, name))
		if err != nil {
			t.Fatal(err)
		}

		x, err := Decode(f)
		if err != nil {
			t.Fatal(err)
		} else if x == nil {
			t.Fatalf("No error and yet %v was not decoded", name)
		}

		t.Logf("checking pic %v", name)
		x.Walk(&walker{name, t})
		cnt++
	}
	if cnt != len(regressExpected) {
		t.Errorf("Did not process enough samples, got %d, want %d", cnt, len(regressExpected))
	}
}

func TestDecodeRawEXIF(t *testing.T) {
	rawFile := filepath.Join(*dataDir, "samples", "raw.exif")
	raw, err := os.ReadFile(rawFile)
	if err != nil {
		t.Fatal(err)
	}
	x, err := Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	got := map[string]string{}
	err = x.Walk(walkFunc(func(name FieldName, tag *tiff.Tag) error {
		got[fmt.Sprint(name)] = fmt.Sprint(tag)
		return nil
	}))
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	// The main part of this test is that Decode can parse an
	// "Exif\x00\x00" body, but also briefly sanity check the
	// parsed results:
	if len(got) < 42 {
		t.Errorf("Got %d tags. Want at least 42. Got: %v", len(got), err)
	}
	if g, w := got["DateTime"], `"2018:04:03 07:54:33"`; g != w {
		t.Errorf("DateTime value = %q; want %q", g, w)
	}
}

type walkFunc func(FieldName, *tiff.Tag) error

func (f walkFunc) Walk(name FieldName, tag *tiff.Tag) error {
	return f(name, tag)
}

type walker struct {
	picName string
	t       *testing.T
}

func (w *walker) Walk(field FieldName, tag *tiff.Tag) error {
	// this needs to be commented out when regenerating regress expected vals
	pic := regressExpected[w.picName]
	if pic == nil {
		w.t.Errorf("   regression data not found")
		return nil
	}

	exp, ok := pic[field]
	if !ok {
		w.t.Errorf("   regression data does not have field %v", field)
		return nil
	}

	s := tag.String()
	if tag.Count == 1 && s != "\"\"" {
		s = fmt.Sprintf("[%s]", s)
	}
	got := tag.String()

	if exp != got {
		fmt.Println("s: ", s)
		fmt.Printf("len(s)=%v\n", len(s))
		w.t.Errorf("   field %v bad tag: expected '%s', got '%s'", field, exp, got)
	}
	return nil
}

func TestMarshal(t *testing.T) {
	name := filepath.Join(*dataDir, "sample1.jpg")
	f, err := os.Open(name)
	if err != nil {
		t.Fatalf("%v\n", err)
	}
	defer f.Close()

	x, err := Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if x == nil {
		t.Fatal("bad err")
	}

	b, err := x.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%s", b)
}

func testSingleParseDegreesString(t *testing.T, s string, w float64) {
	g, err := parseTagDegreesString(s)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(w-g) > 1e-10 {
		t.Errorf("Wrong parsing result %s: Want %.12f, got %.12f", s, w, g)
	}
}

func TestParseTagDegreesString(t *testing.T) {
	// semicolon as decimal mark
	testSingleParseDegreesString(t, "52,00000,50,00000,34,01180", 52.842781055556) // comma as separator
	testSingleParseDegreesString(t, "52,00000;50,00000;34,01180", 52.842781055556) // semicolon as separator

	// point as decimal mark
	testSingleParseDegreesString(t, "14.00000,44.00000,34.01180", 14.742781055556) // comma as separator
	testSingleParseDegreesString(t, "14.00000;44.00000;34.01180", 14.742781055556) // semicolon as separator
	testSingleParseDegreesString(t, "14.00000;44.00000,34.01180", 14.742781055556) // mixed separators

	testSingleParseDegreesString(t, "-008.0,30.0,03.6", -8.501) // leading zeros

	// no decimal places
	testSingleParseDegreesString(t, "-10,15,54", -10.265)
	testSingleParseDegreesString(t, "-10;15;54", -10.265)

	// incorrect mix of comma and point as decimal mark
	s := "-17,00000,15.00000,04.80000"
	if _, err := parseTagDegreesString(s); err == nil {
		t.Error("parseTagDegreesString: false positive for " + s)
	}
}

// Make sure we error out early when a tag had a count of MaxUint32
func TestMaxUint32CountError(t *testing.T) {
	name := filepath.Join(*dataDir, "corrupt/max_uint32_exif.jpg")
	f, err := os.Open(name)
	if err != nil {
		t.Fatalf("%v\n", err)
	}
	defer f.Close()

	_, err = Decode(f)
	if err == nil {
		t.Fatal("no error on bad exif data")
	}
	if !strings.Contains(err.Error(), "invalid Count offset") {
		t.Fatal("wrong error:", err.Error())
	}
}

// Make sure we error out early with tag data sizes larger than the image file
func TestHugeTagError(t *testing.T) {
	name := filepath.Join(*dataDir, "corrupt/huge_tag_exif.jpg")
	f, err := os.Open(name)
	if err != nil {
		t.Fatalf("%v\n", err)
	}
	defer f.Close()

	_, err = Decode(f)
	if err == nil {
		t.Fatal("no error on bad exif data")
	}
	if !strings.Contains(err.Error(), "integer overflow") {
		t.Fatal("wrong error:", err.Error())
	}
}

// Contains a very high tag count that will overflow a uint32 when multiplied by the type size
func TestTagCountOverflowError(t *testing.T) {
	const testData = "II*\x00\x10\x00\x00\x00000000000000\x03\x00\x01\x00\x00\x800000"

	_, err := Decode(bytes.NewReader([]byte(testData)))
	if err == nil {
		t.Fatalf("no error on bad exif data")
	}
	if !strings.Contains(err.Error(), "integer overflow") {
		t.Fatal("wrong error:", err.Error())
	}
}

// Check for a 0-length tag value
func TestZeroLengthTagError(t *testing.T) {
	name := filepath.Join(*dataDir, "corrupt/infinite_loop_exif.jpg")
	f, err := os.Open(name)
	if err != nil {
		t.Fatalf("%v\n", err)
	}
	defer f.Close()

	_, err = Decode(f)
	if err == nil {
		t.Fatal("no error on bad exif data")
	}
	if !strings.Contains(err.Error(), "zero length tag value") {
		t.Fatal("wrong error:", err.Error())
	}
}

// Check for a 0-length tag value
func TestRecursiveIfdsError(t *testing.T) {
	name := filepath.Join(*dataDir, "corrupt/recursive_ifds.dat")
	f, err := os.Open(name)
	if err != nil {
		t.Fatalf("%v\n", err)
	}
	defer f.Close()

	_, err = Decode(f)
	if err == nil {
		t.Fatal("no error on bad exif data")
	}
	if !strings.Contains(err.Error(), "recursive IFD") {
		t.Fatal("wrong error:", err.Error())
	}
}
