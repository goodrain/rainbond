package parseConfig

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strings"
)

func New(file string) Config {
	return Config{file: file}
}

type Config struct {
	file string
	maps map[string]interface{}
}

func (c *Config) Get(name string) interface{} {

	if c.maps == nil {
		c.read()
	}

	if c.maps == nil {
		return nil
	}

	keys := strings.Split(name, " > ")
	l := len(keys)
	if l == 1 {
		return c.maps[name]
	}

	var ret interface{}
	for i := 0; i < l; i++ {
		if i == 0 {
			ret = c.maps[keys[i]]
			if ret == nil {
				return nil
			}
		} else {
			if m, ok := ret.(map[string]interface{}); ok {
				ret = m[keys[i]]
			} else {
				if l == i-1 {
					return ret
				}
				return nil
			}
		}
	}
	return ret
}

func (c *Config) read() {
	if !filepath.IsAbs(c.file) {
		file, err := filepath.Abs(c.file)
		if err != nil {
			panic(err)
		}
		c.file = file
	}

	bts, err := ioutil.ReadFile(c.file)

	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(bts, &c.maps)

	if err != nil {
		panic(err)
	}
}
