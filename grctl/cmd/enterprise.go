// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package cmd

import (
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/urfave/cli"

	"github.com/goodrain/rainbond/util/passwordutil"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

const (
	// TempMail temp email for encrypt and verify password
	TempMail = "operator@goodrain.com"
)

//NewCmdEnterprise -
func NewCmdEnterprise() cli.Command {
	c := cli.Command{
		Name:  "enterprise",
		Usage: "enterprise manage cmd",
		Subcommands: []cli.Command{
			{
				Name:  "admin",
				Usage: "administrator manage",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "database, db",
						Usage: "operator installation database, required",
					},
				},
				Subcommands: []cli.Command{
					{
						Name:   "info",
						Usage:  "show username and password of administrator",
						Action: adminInfo,
					},
					{
						Name:  "reset",
						Usage: "reset password of administrator",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "password",
								Usage: "new password of administrator, required",
							},
						},
						Action: resetAdminPassword,
					},
				},
			},
		},
	}
	return c
}

type enterprise struct {
	db     *gorm.DB
	dbPath string
	c      *cli.Context
	repo   *adminRepo
}

// openDB -
func (e *enterprise) openDB() error {
	if e.dbPath == "" {
		return fmt.Errorf("please specify database path!!")
	}

	// open database
	db, err := gorm.Open("sqlite3", e.dbPath)
	if err != nil {
		return fmt.Errorf("open database failed: %s", err.Error())
	}

	e.db = db

	return nil
}

// closeDB -
func (e *enterprise) closeDB() {
	if err := e.db.Close(); err != nil {
		showError(fmt.Sprintf("close database failed: %s", err.Error()))
	}
	return
}

func (e *enterprise) getAdmin() *admin {
	if err := e.openDB(); err != nil {
		showError(err.Error())
	}
	defer e.closeDB()

	e.repo = &adminRepo{db: e.db}

	adminInfo, err := e.repo.Get()
	if err != nil {
		showError(fmt.Sprintf("get administrator failed: %s", err.Error()))
	}
	if adminInfo == nil {
		showWarn(fmt.Sprintf("administrator has not been generated, maybe next time"))
	}
	return adminInfo
}

func (e *enterprise) validatePassword(password string) {
	// TODO password rule write here
	if password == "" {
		showError(fmt.Sprintf("please specify new password of administrator"))
	}
}

func (e *enterprise) resetAdminPassword() {
	newPassword := e.c.String("password")

	// validate password illegal
	e.validatePassword(newPassword)

	// prepare database
	if err := e.openDB(); err != nil {
		showError(err.Error())
	}
	defer e.closeDB()

	// prepare repo
	e.repo = &adminRepo{db: e.db}

	// use repo query admin
	adminInfo, err := e.repo.Get()
	if err != nil {
		showError(fmt.Sprintf("get administrator failed: %s", err.Error()))
	}
	if adminInfo == nil {
		showWarn(fmt.Sprintf("administrator has not been generated, maybe next time"))
	}

	// encrypt password
	password, err := passwordutil.EncryptionPassword(newPassword, TempMail)
	if err != nil {
		showError(fmt.Sprintf("encrypt password failed: %s", err.Error()))
	}

	// reset
	adminInfo.Password = password
	if err := e.repo.Update(adminInfo); err != nil {
		showError(fmt.Sprintf("reset administrator password failed: %s", err.Error()))
	}
}

func initEnterprise(c *cli.Context) *enterprise {
	dbPath := c.Parent().String("db")
	if dbPath == "" {
		showError(fmt.Sprintf("please specify database path!!"))
	}
	e := &enterprise{
		dbPath: dbPath,
		c:      c,
	}

	return e
}

func adminInfo(c *cli.Context) {
	e := initEnterprise(c)
	adminInfo := e.getAdmin()
	fmt.Println(adminInfo.String())
}

func resetAdminPassword(c *cli.Context) {
	e := initEnterprise(c)
	e.resetAdminPassword()
	showSuccessMsg("reset administrator password")
}

type admin struct {
	gorm.Model
	Username string `gorm:"username"`
	Password string `gorm:"password"`
}

// TableName -
func (a *admin) TableName() string {
	return "users"
}

// String -
func (a *admin) String() string {
	template := `
username: %s
password: %s
	`
	return fmt.Sprintf(template, a.Username, a.Password)
}

type adminRepo struct {
	db *gorm.DB
}

// Update -
func (a *adminRepo) Update(data *admin) error {
	if err := a.db.Table(data.TableName()).Update(data).Error; err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

// Get -
func (a *adminRepo) Get() (*admin, error) {
	var adminInfo admin
	if err := a.db.Where("username=?", "admin").Find(&adminInfo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &adminInfo, nil
}
