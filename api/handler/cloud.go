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

package handler

import (
	"crypto/md5"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"

	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/cmd/api/option"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
)

//CloudAction  cloud action struct
type CloudAction struct {
	RegionTag string
	APISSL    bool
	CAPath    string
	KeyPath   string
}

//CreateCloudManager get cloud manager
func CreateCloudManager(conf option.Config) *CloudAction {
	return &CloudAction{
		APISSL:    conf.APISSL,
		RegionTag: conf.RegionTag,
		CAPath:    conf.APICertFile,
		KeyPath:   conf.APIKeyFile,
	}
}

//TokenDispatcher token
func (c *CloudAction) TokenDispatcher(gt *api_model.GetUserToken) (*api_model.TokenInfo, *util.APIHandleError) {
	//TODO: product token, 启动api时需要添加该参数
	//token包含 eid，数据中心标识，可控范围，有效期
	ti := &api_model.TokenInfo{
		EID: gt.Body.EID,
	}
	token := c.createToken(gt)
	var oldToken string
	tokenInfos, err := db.GetManager().RegionUserInfoDao().GetTokenByEid(gt.Body.EID)
	if err != nil {
		if err.Error() == gorm.ErrRecordNotFound.Error() {
			goto CREATE
		}
		return nil, util.CreateAPIHandleErrorFromDBError("get user token info", err)
	}
	ti.CA = tokenInfos.CA
	//ti.Key = tokenInfos.Key
	ti.Token = token
	oldToken = tokenInfos.Token
	tokenInfos.Token = token
	tokenInfos.ValidityPeriod = gt.Body.ValidityPeriod
	if err := db.GetManager().RegionUserInfoDao().UpdateModel(tokenInfos); err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("recreate region user info", err)
	}
	tokenInfos.CA = ""
	tokenInfos.Key = ""
	GetTokenIdenHandler().DeleteTokenFromMap(oldToken, tokenInfos)
	return ti, nil
CREATE:
	ti.Token = token
	logrus.Debugf("create token %v", token)
	rui := &dbmodel.RegionUserInfo{
		EID:            gt.Body.EID,
		RegionTag:      c.RegionTag,
		APIRange:       gt.Body.Range,
		ValidityPeriod: gt.Body.ValidityPeriod,
		Token:          token,
	}
	if c.APISSL {
		ca, key, err := c.CertDispatcher(gt)
		if err != nil {
			return nil, util.CreateAPIHandleError(500, fmt.Errorf("create ca or key error"))
		}
		rui.CA = string(ca)
		rui.Key = string(key)
		ti.CA = string(ca)
		//ti.Key = string(key)
	}
	if gt.Body.Range == "" {
		rui.APIRange = dbmodel.SERVERSOURCE
	}
	GetTokenIdenHandler().AddTokenIntoMap(rui)
	if err := db.GetManager().RegionUserInfoDao().AddModel(rui); err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("create region user info", err)
	}
	return ti, nil
}

//GetTokenInfo GetTokenInfo
func (c *CloudAction) GetTokenInfo(eid string) (*dbmodel.RegionUserInfo, *util.APIHandleError) {
	tokenInfos, err := db.GetManager().RegionUserInfoDao().GetTokenByEid(eid)
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("get user token info", err)
	}
	return tokenInfos, nil
}

//UpdateTokenTime UpdateTokenTime
func (c *CloudAction) UpdateTokenTime(eid string, vd int) *util.APIHandleError {
	tokenInfos, err := db.GetManager().RegionUserInfoDao().GetTokenByEid(eid)
	if err != nil {
		return util.CreateAPIHandleErrorFromDBError("get user token info", err)
	}
	tokenInfos.ValidityPeriod = vd
	err = db.GetManager().RegionUserInfoDao().UpdateModel(tokenInfos)
	if err != nil {
		return util.CreateAPIHandleErrorFromDBError("update user token info", err)
	}
	return nil
}

//CertDispatcher Cert
func (c *CloudAction) CertDispatcher(gt *api_model.GetUserToken) ([]byte, []byte, error) {
	cert, err := analystCaKey(c.CAPath, "ca")
	if err != nil {
		return nil, nil, err
	}
	//解析私钥
	keyFile, err := analystCaKey(c.KeyPath, "key")
	if err != nil {
		return nil, nil, err
	}
	//keyFile = keyFile.(rsa.PrivateKey)

	validHourTime := (gt.Body.ValidityPeriod - gt.Body.BeforeTime)
	cer := &x509.Certificate{
		SerialNumber: big.NewInt(1), //证书序列号
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("%s@%d", gt.Body.EID, time.Now().Unix()),
			Locality:   []string{c.RegionTag},
		},
		NotBefore:             time.Now(),                                                                 //证书有效期开始时间
		NotAfter:              time.Now().Add(time.Second * time.Duration(validHourTime)),                 //证书有效期结束时间
		BasicConstraintsValid: true,                                                                       //基本的有效性约束
		IsCA:                  false,                                                                      //是否是根证书
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}, //证书用途(客户端认证，数据加密)
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageDataEncipherment,
		//EmailAddresses: []string{"region@test.com"},
		//IPAddresses:    []net.IP{net.ParseIP("192.168.1.59")},
	}
	priKey, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	ca, err := x509.CreateCertificate(crand.Reader, cer, cert.(*x509.Certificate), &priKey.PublicKey, keyFile)
	if err != nil {
		return nil, nil, err
	}

	//编码证书文件和私钥文件
	caPem := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ca,
	}
	ca = pem.EncodeToMemory(caPem)

	buf := x509.MarshalPKCS1PrivateKey(priKey)
	keyPem := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: buf,
	}
	key := pem.EncodeToMemory(keyPem)
	return ca, key, nil
}

func analystCaKey(path, kind string) (interface{}, error) {
	fileInfo, err := ioutil.ReadFile(path)
	if err != nil {
		return "", nil
	}
	fileBlock, _ := pem.Decode(fileInfo)
	switch kind {
	case "ca":
		cert, err := x509.ParseCertificate(fileBlock.Bytes)
		if err != nil {
			return "", nil
		}
		return cert, nil
	case "key":
		praKey, err := x509.ParsePKCS1PrivateKey(fileBlock.Bytes)
		if err != nil {
			return "", nil
		}
		return praKey, nil
	}
	return "", nil
}

func (c *CloudAction) createToken(gt *api_model.GetUserToken) string {
	fullStr := fmt.Sprintf("%s-%s-%s-%d-%d", gt.Body.EID, c.RegionTag, gt.Body.Range, gt.Body.ValidityPeriod, int(time.Now().Unix()))
	h := md5.New()
	h.Write([]byte(fullStr))
	return hex.EncodeToString(h.Sum(nil))
}
