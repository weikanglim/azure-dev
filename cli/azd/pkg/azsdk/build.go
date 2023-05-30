//go:build !record

package azsdk

func NewClientOptionsBuilder() *ClientOptionsBuilder {
	return &ClientOptionsBuilder{}
}
