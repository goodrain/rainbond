//+build license

package middleware

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"plugin"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/api/option"
	httputil "github.com/goodrain/rainbond/util/http"
)

// License -
type License struct {
	cfg *option.Config
}

// NewLicense -
func NewLicense(cfg *option.Config) *License {
	return &License{
		cfg: cfg,
	}
}

// LicInfo license information
type LicInfo struct {
	Code       string   `json:"code"`
	Company    string   `json:"company"`
	Node       int64    `json:"node"`
	CPU        int64    `json:"cpu"`
	Memory     int64    `json:"memory"`
	Tenant     int64    `json:"tenant"`
	EndTime    string   `json:"end_time"`
	StartTime  string   `json:"start_time"`
	DataCenter int64    `json:"data_center"`
	ModuleList []string `json:"module_list"`
}

// Verify parses the license to make the content inside it take effect.
func (l *License) Verify(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := verifyLicense(l.cfg.LicensePath, l.cfg.LicSoPath); err != nil {
			httputil.Return(r, w, 401, httputil.ResponseBody{
				Bean: map[string]interface{}{
					"msg": err.Error(),
					"code": 10401,
				},
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func verifyLicense(licPath, licSoPath string) error {
	lic, err := readFromFile(licPath)
	if err != nil {
		logrus.Errorf("failed to read license from file: %v", err)
		return fmt.Errorf("failed to read license from file: %v", err)
	}
	bytes, err := decrypt(lic, licSoPath)
	if err != nil {
		return fmt.Errorf("error decrypting license: %v", err)
	}
	var licInfo LicInfo
	if err := json.Unmarshal(bytes, &licInfo); err != nil {
		logrus.Errorf("error unmarshalling license: %v", err)
		return fmt.Errorf("error unmarshalling license: %v", err)
	}
	if !takeEffect(&licInfo) {
		return fmt.Errorf("invalid license")
	}
	return nil
}

func takeEffect(licInfo *LicInfo) bool {
	layout := "2006-01-02 15:04:05"
	startTime, err := time.Parse(layout, licInfo.StartTime)
	if err != nil {
		logrus.Errorf("start time: %s; error parsing start time: %v", licInfo.StartTime, err)
		return false
	}
	if startTime.After(time.Now()) {
		return false
	}
	endtTime, err := time.Parse(layout, licInfo.EndTime)
	if err != nil {
		logrus.Errorf("end time: %s; error parsing end time: %v", licInfo.EndTime, err)
		return false
	}
	if endtTime.Before(time.Now()) {
		return false
	}
	return true
}

func readFromFile(lfile string) (string, error) {
	_, err := os.Stat(lfile)
	if err != nil {
		logrus.Errorf("license file is incorrect: %v", err)
		return "", err
	}
	bytes, err := ioutil.ReadFile(lfile)
	if err != nil {
		logrus.Errorf("license file: %s; error reading license file: %v", lfile, err)
		return "", err
	}
	return string(bytes), nil
}

func decrypt(license string, licSoPath string) ([]byte, error) {
	p, err := plugin.Open(licSoPath)
	if err != nil {
		return nil, err
	}
	f, err := p.Lookup("Decrypt")
	if err != nil {
		return nil, err
	}
	bytes, err := f.(func(string) ([]byte, error))(license)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}
