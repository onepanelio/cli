package manifest

import (
	"fmt"
	"os"
)

type Overlay struct {
	path string
	component *Component
}

func CreateOverlay(path string, component *Component) *Overlay {
	return &Overlay{
		path:      path,
		component: component,
	}
}

func (v *Overlay) Path() string {
	return v.path
}

func (v *Overlay) Component() *Component {
	return v.component
}

func (v *Overlay) VarsFilePath() string {
	return fmt.Sprintf("%s%svars.yaml", v.path, string(os.PathSeparator))
}