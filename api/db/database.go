package db

import (
	"context"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/sirupsen/logrus"
)

// Database -
func Database() *ConDB {
	return &ConDB{}
}

// Start -
func (d *ConDB) Start(ctx context.Context, config *configs.Config) error {
	logrus.Info("start db client...")
	return CreateDBManager(config.APIConfig)
}

// CloseHandle -
func (d *ConDB) CloseHandle() {
}
