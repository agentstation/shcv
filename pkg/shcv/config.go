package shcv

// config configures the behavior of Chart processing.
// It allows customization of file locations and default values.
type config struct {
	// ValuesFileName is the name of the values file to use (default: "values.yaml")
	ValuesFileName []string
	// TemplatesDir is the name of the templates directory (default: "templates")
	TemplatesDir string
	// Verbose indicates whether to print verbose messages
	Verbose bool
}

// newConfig creates a new config with the default options.
func newConfig(opts []Option) *config {
	c := defaultConfig()
	c.apply(opts)
	return c
}

// defaultConfig returns the default configuration options for Chart processing.
// This includes standard file locations and common default values.
func defaultConfig() *config {
	return &config{
		ValuesFileName: []string{"values.yaml"},
		TemplatesDir:   "templates",
		Verbose:        false,
	}
}

// apply applies the given options to the config.
func (c *config) apply(opts []Option) {
	for _, option := range opts {
		option(c)
	}
}

// Option is a functional option for configuring the Chart processing.
type Option func(*config)

// WithValuesFileNames sets the values file names.
func WithValuesFileNames(names []string) Option {
	return func(c *config) {
		c.ValuesFileName = append(c.ValuesFileName, names...)
	}
}

// WithTemplatesDir sets the templates directory.
func WithTemplatesDir(dir string) Option {
	return func(c *config) {
		c.TemplatesDir = dir
	}
}

// WithVerbose sets the verbose flag.
func WithVerbose(verbose bool) Option {
	return func(c *config) {
		c.Verbose = verbose
	}
}
