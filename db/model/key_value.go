package model

// KeyValue -
type KeyValue struct {
	K string `gorm:"column:k;type:varchar(100);primary_key" json:"k"`
	V string `gorm:"column:v;type:longtext" json:"v"`
}

// TableName -
func (KeyValue) TableName() string {
	return "key_value"
}
