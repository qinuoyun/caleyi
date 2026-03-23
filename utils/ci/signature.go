package ci

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

type publicKeyData struct {
	Kid       string `json:"kid"`
	Algorithm string `json:"algorithm"`
	PublicKey string `json:"public_key"`
}

type wrappedLicense struct {
	License string `json:"license"`
	Token   string `json:"token"`
	JWT     string `json:"jwt"`
}

var licenseVerifyPublicKey interface{}

// defaultLicenseAPIKey 未配置 license.api_key 时使用的默认 B 端密钥。
const defaultLicenseAPIKey = "XigLIFrteX9EZU0CfiKfzJUYOHawEc3T"

// EnsureSoftwareLicense 启动时执行软件授权检查与自动申请（框架强制开启）。
func EnsureSoftwareLicense() error {
	apiKey := strings.TrimSpace(C("license.api_key"))
	if apiKey == "" {
		apiKey = defaultLicenseAPIKey
	}
	appID := C("license.app_id")
	if appID == "" {
		return fmt.Errorf("license config missing: require license.app_id")
	}
	baseURLs := getLicenseServerCandidates()
	pubKey, err := fetchPublicKeyFromServer(baseURLs, apiKey)
	if err != nil {
		return fmt.Errorf("fetch public key failed: %w", err)
	}
	licenseVerifyPublicKey = pubKey

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
		// 本地证书存在且未过期时，先做离线验签
		if err := verifyLicenseSignature(store.License); err == nil {
			return nil
		}
		_ = os.Remove(storePath)
		store = &licenseStore{}
		fmt.Printf("[license] local license signature invalid, removed store file: %s\n", storePath)
	}
	if isStoreValid(store) {
		return nil
	}

	// 优先复用历史 apply_id 轮询
	applyID := strings.TrimSpace(store.ApplyID)
	if applyID == "" {
		req := buildApplyRequest()
		id, err := submitApply(baseURLs, apiKey, req)
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
		status, err := queryApply(baseURLs, apiKey, applyID)
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
			if err := verifyLicenseSignature(status.License); err != nil {
				return fmt.Errorf("license signature verify failed: %w", err)
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

func submitApply(baseURLs []string, apiKey string, req applyRequest) (string, error) {
	body, _ := json.Marshal(req)
	var lastErr error
	for _, baseURL := range baseURLs {
		url := baseURL + "/api/apply"
		fmt.Printf("[license] apply request -> url=%s app_id=%s software_name=%s device_name=%s machine_id=%s\n",
			url, req.AppID, req.SoftwareName, req.DeviceName, req.MachineID)
		httpReq, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		httpReq.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			fmt.Printf("[license] apply request failed (%s): %v\n", baseURL, err)
			lastErr = err
			continue
		}

		var out apiResp
		decodeErr := json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close()
		if decodeErr != nil {
			lastErr = decodeErr
			continue
		}
		if out.Code != 0 {
			fmt.Printf("[license] apply response error (%s): code=%d message=%s\n", baseURL, out.Code, out.Message)
			lastErr = fmt.Errorf("apply failed: %s", out.Message)
			continue
		}
		var data applyData
		if err := json.Unmarshal(out.Data, &data); err != nil {
			lastErr = err
			continue
		}
		if data.ApplyID == "" {
			lastErr = fmt.Errorf("apply_id is empty")
			continue
		}
		fmt.Printf("[license] apply success: apply_id=%s via=%s\n", data.ApplyID, baseURL)
		return data.ApplyID, nil
	}
	return "", lastErr
}

func queryApply(baseURLs []string, apiKey, applyID string) (*queryData, error) {
	var lastErr error
	for _, baseURL := range baseURLs {
		url := baseURL + "/api/apply/" + applyID
		httpReq, _ := http.NewRequest(http.MethodGet, url, nil)
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			lastErr = err
			continue
		}

		var out apiResp
		decodeErr := json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close()
		if decodeErr != nil {
			lastErr = decodeErr
			continue
		}
		if out.Code != 0 {
			lastErr = fmt.Errorf("query failed: %s", out.Message)
			continue
		}
		var data queryData
		if err := json.Unmarshal(out.Data, &data); err != nil {
			lastErr = err
			continue
		}
		return &data, nil
	}
	return nil, lastErr
}

func getLicenseServerCandidates() []string {
	// 优先使用配置，未配置则按默认地址顺序兜底
	custom := strings.TrimSpace(C("license.server"))
	if custom != "" {
		return []string{strings.TrimRight(custom, "/")}
	}
	return []string{
		"https://auth.caleyi.com",
		"http://auth.caleyi.com",
		"http://authback.caleyi.com",
	}
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

// verifyLicenseSignature 对本地 license 执行 RSA/ECDSA 离线验签（JWT/JWS 格式）。
// 公钥由授权服务器下发，不依赖本地配置。
func verifyLicenseSignature(license string) error {
	fmt.Printf("[license] verify signature GetHardwareUUID=%s\n", GetHardwareUUID())
	if licenseVerifyPublicKey == nil {
		return errors.New("verify public key is not initialized")
	}
	tokenStr, err := normalizeLicenseToken(license)
	if err != nil {
		preview := strings.TrimSpace(license)
		if len(preview) > 80 {
			preview = preview[:80] + "..."
		}
		fmt.Printf("[license] unsupported license format, preview=%q len=%d\n", preview, len(strings.TrimSpace(license)))
		return err
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		switch token.Method.(type) {
		case *jwt.SigningMethodRSA, *jwt.SigningMethodECDSA:
			return licenseVerifyPublicKey, nil
		default:
			return nil, fmt.Errorf("unsupported signing method: %s", token.Method.Alg())
		}
	})
	if err != nil {
		return err
	}
	if !token.Valid {
		return errors.New("invalid license token")
	}
	if err := verifyLicenseMachineBinding(token); err != nil {
		return fmt.Errorf("license machine binding: %w", err)
	}
	return nil
}

// verifyLicenseMachineBinding 离线验签通过后，校验 JWT 内设备码与当前机器一致。
// 支持 claims：machine_id / mid / device_id / hw_id / hardware_uuid（任一有值即参与比对）。
func verifyLicenseMachineBinding(token *jwt.Token) error {
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return errors.New("license claims must be jwt.MapClaims")
	}

	var claimVal string
	for _, key := range []string{"machine_id", "mid", "device_id", "hw_id", "hardware_uuid"} {
		if v, ok := claims[key]; ok && v != nil {
			if s, ok := v.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					claimVal = s
					break
				}
			}
		}
	}
	if claimVal == "" {
		return errors.New("missing machine identifier in license (need machine_id or mid/device_id/hw_id/hardware_uuid)")
	}

	if licenseMachineClaimMatchesLocal(claimVal) {
		return nil
	}
	return fmt.Errorf("mismatch: license machine=%q local_uuid=%q local_mid=%q",
		claimVal, GetHardwareUUID(), buildMachineID())
}

// licenseMachineClaimMatchesLocal 比对授权端下发的设备码与本机 GetHardwareUUID / buildMachineID。
func licenseMachineClaimMatchesLocal(claim string) bool {
	claim = strings.TrimSpace(claim)
	if claim == "" {
		return false
	}
	localUUID := strings.TrimSpace(GetHardwareUUID())
	localMID := strings.TrimSpace(buildMachineID())

	if strings.EqualFold(claim, localMID) {
		return true
	}
	if strings.EqualFold(claim, localUUID) {
		return true
	}
	// 去掉 MID- 前缀后比对 32 位 hex（大小写不敏感）
	if normalizeMachineIDHex(claim) == normalizeMachineIDHex(localMID) {
		return true
	}
	if normalizeMachineIDHex(claim) == strings.ToLower(localUUID) {
		return true
	}
	return false
}

func normalizeMachineIDHex(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(strings.ToUpper(s), "MID-") {
		s = s[4:]
	}
	return strings.ToLower(s)
}

// normalizeLicenseToken 兼容两种 license 格式：
// 1. JWT/JWS 三段式（xxx.yyy.zzz）
// 2. Base64 包裹的 JWT（解码后为三段式）
func normalizeLicenseToken(license string) (string, error) {
	raw := strings.TrimSpace(license)
	if strings.Count(raw, ".") == 2 {
		return raw, nil
	}
	// 先尝试 JSON 包裹格式
	if s := extractJWTFromJSON(raw); s != "" {
		return s, nil
	}
	// 尝试标准 Base64
	if b, err := base64.StdEncoding.DecodeString(raw); err == nil {
		s := strings.TrimSpace(string(b))
		if strings.Count(s, ".") == 2 {
			return s, nil
		}
		if v := extractJWTFromJSON(s); v != "" {
			return v, nil
		}
	}
	// 尝试 URL Safe Base64
	if b, err := base64.RawURLEncoding.DecodeString(raw); err == nil {
		s := strings.TrimSpace(string(b))
		if strings.Count(s, ".") == 2 {
			return s, nil
		}
		if v := extractJWTFromJSON(s); v != "" {
			return v, nil
		}
	}
	if b, err := base64.URLEncoding.DecodeString(raw); err == nil {
		s := strings.TrimSpace(string(b))
		if strings.Count(s, ".") == 2 {
			return s, nil
		}
		if v := extractJWTFromJSON(s); v != "" {
			return v, nil
		}
	}
	return "", errors.New("license format unsupported: require JWT or base64-encoded JWT")
}

func extractJWTFromJSON(raw string) string {
	var w wrappedLicense
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		return ""
	}
	candidates := []string{strings.TrimSpace(w.License), strings.TrimSpace(w.Token), strings.TrimSpace(w.JWT)}
	for _, c := range candidates {
		if strings.Count(c, ".") == 2 {
			return c
		}
	}
	return ""
}

func fetchPublicKeyFromServer(baseURLs []string, apiKey string) (interface{}, error) {
	endpoints := []string{"/api/public-key", "/api/sign/public-key", "/public-key"}
	var lastErr error
	client := &http.Client{Timeout: 15 * time.Second}
	for _, baseURL := range baseURLs {
		for _, ep := range endpoints {
			url := baseURL + ep
			req, _ := http.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("Authorization", "Bearer "+apiKey)
			resp, err := client.Do(req)
			if err != nil {
				lastErr = err
				continue
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				lastErr = fmt.Errorf("status %d", resp.StatusCode)
				continue
			}
			raw := strings.TrimSpace(string(body))
			// 兼容统一响应格式
			var out apiResp
			if json.Unmarshal(body, &out) == nil && len(out.Data) > 0 {
				var pkData publicKeyData
				if json.Unmarshal(out.Data, &pkData) == nil && strings.TrimSpace(pkData.PublicKey) != "" {
					if pkData.Algorithm != "" {
						alg := strings.ToUpper(strings.TrimSpace(pkData.Algorithm))
						if alg != "RSA" && alg != "ECDSA" {
							lastErr = fmt.Errorf("unsupported public key algorithm: %s", pkData.Algorithm)
							continue
						}
					}
					raw = pkData.PublicKey
					if strings.TrimSpace(pkData.Kid) != "" {
						fmt.Printf("[license] public key metadata: kid=%s algorithm=%s\n", pkData.Kid, pkData.Algorithm)
					}
				}
			}
			pubKey, err := parsePublicKey(raw)
			if err == nil {
				fmt.Printf("[license] public key loaded from %s\n", url)
				return pubKey, nil
			}
			lastErr = err
		}
	}
	return nil, lastErr
}

func parsePublicKey(raw string) (interface{}, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, errors.New("invalid public key pem")
	}
	pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	switch k := pubAny.(type) {
	case *rsa.PublicKey:
		return k, nil
	case *ecdsa.PublicKey:
		return k, nil
	default:
		return nil, fmt.Errorf("unsupported public key type: %T", pubAny)
	}
}
