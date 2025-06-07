package ci

import (
	"gorm.io/gorm"
)

// Model 自定义基础模型，包含通用字段
type Model struct {
	gorm.Model
	UniqueID string `gorm:"type:char(36);column:unique_id"`
}

//
//// BeforeCreate 创建前设置UniqueID
//func (m *Model) BeforeCreate(tx *gorm.DB) error {
//	// 从上下文中获取UniqueID
//	uniqueID, _ := tx.Statement.Context.Value("uniqueid").(string)
//	m.UniqueID = uniqueID
//	return nil
//}
