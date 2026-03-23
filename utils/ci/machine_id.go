package ci

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

// GetHardwareUUID 读取固件级硬件 UUID，重装系统不变；统一为 32 位小写 MD5。
func GetHardwareUUID() string {
	var uuid string

	switch runtime.GOOS {
	case "windows":
		uuid = runCmd("wmic", "csproduct", "get", "UUID")
	case "darwin":
		out := runCmd("ioreg", "-rd1", "-c", "IOPlatformExpertDevice")
		re := regexp.MustCompile(`"IOPlatformUUID"\s*=\s*"([^"]+)"`)
		if m := re.FindStringSubmatch(out); len(m) > 1 {
			uuid = strings.TrimSpace(m[1])
		}
	case "linux":
		uuid = mustLinuxSystemUUID()
	}

	uuid = cleanUUID(uuid)
	if uuid != "" {
		h := md5.Sum([]byte(strings.ToLower(uuid)))
		return hex.EncodeToString(h[:])
	}
	return "unknown-device"
}

// buildMachineID 作为对外 machine_id：基于 GetHardwareUUID，格式 MID-<32位大写十六进制>。
func buildMachineID() string {
	id := GetHardwareUUID()
	if id == "unknown-device" {
		host, _ := os.Hostname()
		h := md5.Sum([]byte(fmt.Sprintf("fallback:%s:%s:%s", runtime.GOOS, runtime.GOARCH, host)))
		id = hex.EncodeToString(h[:])
	}
	return "MID-" + strings.ToUpper(id)
}

func runCmd(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return string(out)
}

func cleanUUID(s string) string {
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if len(t) >= 32 && strings.Contains(t, "-") {
			return t
		}
	}
	return ""
}

// mustLinuxSystemUUID 通过 dmidecode 读取主板 system-uuid；未安装或失败则终止进程。
func mustLinuxSystemUUID() string {
	if _, err := exec.LookPath("dmidecode"); err != nil {
		log.Fatalf("[machine_id] Linux 未安装 dmidecode，无法获取硬件 UUID。请先安装后重试：\n"+
			"  Debian/Ubuntu: sudo apt install dmidecode\n"+
			"  RHEL/CentOS:     sudo yum install dmidecode 或 sudo dnf install dmidecode\n"+
			"错误: %v", err)
	}
	cmd := exec.Command("dmidecode", "-s", "system-uuid")
	out, err := cmd.CombinedOutput()
	raw := strings.TrimSpace(string(out))
	if err != nil {
		log.Fatalf("[machine_id] Linux 执行 dmidecode 失败（请确认已安装 dmidecode，且当前用户有权限读取 DMI，必要时使用 root/sudo 运行）：\n"+
			"  %v\n输出: %s", err, raw)
	}
	if raw == "" {
		log.Fatal("[machine_id] Linux dmidecode 未返回 system-uuid，请检查硬件/DMI 是否可用")
	}
	uuid := cleanUUID(raw)
	if uuid == "" {
		log.Fatalf("[machine_id] Linux 无法从 dmidecode 输出解析出有效 UUID，原始输出: %q", raw)
	}
	return uuid
}
