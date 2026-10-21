package opencode

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const maxModelsBody = 8 << 20

var modelsHTTPClient = &http.Client{Timeout: 20 * time.Second}

var envRefRe = regexp.MustCompile(`^\{env:([^}]+)\}$`)

// RemoteModel is a model returned by a provider's OpenAI-compatible /models API.
type RemoteModel struct {
	ID   string
	Name string
}

// ResolveAPIKey expands "{env:VAR}" references; other values are returned as-is.
func ResolveAPIKey(v string) (string, error) {
	v = strings.TrimSpace(v)
	if m := envRefRe.FindStringSubmatch(v); m != nil {
		name := strings.TrimSpace(m[1])
		if name == "" {
			return "", fmt.Errorf("apiKey 环境变量名不能为空")
		}
		val := os.Getenv(name)
		if val == "" {
			return "", fmt.Errorf("环境变量 %s 未设置", name)
		}
		return val, nil
	}
	return v, nil
}

// ModelsURL appends /models to an OpenAI-compatible baseURL unless already present.
func ModelsURL(baseURL string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "", fmt.Errorf("baseURL 不能为空")
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("无效 baseURL: %s", baseURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("baseURL 仅支持 http(s)")
	}
	path := strings.TrimSuffix(u.Path, "/")
	if !strings.HasSuffix(path, "/models") {
		path += "/models"
	}
	u.Path = path
	return u.String(), nil
}

// FetchModels lists models from an OpenAI-compatible provider.
func FetchModels(baseURL, apiKey string) ([]RemoteModel, error) {
	return fetchModels(modelsHTTPClient, baseURL, apiKey)
}

func fetchModels(client *http.Client, baseURL, apiKey string) ([]RemoteModel, error) {
	endpoint, err := ModelsURL(baseURL)
	if err != nil {
		return nil, err
	}
	key, err := ResolveAPIKey(apiKey)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if key != "" {
		if strings.HasPrefix(strings.ToLower(key), "bearer ") {
			req.Header.Set("Authorization", key)
		} else {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("拉取模型失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxModelsBody+1))
	if err != nil {
		return nil, fmt.Errorf("读取模型列表失败: %w", err)
	}
	if len(body) > maxModelsBody {
		return nil, fmt.Errorf("模型列表响应过大")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("拉取模型失败: HTTP %d %s", resp.StatusCode, snippet(body, 160))
	}
	models, err := parseModelsResponse(body)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("接口未返回任何模型")
	}
	return models, nil
}

func parseModelsResponse(body []byte) ([]RemoteModel, error) {
	body = bytesTrim(body)
	if len(body) == 0 {
		return nil, fmt.Errorf("解析模型列表失败: 空响应")
	}
	if body[0] == '[' {
		return parseModelList(body)
	}
	var wrap struct {
		Data   json.RawMessage `json:"data"`
		Models json.RawMessage `json:"models"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		return nil, fmt.Errorf("解析模型列表失败: %w", err)
	}
	raw := wrap.Data
	if len(raw) == 0 {
		raw = wrap.Models
	}
	return parseModelList(raw)
}

func parseModelList(raw json.RawMessage) ([]RemoteModel, error) {
	raw = bytesTrim(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var objects []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Model       string `json:"model"`
	}
	if err := json.Unmarshal(raw, &objects); err == nil {
		out := make([]RemoteModel, 0, len(objects))
		for _, o := range objects {
			id := firstNonEmpty(o.ID, o.Model, o.Name, o.DisplayName)
			name := firstNonEmpty(o.Name, o.DisplayName, id)
			if id == "" {
				continue
			}
			out = append(out, RemoteModel{ID: id, Name: name})
		}
		if len(out) > 0 || len(objects) == 0 {
			return dedupeModels(out), nil
		}
	}
	var ids []string
	if err := json.Unmarshal(raw, &ids); err == nil {
		out := make([]RemoteModel, 0, len(ids))
		for _, id := range ids {
			out = append(out, RemoteModel{ID: id, Name: id})
		}
		return dedupeModels(out), nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err == nil {
		out := make([]RemoteModel, 0, len(m))
		for id, item := range m {
			name := id
			var obj struct {
				Name        string `json:"name"`
				DisplayName string `json:"display_name"`
			}
			if json.Unmarshal(item, &obj) == nil {
				if n := firstNonEmpty(obj.Name, obj.DisplayName); n != "" {
					name = n
				}
			}
			out = append(out, RemoteModel{ID: id, Name: name})
		}
		return dedupeModels(out), nil
	}
	return nil, fmt.Errorf("解析模型列表失败: 无法识别的响应格式")
}

func dedupeModels(in []RemoteModel) []RemoteModel {
	seen := map[string]struct{}{}
	out := make([]RemoteModel, 0, len(in))
	for _, m := range in {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		name := strings.TrimSpace(m.Name)
		if name == "" {
			name = id
		}
		out = append(out, RemoteModel{ID: id, Name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// MergeRemoteModels adds fetched models that are not already present.
// Existing models keep their name, variants and extra keys; empty names are filled.
func (p *Provider) MergeRemoteModels(models []RemoteModel) (added, updated int) {
	for _, rm := range models {
		if rm.ID == "" {
			continue
		}
		if existing := p.FindModel(rm.ID); existing != nil {
			if existing.Name == "" && rm.Name != "" {
				existing.Name = rm.Name
				updated++
			}
			continue
		}
		name := rm.Name
		if name == "" {
			name = rm.ID
		}
		p.Models = append(p.Models, &Model{
			ID:   rm.ID,
			Name: name,
			raw:  map[string]json.RawMessage{},
		})
		added++
	}
	sort.Slice(p.Models, func(i, j int) bool { return p.Models[i].ID < p.Models[j].ID })
	return added, updated
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func snippet(b []byte, n int) string {
	s := strings.Join(strings.Fields(string(b)), " ")
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) > n {
		r := []rune(s)
		s = string(r[:n]) + "…"
	}
	return s
}
