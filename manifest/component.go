package manifest

import (
	"fmt"
	"os"
	"strings"
)

type Component struct {
	path string
	overlays []*Overlay
}

func (c *Component) Path() string {
	return c.path
}

func (c *Component) PathWithBase() string {
	return c.path + string(os.PathSeparator) + "base"
}

func (c *Component) VarsFilePath() string {
	return fmt.Sprintf("%s%sbase%svars.yaml", c.path, string(os.PathSeparator), string(os.PathSeparator))
}

func (c *Component) Overlays() []*Overlay {
	return c.overlays
}

func CreateComponent(path string) *Component {
	component := &Component{
		path:     path,
		overlays: make([]*Overlay, 0),
	}

	return component
}

func (c *Component) AddOverlay(overlay *Overlay) {
	c.overlays = append(c.overlays, overlay)
}

func (c *Component) IsCommon() bool {
	return strings.HasPrefix(c.path, "common")
}