package skynet

const (
	// KiB kilobyte
	KiB = 1024
	// MiB megabyte
	MiB = 1024 * KiB
	// GiB gigabyte
	GiB = 1024 * MiB
	// TiB terabyte
	TiB = 1024 * GiB

	// SizeBaseSector is the size of a base sector.
	SizeBaseSector = 4 * MiB
	// SizeChunk is the size of a chunk.
	SizeChunk = 40 * MiB

	// RedundancyBaseSector describes the base sector redundancy the portal is
	// using. This is not freely configurable because we need database
	// consistency.
	RedundancyBaseSector = 10
	// RedundancyChunk describes the redundancy of regular chunks the
	// portal is using. This is not freely configurable because we need database
	// consistency.
	RedundancyChunk = 3

	// CostBandwidthRegistryWrite the bandwidth cost of a single registry write
	CostBandwidthRegistryWrite = 5 * MiB
	// CostBandwidthRegistryRead the bandwidth cost of a single registry read
	CostBandwidthRegistryRead = MiB

	// CostBandwidthUploadBase is the baseline bandwidth price for each upload.
	// This is the cost of uploading the base sector.
	CostBandwidthUploadBase = 40 * MiB
	// CostBandwidthUploadIncrement is the bandwidth price per 40MB uploaded
	// data, beyond the base sector (beyond the first 4MB). Rounded up.
	CostBandwidthUploadIncrement = 120 * MiB
	// CostBandwidthDownloadBase is the baseline bandwidth price for each Download.
	CostBandwidthDownloadBase = 200 * KiB
	// CostBandwidthDownloadIncrement is the bandwidth price per 64B. Rounded up.
	CostBandwidthDownloadIncrement = 64

	// CostStorageUploadBase is the baseline storage price for each upload.
	// This is the cost of uploading the base sector.
	CostStorageUploadBase = 4 * MiB
	// CostStorageUploadIncrement is the storage price for each 40MB beyond
	// the base sector (beyond the first 4MB). Rounded up.
	CostStorageUploadIncrement = 40 * MiB
)

// BandwidthUploadCost calculates the bandwidth cost of an upload with the given
// size. The base sector is uploaded with 10x redundancy. Each chunk is uploaded
// with 3x redundancy.
func BandwidthUploadCost(size int64) int64 {
	return CostBandwidthUploadBase + numChunks(size)*CostBandwidthUploadIncrement
}

// BandwidthDownloadCost calculates the bandwidth cost of a download with the
// given size.
func BandwidthDownloadCost(size int64) int64 {
	chunks := size / 64
	if size%64 > 0 {
		chunks++
	}
	return CostBandwidthDownloadBase + chunks*CostBandwidthDownloadIncrement
}

// RawStorageUsed calculates how much storage an upload with a given size
// actually uses. This method returns the total underlying storage used and not
// the adjusted number users see. Users see adjusted numbers in order to
// shield them from the complexity of base/chunk redundancy.
func RawStorageUsed(uploadSize int64) int64 {
	baseSectorStorage := int64(CostStorageUploadBase * RedundancyBaseSector)
	chunkStorage := numChunks(uploadSize) * CostStorageUploadIncrement * RedundancyChunk
	return baseSectorStorage + chunkStorage
}

// numChunks returns the number of 40MB chunks a file of this size uses, beyond
// the 4MB in the base sector.
func numChunks(size int64) int64 {
	if size <= SizeBaseSector {
		return 0
	}
	chunksBeyondBase := (size - SizeBaseSector) / SizeChunk
	if (size-SizeBaseSector)%SizeChunk > 0 {
		chunksBeyondBase++
	}
	return chunksBeyondBase
}
