package webcli

import (
	"context"
	"github.com/goodrain/rainbond/api/webcli/app"
	"github.com/goodrain/rainbond/cmd/webcli/option"
	"github.com/goodrain/rainbond/pkg/gogo"
	"github.com/spf13/pflag"
	"strconv"

	"github.com/goodrain/rainbond/config/configs"
)

var defaultWebCliComponent *Component

// Component -
type Component struct {
	app *app.App
}

func (c *Component) Start(ctx context.Context, cfg *configs.Config) error {
	_ = gogo.Go(func(ctx context.Context) error {
		s := option.NewWebCliServer()
		s.AddFlags(pflag.CommandLine)
		pflag.Parse()
		s.SetLog()
		option := app.DefaultOptions
		option.Address = s.Address
		option.Port = strconv.Itoa(s.Port)
		option.SessionKey = s.SessionKey
		option.K8SConfPath = s.K8SConfPath
		ap, err := app.New(&option)
		c.app = ap
		if err != nil {
			return err
		}
		return ap.Run()
	})
	return nil
}

func (c *Component) CloseHandle() {
	c.app.Exit()
}

// New -
func New() *Component {
	defaultWebCliComponent = &Component{}
	return defaultWebCliComponent
}

// Default -
func Default() *Component {
	return defaultWebCliComponent
}
