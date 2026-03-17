//go:build !linux

package health

func readDiskUsageOS(path string) (free, total int64) {
	return 0, 0
}
