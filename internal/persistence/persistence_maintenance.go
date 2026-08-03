// Purpose: Calculates project storage size and enforces size-based eviction of oldest namespaces.
// Why: Separates storage maintenance and eviction from CRUD and initialization.
package persistence

import (
	"os"
)

func (s *SessionStore) projectSize() (int64, error) {
	var total int64
	err := s.filesystem().Walk(s.projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}
