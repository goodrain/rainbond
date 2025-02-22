package dao

import (
	"fmt"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// OverScoreDaoImpl over score pack
type OverScoreDaoImpl struct {
	DB *gorm.DB
}

// AddModel add model
func (t *OverScoreDaoImpl) AddModel(mo model.Interface) error {
	overScore, ok := mo.(*model.EnterpriseOverScore)
	if !ok {
		return fmt.Errorf("mo.(*model.EnterpriseOverScore) err")
	}
	return t.DB.Create(overScore).Error
}

// UpdateModel update model
func (t *OverScoreDaoImpl) UpdateModel(mo model.Interface) error {
	overScore, ok := mo.(*model.EnterpriseOverScore)
	if !ok {
		return fmt.Errorf("mo.(*model.overScore) err")
	}
	return t.DB.Save(overScore).Error
}

// UpdateOverScoreRat update
func (t *OverScoreDaoImpl) UpdateOverScoreRat(OverScoreRate string) error {
	osr, err := t.GetOverScoreRate()
	if err != nil {
		return err
	} else {
		osr.OverScoreRate = OverScoreRate
	}
	return t.DB.Save(osr).Error
}

// GetOverScoreRate get over score rate
func (t *OverScoreDaoImpl) GetOverScoreRate() (*model.EnterpriseOverScore, error) {
	var overScore model.EnterpriseOverScore
	if err := t.DB.First(&overScore).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create default record with rate 1
			overScore = model.EnterpriseOverScore{
				OverScoreRate: "{\"CPU\":1,\"MEMORY\":1}",
			}
			if err := t.DB.Create(&overScore).Error; err != nil {
				return nil, err
			}
			return &overScore, nil
		}
		return nil, err
	}
	return &overScore, nil
}
