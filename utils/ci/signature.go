package ci

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type licenseStore struct {
	ApplyID   string `json:"apply_id"`
	License   string `json:"license"`
	ExpireAt  int64  `json:"expire_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type applyRequest struct {
	AppID        string `json:"app_id"`
	SoftwareName string `json:"software_name"`
	DeviceName   string `json:"device_name"`
	MachineID    string `json:"machine_id"`
	Remark       string `json:"remark,omitempty"`
}

type apiResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type applyData struct {
	ApplyID string `json:"apply_id"`
}

type queryData struct {
	Status    int    `json:"status"`
	ExpireDay int    `json:"expire_day"`
	License   string `json:"license"`
	ApplyID   string `json:"apply_id"`
}

// EnsureSoftwareLicense 启动时执行软件授权检查与自动申请（框架强制开启）。
func EnsureSoftwareLicense() error {
	baseURL := strings.TrimRight(C("license.server"), "/")
	apiKey := C("license.api_key")
	appID := C("license.app_id")
	if baseURL == "" || apiKey == "" || appID == "" {
		return fmt.Errorf("license config missing: require license.server/license.api_key/license.app_id")
	}

	storePath := C("license.store_file")
	if storePath == "" {
		storePath = "runtime/license.json"
	}

	store, _ := loadLicenseStore(storePath)
	// 证书过期：先删除本地证书，再走重新申请流程
	if isStoreExpired(store) {
		_ = os.Remove(storePath)
		store = &licenseStore{}
		fmt.Printf("[license] local license expired, removed store file: %s\n", storePath)
	}
	if isStoreValid(store) {
		return nil
	}

	// 优先复用历史 apply_id 轮询
	applyID := strings.TrimSpace(store.ApplyID)
	if applyID == "" {
		req := buildApplyRequest()
		id, err := submitApply(baseURL, apiKey, req)
		if err != nil {
			return err
		}
		applyID = id
		store.ApplyID = id
		_ = saveLicenseStore(storePath, store)
	}

	intervalSec := toPositiveInt(C("license.poll_interval_sec"), 5)
	timeoutSec := toPositiveInt(C("license.poll_timeout_sec"), 600)
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)

	for time.Now().Before(deadline) {
		status, err := queryApply(baseURL, apiKey, applyID)
		if err != nil {
			time.Sleep(time.Duration(intervalSec) * time.Second)
			continue
		}
		switch status.Status {
		case 0:
			time.Sleep(time.Duration(intervalSec) * time.Second)
		case 1:
			if status.License == "" {
				return fmt.Errorf("approve success but license is empty")
			}
			expireAt := time.Now().Add(time.Duration(status.ExpireDay) * 24 * time.Hour).Unix()
			if status.ExpireDay <= 0 {
				expireAt = 0
			}
			store.ApplyID = applyID
			store.License = status.License
			store.ExpireAt = expireAt
			store.UpdatedAt = time.Now().Unix()
			return saveLicenseStore(storePath, store)
		case 2:
			return fmt.Errorf("license apply rejected")
		default:
			time.Sleep(time.Duration(intervalSec) * time.Second)
		}
	}

	return fmt.Errorf("license poll timeout")
}

func buildApplyRequest() applyRequest {
	host, _ := os.Hostname()
	deviceName := C("license.device_name")
	if deviceName == "" {
		deviceName = host
	}
	softwareName := C("app.app_name")
	machineID := buildMachineID()
	return applyRequest{
		AppID:        C("license.app_id"),
		SoftwareName: softwareName,
		DeviceName:   deviceName,
		MachineID:    machineID,
		Remark:       C("license.remark"),
	}
}

func submitApply(baseURL, apiKey string, req applyRequest) (string, error) {
	body, _ := json.Marshal(req)
	url := baseURL + "/api/apply"
	fmt.Printf("[license] apply request -> url=%s app_id=%s software_name=%s device_name=%s machine_id=%s\n",
		url, req.AppID, req.SoftwareName, req.DeviceName, req.MachineID)
	httpReq, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		fmt.Printf("[license] apply request failed: %v\n", err)
		return "", err
	}
	defer resp.Body.Close()

	var out apiResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Code != 0 {
		fmt.Printf("[license] apply response error: code=%d message=%s\n", out.Code, out.Message)
		return "", fmt.Errorf("apply failed: %s", out.Message)
	}
	var data applyData
	if err := json.Unmarshal(out.Data, &data); err != nil {
		return "", err
	}
	if data.ApplyID == "" {
		return "", fmt.Errorf("apply_id is empty")
	}
	fmt.Printf("[license] apply success: apply_id=%s\n", data.ApplyID)
	return data.ApplyID, nil
}

func queryApply(baseURL, apiKey, applyID string) (*queryData, error) {
	url := baseURL + "/api/apply/" + applyID
	httpReq, _ := http.NewRequest(http.MethodGet, url, nil)
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out apiResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Code != 0 {
		return nil, fmt.Errorf("query failed: %s", out.Message)
	}
	var data queryData
	if err := json.Unmarshal(out.Data, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func loadLicenseStore(path string) (*licenseStore, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return &licenseStore{}, err
	}
	var s licenseStore
	if err := json.Unmarshal(b, &s); err != nil {
		return &licenseStore{}, err
	}
	return &s, nil
}

func saveLicenseStore(path string, s *licenseStore) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(path, b, 0600)
}

func isStoreValid(s *licenseStore) bool {
	if s == nil || strings.TrimSpace(s.License) == "" {
		return false
	}
	// ExpireAt=0 视为不过期
	if s.ExpireAt == 0 {
		return true
	}
	return time.Now().Unix() < s.ExpireAt
}

func isStoreExpired(s *licenseStore) bool {
	if s == nil || strings.TrimSpace(s.License) == "" {
		return false
	}
	if s.ExpireAt == 0 {
		return false
	}
	return time.Now().Unix() >= s.ExpireAt
}

func toPositiveInt(raw string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// buildMachineID 生成跨平台稳定机器码（Mac/Win/Linux）。
// 规则：OS/ARCH + 主机名 + 系统机器ID + 物理网卡 MAC，做 SHA256。
func buildMachineID() string {
	host, _ := os.Hostname()
	var macs []string
	ifaces, _ := net.Interfaces()
	for _, it := range ifaces {
		// 过滤回环、虚拟和无硬件地址网卡
		if it.Flags&net.FlagLoopback != 0 || len(it.HardwareAddr) == 0 {
			continue
		}
		name := strings.ToLower(it.Name)
		if strings.Contains(name, "docker") || strings.Contains(name, "veth") || strings.Contains(name, "vmnet") {
			continue
		}
		macs = append(macs, strings.ToLower(it.HardwareAddr.String()))
	}
	sort.Strings(macs)
	platformID := getPlatformMachineID()
	base := runtime.GOOS + "|" + runtime.GOARCH + "|" + strings.ToLower(host) + "|" + platformID + "|" + strings.Join(macs, ",")
	sum := sha256.Sum256([]byte(base))
	return "MID-" + strings.ToUpper(hex.EncodeToString(sum[:16]))
}

// getPlatformMachineID 获取系统级机器唯一标识（尽力而为，失败返回空字符串）。
func getPlatformMachineID() string {
	switch runtime.GOOS {
	case "linux":
		if v := readFirstLine("/etc/machine-id"); v != "" {
			return strings.ToLower(v)
		}
		if v := readFirstLine("/var/lib/dbus/machine-id"); v != "" {
			return strings.ToLower(v)
		}
	case "darwin":
		out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
		if err == nil {
			re := regexp.MustCompile(`"IOPlatformUUID"\s*=\s*"([^"]+)"`)
			m := re.FindStringSubmatch(string(out))
			if len(m) > 1 {
				return strings.ToLower(strings.TrimSpace(m[1]))
			}
		}
	case "windows":
		out, err := exec.Command("reg", "query", `HKLM\SOFTWARE\Microsoft\Cryptography`, "/v", "MachineGuid").Output()
		if err == nil {
			re := regexp.MustCompile(`MachineGuid\s+REG_\w+\s+([^\r\n]+)`)
			m := re.FindStringSubmatch(string(out))
			if len(m) > 1 {
				return strings.ToLower(strings.TrimSpace(m[1]))
			}
		}
	}
	return ""
}

func readFirstLine(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(b))
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	return line
}

