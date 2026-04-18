package configbundle

import "fmt"

func extractArchive(path, dest string, format Format) error {
	switch format {
	case FormatZip:
		return extractZip(path, dest)
	case FormatTarGz:
		return extractTarGz(path, dest)
	default:
		return fmt.Errorf("configbundle: unsupported format %s", format)
	}
}
