// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package sources

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/tidwall/gjson"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/sirupsen/logrus"

	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/pem"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	netssh "golang.org/x/crypto/ssh"
	sshkey "golang.org/x/crypto/ssh"
)

// dirLockManager 目录锁管理器，用于防止对同一目录的并发 Git 操作
type dirLockManager struct {
	locks sync.Map // key: directory path, value: *sync.Mutex
}

// globalDirLockManager 全局目录锁管理器实例
var globalDirLockManager = &dirLockManager{}

// getLock 获取指定目录的锁
func (m *dirLockManager) getLock(dir string) *sync.Mutex {
	lock, _ := m.locks.LoadOrStore(dir, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// lock 对指定目录加锁
func (m *dirLockManager) lock(dir string) {
	mutex := m.getLock(dir)
	logrus.Debugf("Acquiring lock for directory: %s", dir)
	mutex.Lock()
	logrus.Debugf("Lock acquired for directory: %s", dir)
}

// unlock 对指定目录解锁
func (m *dirLockManager) unlock(dir string) {
	mutex := m.getLock(dir)
	mutex.Unlock()
	logrus.Debugf("Lock released for directory: %s", dir)
}

// CodeSourceInfo 代码源信息
type CodeSourceInfo struct {
	ServerType     string                  `json:"server_type"`
	RepositoryURL  string                  `json:"repository_url"`
	Branch         string                  `json:"branch"`
	User           string                  `json:"user"`
	Password       string                  `json:"password"`
	Configs        map[string]gjson.Result `json:"configs"`
	DockerfilePath string                  `json:"dockerfile_path"` // Dockerfile路径，用于指定子目录中的Dockerfile
	//避免项目之间冲突，代码缓存目录提高到租户
	TenantID  string `json:"tenant_id"`
	ServiceID string `json:"service_id"`
}

// GetCodeSourceDir get source storage directory
func (c CodeSourceInfo) GetCodeSourceDir() string {
	return GetCodeSourceDir(c.RepositoryURL, c.Branch, c.TenantID, c.ServiceID)
}

// GetCodeSourceDir get source storage directory
// it changes as gitrepostory address, branch, and service id change
func GetCodeSourceDir(RepositoryURL, branch, tenantID string, ServiceID string) string {
	sourceDir := os.Getenv("SOURCE_DIR")
	if sourceDir == "" {
		sourceDir = "/grdata/source"
	}
	h := sha1.New()
	h.Write([]byte(RepositoryURL + branch + ServiceID))
	bs := h.Sum(nil)
	bsStr := fmt.Sprintf("%x", bs)
	targetDir := path.Join(sourceDir, "build", tenantID, bsStr)

	// Check if directory exists, create if not
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			logrus.Errorf("Failed to create source directory %s: %v", targetDir, err)
		}
	}

	return targetDir
}

// CheckFileExist CheckFileExist
func CheckFileExist(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}

// RemoveDir RemoveDir
func RemoveDir(path string) error {
	if path == "/" {
		return fmt.Errorf("remove wrong dir")
	}
	return os.RemoveAll(path)
}
func getShowURL(rurl string) string {
	urlpath, _ := url.Parse(rurl)
	if urlpath != nil {
		showURL := fmt.Sprintf("%s://%s%s", urlpath.Scheme, urlpath.Host, urlpath.Path)
		return showURL
	}
	return ""
}

// gitCloneInternal git clone code 内部实现（不加锁）
func gitCloneInternal(csi CodeSourceInfo, sourceDir string, logger event.Logger, timeout int) (*git.Repository, string, error) {
	GetPrivateFileParam := csi.TenantID
	if !strings.HasSuffix(csi.RepositoryURL, ".git") {
		csi.RepositoryURL = csi.RepositoryURL + ".git"
	}
	flag := true
Loop:
	if logger != nil {
		//Hide possible account key information
		logger.Info(fmt.Sprintf("Start clone source code from %s", getShowURL(csi.RepositoryURL)), map[string]string{"step": "clone_code"})
	}
	ep, err := transport.NewEndpoint(csi.RepositoryURL)
	if err != nil {
		return nil, "", err
	}
	if timeout < 1 {
		timeout = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*time.Duration(timeout))
	defer cancel()
	writer := logger.GetWriter("progress", "debug")
	writer.SetFormat(map[string]interface{}{"progress": "%s", "id": "Clone:"})
	opts := &git.CloneOptions{
		URL:               csi.RepositoryURL,
		Progress:          writer,
		SingleBranch:      true,
		Tags:              git.NoTags,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		Depth:             1,
	}
	if csi.Branch != "" {
		opts.ReferenceName = getBranch(csi.Branch)
	}
	var rs *git.Repository
	if ep.Protocol == "ssh" {
		publichFile := GetPrivateFile(GetPrivateFileParam)
		sshAuth, auerr := ssh.NewPublicKeysFromFile("git", publichFile, "")
		if auerr != nil {
			errMsg := util.Translation("Create SSH public keys error")
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "pull-code", "status": "failure"})
			}
			return nil, errMsg, auerr
		}
		sshAuth.HostKeyCallbackHelper.HostKeyCallback = netssh.InsecureIgnoreHostKey()
		opts.Auth = sshAuth
		rs, err = git.PlainCloneContext(ctx, sourceDir, false, opts)
	} else {
		// only proxy github
		// but when setting, other request will be proxyed
		customClient := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			Timeout: time.Minute * time.Duration(timeout),
		}
		if strings.Contains(csi.RepositoryURL, "github.com") && os.Getenv("GITHUB_PROXY") != "" {
			proxyURL, err := url.Parse(os.Getenv("GITHUB_PROXY"))
			if err == nil {
				customClient.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL), TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
			} else {
				logrus.Error(err)
			}
		}
		if csi.User != "" && csi.Password != "" {
			httpAuth := &githttp.BasicAuth{
				Username: csi.User,
				Password: csi.Password,
			}
			opts.Auth = httpAuth
		}
		client.InstallProtocol("https", githttp.NewClient(customClient))
		defer func() {
			client.InstallProtocol("https", githttp.DefaultClient)
		}()
		rs, err = git.PlainCloneContext(ctx, sourceDir, false, opts)
	}
	if err != nil {
		if reerr := os.RemoveAll(sourceDir); reerr != nil {
			errMsg := util.Translation("Pull code error, failed to delete code directory")
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "clone-code", "status": "failure"})
			}
		}

		// Get error string for pattern matching
		errLower := strings.ToLower(err.Error())

		// Log detailed error information for debugging
		logrus.Errorf("[Git Clone Error] URL: %s, Error Type: %T, Error String: %s, HasUser: %v, HasPassword: %v",
			csi.RepositoryURL, err, err.Error(), csi.User != "", csi.Password != "")

		// 1. Network errors - check first as they are most common infrastructure issues
		if strings.Contains(errLower, "dial tcp") ||
		   strings.Contains(errLower, "connection refused") ||
		   strings.Contains(errLower, "no route to host") ||
		   strings.Contains(errLower, "network is unreachable") ||
		   strings.Contains(errLower, "i/o timeout") ||
		   strings.Contains(errLower, "connection timed out") {
			errMsg := fmt.Sprintf("网络连接失败：%s", err.Error())
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "clone-code", "status": "failure"})
			}
			logrus.Errorf("Git clone network error: %v", err)
			return rs, errMsg, err
		}

		// 2. Timeout errors
		if strings.Contains(err.Error(), "context deadline exceeded") {
			errMsg := "获取代码超时，请检查网络连接或增加超时时间"
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "clone-code", "status": "failure"})
			}
			return rs, errMsg, err
		}

		// 3. Repository not found (404) - check type first before string patterns
		// This should be checked before authentication errors because Gitee may return 404
		// with "authentication required" message for both non-existent and private repos
		if err == transport.ErrRepositoryNotFound {
			// Check if credentials were provided
			hasCredentials := (csi.User != "" && csi.Password != "") || csi.Password != ""

			// Detailed logging for debugging
			logrus.Errorf("[404 Detection] URL: %s, HasCredentials: %v, ErrorContains404: %v, ErrorContainsAuth: %v, ErrorContainsUnauth: %v, FullError: %s",
				csi.RepositoryURL, hasCredentials,
				strings.Contains(errLower, "404"),
				strings.Contains(errLower, "authentication"),
				strings.Contains(errLower, "unauthorized"),
				err.Error())

			if hasCredentials {
				// Credentials provided but still got 404 - repository likely doesn't exist
				logrus.Infof("[404 Detection] Decision: Repository doesn't exist (has credentials)")
				errMsg := fmt.Sprintf("仓库不存在，请检查仓库地址是否正确：%s", csi.RepositoryURL)
				if logger != nil {
					logger.Error(errMsg, map[string]string{"step": "clone-code", "status": "failure"})
				}
				return rs, errMsg, err
			} else {
				// No credentials provided - could be private repo or truly doesn't exist
				// Check if error message contains 404 vs authentication keywords
				if strings.Contains(errLower, "404") && !strings.Contains(errLower, "authentication") && !strings.Contains(errLower, "unauthorized") {
					// Pure 404 error, likely repository doesn't exist
					logrus.Infof("[404 Detection] Decision: Repository doesn't exist (pure 404)")
					errMsg := fmt.Sprintf("仓库不存在，请检查仓库地址是否正确：%s", csi.RepositoryURL)
					if logger != nil {
						logger.Error(errMsg, map[string]string{"step": "clone-code", "status": "failure"})
					}
					return rs, errMsg, err
				} else {
					// 404 with authentication keywords, could be private repo
					logrus.Infof("[404 Detection] Decision: Private repo or doesn't exist (404 with auth keywords)")
					errMsg := "仓库不存在或为私有仓库。如果是私有仓库，请配置访问凭证（用户名和密码或 Access Token）"
					if logger != nil {
						logger.Error(errMsg, map[string]string{"step": "clone-code", "status": "failure"})
					}
					return rs, errMsg, err
				}
			}
		}

		// 4. Authentication required (401) - clear authentication needed
		if err == transport.ErrAuthenticationRequired ||
		   strings.Contains(errLower, "authentication required") ||
		   strings.Contains(errLower, "unauthorized") ||
		   strings.Contains(errLower, "401 unauthorized") ||
		   strings.Contains(errLower, "401") {
			logrus.Errorf("[Auth Detection] URL: %s, IsAuthRequiredType: %v, ContainsAuthReq: %v, ContainsUnauth: %v, Contains401: %v, FullError: %s",
				csi.RepositoryURL,
				err == transport.ErrAuthenticationRequired,
				strings.Contains(errLower, "authentication required"),
				strings.Contains(errLower, "unauthorized"),
				strings.Contains(errLower, "401"),
				err.Error())
			errMsg := "访问仓库失败。可能的原因：① 仓库地址错误或不存在；② 仓库为私有，需要配置访问凭证；③ 分支名称错误。请检查仓库地址和分支名称，或配置用户名和密码/Access Token"
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "clone-code", "status": "failure"})
			}
			return rs, errMsg, err
		}

		// 5. Authorization failed (403) - credentials provided but insufficient permissions
		if err == transport.ErrAuthorizationFailed ||
		   strings.Contains(errLower, "403 forbidden") ||
		   strings.Contains(errLower, "403") {
			errMsg := "访问权限不足，请检查提供的凭证是否有访问该仓库的权限"
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "clone-code", "status": "failure"})
			}
			return rs, errMsg, err
		}

		// 6. Invalid credentials
		if strings.Contains(errLower, "invalid username or password") ||
		   strings.Contains(errLower, "authentication failed") ||
		   strings.Contains(errLower, "incorrect username or password") {
			errMsg := "用户名或密码错误，请检查访问凭证"
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "clone-code", "status": "failure"})
			}
			return rs, errMsg, err
		}

		// 7. Empty repository
		if err == transport.ErrEmptyRemoteRepository {
			errMsg := "仓库为空，请确保仓库中至少有一个提交"
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "clone-code", "status": "failure"})
			}
			return rs, errMsg, err
		}

		// 8. Branch/reference not found
		if err == plumbing.ErrReferenceNotFound || strings.Contains(err.Error(), "couldn't find remote ref") {
			errMsg := fmt.Sprintf("分支不存在：%s，请检查分支名称是否正确", csi.Branch)
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "clone-code", "status": "failure"})
			}
			return rs, errMsg, fmt.Errorf("branch %s is not exist", csi.Branch)
		}

		// 9. SSH authentication errors
		if strings.Contains(err.Error(), "ssh: unable to authenticate") {
			if flag {
				GetPrivateFileParam = "builder_rsa"
				flag = false
				goto Loop
			}
			errMsg := "SSH 密钥认证失败，请检查 SSH 密钥配置"
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "clone-code", "status": "failure"})
			}
			return rs, errMsg, err
		}

		// 10. DNS resolution errors
		if strings.Contains(errLower, "no such host") ||
		   strings.Contains(errLower, "lookup") && strings.Contains(errLower, "no such host") {
			errMsg := fmt.Sprintf("DNS 解析失败，无法解析域名：%s", err.Error())
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "clone-code", "status": "failure"})
			}
			logrus.Errorf("Git clone DNS resolution error: %v", err)
			return rs, errMsg, err
		}

		// 11. Unhandled errors - log and return detailed message
		logrus.Errorf("Git clone failed with unhandled error type: %v (error string: %s)", err, err.Error())
		errMsg := fmt.Sprintf("拉取代码失败：%s", err.Error())
		if logger != nil {
			logger.Error(errMsg, map[string]string{"step": "clone-code", "status": "failure"})
		}
		return rs, errMsg, err
	}
	return rs, "", err
}

// GitClone git clone code（带锁的外部接口）
func GitClone(csi CodeSourceInfo, sourceDir string, logger event.Logger, timeout int) (*git.Repository, string, error) {
	// 对目录加锁，防止并发 Git 操作冲突
	globalDirLockManager.lock(sourceDir)
	defer globalDirLockManager.unlock(sourceDir)

	return gitCloneInternal(csi, sourceDir, logger, timeout)
}

func retryAuth(ep *transport.Endpoint, csi CodeSourceInfo) (transport.AuthMethod, error) {
	switch ep.Protocol {
	case "ssh":
		home, _ := Home()
		sshAuth, err := ssh.NewPublicKeysFromFile("git", path.Join(home, "/.ssh/id_rsa"), "")
		if err != nil {
			return nil, err
		}
		return sshAuth, nil
	case "http", "https":
		//return http.NewBasicAuth(csi.User, csi.Password), nil
	}
	return nil, nil
}

// gitPullInternal git pull code 内部实现（不加锁）
func gitPullInternal(csi CodeSourceInfo, sourceDir string, logger event.Logger, timeout int) (*git.Repository, string, error) {
	GetPrivateFileParam := csi.TenantID
	flag := true
Loop:
	if logger != nil {
		logger.Info(fmt.Sprintf("Start pull source code from %s", csi.RepositoryURL), map[string]string{"step": "clone_code"})
	}
	if timeout < 1 {
		timeout = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*time.Duration(timeout))
	defer cancel()
	writer := logger.GetWriter("progress", "debug")
	writer.SetFormat(map[string]interface{}{"progress": "%s", "id": "Pull:"})
	opts := &git.PullOptions{
		Progress:     writer,
		SingleBranch: true,
		Depth:        1,
	}
	if csi.Branch != "" {
		opts.ReferenceName = getBranch(csi.Branch)
	}
	ep, err := transport.NewEndpoint(csi.RepositoryURL)
	if err != nil {
		return nil, "", err
	}
	if ep.Protocol == "ssh" {
		publichFile := GetPrivateFile(GetPrivateFileParam)
		sshAuth, auerr := ssh.NewPublicKeysFromFile("git", publichFile, "")
		if auerr != nil {
			errMsg := util.Translation("Create SSH public keys error")
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "pull-code", "status": "failure"})
			}
			return nil, errMsg, auerr
		}
		sshAuth.HostKeyCallbackHelper.HostKeyCallback = netssh.InsecureIgnoreHostKey()
		opts.Auth = sshAuth
	} else {
		// only proxy github
		// but when setting, other request will be proxyed
		if strings.Contains(csi.RepositoryURL, "github.com") && os.Getenv("GITHUB_PROXY") != "" {
			proxyURL, _ := url.Parse(os.Getenv("GITHUB_PROXY"))
			customClient := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
			customClient.Timeout = time.Minute * time.Duration(timeout)
			client.InstallProtocol("https", githttp.NewClient(customClient))
			defer func() {
				client.InstallProtocol("https", githttp.DefaultClient)
			}()
		}
		if csi.User != "" && csi.Password != "" {
			httpAuth := &githttp.BasicAuth{
				Username: csi.User,
				Password: csi.Password,
			}
			opts.Auth = httpAuth
		}
	}
	rs, err := git.PlainOpen(sourceDir)
	if err != nil {
		return nil, "", err
	}
	tree, err := rs.Worktree()
	if err != nil {
		return nil, "", err
	}
	err = tree.PullContext(ctx, opts)
	if err != nil {
		if err == transport.ErrAuthenticationRequired {
			errMsg := fmt.Sprintf("更新代码失败。可能的原因：① 仓库地址错误或不存在；② 仓库为私有，需要配置访问凭证；③ 分支名称错误。请检查仓库地址和分支名称，或配置访问凭证")
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "pull-code", "status": "failure"})
			}
			return rs, errMsg, err
		}
		if err == transport.ErrAuthorizationFailed {
			errMsg := fmt.Sprintf("更新代码发生错误，代码源鉴权失败。")
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "pull-code", "status": "failure"})
			}
			return rs, errMsg, err
		}
		if err == transport.ErrRepositoryNotFound {
			errMsg := fmt.Sprintf("更新代码发生错误，仓库不存在。")
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "pull-code", "status": "failure"})
			}
			return rs, errMsg, err
		}
		if err == transport.ErrEmptyRemoteRepository {
			errMsg := fmt.Sprintf("更新代码发生错误，远程仓库为空。")
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "pull-code", "status": "failure"})
			}
			return rs, errMsg, err
		}
		if err == plumbing.ErrReferenceNotFound {
			errMsg := fmt.Sprintf("%s: %s", util.Translation("Code branch does not exist"), csi.Branch)
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "pull-code", "status": "failure"})
			}
			return rs, errMsg, fmt.Errorf("branch %s is not exist", csi.Branch)
		}
		if strings.Contains(err.Error(), "ssh: unable to authenticate") {
			if flag {
				GetPrivateFileParam = "builder_rsa"
				flag = false
				goto Loop
			}
			errMsg := util.Translation("Remote repository requires SSH key configuration")
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "pull-code", "status": "failure"})
			}
			return rs, errMsg, err
		}
		if strings.Contains(err.Error(), "context deadline exceeded") {
			errMsg := util.Translation("Pull code timeout")
			if logger != nil {
				logger.Error(errMsg, map[string]string{"step": "pull-code", "status": "failure"})
			}
			return rs, errMsg, err
		}
		if err == git.NoErrAlreadyUpToDate {
			return rs, "", nil
		}
	}
	return rs, "", err
}

// GitPull git pull code（带锁的外部接口）
func GitPull(csi CodeSourceInfo, sourceDir string, logger event.Logger, timeout int) (*git.Repository, string, error) {
	// 对目录加锁，防止并发 Git 操作冲突
	globalDirLockManager.lock(sourceDir)
	defer globalDirLockManager.unlock(sourceDir)

	return gitPullInternal(csi, sourceDir, logger, timeout)
}

// GitCloneOrPull if code exist in local,use git pull.
func GitCloneOrPull(csi CodeSourceInfo, sourceDir string, logger event.Logger, timeout int) (*git.Repository, string, error) {
	// 对目录加锁，防止并发 Git 操作冲突
	// 确保检查 .git 目录和执行 pull/clone 操作是原子的
	globalDirLockManager.lock(sourceDir)
	defer globalDirLockManager.unlock(sourceDir)

	if ok, err := util.FileExists(path.Join(sourceDir, ".git")); err == nil && ok && !strings.HasPrefix(csi.Branch, "tag:") {
		re, msg, err := gitPullInternal(csi, sourceDir, logger, timeout)
		if err == nil && re != nil {
			return re, msg, nil
		}
		logrus.Error("git pull source code error,", err.Error())
	}
	// empty the sourceDir
	if reerr := os.RemoveAll(sourceDir); reerr != nil {
		logrus.Error("empty the source code dir error,", reerr.Error())
		if logger != nil {
			logger.Error(util.Translation("Clear code directory failed"), map[string]string{"step": "clone-code", "status": "failure"})
		}
	}
	return gitCloneInternal(csi, sourceDir, logger, timeout)
}

// GitCheckout checkout the specified branch
func GitCheckout(sourceDir, branch string) error {
	// option := git.CheckoutOptions{
	// 	Branch: getBranch(branch),
	// }
	return nil
}
func getBranch(branch string) plumbing.ReferenceName {
	if strings.HasPrefix(branch, "tag:") {
		return plumbing.ReferenceName(fmt.Sprintf("refs/tags/%s", branch[4:]))
	}
	return plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branch))
}

// GetLastCommit get last commit info
// get commit by head reference
func GetLastCommit(re *git.Repository) (*object.Commit, error) {
	ref, err := re.Head()
	if err != nil {
		return nil, err
	}
	return re.CommitObject(ref.Hash())
}

// GetPrivateFile 获取私钥文件地址
func GetPrivateFile(tenantID string) string {
	home, _ := Home()
	if home == "" {
		home = "/root"
	}
	if ok, _ := util.FileExists(path.Join(home, "/.ssh/"+tenantID)); ok {
		return path.Join(home, "/.ssh/"+tenantID)
	}
	if ok, _ := util.FileExists(path.Join(home, "/.ssh/builder_rsa")); ok {
		return path.Join(home, "/.ssh/builder_rsa")
	}
	return path.Join(home, "/.ssh/id_rsa")

}

// GetPublicKey 获取公钥
func GetPublicKey(tenantID string) string {
	home, _ := Home()
	if home == "" {
		home = "/root"
	}
	PublicKey := tenantID + ".pub"
	PrivateKey := tenantID

	if ok, _ := util.FileExists(path.Join(home, "/.ssh/"+PublicKey)); ok {
		body, _ := ioutil.ReadFile(path.Join(home, "/.ssh/"+PublicKey))
		return string(body)
	}
	Private, Public, err := MakeSSHKeyPair()
	if err != nil {
		logrus.Error("MakeSSHKeyPairError:", err)
	}
	sshDir := path.Join(home, ".ssh")
	// 确保目录存在
	err = os.MkdirAll(sshDir, 0700)
	if err != nil {
		logrus.Errorf("Failed to create directory: %v\n", err)
		return ""
	}
	PrivateKeyFile, err := os.Create(path.Join(home, "/.ssh/"+PrivateKey))
	if err != nil {
		logrus.Errorf("create private key failure: %v", err)
		return ""
	} else {
		_, err = PrivateKeyFile.WriteString(Private)
		if err != nil {
			logrus.Errorf("write private key failure: %v", err)
			return ""
		}
	}
	PublicKeyFile, err := os.Create(path.Join(home, "/.ssh/"+PublicKey))
	if err != nil {
		logrus.Errorf("create public key failure: %v", err)
	} else {
		_, err = PublicKeyFile.WriteString(Public)
		if err != nil {
			logrus.Errorf("write public key failure: %v", err)
			return ""
		}
	}
	body, _ := ioutil.ReadFile(path.Join(home, "/.ssh/"+PublicKey))
	return string(body)

}

// GenerateKey GenerateKey
func GenerateKey(bits int) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	private, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, err
	}
	return private, &private.PublicKey, nil

}

// EncodePrivateKey EncodePrivateKey
func EncodePrivateKey(private *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Bytes: x509.MarshalPKCS1PrivateKey(private),
		Type:  "RSA PRIVATE KEY",
	})
}

// EncodeSSHKey EncodeSSHKey
func EncodeSSHKey(public *rsa.PublicKey) ([]byte, error) {
	publicKey, err := sshkey.NewPublicKey(public)
	if err != nil {
		return nil, err
	}
	return sshkey.MarshalAuthorizedKey(publicKey), nil
}

// MakeSSHKeyPair make ssh key
func MakeSSHKeyPair() (string, string, error) {

	pkey, pubkey, err := GenerateKey(2048)
	if err != nil {
		return "", "", err
	}

	pub, err := EncodeSSHKey(pubkey)
	if err != nil {
		return "", "", err
	}

	return string(EncodePrivateKey(pkey)), string(pub), nil
}
