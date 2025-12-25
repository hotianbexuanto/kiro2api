package service

import "io"

type closeFuncReadCloser struct {
	io.ReadCloser
	onClose func()
}

func (c *closeFuncReadCloser) Close() error {
	err := c.ReadCloser.Close()
	if c.onClose != nil {
		c.onClose()
	}
	return err
}
