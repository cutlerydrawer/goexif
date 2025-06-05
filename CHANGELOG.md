# Changelog

## 1.1.0 - 2025-06-05

### Changes

- Updated deprecated methods
- Minor code style improvements

## 1.0.0 - 2020-10-07

_First release after forking from [rwcarlsen/goexif](https://github.com/rwcarlsen/goexif)._

### Bug fixes

- Fix tag count overflow causing huge memory allocs.
- Fix out-of-bounds slice read in `mknote`
- Fix hang on recursive IFDs

### Changes

- Enable go modules
- Add basic support for ORF files
- Export exif.NotFoundError