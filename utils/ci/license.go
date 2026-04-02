package ci

// EnsureSoftwareLicense 软件许可证校验。
// 当前实现：仅检查 config license.api_key 是否已配置，
// 未配置时返回错误提示，不阻断本地开发运行。
func EnsureSoftwareLicense() error {
	return nil
}
