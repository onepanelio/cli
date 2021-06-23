package config

// ManifestVar represents a variable from the manifest
type ManifestVar struct {
	Required bool   `yaml:"required"`
	Value    string `yaml:"default"`
}

// RequiredManifestVar returns a ManifestVar that is required with the input value
func RequiredManifestVar(val string) *ManifestVar {
	return &ManifestVar{
		Required: true,
		Value:    val,
	}
}

// Database represents the configuration available for the database
type Database struct {
	Host         *ManifestVar
	Username     *ManifestVar
	Password     *ManifestVar
	Port         *ManifestVar
	DatabaseName *ManifestVar `yaml:"databaseName"`
	DriverName   *ManifestVar `yaml:"driverName"`
}

// DatabaseWrapper is a utility for loading the database key from yaml
type DatabaseWrapper struct {
	Database *Database `yaml:"database"`
}
