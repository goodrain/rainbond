package db

import (
	"testing"
	"time"

	dbconfig "github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/model"
)

func TestMysql(t *testing.T) {
	connectionInfo := "fanyangyang:root@tcp(127.0.0.1:3308)/region"
	t.Run("region_api_class", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.RegionAPIClass{
			ClassLevel: "server_source",
			Prefix:     "/v2/builder",
			URI:        "",
			Alias:      "",
			Remark:     "",
		}
		// 增加
		err := GetManager().RegionAPIClassDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		list, err := GetManager().RegionAPIClassDao().GetPrefixesByClass("server_source")
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 1 {
			t.Fatal("list len is not equal to 1")
		}
		// 修改
		data.Alias = "alias"
		err = GetManager().RegionAPIClassDao().UpdateModel(data)
		if err != nil {
			t.Fatal(err)
		}

		// 删除
		err = GetManager().RegionAPIClassDao().DeletePrefixInClass("server_source", "/v2/builder")
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("region_app_status", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.AppStatus{
			EventID:     "03087c3496294acb97babf8c4978fec1",
			Format:      "rainbond-app",
			SourceDir:   "/grdata/app/import/03087c3496294acb97babf8c4978fec1",
			Apps:        "5.1.7export-v1.0.zip:failed",
			Status:      "cleaned",
			TarFileHref: "/v2/app/download/rainbond-app/test-v1.1.zip",
			Metadata:    "",
		}
		// 增加
		err := GetManager().AppDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().AppDao().GetByEventId("03087c3496294acb97babf8c4978fec1")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		data.Format = "docker-compose"
		data.SourceDir = "/v2/app/download/docker-compose/5.1.6export-v1.0.zip"
		err = GetManager().AppDao().UpdateModel(data)
		if err != nil {
			t.Fatal(err)
		}

		// 删除
		err = GetManager().AppDao().DeleteModelByEventId("03087c3496294acb97babf8c4978fec1")
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("region_app_backup", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.AppBackup{
			EventID:    "547515b3d12f43db9bb661d4f51c7c9e",
			BackupID:   "d5859360d9d94e488f299fc9fbece7eb",
			GroupID:    "46180693a12a4b9bbafbd160d7d6a2bd",
			Status:     "success",
			Version:    "20181228172011",
			SourceDir:  "/app_publish/a5qw69mz/backup/a4baa0891e914b17a3b8976505cc6bf9_20181228172011/metadata_data.zip",
			SourceType: "local",
			BackupMode: "full-offline",
			BuckupSize: 651225864,
			Deleted:    false,
		}
		// 增加
		err := GetManager().AppBackupDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().AppBackupDao().GetAppBackup("d5859360d9d94e488f299fc9fbece7eb")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.Deleted = true
		err = GetManager().AppBackupDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}

		// 删除
		err = GetManager().AppBackupDao().DeleteAppBackup("d5859360d9d94e488f299fc9fbece7eb")
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("user_region_info", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.RegionUserInfo{
			EID:            "f14b3e7d22f441539f369162546b95d7",
			APIRange:       "server_source",
			RegionTag:      "中文tag",
			ValidityPeriod: 1581920313,
			Token:          "053f7187880d618133480d6512345678",
			CA:             "-----BEGIN CERTIFICATE-----balabalabalablaba-----END CERTIFICATE-----",
			Key:            "-----BEGIN PRIVATE KEY-----dededededededetatatatatata-----END PRIVATE KEY-----",
		}
		// 增加
		err := GetManager().RegionUserInfoDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().RegionUserInfoDao().GetTokenByEid("f14b3e7d22f441539f369162546b95d7")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.Token = "JpE8ambU5vZnffA1ghdPaIeEqqk12312"
		err = GetManager().RegionUserInfoDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("tenant_services_codecheck", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.CodeCheckResult{
			ServiceID:       "b0baf29788500c429a242185605f8cf6",
			Condition:       "",
			Language:        "Python",
			CheckType:       "",
			GitURL:          "https://github.com/goodrain/rainbond-console.git",
			CodeVersion:     "msater",
			GitProjectId:    "0",
			CodeFrom:        "git",
			URLRepos:        "",
			DockerFileReady: false,
			InnerPort:       "5000",
			VolumeMountPath: "/",
			BuildImageName:  "rainbond-console",
			PortList:        "['3000']",
			VolumeList:      "",
		}
		// 增加
		err := GetManager().CodeCheckResultDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().CodeCheckResultDao().GetCodeCheckResult("b0baf29788500c429a242185605f8cf6")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.Language = "go"
		err = GetManager().CodeCheckResultDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("tenant_services_event", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.ServiceEvent{
			EventID:     "788c4e8031484c79a23054bd626ed5d2",
			TenantID:    "3b1f4056edb2411cac3f993fde23a85f",
			ServiceID:   "ac96eed7c78dcda7106bbcd63c78816a",
			Target:      "service",
			TargetID:    "ac96eed7c78dcda7106bbcd63c78816a",
			RequestBody: `{"kind": "build_from_source_code", "envs": {"PROCFILE": "web: java $JAVA_OPTS -jar target/java-maven-demo-0.0.1.jar", "PROC_ENV": "{"procfile": "", "dependencies": {}, "language": "Java-maven", "runtimes": "1.8"}"}, "operator": "lius", "code_info": {"lang": "Java-maven", "cmd": "start web", "branch": "master", "repo_url": "https://github.com/goodrain/java-maven-demo.git", "server_type": "git"}, "action": "upgrade", "service_id": "866cf9d9ed37b98e50581ee76a72d597", "configs": {}}`,
			UserName:    "unknown",
			StartTime:   "2019-10-24T18:21:42+08:00",
			EndTime:     "2019-10-24T18:21:42+08:00",
			OptType:     "build-service",
			SynType:     0,
			Status:      "",
			FinalStatus: "",
			Message:     "",
		}
		// 增加
		err := GetManager().ServiceEventDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().ServiceEventDao().GetEventByEventID("788c4e8031484c79a23054bd626ed5d2")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}

		// 修改
		ret.Status = "success"
		ret.FinalStatus = "complete"
		err = GetManager().ServiceEventDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}

		// 删除
		err = GetManager().ServiceEventDao().DelEventByServiceID("788c4e8031484c79a23054bd626ed5d2")
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("region_notification_event", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.NotificationEvent{
			Kind:          "service",
			KindID:        "560089dc09fb62e294f60243d661c8f1",
			Hash:          "8db0544517a32a464080a192a39089bf7a81e02d3f22c077837c25ef11e19a63",
			Type:          "UnNormal",
			Message:       "Container 560089dc09fb62e294f60243d661c8f1 restart",
			Reason:        "Error",
			Count:         0,
			LastTime:      time.Now(),
			FirstTime:     time.Now(),
			IsHandle:      false,
			HandleMessage: "",
			ServiceName:   "gr61c8f1",
			TenantName:    "4f6ad5fbb2f844d7b1ba12df520c15a7",
		}
		// 增加
		err := GetManager().NotificationEventDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().NotificationEventDao().GetNotificationEventByHash("8db0544517a32a464080a192a39089bf7a81e02d3f22c077837c25ef11e19a63")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.IsHandle = true
		ret.Count = 1
		err = GetManager().NotificationEventDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("gateway_certificate", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.Certificate{
			UUID:            "e2ff61231231237643c28b1d50b0a7af6b70",
			CertificateName: "cert-494457ab",
			Certificate:     "-----BEGIN CERTIFICATE-----babalalbaxiaomixian -----END CERTIFICATE-----",
			PrivateKey:      "-----BEGIN RSA PRIVATE KEY-----dedededetatataxioabufa -----END RSA PRIVATE KEY-----",
		}
		// 增加
		err := GetManager().CertificateDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().CertificateDao().GetCertificateByID("e2ff61231231237643c28b1d50b0a7af6b70")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.CertificateName = "newcert-4949"
		err = GetManager().CertificateDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}
		ret, err = GetManager().CertificateDao().GetCertificateByID("e2ff61231231237643c28b1d50b0a7af6b70")

		// 删除
		err = GetManager().CertificateDao().DeleteCertificateByID("e2ff61231231237643c28b1d50b0a7af6b70")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("gateway_rule_extension", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.RuleExtension{
			UUID:   "21978cafd9964b15ad9ad6cf90cfa67d",
			RuleID: "3a150662435aa6999f32951173f49a1e",
			Key:    "httptohttps",
			Value:  "true",
		}
		// 增加
		err := GetManager().RuleExtensionDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().RuleExtensionDao().GetRuleExtensionByRuleID("3a150662435aa6999f32951173f49a1e")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil || len(ret) != 1 {
			t.Fatal("ret len is not equal to 1")
		}
		// 修改
		ret[0].Key = "lb-type"
		ret[0].Value = "round-robin"
		err = GetManager().RuleExtensionDao().UpdateModel(ret[0])
		if err != nil {
			t.Fatal(err)
		}

		// 删除
		err = GetManager().RuleExtensionDao().DeleteRuleExtensionByRuleID("3a150662435aa6999f32951173f49a1e")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("gateway_http_rule", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.HTTPRule{
			UUID:          "baf42f546bd50e4928dafc6407ee9203",
			ServiceID:     "749a27a536850fb732a0d310fd84693e",
			ContainerPort: 12306,
			Domain:        "5000.gr84693e.mkyjeqbw.0196bd.grapps.cn",
			Path:          "",
			Header:        "",
			Cookie:        "",
			Weight:        0,
			IP:            "",
			CertificateID: "",
		}
		// 增加
		err := GetManager().HTTPRuleDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().HTTPRuleDao().GetHTTPRuleByID("baf42f546bd50e4928dafc6407ee9203")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.IP = "127.0.0.1"
		ret.Header = "Host:127.0.0.1"
		err = GetManager().HTTPRuleDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}

		// 删除
		err = GetManager().HTTPRuleDao().DeleteHTTPRuleByID("baf42f546bd50e4928dafc6407ee9203")
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("gateway_tcp_rule", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.TCPRule{
			UUID:          "8e8e456e85dcb0fc51170f1f56e9fc91",
			ServiceID:     "ec20cb954c4b4e64b3b295cc17c07b5e",
			ContainerPort: 5000,
			IP:            "127.0.0.1",
			Port:          4000,
		}
		// 增加
		err := GetManager().TCPRuleDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().TCPRuleDao().GetTCPRuleByID("8e8e456e85dcb0fc51170f1f56e9fc91")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.IP = "0.0.0.0"
		ret.Port = 3306
		err = GetManager().TCPRuleDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}

		// 删除
		err = GetManager().TCPRuleDao().DeleteByID("8e8e456e85dcb0fc51170f1f56e9fc91")
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("gateway_rule_config", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.GwRuleConfig{
			RuleID: "88cff5cc58285409744081b7cc456fbf",
			Key:    "proxy-send-timeout",
			Value:  "60",
		}
		// 增加
		err := GetManager().GwRuleConfigDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().GwRuleConfigDao().ListByRuleID("88cff5cc58285409744081b7cc456fbf")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil || len(ret) != 1 {
			t.Fatal("ret len is not equal to 1")
		}
		// TODO ret is list
		// 修改
		ret[0].Value = "70"
		err = GetManager().GwRuleConfigDao().UpdateModel(ret[0])
		if err != nil {
			t.Fatal(err)
		}

		// 删除
		err = GetManager().GwRuleConfigDao().DeleteByRuleID("88cff5cc58285409744081b7cc456fbf")
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("local_scheduler", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.LocalScheduler{
			ServiceID: "ec20cb954c4b4e64b3b295cc17c07b5e",
			NodeIP:    "192.168.2.203",
			PodName:   "unknown",
		}
		// 增加
		err := GetManager().LocalSchedulerDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().LocalSchedulerDao().GetLocalScheduler("ec20cb954c4b4e64b3b295cc17c07b5e")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil || len(ret) != 1 {
			t.Fatal("ret len is not equal to 1")
		}
		// TODO ret is list
		// 修改
		ret[0].PodName = "service-pod-gr12d83"
		err = GetManager().LocalSchedulerDao().UpdateModel(ret[0])
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("rainbond_license", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.LicenseInfo{
			Token:   "zcc1fegalctjeiumqwf7dhxi",
			License: "hwics0867bJNvm3KIDfw2ZNaDwTXcJ38dc0S4AoepQml9ueaAkYYbKwh2jxPoS5k3Uir0+uvsm+npGIroJexif6BvuNpEXZlHENtuxh9As8TKH5bPb1ixAaKIi/OXaM2okhFMY7V6AYA+NxRG76QflnHU",
			Label:   "first lable",
		}
		// 增加
		err := GetManager().LicenseDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().LicenseDao().ListLicenses()
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil || len(ret) != 1 {
			t.Fatal("ret len is not equal to 1")
		}
		// TODO ret is list
		// 修改
		ret[0].License = "hwics0867bJNvm3KIDfw2ZNaDwTXcJ38dc0S4AoepQml9ueaAkYYbKwh2jxPoS5k3Uir0+uvsm+npGIroJexif6BvuNpEXZlHENtuxh9As8TKH5bPb1ixAaKIi/OXaM2okhFMY7V6AYA+NxRG76QflnHUy5uZLkCgYEArkeqsMJvK0bpalgBXNhi5U6ObiCBSPWUm906G53spZsnLz2"
		ret[0].Label = "updated label"
		err = GetManager().LicenseDao().UpdateModel(ret[0])
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tenant_plugin", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.TenantPlugin{
			PluginID:    "1ddd63a76cb64308b725b531f528313c",
			PluginName:  "gr1ddd63",
			PluginInfo:  "chinese info is not useful here, waiting fix",
			ImageURL:    "goodrain.me/goodrain/mesh_plugin:latest_20180703145620",
			GitURL:      "",
			BuildModel:  "image",
			PluginModel: "net-plugin:down",
			TenantID:    "4f6ad5fbb2f844d7b1ba12df520c15a7",
			Domain:      "jdgn6pk5",
			CodeFrom:    "",
		}
		// 增加
		err := GetManager().TenantPluginDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().TenantPluginDao().GetPluginByID("1ddd63a76cb64308b725b531f528313c", "4f6ad5fbb2f844d7b1ba12df520c15a7")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.Domain = "newdomain"
		ret.CodeFrom = "market"
		ret.ImageURL = "goodrain.me/goodrain/mesh_plugin:latest"
		err = GetManager().TenantPluginDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().TenantPluginDao().DeletePluginByID("1ddd63a76cb64308b725b531f528313c", "4f6ad5fbb2f844d7b1ba12df520c15a7")
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("tenant_plugin_build_version", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.TenantPluginBuildVersion{
			VersionID:       "20180703145620",
			DeployVersion:   "201922120312584760823",
			PluginID:        "1ddd63a76cb64308b725b531f528313c",
			Kind:            "image",
			BaseImage:       "goodrain.me/goodrain/mesh_plugin:latest",
			BuildLocalImage: "goodrain.me/goodrain/mesh_plugin:latest_201929398",
			BuildTime:       "2019-02-21T20:31:25+08:00",
			Repo:            "",
			GitURL:          "",
			Info:            "",
			Status:          "complete",
			ContainerCPU:    1,
			ContainerMemory: 2048,
			ContainerCMD:    "",
		}
		// 增加
		err := GetManager().TenantPluginBuildVersionDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().TenantPluginBuildVersionDao().GetBuildVersionByDeployVersion("1ddd63a76cb64308b725b531f528313c", "20180703145620", "201922120312584760823")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.BuildLocalImage = "goodrain.me/goodrain/mesh_plugin:latest_201910301107"
		err = GetManager().TenantPluginBuildVersionDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().TenantPluginBuildVersionDao().DeleteBuildVersionByPluginID("1ddd63a76cb64308b725b531f528313c")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tenant_plugin_version_env", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.TenantPluginVersionEnv{
			PluginID:  "eecbb7ed9ac24f4dbf96053197d41b17",
			EnvName:   "ES_PORT",
			EnvValue:  "9200",
			ServiceID: "f6f4047c6737143eb48e123d3b9cc980",
		}
		// 增加
		err := GetManager().TenantPluginVersionENVDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().TenantPluginVersionENVDao().GetVersionEnvByEnvName("f6f4047c6737143eb48e123d3b9cc980", "eecbb7ed9ac24f4dbf96053197d41b17", "ES_PORT")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.EnvValue = "9300"
		err = GetManager().TenantPluginVersionENVDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().TenantPluginVersionENVDao().DeleteEnvByEnvName("ES_PORT", "eecbb7ed9ac24f4dbf96053197d41b17", "f6f4047c6737143eb48e123d3b9cc980")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tenant_plugin_version_config", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.TenantPluginVersionDiscoverConfig{
			PluginID:  "af30e5124e704752819b6e78a1984851",
			ServiceID: "94203e159ad6428f9bb5308fab74ef86",
			ConfigStr: `{"base_ports":[{"service_alias":"gr74ef86","service_id":"94203e159ad6428f9bb5308fab74ef86","port":3306,"listen_port":0,"protocol":"mysql","options":{"OPEN":"YES"}}],"base_services":[],"base_normal":{"options":null}}`,
		}
		// 增加
		err := GetManager().TenantPluginVersionConfigDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().TenantPluginVersionConfigDao().GetPluginConfig("94203e159ad6428f9bb5308fab74ef86", "af30e5124e704752819b6e78a1984851")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.ConfigStr = `{"base_ports":[{"service_alias":"gr74ef86","service_id":"94203e159ad6428f9bb5308fab74ef86","port":3307,"listen_port":0,"protocol":"mysql","options":{"OPEN":"NO"}}],"base_services":[],"base_normal":{"options":null}}`
		err = GetManager().TenantPluginVersionConfigDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().TenantPluginVersionConfigDao().DeletePluginConfig("94203e159ad6428f9bb5308fab74ef86", "af30e5124e704752819b6e78a1984851")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tenant_service_plugin_relation", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.TenantServicePluginRelation{
			VersionID:       "20190223100552",
			PluginID:        "c051db61692547149937e7c93ed71c24",
			ServiceID:       "dde947ccc8cc6fe46c734dddd13698ab",
			PluginModel:     "analyst-plugin:perf",
			ContainerCPU:    1,
			ContainerMemory: 2038,
			Switch:          false,
		}
		// 增加
		err := GetManager().TenantServicePluginRelationDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().TenantServicePluginRelationDao().GetALLRelationByServiceID("dde947ccc8cc6fe46c734dddd13698ab")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil || len(ret) != 1 {
			t.Fatal("ret len is not equal to 1")
		}
		// 修改
		ret[0].Switch = true
		ret[0].ContainerCPU = 2
		err = GetManager().TenantServicePluginRelationDao().UpdateModel(ret[0])
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().TenantServicePluginRelationDao().DeleteALLRelationByPluginID("c051db61692547149937e7c93ed71c24")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tenant_services_stream_plugin_port", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.TenantServicesStreamPluginPort{
			TenantID:      "157b2015f1c74b219f38849f7857d382",
			ServiceID:     "e8adef6845db43c5afc1c9dca4fd6be8",
			PluginModel:   "net-plugin:up",
			ContainerPort: 4000,
			PluginPort:    2000,
		}
		// 增加
		err := GetManager().TenantServicesStreamPluginPortDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().TenantServicesStreamPluginPortDao().GetPluginMappingPortByServiceIDAndContainerPort("e8adef6845db43c5afc1c9dca4fd6be8", "net-plugin:up", 4000)
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.PluginPort = 8080
		ret.ContainerPort = 8000
		err = GetManager().TenantServicesStreamPluginPortDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().TenantServicesStreamPluginPortDao().DeleteAllPluginMappingPortByServiceID("e8adef6845db43c5afc1c9dca4fd6be8")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("region_protocols", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.RegionProcotols{
			ProtocolGroup: "stream",
			ProtocolChild: "mysql",
			APIVersion:    "v2",
			IsSupport:     false,
		}
		// 增加
		err := GetManager().RegionProcotolsDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().RegionProcotolsDao().GetProtocolGroupByProtocolChild("v2", "mysql")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.IsSupport = true
		err = GetManager().RegionProcotolsDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tenants", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.Tenants{
			Name:        "ew4xpfs8",
			UUID:        "4f6ad5fbb2f844d7b1ba12df520c15a7",
			EID:         "bf952b88223a44d7adbd260af7b6296d",
			LimitMemory: 4096,
		}
		// 增加
		err := GetManager().TenantDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().TenantDao().GetALLTenants("")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil || len(ret) != 1 {
			t.Fatal("ret len is not equal to 1")
		}
		// 修改
		ret[0].LimitMemory = 8192
		err = GetManager().TenantDao().UpdateModel(ret[0])
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tenant_services", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.TenantServices{
			TenantID:        "b73e01d3b83546cc8d33d60a1618a79f",
			ServiceID:       "b0baf29788500c429a242185605f8cf6",
			ServiceKey:      "",
			ServiceAlias:    "gr5f8cf6",
			ServiceName:     "gr5f8cf6",
			Comment:         "application info",
			ContainerCPU:    80,
			ContainerMemory: 512,
			UpgradeMethod:   "Rolling",
			ExtendMethod:    "stateless",
			Replicas:        1,
			DeployVersion:   "20190330184526",
			Category:        "application",
			CurStatus:       "undeploy",
			Status:          0,
			EventID:         "",
			Namespace:       "goodrain",
			UpdateTime:      time.Now(),
			ServiceOrigin:   "assistant",
			Kind:            "internal",
		}
		// 增加
		err := GetManager().TenantServiceDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().TenantServiceDao().GetServicesAllInfoByTenantID("b73e01d3b83546cc8d33d60a1618a79f")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil || len(ret) != 1 {
			t.Fatal("ret len is not equal to 1")
		}
		// 修改
		ret[0].Status = 1
		ret[0].CurStatus = "running"
		err = GetManager().TenantServiceDao().UpdateModel(ret[0])
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().TenantServiceDao().DeleteServiceByServiceID("b0baf29788500c429a242185605f8cf6")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tenant_services_delete", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		now := time.Now()
		data := &model.TenantServicesDelete{
			TenantID:        "b73e01d3b83546cc8d33d60a1618a79f",
			ServiceID:       "b0baf29788500c429a242185605f8cf6",
			ServiceKey:      "",
			ServiceAlias:    "gr5f8cf6",
			ServiceName:     "gr5f8cf6",
			Comment:         "application info",
			ContainerCPU:    80,
			ContainerMemory: 512,
			UpgradeMethod:   "Rolling",
			ExtendMethod:    "stateless",
			Replicas:        1,
			DeployVersion:   "20190330184526",
			Category:        "application",
			CurStatus:       "undeploy",
			Status:          0,
			EventID:         "",
			Namespace:       "goodrain",
			UpdateTime:      time.Now(),
			ServiceOrigin:   "assistant",
			Kind:            "internal",
		}
		// 增加
		err := GetManager().TenantServiceDeleteDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().TenantServiceDeleteDao().GetTenantServicesDeleteByCreateTime(now.Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil || len(ret) != 1 {
			t.Fatal("ret len is not equal to 1")
		}
		// 修改
		ret[0].Status = 1
		ret[0].CurStatus = "running"
		err = GetManager().TenantServiceDeleteDao().UpdateModel(ret[0])
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().TenantServiceDeleteDao().DeleteTenantServicesDelete(ret[0])
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("tenant_lb_mapping_port", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.TenantServiceLBMappingPort{
			ServiceID:     "3ceb45680e2e8b83197c56a05d7cdbaf",
			Port:          5000,
			ContainerPort: 5000,
		}
		// 增加
		err := GetManager().TenantServiceLBMappingPortDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().TenantServiceLBMappingPortDao().GetLBMappingPortByServiceIDAndPort("3ceb45680e2e8b83197c56a05d7cdbaf", 5000)
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.Port = 5999
		err = GetManager().TenantServiceLBMappingPortDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().TenantServiceLBMappingPortDao().DELServiceLBMappingPortByServiceID("3ceb45680e2e8b83197c56a05d7cdbaf")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tenant_services_relation", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.TenantServiceRelation{
			TenantID:          "b73e01d3b83546cc8d33d60a1618a79f",
			ServiceID:         "e9fef8bb75e3854bb6de4e0367417a7d",
			DependServiceID:   "85905961a178441cb49f96c7943ae2bf",
			DependServiceType: "application",
			DependOrder:       0,
		}
		// 增加
		err := GetManager().TenantServiceRelationDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().TenantServiceRelationDao().GetTenantServiceRelations("e9fef8bb75e3854bb6de4e0367417a7d")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil || len(ret) != 1 {
			t.Fatal("ret len is not equal to 1")
		}
		// 修改
		ret[0].DependOrder = 1
		err = GetManager().TenantServiceRelationDao().UpdateModel(ret[0])
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().TenantServiceRelationDao().DELRelationsByServiceID("e9fef8bb75e3854bb6de4e0367417a7d")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tenant_services_envs", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.TenantServiceEnvVar{
			TenantID:      "b73e01d3b83546cc8d33d60a1618a79f",
			ServiceID:     "ff4cd2b8beb64b4eb0e1b95ed1671cec",
			ContainerPort: 0,
			Name:          "PERCONA_MAJOR",
			AttrName:      "PERCONA_MAJOR",
			AttrValue:     "5.5",
			IsChange:      false,
			Scope:         "inner",
		}
		// 增加
		err := GetManager().TenantServiceEnvVarDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().TenantServiceEnvVarDao().GetEnv("ff4cd2b8beb64b4eb0e1b95ed1671cec", "PERCONA_MAJOR")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.AttrValue = "5.6"
		err = GetManager().TenantServiceEnvVarDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().TenantServiceEnvVarDao().DELServiceEnvsByServiceID("ff4cd2b8beb64b4eb0e1b95ed1671cec")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tenant_services_mnt_relation", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.TenantServiceMountRelation{
			TenantID:        "4f6ad5fbb2f844d7b1ba12df520c15a7",
			ServiceID:       "e85a0344e4e94e4fae035335cb7f6b65",
			DependServiceID: "10924060321b4f29b7de3c086de5c13b",
			VolumePath:      "/home/test/upload",
			HostPath:        "/grdata/tenant/b7584c080ad24fafaa812a7739174b50/service/7a6620d5fdae5a98bc5fcd6724466ebe/opt/mule/apps",
			VolumeName:      "GR466EBE_3",
			VolumeType:      "",
		}
		// 增加
		err := GetManager().TenantServiceMountRelationDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().TenantServiceMountRelationDao().GetTenantServiceMountRelationsByService("e85a0344e4e94e4fae035335cb7f6b65")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil || len(ret) != 1 {
			t.Fatal("ret len is not equal to 1")
		}
		// 修改
		ret[0].VolumePath = "/data/config"
		ret[0].HostPath = "/gradata/tenant/12j3j3d/service/3idkd9sern/data/config"
		err = GetManager().TenantServiceMountRelationDao().UpdateModel(ret[0])
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().TenantServiceMountRelationDao().DELTenantServiceMountRelationByServiceID("e85a0344e4e94e4fae035335cb7f6b65")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tenant_services_volume", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.TenantServiceVolume{
			ServiceID:  "85905961a178441cb49f96c7943ae2bf",
			Category:   "app_publish",
			VolumeType: "share-file",
			VolumeName: "GRDCBC2B_2",
			HostPath:   "/grdata/tenant/b73e01d3b83546cc8d33d60a1618a79f/service/85905961a178441cb49f96c7943ae2bf/var/log/mysql",
			VolumePath: "/var/log/mysql",
			IsReadOnly: false,
		}
		// 增加
		err := GetManager().TenantServiceVolumeDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().TenantServiceVolumeDao().GetAllVolumes()
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil || len(ret) != 1 {
			t.Fatal("ret len is not equal to 1")
		}
		// 修改
		ret[0].VolumePath = "/var/mysql/log"
		err = GetManager().TenantServiceVolumeDao().UpdateModel(ret[0])
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().TenantServiceVolumeDao().DelShareableBySID("85905961a178441cb49f96c7943ae2bf")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tenant_service_config_file", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.TenantServiceConfigFile{
			ServiceID:   "125af13d68e0e48a6788c39bc4b4f1cc",
			VolumeName:  "config",
			FileContent: "version: '3' services: nginx: image: nginx",
		}
		// 增加
		err := GetManager().TenantServiceConfigFileDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().TenantServiceConfigFileDao().GetByVolumeName("125af13d68e0e48a6788c39bc4b4f1cc", "config")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.FileContent = "jobs \n -name: 'global'"
		err = GetManager().TenantServiceConfigFileDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().TenantServiceConfigFileDao().DelByServiceID("125af13d68e0e48a6788c39bc4b4f1cc")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tenant_services_label", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.TenantServiceLable{
			ServiceID:  "b0baf29788500c429a242185605f8cf6",
			LabelKey:   "service-type",
			LabelValue: "StatelessServiceType",
		}
		// 增加
		err := GetManager().TenantServiceLabelDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().TenantServiceLabelDao().GetTenantServiceLabel("b0baf29788500c429a242185605f8cf6")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil || len(ret) != 1 {
			t.Fatal("ret is nil")
		}
		// 修改
		ret[0].LabelValue = "new value"
		err = GetManager().TenantServiceLabelDao().UpdateModel(ret[0])
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().TenantServiceLabelDao().DelTenantServiceLabelsByLabelValuesAndServiceID("b0baf29788500c429a242185605f8cf6")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tenant_services_probe", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		var Flag int
		Flag = 0
		data := &model.TenantServiceProbe{
			ServiceID:          "85905961a178441cb49f96c7943ae2bf",
			ProbeID:            "745b7a7d010941219083f379c8d474a8",
			Mode:               "readiness",
			Scheme:             "tcp",
			Path:               "",
			Port:               3306,
			Cmd:                "",
			HTTPHeader:         "",
			InitialDelaySecond: 2,
			PeriodSecond:       3,
			TimeoutSecond:      30,
			IsUsed:             &Flag,
			FailureThreshold:   3,
			SuccessThreshold:   1,
		}
		// 增加
		err := GetManager().ServiceProbeDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().ServiceProbeDao().GetServiceProbes("85905961a178441cb49f96c7943ae2bf")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil || len(ret) != 1 {
			t.Fatal("ret len is not equal to 1")
		}
		// 修改
		ret[0].FailureThreshold = 5
		err = GetManager().ServiceProbeDao().UpdateModel(ret[0])
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().ServiceProbeDao().DelByServiceID("85905961a178441cb49f96c7943ae2bf")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tenant_service_3rd_party_endpoints", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		var flag bool
		flag = false
		data := &model.Endpoint{
			UUID:      "cb4b176d0cc24a41892536fd2dcce457",
			ServiceID: "de2bca6d5089d2274aba882c08038429",
			IP:        "10.10.10.10",
			Port:      3306,
			IsOnline:  &flag,
		}
		// 增加
		err := GetManager().EndpointsDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().EndpointsDao().GetByUUID("cb4b176d0cc24a41892536fd2dcce457")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.IP = "127.0.0.1"
		ret.Port = 80
		err = GetManager().EndpointsDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().EndpointsDao().DelByUUID("cb4b176d0cc24a41892536fd2dcce457")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tenant_service_3rd_party_discovery_cfg", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.ThirdPartySvcDiscoveryCfg{
			ServiceID: "de2bca6d5089d2274aba882c08038429",
			Type:      "etcd",
			Servers:   "127.0.0.1:2379",
			Key:       "/fanyangyang",
			Username:  "",
			Password:  "",
		}
		// 增加
		err := GetManager().ThirdPartySvcDiscoveryCfgDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().ThirdPartySvcDiscoveryCfgDao().GetByServiceID("de2bca6d5089d2274aba882c08038429")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.Key = "/fanyangyang/etcd"
		err = GetManager().ThirdPartySvcDiscoveryCfgDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().ThirdPartySvcDiscoveryCfgDao().DeleteByServiceID("de2bca6d5089d2274aba882c08038429")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tenant_service_version", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		data := &model.VersionInfo{
			BuildVersion:  "20190218165008",
			EventID:       "788c4e8031484c79a23054bd626ed5d2",
			ServiceID:     "ac96eed7c78dcda7106bbcd63c78816a",
			Kind:          "build_from_source_code",
			DeliveredType: "image",
			DeliveredPath: "",
			ImageName:     "",
			Cmd:           "start web",
			RepoURL:       "//github.com/goodrain/java-maven-demo.git",
			CodeVersion:   "",
			CodeBranch:    "",
			CommitMsg:     "",
			Author:        "lius",
			FinalStatus:   "",
			FinishTime:    time.Now(),
		}
		// 增加
		err := GetManager().VersionInfoDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().VersionInfoDao().GetAllVersionByServiceID("ac96eed7c78dcda7106bbcd63c78816a")
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil || len(ret) != 1 {
			t.Fatal("ret len is not equal to 1")
		}
		// 修改
		ret[0].ImageName = "goodrain.me/java-maven-demo:latest"
		err = GetManager().VersionInfoDao().UpdateModel(ret[0])
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().VersionInfoDao().DeleteVersionByEventID("788c4e8031484c79a23054bd626ed5d2")
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("tenant_services_port", func(t *testing.T) {
		if err := CreateManager(dbconfig.Config{
			MysqlConnectionInfo: connectionInfo,
			DBType:              "mysql",
		}); err != nil {
			t.Fatal(err)
		}
		var Flag bool
		Flag = true
		data := &model.TenantServicesPort{
			TenantID:       "b73e01d3b83546cc8d33d60a1618a79f",
			ServiceID:      "3ceb45680e2e8b83197c56a05d7cdbaf",
			ContainerPort:  5000,
			MappingPort:    5000,
			Protocol:       "http",
			PortAlias:      "GR5F8CF65000",
			IsInnerService: &Flag,
			IsOuterService: &Flag,
		}
		// 增加
		err := GetManager().TenantServicesPortDao().AddModel(data)
		if err != nil {
			t.Fatal(err)
		}
		// 查询
		ret, err := GetManager().TenantServicesPortDao().GetPort("3ceb45680e2e8b83197c56a05d7cdbaf", 5000)
		if err != nil {
			t.Fatal(err)
		}
		if ret == nil {
			t.Fatal("ret is nil")
		}
		// 修改
		ret.MappingPort = 5999
		err = GetManager().TenantServicesPortDao().UpdateModel(ret)
		if err != nil {
			t.Fatal(err)
		}
		// 删除
		err = GetManager().TenantServicesPortDao().DelByServiceID("3ceb45680e2e8b83197c56a05d7cdbaf")
		if err != nil {
			t.Fatal(err)
		}
	})
}
