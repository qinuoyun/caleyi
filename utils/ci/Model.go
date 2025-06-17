package ci

import (
	"fmt"
	"gorm.io/gorm"
)

// Model 自定义基础模型，包含通用字段
type Model struct {
	gorm.Model
	TenantID string `gorm:"type:varchar(32);not null;column:tenant_id"`
}

// BeforeCreate 创建前从上下文中获取 TenantID
func (m *Model) BeforeCreate(tx *gorm.DB) error {
	fmt.Printf("进入 BeforeCreate 方法，上下文: %+v\n", tx.Statement.Context)
	// 从 GORM 事务的上下文中获取 TenantID
	tenantID, ok := tx.Statement.Context.Value("tenant_id").(string)
	if ok && tenantID != "" {
		m.TenantID = tenantID
	} else {
		// 处理 TenantID 未找到的情况，例如返回错误
		return fmt.Errorf("tenant ID not found in context")
	}
	return nil
}

// BeforeQuery 查询前从上下文中获取 TenantID 并添加到查询条件
func (m *Model) BeforeQuery(tx *gorm.DB) error {
	// 从上下文中获取 TenantID
	tenantID, ok := tx.Statement.Context.Value("tenant_id").(string)
	if ok && tenantID != "" {
		// 添加 TenantID 作为查询条件
		tx.Where("tenant_id = ?", tenantID)
	} else {
		// 处理 TenantID 未找到的情况，例如返回错误
		return fmt.Errorf("tenant ID not found in context")
	}
	return nil
}

// BeforeDelete 删除前从上下文中获取 TenantID 并添加到删除条件
func (m *Model) BeforeDelete(tx *gorm.DB) error {
	// 从上下文中获取 TenantID
	tenantID, ok := tx.Statement.Context.Value("tenant_id").(string)
	if ok && tenantID != "" {
		// 添加 TenantID 作为删除条件
		tx.Where("tenant_id = ?", tenantID)
	} else {
		// 处理 TenantID 未找到的情况，例如返回错误
		return fmt.Errorf("tenant ID not found in context")
	}
	return nil
}

// BeforeUpdate 更新前从上下文中获取 TenantID 并添加到更新条件
func (m *Model) BeforeUpdate(tx *gorm.DB) error {
	// 从上下文中获取 TenantID
	tenantID, ok := tx.Statement.Context.Value("tenant_id").(string)
	if ok && tenantID != "" {
		// 添加 TenantID 作为更新条件
		tx.Where("tenant_id = ?", tenantID)
	} else {
		// 处理 TenantID 未找到的情况，例如返回错误
		return fmt.Errorf("tenant ID not found in context")
	}
	return nil
}
