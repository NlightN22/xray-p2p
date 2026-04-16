//go:build !linux && !windows

package server

func ListReverse(ReverseListOptions) ([]ReverseRecord, error) {
	return nil, ErrUnsupported
}
