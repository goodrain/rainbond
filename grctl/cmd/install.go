package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

//NewCmdInstall -
func NewCmdInstall() cli.Command {
	c := cli.Command{
		Name:   "install",
		Hidden: true,
		Usage:  "grctl install",
		Action: func(c *cli.Context) error {
			//step finally: listen Signal
			term := make(chan os.Signal)
			signal.Notify(term, os.Interrupt, syscall.SIGTERM)
			select {
			case s := <-term:
				logrus.Infof("Received a Signal  %s, exiting gracefully...", s.String())
			}
			logrus.Info("See you next time!")
			return nil
		},
	}
	return c
}
