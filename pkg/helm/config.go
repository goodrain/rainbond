package helm

import "path"

type Config struct {
	helmCache string
}

func (c *Config) RepoFile() string {
	return path.Join(c.helmCache, "/repository/repositories.yaml")
}

func (c *Config) RepoCache() string {
	return path.Join(c.helmCache, "cache")
}

func (c *Config) ChartCache() string {
	return path.Join(c.helmCache, "chart")
}
