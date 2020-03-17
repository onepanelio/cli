package files

type ConfigVar struct {
	Required bool    `yaml:"required"`
	Default  *string `yaml:"default"`
}

func (c *ConfigVar) HasDefault() bool {
	return c.Default != nil
}

type ComponentConfigVar struct {
	ComponantPath string
	ConfigVar     *ConfigVar
}
