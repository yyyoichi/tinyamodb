package tinyamodb

type Config struct {
	Partition struct {
		Num        uint8
		IndexDgree int
	}
	Segment struct {
		MaxStoreBytes uint64
		MaxIndexBytes uint64
	}
	Table struct {
		PartitionKey string
		SortKey      string
	}
}
