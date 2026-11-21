package app

import "strings"

// ForgetPrefix removes decoded values; any later load must pass source authorization again.
func (c *asyncCache[V]) ForgetPrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	kept := c.order[:0]
	for _, key := range c.order {
		if strings.HasPrefix(key, prefix) {
			delete(c.values, key)
		} else {
			kept = append(kept, key)
		}
	}
	c.order = kept
}

func (s *fileService) forgetGridPicture(path string) {
	if s.gridPictures != nil {
		s.gridPictures.ForgetPrefix(path + "\x00")
	}
}
