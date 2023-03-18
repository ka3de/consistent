package consistent

import "hash/crc32"

type CRCHasher struct{}

func NewCRCHasher() *CRCHasher {
	return &CRCHasher{}
}

func (h *CRCHasher) Hash(key string) Hash {
	return Hash(crc32.ChecksumIEEE([]byte(key)))
}
