package ci

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

type (
	Map        = map[string]interface{}      // Map is alias of frequently-used map type map[string]interface{}.
	MapAnyAny  = map[interface{}]interface{} // MapAnyAny is alias of frequently-used map type map[interface{}]interface{}.
	MapAnyStr  = map[interface{}]string      // MapAnyStr is alias of frequently-used map type map[interface{}]string.
	MapAnyInt  = map[interface{}]int         // MapAnyInt is alias of frequently-used map type map[interface{}]int.
	MapStrAny  = map[string]interface{}      // MapStrAny is alias of frequently-used map type map[string]interface{}.
	MapStrStr  = map[string]string           // MapStrStr is alias of frequently-used map type map[string]string.
	MapStrInt  = map[string]int              // MapStrInt is alias of frequently-used map type map[string]int.
	MapIntAny  = map[int]interface{}         // MapIntAny is alias of frequently-used map type map[int]interface{}.
	MapIntStr  = map[int]string              // MapIntStr is alias of frequently-used map type map[int]string.
	MapIntInt  = map[int]int                 // MapIntInt is alias of frequently-used map type map[int]int.
	MapAnyBool = map[interface{}]bool        // MapAnyBool is alias of frequently-used map type map[interface{}]bool.
	MapStrBool = map[string]bool             // MapStrBool is alias of frequently-used map type map[string]bool.
	MapIntBool = map[int]bool                // MapIntBool is alias of frequently-used map type map[int]bool.
)

type (
	List        = []Map        // List is alias of frequently-used slice type []Map.
	ListAnyAny  = []MapAnyAny  // ListAnyAny is alias of frequently-used slice type []MapAnyAny.
	ListAnyStr  = []MapAnyStr  // ListAnyStr is alias of frequently-used slice type []MapAnyStr.
	ListAnyInt  = []MapAnyInt  // ListAnyInt is alias of frequently-used slice type []MapAnyInt.
	ListStrAny  = []MapStrAny  // ListStrAny is alias of frequently-used slice type []MapStrAny.
	ListStrStr  = []MapStrStr  // ListStrStr is alias of frequently-used slice type []MapStrStr.
	ListStrInt  = []MapStrInt  // ListStrInt is alias of frequently-used slice type []MapStrInt.
	ListIntAny  = []MapIntAny  // ListIntAny is alias of frequently-used slice type []MapIntAny.
	ListIntStr  = []MapIntStr  // ListIntStr is alias of frequently-used slice type []MapIntStr.
	ListIntInt  = []MapIntInt  // ListIntInt is alias of frequently-used slice type []MapIntInt.
	ListAnyBool = []MapAnyBool // ListAnyBool is alias of frequently-used slice type []MapAnyBool.
	ListStrBool = []MapStrBool // ListStrBool is alias of frequently-used slice type []MapStrBool.
	ListIntBool = []MapIntBool // ListIntBool is alias of frequently-used slice type []MapIntBool.
)

type (
	Slice    = []interface{} // Slice is alias of frequently-used slice type []interface{}.
	SliceAny = []interface{} // SliceAny is alias of frequently-used slice type []interface{}.
	SliceStr = []string      // SliceStr is alias of frequently-used slice type []string.
	SliceInt = []int         // SliceInt is alias of frequently-used slice type []int.
)

type (
	Array    = []interface{} // Array is alias of frequently-used slice type []interface{}.
	ArrayAny = []interface{} // ArrayAny is alias of frequently-used slice type []interface{}.
	ArrayStr = []string      // ArrayStr is alias of frequently-used slice type []string.
	ArrayInt = []int         // ArrayInt is alias of frequently-used slice type []int.
)
type (
	// JSON StringSlice 关键修改：使用类型定义（去掉等号 =）
	JSON []string // ✅ 创建新类型，允许定义方法
)

// Scan 实现 Scanner 接口（读取时处理）
func (s *JSON) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("database value is not []byte")
	}

	// 反序列化到 JSON 类型
	return json.Unmarshal(bytes, s)
}

// Value 实现 Valuer 接口（写入时处理）
func (s JSON) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}
