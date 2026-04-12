# 测试能力清单

> 此文件由 `scripts/manage_test_manifest.py render` 自动生成，请勿手工编辑。

| Capability ID | 中文标题 | 状态 | 测试类型 | 业务入口 | 测试文件 |
|---|---|---|---|---|---|
| rainbond.app-backup.metadata-version-detect | 识别旧版与新版应用备份元数据结构 | active | regression | builder/exector.judgeMetadataVersion | builder/exector/groupapp_backup_test.go::TestJudgeMetadataVersion |
| rainbond.app-backup.service-volume-archive | 将服务卷数据归档为备份包 | active | regression | builder/exector.BackupAPPNew.backupServiceInfo | builder/exector/groupapp_backup_test.go::TestBackupServiceVolume |
| rainbond.app-backup.upload-package | 将应用备份包上传到外部存储 | active | integration | builder/exector.BackupAPPNew.uploadPkg | builder/exector/groupapp_backup_test.go::TestUploadPkg |
| rainbond.app-backup.upload-package-download-guard | 在备份包上传流程中保护已移除的下载接口 | active | integration | builder/exector.BackupAPPNew.uploadPkg | builder/exector/groupapp_backup_test.go::TestUploadPkg2 |
| rainbond.app-backup.volume-dir-defaults | 根据环境变量解析本地与共享备份卷目录 | active | regression | builder/exector.GetVolumeDir | builder/exector/groupapp_backup_test.go::TestGetVolumeDir |
| rainbond.app-config-group.bind-component | 将组件绑定到应用配置组 | active | regression | db/mysql/dao.AppConfigGroupServiceDaoImpl.AddModel | db/mysql/dao/application_config_group_test.go::TestAppConfigGroupServiceDaoAddModel |
| rainbond.app-config-group.create | 创建应用配置组记录 | active | regression | db/mysql/dao.AppConfigGroupDaoImpl.AddModel | db/mysql/dao/application_config_group_test.go::TestAppConfigGroupDaoAddModel |
| rainbond.app-config-group.delete | 删除应用配置组记录 | active | regression | db/mysql/dao.AppConfigGroupDaoImpl.DeleteConfigGroup | db/mysql/dao/application_config_group_test.go::TestDeleteConfigGroup |
| rainbond.app-config-group.get | 查询应用配置组记录 | active | regression | db/mysql/dao.AppConfigGroupDaoImpl.GetConfigGroupByID | db/mysql/dao/application_config_group_test.go::TestAppGetConfigGroupByID |
| rainbond.app-config-group.item-create | 创建应用配置项 | active | regression | db/mysql/dao.AppConfigGroupItemDaoImpl.AddModel | db/mysql/dao/application_config_group_test.go::TestAppConfigGroupItemDaoAddModel |
| rainbond.app-config-group.item-delete | 删除应用配置项 | active | regression | db/mysql/dao.AppConfigGroupItemDaoImpl.DeleteConfigGroupItem | db/mysql/dao/application_config_group_test.go::TestDeleteConfigGroupItem |
| rainbond.app-config-group.item-update | 更新应用配置项 | active | regression | db/mysql/dao.AppConfigGroupItemDaoImpl.UpdateModel | db/mysql/dao/application_config_group_test.go::TestAppConfigGroupItemDaoUpdateModel |
| rainbond.app-config-group.unbind-components | 移除应用配置组组件绑定 | active | regression | db/mysql/dao.AppConfigGroupServiceDaoImpl.DeleteConfigGroupService | db/mysql/dao/application_config_group_test.go::TestDeleteConfigGroupService |
| rainbond.app-import.package-name-normalize | 从 Linux 文件名还原导入镜像包名 | active | regression | builder/exector.buildFromLinuxFileName | builder/exector/import_app_test.go::TestBuildFromLinuxFileName |
| rainbond.app-import.status-serialization | 序列化并解析按应用记录的导入状态 | active | regression | builder/exector.map2str | builder/exector/import_app_test.go::TestAppStatusMapRoundTrip |
| rainbond.app-restore.image-registry-rewrite | 应用恢复时重写镜像仓库地址 | active | regression | builder/exector.getNewImageName | builder/exector/groupapp_restore_test.go::TestGetImageName |
| rainbond.app-restore.service-id-lookup | 从恢复后的服务映射中反查原始服务 ID | active | regression | builder/exector.BackupAPPRestore.getOldServiceID | builder/exector/groupapp_restore_test.go::TestGetOldServiceID |
| rainbond.app-restore.snapshot-relationship-rewrite | 应用恢复时重写服务依赖关系 | active | regression | builder/exector.BackupAPPRestore.modify | builder/exector/groupapp_restore_test.go::TestModify |
| rainbond.app-restore.unzip-all-data | 在恢复时解压完整备份数据包 | active | regression | builder/exector.BackupAPPRestore | builder/exector/groupapp_restore_test.go::TestUnzipAllDataFile |
| rainbond.build.select-builder-by-language | 按源码语言和构建类型选择构建器 | active | regression | builder/build.GetBuildByType | builder/build/build_type_matrix_test.go::TestGetBuildByType_SourceBuildLanguageMatrix |
| rainbond.builder.registered-worker-dispatch | 已注册 worker 分发时不再误报未知任务 | active | regression | builder/exector.exectorManager.RunTask | builder/exector/exector_test.go::TestRunTaskDoesNotWarnForRegisteredWorker |
| rainbond.cloud-storage.alioss-error-map | 将 AliOSS 服务错误转换为统一存储 SDK 错误 | active | regression | builder/cloudos.svcErrToS3SDKError | builder/cloudos/alioss_test.go::TestSvcErrToS3SDKError |
| rainbond.cloud-storage.driver-factory | 将云存储配置分发到正确的驱动实现 | active | regression | builder/cloudos.New | builder/cloudos/cloudos_test.go::TestNewDispatchesProviderDrivers |
| rainbond.cloud-storage.provider-parse | 解析云存储 provider 配置值 | active | regression | builder/cloudos.Str2S3Provider | builder/cloudos/cloudos_test.go::TestStr2S3Provider |
| rainbond.cloud-storage.s3-driver-config | 按预期配置初始化 S3 存储驱动 | active | regression | builder/cloudos.newS3 | builder/cloudos/s3_test.go::TestNewS3DriverKeepsConfig |
| rainbond.cluster-resource.detect-subresource | 识别集群资源子路径 | active | regression | api/handler.containsSlash | api/handler/cluster_resource_test.go::TestContainsSlash |
| rainbond.cluster-resource.handler-singleton | 复用集群资源处理器单例 | active | unit | api/handler.GetClusterResourceHandler | api/handler/cluster_resource_test.go::TestGetClusterResourceHandlerSingleton |
| rainbond.cluster-resource.validate-gvr | 校验集群资源 GVR 参数 | active | regression | api/handler.validateGVRParams | api/handler/cluster_resource_test.go::TestValidateGVRParams |
| rainbond.cnb-version.extract-major | 从 CNB 版本表达式提取主版本 | active | regression | builder/parser/code.extractMajorFromSpec | builder/parser/code/cnb_versions_test.go::TestExtractMajorFromSpec |
| rainbond.cnb-version.golang-order-and-default | 保持 Go CNB 版本顺序并将最新版本设为默认 | active | regression | builder/parser/code.GetCNBVersions | builder/parser/code/cnb_versions_test.go::TestGetCNBVersionsGoOrderingAndDefault |
| rainbond.cnb-version.match-golang | 归一化并匹配 Go CNB 版本表达式 | active | regression | builder/parser/code.MatchCNBVersion | builder/parser/code/cnb_versions_test.go::TestMatchCNBVersion_Golang |
| rainbond.cnb-version.match-language | 为复合语言匹配 CNB 版本 | active | regression | builder/parser/code.MatchCNBVersion | builder/parser/code/cnb_versions_test.go::TestMatchCNBVersion_CompositeLanguage |
| rainbond.cnb-version.python-order-and-default | 保持 Python CNB 版本顺序并将最新版本设为默认 | active | regression | builder/parser/code.GetCNBVersions | builder/parser/code/cnb_versions_test.go::TestGetCNBVersionsPythonOrderingAndDefault |
| rainbond.cnb-version.resolve-supported | 按语言解析支持的 CNB 版本 | active | regression | builder/parser/code.GetCNBVersions | builder/parser/code/cnb_versions_test.go::TestGetCNBVersions |
| rainbond.cnb.annotation-key-decode | 将 CNB 注解键解码为 BP 环境变量名 | active | regression | builder/build/cnb.annotationKeyToBPEnv | builder/build/cnb/cnb_test.go::TestAnnotationKeyToBPEnv |
| rainbond.cnb.annotation-key-encode | 将 BP 环境变量名编码为 CNB 注解键 | active | regression | builder/build/cnb.bpEnvToAnnotationKey | builder/build/cnb/cnb_test.go::TestBpEnvToAnnotationKey |
| rainbond.cnb.bp-annotation-priority | 显式 CNB 注解优先于 BP 透传值 | active | regression | builder/build/cnb.Builder.buildPlatformAnnotations | builder/build/cnb/cnb_test.go::TestBuildPlatformAnnotationsBPNoOverride |
| rainbond.cnb.build-job-execution | 执行 CNB 构建任务全流程 | active | regression | builder/build/cnb.Builder.runCNBBuildJob | builder/build/cnb/cnb_test.go::TestRunCNBBuildJob |
| rainbond.cnb.builder-image | 解析 CNB builder 镜像 | active | regression | builder/build/cnb.GetCNBBuilderImage | builder/build/cnb/cnb_test.go::TestGetCNBBuilderImage |
| rainbond.cnb.config-file | 注入指定的 CNB 配置文件内容 | active | regression | builder/build/cnb.injectConfigFile | builder/build/cnb/cnb_test.go::TestInjectConfigFile |
| rainbond.cnb.creator-args | 生成 CNB creator 命令参数 | active | regression | builder/build/cnb.Builder.buildCreatorArgs | builder/build/cnb/cnb_test.go::TestBuildCreatorArgs |
| rainbond.cnb.creator-args-insecure-registry | 为 CNB creator 附加 insecure registry 参数 | active | regression | builder/build/cnb.Builder.buildCreatorArgs | builder/build/cnb/cnb_test.go::TestBuildCreatorArgsInsecureRegistry |
| rainbond.cnb.dependency-mirror | 解析 CNB 依赖镜像源 | active | regression | builder/build/cnb.getDependencyMirror | builder/build/cnb/cnb_test.go::TestBuildPlatformAnnotationsMirrorDefault<br>builder/build/cnb/cnb_test.go::TestBuildPlatformAnnotationsMirrorExplicit<br>builder/build/cnb/cnb_test.go::TestGetDependencyMirrorOffline |
| rainbond.cnb.env-vars | 生成 CNB 构建任务环境变量 | active | regression | builder/build/cnb.Builder.buildEnvVars | builder/build/cnb/cnb_test.go::TestBuildEnvVars |
| rainbond.cnb.mirror-config | 按来源模式注入 CNB 镜像配置 | active | regression | builder/build/cnb.nodejsConfig.InjectMirrorConfig | builder/build/cnb/cnb_test.go::TestInjectMirrorConfig |
| rainbond.cnb.mirror-config-write-error | 传播 CNB 镜像配置写入失败 | active | regression | builder/build/cnb.nodejsConfig.InjectMirrorConfig | builder/build/cnb/cnb_test.go::TestInjectMirrorConfigWriteError |
| rainbond.cnb.new-builder | 创建 CNB 构建器实例 | active | regression | builder/build/cnb.NewBuilder | builder/build/cnb/cnb_test.go::TestNewBuilder |
| rainbond.cnb.offline-mode | 解析离线模式下的 CNB 镜像行为 | active | regression | builder/build/cnb offline mode helpers | builder/build/cnb/cnb_test.go::TestIsOfflineMode<br>builder/build/cnb/cnb_test.go::TestGetCNBBuilderImageOffline<br>builder/build/cnb/cnb_test.go::TestGetCNBRunImageOffline |
| rainbond.cnb.order-toml | 写入自定义 CNB order 定义 | active | regression | builder/build/cnb.Builder.writeCustomOrder | builder/build/cnb/cnb_test.go::TestWriteCustomOrder |
| rainbond.cnb.order-write-failure | CNB order 文件写入失败时返回空标记 | active | regression | builder/build/cnb.Builder.writeCustomOrder | builder/build/cnb/cnb_test.go::TestWriteCustomOrderFailure |
| rainbond.cnb.platform-annotations | 根据构建环境生成 CNB 平台注解 | active | regression | builder/build/cnb.Builder.buildPlatformAnnotations | builder/build/cnb/cnb_test.go::TestBuildPlatformAnnotations |
| rainbond.cnb.platform-volume | 根据注解创建 CNB 平台卷 | active | regression | builder/build/cnb.Builder.createPlatformVolume | builder/build/cnb/cnb_test.go::TestCreatePlatformVolume<br>builder/build/cnb/cnb_test.go::TestCreatePlatformVolumeEmpty<br>builder/build/cnb/cnb_test.go::TestCreatePlatformVolumeNonCNBKeys |
| rainbond.cnb.prebuild-job-cleanup | 清理历史 CNB 预构建任务 | active | regression | builder/build/cnb.Builder.stopPreBuildJob | builder/build/cnb/cnb_test.go::TestStopPreBuildJob |
| rainbond.cnb.project-file-validation | CNB 构建前校验项目文件 | active | regression | builder/build/cnb.Builder.validateProjectFiles | builder/build/cnb/cnb_test.go::TestValidateProjectFiles |
| rainbond.cnb.run-image | 解析 CNB run 镜像 | active | regression | builder/build/cnb.GetCNBRunImage | builder/build/cnb/cnb_test.go::TestGetCNBRunImage |
| rainbond.cnb.source-dir-permissions | CNB 构建前规范化源码目录权限 | active | regression | builder/build/cnb.Builder.setSourceDirPermissions | builder/build/cnb/cnb_test.go::TestSetSourceDirPermissions<br>builder/build/cnb/cnb_test.go::TestSetSourceDirPermissionsNonexistent |
| rainbond.cnb.static-buildpacks | 纯静态源码使用 nginx buildpack | active | regression | builder/build/cnb.staticConfig.CustomOrder | builder/build/cnb/cnb_test.go::TestStaticBuildpacks |
| rainbond.cnb.volume-mounts | 创建 CNB 构建卷与挂载 | active | regression | builder/build/cnb.Builder.createVolumeAndMount | builder/build/cnb/cnb_test.go::TestCreateVolumeAndMount |
| rainbond.cnb.waiting-complete | 等待 CNB 构建任务完成状态 | active | regression | builder/build/cnb.Builder.waitingComplete | builder/build/cnb/cnb_test.go::TestWaitingComplete |
| rainbond.compose.config-volume-file-content | 保留配置卷文件内容字段语义 | active | regression | builder/parser/types.Volume.FileContent | builder/parser/file_content_test.go::TestVolumeFileContent |
| rainbond.compose.detect-config-file-mount | 识别配置文件类型的挂载路径 | active | regression | builder/parser/compose.isConfigFile | builder/parser/compose/version_detect_test.go::TestIsConfigFile |
| rainbond.compose.detect-version | 根据语法特征推断 compose 版本 | active | regression | builder/parser/compose.inferComposeVersion | builder/parser/compose/version_detect_test.go::TestInferComposeVersion |
| rainbond.compose.parse-warnings | 解析 docker compose 并返回降级告警 | active | regression | builder/parser.CreateDockerComposeParse.Parse | builder/parser/docker_compose_warnings_test.go::TestDockerComposeParseWithWarnings |
| rainbond.compose.yaml-anchor-support | 支持 docker compose 中的 YAML anchors | active | regression | builder/parser.CreateDockerComposeParse.Parse | builder/parser/docker_compose_warnings_test.go::TestDockerComposeParseWithYAMLAnchors |
| rainbond.config-files.detect | 识别源码目录中的 npm 和 yarn 配置文件 | active | regression | builder/parser/code.DetectConfigFiles | builder/parser/code/config_files_test.go::TestDetectConfigFiles_Npmrc<br>builder/parser/code/config_files_test.go::TestDetectConfigFiles_YarnrcClassic<br>builder/parser/code/config_files_test.go::TestDetectConfigFiles_YarnrcYml<br>builder/parser/code/config_files_test.go::TestDetectConfigFiles_Multiple<br>builder/parser/code/config_files_test.go::TestDetectConfigFiles_None |
| rainbond.config-files.has-any | 检测源码中是否存在包管理器配置文件 | active | regression | builder/parser/code.ConfigFiles.HasAnyConfigFile | builder/parser/code/config_files_test.go::TestConfigFiles_HasAnyConfigFile |
| rainbond.config-files.read-npmrc | 读取源码中的 npmrc 内容 | active | regression | builder/parser/code.ConfigFiles.GetNpmrcContent | builder/parser/code/config_files_test.go::TestConfigFiles_GetNpmrcContent |
| rainbond.config-files.read-yarnrc | 读取源码中的 yarnrc 内容 | active | regression | builder/parser/code.ConfigFiles.GetYarnrcContent | builder/parser/code/config_files_test.go::TestConfigFiles_GetYarnrcContent |
| rainbond.config-files.resolve-relevant-file | 为包管理器选择相关配置文件 | active | regression | builder/parser/code.ConfigFiles.GetRelevantConfigFile | builder/parser/code/config_files_test.go::TestConfigFiles_GetRelevantConfigFile |
| rainbond.dockerfile.line-info | 跟踪 Dockerfile AST 的行号信息 | active | regression | util/dockerfile/parser.Parse | util/dockerfile/parser/parser_test.go::TestLineInformation |
| rainbond.dockerfile.parse-fixtures | 将标准 Dockerfile 示例解析为稳定 AST | active | regression | util/dockerfile/parser.Parse | util/dockerfile/parser/parser_test.go::TestTestData |
| rainbond.dockerfile.parse-json-array | 解析 Dockerfile 指令中的 JSON 数组语法 | active | regression | util/dockerfile/parser.parseJSON | util/dockerfile/parser/json_test.go::TestJSONArraysOfStrings |
| rainbond.dockerfile.parse-words | 按引号与转义规则拆分 Dockerfile 指令参数 | active | regression | util/dockerfile/parser.parseWords | util/dockerfile/parser/parser_test.go::TestParseWords |
| rainbond.dockerfile.reject-invalid | 解析 Dockerfile 时拒绝无效示例 | active | regression | util/dockerfile/parser.Parse | util/dockerfile/parser/parser_test.go::TestTestNegative |
| rainbond.endpoint.address-split | 从协议与端口中拆分端点地址 | active | regression | util/endpoint.SplitEndpointAddress | util/endpoint/validation_test.go::TestSplitEndpointAddress |
| rainbond.endpoint.domain-not-ip | 判断端点地址应按域名而不是 IP 处理 | active | regression | util/endpoint.IsDomainNotIP | util/endpoint/validation_test.go::TestIsDomainNotIP |
| rainbond.endpoint.domain-validate | 校验端点域名及通配域名 | active | regression | util/endpoint.ValidateDomain | util/endpoint/validation_test.go::TestValidateDomain |
| rainbond.endpoint.ip-validate | 校验端点 IP 地址并拒绝受限网段 | active | regression | util/endpoint.ValidateEndpointIP | util/endpoint/validation_test.go::TestValidateEndpointIP |
| rainbond.envutil.custom-memory | 判断内存大小是自定义值还是预设值 | active | regression | util/envutil.IsCustomMemory | util/envutil/envutil_test.go::TestIsCustomMemory |
| rainbond.envutil.getenv-default | 在 envutil 中为缺失环境变量返回默认值 | active | regression | util/envutil.GetenvDefault | util/envutil/envutil_test.go::TestGetenvDefault |
| rainbond.envutil.memory-label | 将内存大小映射为预设内存标签 | active | regression | util/envutil.GetMemoryType | util/envutil/envutil_test.go::TestGetMemoryType |
| rainbond.eventlog.file-store | 事件日志文件存储的追加读取与清理 | active | regression | api/eventlog/store.JSONLinesFileStore | api/eventlog/store/filestore_test.go::TestJSONLinesFileStore |
| rainbond.eventlog.file-store-concurrency | 事件日志文件存储支持并发写入 | active | regression | api/eventlog/store.JSONLinesFileStore.Append | api/eventlog/store/filestore_test.go::TestFileStoreConcurrency |
| rainbond.filepersistence.volcengine-client-init | 幂等初始化并复用火山引擎 NAS 客户端 | active | regression | pkg/component/filepersistence.VolcengineProvider.init | pkg/component/filepersistence/volcengine_test.go::TestVolcengineProviderInitIsIdempotent |
| rainbond.framework-detect.angular-spa | 识别 Angular SPA 模式 | active | regression | builder/parser/code.DetectFramework | builder/parser/code/framework_test.go::TestDetectFramework_Angular_SPA |
| rainbond.framework-detect.angular-ssr | 识别 Angular SSR 模式 | active | regression | builder/parser/code.DetectFramework | builder/parser/code/framework_test.go::TestDetectFramework_Angular_SSR |
| rainbond.framework-detect.cra | 识别 Create React App 框架 | active | regression | builder/parser/code.DetectFramework | builder/parser/code/framework_test.go::TestDetectFramework_CRA |
| rainbond.framework-detect.display-name | 解析框架展示名称 | active | regression | builder/parser/code.GetDisplayName | builder/parser/code/framework_test.go::TestGetDisplayName |
| rainbond.framework-detect.express | 识别 Express 框架 | active | regression | builder/parser/code.DetectFramework | builder/parser/code/framework_test.go::TestDetectFramework_Express |
| rainbond.framework-detect.nestjs | 识别 NestJS 框架 | active | regression | builder/parser/code.DetectFramework | builder/parser/code/framework_test.go::TestDetectFramework_NestJS |
| rainbond.framework-detect.nextjs | 从源码目录识别 Next.js 框架 | active | regression | builder/parser/code.DetectFramework | builder/parser/code/framework_test.go::TestDetectFramework_NextJS |
| rainbond.framework-detect.nextjs-no-config | 无配置文件时识别 Next.js | active | regression | builder/parser/code.DetectFramework | builder/parser/code/framework_test.go::TestDetectFramework_NextJS_NoConfigFile |
| rainbond.framework-detect.nextjs-ssr | 识别 Next.js SSR 模式 | active | regression | builder/parser/code.DetectFramework | builder/parser/code/framework_test.go::TestDetectFramework_NextJS_SSR |
| rainbond.framework-detect.nextjs-static-export | 识别 Next.js 静态导出模式 | active | regression | builder/parser/code.DetectFramework | builder/parser/code/framework_test.go::TestDetectFramework_NextJS_StaticExport |
| rainbond.framework-detect.no-framework | 普通 Node 项目不识别框架 | active | regression | builder/parser/code.DetectFramework | builder/parser/code/framework_test.go::TestDetectFramework_NoFramework |
| rainbond.framework-detect.no-package-json | 缺少 package.json 时不识别前端框架 | active | regression | builder/parser/code.DetectFramework | builder/parser/code/framework_test.go::TestDetectFramework_NoPackageJSON |
| rainbond.framework-detect.nuxt | 从源码目录识别 Nuxt 框架 | active | regression | builder/parser/code.DetectFramework | builder/parser/code/framework_test.go::TestDetectFramework_Nuxt |
| rainbond.framework-detect.nuxt-static-target | 识别 Nuxt 静态目标模式 | active | regression | builder/parser/code.DetectFramework | builder/parser/code/framework_test.go::TestDetectFramework_Nuxt_StaticTarget |
| rainbond.framework-detect.nuxt3-ssr-false | 识别关闭 SSR 的 Nuxt3 | active | regression | builder/parser/code.DetectFramework | builder/parser/code/framework_test.go::TestDetectFramework_Nuxt3_SSRFalse |
| rainbond.framework-detect.nuxt3-static | 识别 Nuxt3 静态输出模式 | active | regression | builder/parser/code.DetectFramework | builder/parser/code/framework_test.go::TestDetectFramework_Nuxt3_NitroStatic |
| rainbond.framework-detect.supported-list | 列出支持识别的前端框架 | active | regression | builder/parser/code.GetSupportedFrameworks | builder/parser/code/framework_test.go::TestGetSupportedFrameworks |
| rainbond.framework-detect.version-normalization | 规范化框架依赖版本号 | active | regression | builder/parser/code.cleanVersion | builder/parser/code/framework_test.go::TestCleanVersion |
| rainbond.framework-detect.vite | 识别 Vite 框架 | active | regression | builder/parser/code.DetectFramework | builder/parser/code/framework_test.go::TestDetectFramework_Vite |
| rainbond.gateway.allocate-lb-port | 分配可用网关负载均衡端口 | active | regression | api/handler.selectAvailablePort | api/handler/gateway_action_test.go::TestSelectAvailablePort |
| rainbond.helm-release.app-version-format | 为 Helm 历史输出格式化应用版本号 | active | regression | pkg/helm.formatAppVersion | pkg/helm/helm_release_test.go::TestGetReleaseHistory |
| rainbond.helm-release.chart-name-format | 为历史和摘要输出格式化 Helm chart 名称 | active | regression | pkg/helm.formatChartName | pkg/helm/helm_release_test.go::TestGetReleaseHistory |
| rainbond.helm-release.classify-resources | 按资源类型归类 Helm 发布资源 | active | regression | api/handler.splitHelmReleaseResources | api/handler/helm_release_test.go::TestSplitHelmReleaseResourcesClassifiesKinds |
| rainbond.helm-release.default-namespace | 推导 Helm 发布默认命名空间 | active | regression | api/handler.helmReleaseNamespace | api/handler/helm_release_test.go::TestHelmReleaseNamespaceUsesTenantNamespaceWhenPresent<br>api/handler/helm_release_test.go::TestHelmReleaseNamespaceFallsBackToTenantUUID |
| rainbond.helm-release.detail-summary | 汇总 Helm 发布详情 | active | regression | api/handler.summarizeHelmReleaseDetail | api/handler/helm_release_test.go::TestSummarizeHelmReleaseDetailBuildsStableDTO |
| rainbond.helm-release.history-summary | 汇总 Helm 发布历史 | active | regression | api/handler.summarizeHelmReleaseHistory | api/handler/helm_release_test.go::TestSummarizeHelmReleaseHistoryBuildsStableDTO<br>pkg/helm/helm_release_test.go::TestGetReleaseHistory |
| rainbond.helm-release.install-defaults | 规范 Helm 安装默认参数 | active | regression | api/handler.HelmReleaseInstallRequest.Normalize | api/handler/helm_release_test.go::TestHelmReleaseInstallRequestNormalizeDefaults |
| rainbond.helm-release.install-validate | 校验 Helm 安装请求 | active | regression | api/handler.HelmReleaseInstallRequest.Validate | api/handler/helm_release_test.go::TestHelmReleaseInstallRequestValidate |
| rainbond.helm-release.installable-check | 拒绝不可安装的 Helm chart 类型 | active | regression | pkg/helm.checkIfInstallable | pkg/helm/helm_release_test.go::TestCheckIfInstallable |
| rainbond.helm-release.list-summary | 汇总 Helm 发布列表项 | active | regression | api/handler.summarizeHelmRelease | api/handler/helm_release_test.go::TestSummarizeHelmReleaseBuildsStableDTO |
| rainbond.helm-release.match-managed-resource | 识别 Helm 托管资源归属 | active | regression | api/handler.isHelmReleaseResource | api/handler/helm_release_test.go::TestIsHelmReleaseResourceMatchesManagedByAndInstanceLabels |
| rainbond.helm-release.oci-reference-normalize | 规范化 OCI chart 引用并推导版本标签 | active | regression | pkg/helm.normalizeOCIChartReference | pkg/helm/helm_release_test.go::TestNormalizeOCIChartReference |
| rainbond.helm-release.preview-source-error | 将 Helm 预览来源错误转换为错误请求 | active | regression | api/handler.wrapHelmChartPreviewSourceError | api/handler/helm_release_test.go::TestWrapHelmChartPreviewSourceErrorConvertsToBadRequest<br>api/handler/helm_release_test.go::TestWrapHelmChartPreviewSourceErrorPreservesBadRequest |
| rainbond.helm-release.resolve-namespace | 解析 Helm 发布命名空间 | active | regression | api/handler.HelmReleaseHandler.resolveNamespace | api/handler/helm_release_test.go::TestResolveHelmReleaseNamespaceUsesExplicitNamespace |
| rainbond.helm-release.resolve-namespace-fallback | 请求未指定时从团队推导 Helm 命名空间 | active | regression | api/handler.(*HelmReleaseHandler).resolveNamespace | api/handler/helm_release_test.go::TestResolveHelmReleaseNamespaceFallsBackToTenantNamespace |
| rainbond.helm-release.rollback-validate | 校验 Helm 回滚版本 | active | regression | api/handler.HelmReleaseRollbackRequest.Validate | api/handler/helm_release_test.go::TestHelmReleaseRollbackRequestValidate |
| rainbond.helm-release.strip-kube-version | 在安装或加载前移除 chart 的 kubeVersion 要求 | active | regression | pkg/helm.removeKubeVersionFromChart | pkg/helm/helm_release_test.go::TestCheckIfInstallable |
| rainbond.helm-release.upgrade-chart-guard | 拦截 Helm 升级图表不匹配 | active | regression | api/handler.validateUpgradeChartName | api/handler/helm_release_test.go::TestValidateUpgradeChartNameRejectsMismatchByDefault<br>api/handler/helm_release_test.go::TestValidateUpgradeChartNameAllowsMismatchWithExplicitConfirmation |
| rainbond.helm-release.values-yaml | 将 Helm values YAML 解析为可安装的 values 映射 | active | regression | pkg/helm.parseValuesYAML | pkg/helm/helm_release_test.go::TestParseValuesYAML |
| rainbond.helm-repo.add | 添加 Helm 仓库 | active | integration | pkg/helm.Repo.Add | pkg/helm/repo_test.go::TestRepoAdd |
| rainbond.helm-repo.add-idempotent | 当相同 Helm 仓库已存在时跳过重复添加 | active | regression | pkg/helm.Repo.Add | pkg/helm/repo_test.go::TestRepoAddSkipsExistingConfig |
| rainbond.helm-repo.reject-deprecated | 拒绝已废弃的 Helm 仓库地址 | active | regression | pkg/helm.Repo.Add | pkg/helm/repo_test.go::TestRepoAddRejectsDeprecatedRepo |
| rainbond.helm-repo.requested-filter | 校验并匹配请求更新的 Helm 仓库名称 | active | regression | pkg/helm.checkRequestedRepos | pkg/helm/helm_release_test.go::TestCheckRequestedRepos |
| rainbond.image-clean.registry-gc-noop | 当没有匹配的仓库 Pod 时跳过垃圾回收执行 | active | regression | builder/clean.Manager.PodExecCmd | builder/clean/clean_test.go::TestPodExecCmdNoMatchingPod |
| rainbond.image-clean.stop-loop | 通过取消上下文停止镜像清理管理器循环 | active | regression | builder/clean.Manager.Stop | builder/clean/clean_test.go::TestManagerStopCancelsContext |
| rainbond.ingress-nginx.meta-namespace-key | 为 ingress-nginx 监听对象构建 namespace/name 键 | active | regression | util/ingress-nginx/k8s.MetaNamespaceKey | util/ingress-nginx/k8s/main_test.go::TestMetaNamespaceKey |
| rainbond.ingress-nginx.name-namespace-parse | 解析 ingress-nginx 资源的 namespace/name 标识 | active | regression | util/ingress-nginx/k8s.ParseNameNS | util/ingress-nginx/k8s/main_test.go::TestParseNameNS |
| rainbond.ingress-nginx.node-ip-resolve | 为 ingress-nginx helper 解析节点内外网 IP | active | regression | util/ingress-nginx/k8s.GetNodeIPOrName | util/ingress-nginx/k8s/main_test.go::TestGetNodeIPOrName |
| rainbond.ingress-nginx.pod-details | 根据环境变量和集群状态解析 ingress-nginx Pod 详情 | active | regression | util/ingress-nginx/k8s.GetPodDetails | util/ingress-nginx/k8s/main_test.go::TestGetPodDetails |
| rainbond.k8s.scheme-registers-kubevirt-vm | K8s scheme registers KubeVirt VirtualMachine | active | regression | pkg/component/k8s.init | pkg/component/k8s/k8sComponent_test.go::TestSchemeRegistersKubeVirtVirtualMachine |
| rainbond.kb-adapter.backup-repo.list-ready | 列出 kb-adapter 可用的备份仓库 | active | regression | plugins/kb-adapter-rbdplugin/service/backup.Service.ListAvailableBackupRepos | plugins/kb-adapter-rbdplugin/service/backup/backup_test.go::TestListAvailableBackupRepos |
| rainbond.kb-adapter.cluster-backup.delete | 按服务范围删除允许清理的集群备份 | active | regression | plugins/kb-adapter-rbdplugin/service/backup.Service.DeleteBackups | plugins/kb-adapter-rbdplugin/service/backup/backup_test.go::TestDeleteBackups |
| rainbond.kb-adapter.cluster-backup.delete-guard | 判断集群备份是否允许安全删除 | active | regression | plugins/kb-adapter-rbdplugin/service/backup.Service.canDeleteBackup | plugins/kb-adapter-rbdplugin/service/backup/backup_test.go::TestCanDeleteBackup |
| rainbond.kb-adapter.cluster-backup.list | 列出目标服务对应的集群备份 | active | regression | plugins/kb-adapter-rbdplugin/service/backup.Service.ListBackups | plugins/kb-adapter-rbdplugin/service/backup/backup_test.go::TestListBackups |
| rainbond.kb-adapter.cluster-backup.schedule-reconcile | 根据插件输入创建更新或关闭集群备份计划 | active | regression | plugins/kb-adapter-rbdplugin/service/backup.Service.ReScheduleBackup | plugins/kb-adapter-rbdplugin/service/backup/backup_test.go::TestReScheduleBackup |
| rainbond.kb-adapter.cluster.associate-service-id | 将 KubeBlocks 集群关联到 Rainbond 服务 ID | active | regression | plugins/kb-adapter-rbdplugin/service/cluster.Service.associateToKubeBlocksComponent | plugins/kb-adapter-rbdplugin/service/cluster/cluster_test.go::TestAssociateToKubeBlocksComponent |
| rainbond.kb-adapter.cluster.connection-info | 获取插件详情页所需的集群连接凭据 | active | regression | plugins/kb-adapter-rbdplugin/service/cluster.Service.GetConnectInfo | plugins/kb-adapter-rbdplugin/service/cluster/info_test.go::TestGetConnectInfo |
| rainbond.kb-adapter.cluster.create | 通过 kb-adapter 插件创建 KubeBlocks 集群 | active | regression | plugins/kb-adapter-rbdplugin/service/cluster.Service.CreateCluster | plugins/kb-adapter-rbdplugin/service/cluster/lifecycle_test.go::TestCreateCluster |
| rainbond.kb-adapter.cluster.delete-cleanup | 在集群清理时删除 OpsRequest 与关联 Secret | active | regression | plugins/kb-adapter-rbdplugin/service/cluster.Service.cleanupClusterOpsRequests | plugins/kb-adapter-rbdplugin/service/cluster/lifecycle_test.go::TestCleanupClusterOpsRequests |
| rainbond.kb-adapter.cluster.detail-summary | 构建插件集群详情摘要并包含资源与备份信息 | active | regression | plugins/kb-adapter-rbdplugin/service/cluster.Service.GetClusterDetail | plugins/kb-adapter-rbdplugin/service/cluster/info_test.go::TestGetClusterDetail |
| rainbond.kb-adapter.cluster.event-timeline | 根据 OpsRequest 构建集群操作事件时间线 | active | regression | plugins/kb-adapter-rbdplugin/service/cluster.Service.GetClusterEvents | plugins/kb-adapter-rbdplugin/service/cluster/event_test.go::TestGetClusterEvents |
| rainbond.kb-adapter.cluster.list-pods | 为插件视图列出集群 Pod 并解析 InstanceSet | active | regression | plugins/kb-adapter-rbdplugin/service/cluster.Service.getClusterPods | plugins/kb-adapter-rbdplugin/service/cluster/cluster_test.go::TestGetClusterPods |
| rainbond.kb-adapter.cluster.parameter-constraint-merge | 合并实时参数项与参数约束定义 | active | regression | plugins/kb-adapter-rbdplugin/service/cluster.mergeEntriesAndConstraints | plugins/kb-adapter-rbdplugin/service/cluster/parameter_test.go::TestMergeEntriesAndConstraints |
| rainbond.kb-adapter.cluster.pod-detail | 为插件诊断页构建详细 Pod 诊断信息 | active | regression | plugins/kb-adapter-rbdplugin/service/cluster.Service.GetPodDetail | plugins/kb-adapter-rbdplugin/service/cluster/pod_test.go::TestGetPodDetail |
| rainbond.kb-adapter.cluster.restore-from-backup | 从备份恢复集群并清理失败的恢复操作 | active | regression | plugins/kb-adapter-rbdplugin/service/cluster.Service.RestoreFromBackup | plugins/kb-adapter-rbdplugin/service/cluster/restore_test.go::TestRestoreFromBackup |
| rainbond.kb-adapter.cluster.scale | 通过插件工作流扩缩集群副本资源与存储 | active | regression | plugins/kb-adapter-rbdplugin/service/cluster.Service.ExpansionCluster | plugins/kb-adapter-rbdplugin/service/cluster/scaling_test.go::TestExpansionCluster |
| rainbond.kb-adapter.coordinator.parameter-value-parse | 将插件参数值解析为带类型的协调器参数项 | active | regression | plugins/kb-adapter-rbdplugin/service/coordinator.Coordinator.ParseParameters | plugins/kb-adapter-rbdplugin/service/coordinator/coordinator_test.go::TestBase_ParseParameters |
| rainbond.kb-adapter.opsrequest.blocking-ops-management | 列出并清理阻塞中的未终态 OpsRequest | active | regression | plugins/kb-adapter-rbdplugin/service/kbkit.GetAllNonFinalOpsRequests | plugins/kb-adapter-rbdplugin/service/kbkit/opsrequest_test.go::TestGetAllNonFinalOpsRequests |
| rainbond.kb-adapter.opsrequest.create-supported-ops | 创建生命周期备份扩缩容参数变更与恢复等 OpsRequest | active | regression | plugins/kb-adapter-rbdplugin/service/kbkit.CreateLifecycleOpsRequest | plugins/kb-adapter-rbdplugin/service/kbkit/opsrequest_test.go::TestCreateLifecycleOpsRequest |
| rainbond.kb-adapter.opsrequest.preflight-arbitration | 在提交新操作前裁决冲突的 OpsRequest | active | regression | plugins/kb-adapter-rbdplugin/service/kbkit.preflightCheck | plugins/kb-adapter-rbdplugin/service/kbkit/opsrequest_preflight_test.go::TestUniqueOpsDecide |
| rainbond.kb-adapter.server-config.dev-mode | 识别 kb-adapter 开发环境模式 | active | regression | plugins/kb-adapter-rbdplugin/internal/config.InDevelopment | plugins/kb-adapter-rbdplugin/internal/config/config_test.go::TestInDevelopment |
| rainbond.kb-adapter.server-config.load-from-env | 从环境变量加载 kb-adapter 服务配置 | active | regression | plugins/kb-adapter-rbdplugin/internal/config.LoadConfigFromEnv | plugins/kb-adapter-rbdplugin/internal/config/config_test.go::TestLoadConfigFromEnv |
| rainbond.kb-adapter.server-config.must-load | 强制加载并校验 kb-adapter 服务配置 | active | regression | plugins/kb-adapter-rbdplugin/internal/config.MustLoad | plugins/kb-adapter-rbdplugin/internal/config/config_test.go::TestMustLoad |
| rainbond.kb-adapter.server-config.validate | 校验 kb-adapter 服务配置项 | active | regression | plugins/kb-adapter-rbdplugin/internal/config.ServerConfig.Validate | plugins/kb-adapter-rbdplugin/internal/config/config_test.go::TestServerConfig_Validate |
| rainbond.kubeblocks.component-selector | 为 KubeBlocks 组件生成标签选择器 | active | regression | util/kubeblocks.GenerateKubeBlocksSelector | util/kubeblocks/kubeblocks_test.go::TestGenerateKubeBlocksSelector |
| rainbond.license.decode | 解码并解析许可证令牌内容 | active | regression | api/util/license.DecodeLicense | api/util/license/rsa_license_test.go::TestDecodeLicense |
| rainbond.license.parse-public-key | 解析 PEM 编码的 RSA 公钥 | active | regression | api/util/license.ParsePublicKey | api/util/license/rsa_license_test.go::TestParsePublicKey |
| rainbond.license.plugin-allowlist | 根据许可证映射判断插件是否允许使用 | active | regression | api/util/license.IsPluginAllowed | api/util/license/rsa_license_test.go::TestIsPluginAllowed_Wildcard |
| rainbond.license.round-trip | 往返编码解码并校验许可证令牌 | active | regression | api/util/license.EncodeLicense | api/util/license/rsa_license_test.go::TestRoundTrip |
| rainbond.license.status-projection | 将许可证令牌投影为状态响应 | active | regression | api/util/license.TokenToStatus | api/util/license/rsa_license_test.go::TestTokenToStatus |
| rainbond.license.validate-token | 校验许可证企业绑定与生效时间窗口 | active | regression | api/util/license.ValidateToken | api/util/license/rsa_license_test.go::TestValidateToken_Valid |
| rainbond.license.verify-signature | 校验许可证 RSA 签名 | active | regression | api/util/license.VerifySignature | api/util/license/rsa_license_test.go::TestVerifySignature_Valid |
| rainbond.maven.list-modules | 列出 Maven 多服务模块 | active | regression | builder/parser/code/multisvc.maven.ListModules | builder/parser/code/multisvc/maven_test.go::TestMaven_ListModules |
| rainbond.maven.parse-pom | 解析 Maven 父 pom 的模块与打包方式 | active | regression | builder/parser/code/multisvc.parsePom | builder/parser/code/multisvc/maven_test.go::TestMaven_ParsePom |
| rainbond.multisvc.ignore-non-java | 在多服务解析器选择中忽略非 Java 语言 | active | regression | builder/parser/code/multisvc.NewMultiServiceI | builder/parser/code/multisvc/multi_services_test.go::TestNewMultiServiceI_IgnoresLanguagesWithoutJavaMaven |
| rainbond.multisvc.select-java-maven | 为复合语言选择 Java Maven 多服务解析器 | active | regression | builder/parser/code/multisvc.NewMultiServiceI | builder/parser/code/multisvc/multi_services_test.go::TestNewMultiServiceI_SupportsCompositeJavaMaven |
| rainbond.node-version.display-info | 汇总 Node 版本展示与派生信息 | active | regression | builder/parser/code.NodeVersionInfo helpers | builder/parser/code/node_version_test.go::TestCleanVersionSpec<br>builder/parser/code/node_version_test.go::TestExtractMajorVersion<br>builder/parser/code/node_version_test.go::TestExtractMinorPatch<br>builder/parser/code/node_version_test.go::TestNodeVersionInfo_IsLTS<br>builder/parser/code/node_version_test.go::TestNodeVersionInfo_GetNodeVersionDisplay |
| rainbond.node-version.fallback-supported-range | 将不受支持的 Node.js 版本回退到支持范围 | active | regression | builder/parser/code.ResolveNodeVersion | builder/parser/code/node_version_test.go::TestResolveNodeVersion_UnsupportedVersion |
| rainbond.node-version.normalize-v-prefix | 规范化带 v 前缀的 Node.js 版本 | active | regression | builder/parser/code.ResolveNodeVersion | builder/parser/code/node_version_test.go::TestResolveNodeVersion_WithVPrefix |
| rainbond.node-version.parse-package-json | 从 package.json 解析 Node 版本要求 | active | regression | builder/parser/code.ParseNodeVersionFromPackageJSON | builder/parser/code/node_version_test.go::TestParseNodeVersionFromPackageJSON<br>builder/parser/code/node_version_test.go::TestParseNodeVersionFromPackageJSON_NoEngines<br>builder/parser/code/node_version_test.go::TestParseNodeVersionFromPackageJSON_NoPackageJSON |
| rainbond.node-version.resolve-range | 解析范围形式的 Node.js 版本约束 | active | regression | builder/parser/code.ResolveNodeVersion | builder/parser/code/node_version_test.go::TestResolveNodeVersion_Range |
| rainbond.node-version.resolve-spec | 将 Node 版本表达式解析为支持的运行时版本 | active | regression | builder/parser/code.ResolveNodeVersion | builder/parser/code/node_version_test.go::TestResolveNodeVersion_Empty<br>builder/parser/code/node_version_test.go::TestResolveNodeVersion_Wildcard<br>builder/parser/code/node_version_test.go::TestResolveNodeVersion_GreaterThanOrEqual<br>builder/parser/code/node_version_test.go::TestResolveNodeVersion_Caret<br>builder/parser/code/node_version_test.go::TestResolveNodeVersion_Tilde<br>builder/parser/code/node_version_test.go::TestResolveNodeVersion_XNotation<br>builder/parser/code/node_version_test.go::TestResolveNodeVersion_MajorOnly<br>builder/parser/code/node_version_test.go::TestResolveNodeVersion_ExactVersion |
| rainbond.ns-resource.detect-source | 识别命名空间资源来源 | active | regression | api/handler.detectResourceSource | api/handler/ns_resource_test.go::TestDetectResourceSource |
| rainbond.ns-resource.handler-singleton | 复用命名空间资源处理器单例 | active | unit | api/handler.GetNsResourceHandler | api/handler/ns_resource_test.go::TestGetNsResourceHandlerSingleton |
| rainbond.ns-resource.mark-source | 标记命名空间资源来源 | active | regression | api/handler.injectSourceLabel | api/handler/ns_resource_test.go::TestInjectSourceLabelYaml<br>api/handler/ns_resource_test.go::TestInjectSourceLabelManual |
| rainbond.ns-resource.resolve-tenant-namespace | 解析团队命名空间 | active | regression | api/handler.(*NsResourceHandler).getTenantNamespace | api/handler/ns_resource_test.go::TestGetTenantNamespaceUsesNamespaceField<br>api/handler/ns_resource_test.go::TestGetTenantNamespaceFallsBackToUUIDWhenNamespaceEmpty |
| rainbond.package-manager.commands | 按包管理器生成安装构建启动命令 | active | regression | builder/parser/code.PackageManagerInfo | builder/parser/code/package_manager_test.go::TestPackageManagerInfo_GetCommands |
| rainbond.package-manager.default-npm | 无锁文件时默认使用 npm | active | regression | builder/parser/code.DetectPackageManager | builder/parser/code/package_manager_test.go::TestDetectPackageManager_Default |
| rainbond.package-manager.detect-lockfile | 通过锁文件识别包管理器 | active | regression | builder/parser/code.DetectPackageManager | builder/parser/code/package_manager_test.go::TestDetectPackageManager_PNPM<br>builder/parser/code/package_manager_test.go::TestDetectPackageManager_Yarn<br>builder/parser/code/package_manager_test.go::TestDetectPackageManager_NPM |
| rainbond.package-manager.package-json-field | 通过 package.json 字段识别包管理器 | active | regression | builder/parser/code.DetectPackageManager | builder/parser/code/package_manager_test.go::TestDetectPackageManager_PackageManagerField<br>builder/parser/code/package_manager_test.go::TestDetectPackageManager_PackageManagerFieldYarn |
| rainbond.package-manager.parse-package-manager-field | 解析 packageManager 字段语法 | active | regression | builder/parser/code.parsePackageManagerField | builder/parser/code/package_manager_test.go::TestParsePackageManagerField |
| rainbond.package-manager.priority | 多锁文件场景下按优先级识别包管理器 | active | regression | builder/parser/code.DetectPackageManager | builder/parser/code/package_manager_test.go::TestDetectPackageManager_Priority<br>builder/parser/code/package_manager_test.go::TestDetectPackageManager_YarnOverNPM |
| rainbond.package-manager.stringer | 将包管理器枚举渲染为字符串 | active | regression | builder/parser/code.PackageManager.String | builder/parser/code/package_manager_test.go::TestPackageManager_String |
| rainbond.plugin-build.detect-dockerfile | 检测插件源码目录中是否存在 Dockerfile | active | regression | builder/exector.checkDockerfile | builder/exector/plugin_dockerfile_test.go::TestCheckDockerfile |
| rainbond.plugin-build.image-input-validate | 在插件镜像构建前拒绝空值或非法镜像引用 | active | regression | builder/exector.exectorManager.run | builder/exector/plugin_image_test.go::TestPluginImageRunRejectsEmptyImageURL |
| rainbond.plugin-build.image-tag | 根据源镜像名和版本生成插件镜像标签 | active | regression | builder/exector.createPluginImageTag | builder/exector/plugin_image_test.go::TestCreatePluginImageTag |
| rainbond.rainbondfile.missing | 缺少 rainbondfile 时返回未找到 | active | regression | builder/parser/code.ReadRainbondFile | builder/parser/code/rainbondfile_test.go::TestReadRainbondFile_ReturnsNotFoundWhenMissing |
| rainbond.rainbondfile.parse | 解析 rainbondfile YAML 配置 | active | regression | builder/parser/code.ReadRainbondFile | builder/parser/code/rainbondfile_test.go::TestReadRainbondFile_ParsesYamlConfig |
| rainbond.rainbondfile.read-project-root | 从项目根目录读取 rainbondfile | active | unit | builder/parser/code.ReadRainbondFile | builder/parser/code/rainbondfile_test.go::TestReadRainbondFile |
| rainbond.registry.manifest-exists-oci | 备份校验支持 OCI 镜像清单 | active | regression | builder/sources/registry.Registry.ManifestExists | builder/sources/registry/manifest_test.go::TestManifestExistsAcceptsOCIManifestTypes |
| rainbond.resource-center.collect-ingress-services | 收集 Ingress 后端服务名 | active | regression | api/handler.collectIngressServiceNames | api/handler/resource_center_test.go::TestCollectIngressServiceNames |
| rainbond.resource-center.event-summary | 汇总资源事件信息 | active | regression | api/handler.toResourceEventInfo | api/handler/resource_center_test.go::TestToResourceEventInfo |
| rainbond.resource-center.match-selector | 按选择器匹配资源标签 | active | regression | api/handler.labelsMatchSelector | api/handler/resource_center_test.go::TestLabelsMatchSelector |
| rainbond.runtime.composite-nodejs | 复合语言场景使用 Node 运行时解析 | active | regression | builder/parser/code.CheckRuntime | builder/parser/code/runtime_test.go::TestCheckRuntime_CompositeNodejsLanguageUsesNodeRuntime |
| rainbond.runtime.node-defaults | 从 package.json 返回默认 Node 运行时信息 | active | regression | builder/parser/code.CheckRuntime | builder/parser/code/runtime_test.go::TestCheckRuntime_NodejsReturnsDefaultRuntimeInfoFromPackageJson |
| rainbond.runtime.static-empty | 静态语言返回空运行时信息 | active | regression | builder/parser/code.CheckRuntime | builder/parser/code/runtime_test.go::TestCheckRuntime_StaticReturnsEmptyRuntimeInfo |
| rainbond.service-check.completion-log-summary | 服务检测完成摘要日志反映真实检测状态 | active | regression | builder/exector.serviceCheckCompletionLogSummary | builder/exector/service_check_test.go::TestServiceCheckCompletionLogSummary |
| rainbond.share.image-from-snapshot-deploy-version | 镜像分享使用请求中的快照部署版本 | active | regression | api/handler/share.ServiceShareHandle.Share | api/handler/share/service_share_test.go::TestServiceShareUsesRequestedDeployVersionForImageShare |
| rainbond.share.slug-from-snapshot-deploy-version | Slug 分享使用请求中的快照部署版本 | active | regression | api/handler/share.ServiceShareHandle.Share | api/handler/share/service_share_test.go::TestServiceShareUsesRequestedDeployVersionForSlugShare |
| rainbond.source-args.default-cnb-ports | 为多语言项目应用默认 CNB 端口 | active | regression | builder/parser.applyCNBDefaultPorts | builder/parser/source_code_args_test.go::TestCNBDefaultPorts_MultiLanguage |
| rainbond.source-args.multi-language | 为多语言项目解析源码构建参数 | active | regression | builder/parser.SourceCodeParse.GetArgs | builder/parser/source_code_args_test.go::TestGetArgs_MultiLanguage |
| rainbond.source-args.normalize-multi-module-lang | 规范化多模块 Java 项目的语言类型 | active | regression | builder/parser.SourceCodeParse.GetServiceInfo | builder/parser/source_code_args_test.go::TestGetServiceInfo_MultiModulesNormalizeJavaMavenLanguage |
| rainbond.source-detect.dockerfile-subdir | 识别子目录中的 Dockerfile | active | regression | builder/parser/code.GetLangType | builder/parser/code/language_matrix_test.go::TestGetLangType_DetectsDockerfileInSubDirectory |
| rainbond.source-detect.hidden-dockerfiles | 识别隐藏目录中的 Dockerfile | active | regression | builder/parser/code.FindDockerfiles | builder/parser/code/lang_test.go::TestFindDockerfilesInHiddenDirs |
| rainbond.source-detect.ignore-excluded-dirs | 扫描 Dockerfile 时忽略排除目录 | active | regression | builder/parser/code.FindDockerfiles | builder/parser/code/lang_test.go::TestFindDockerfilesIgnoreSpecificDirs |
| rainbond.source-detect.language-matrix | 识别支持的源码构建语言矩阵 | active | regression | builder/parser/code.GetLangType | builder/parser/code/language_matrix_test.go::TestGetLangType_SupportedSourceBuildLanguages |
| rainbond.source-detect.nodejs-over-static | 存在 package.json 时优先识别为 Node.js | active | regression | builder/parser/code.GetLangType | builder/parser/code/language_matrix_test.go::TestGetLangType_NodeJsWinsOverStaticWhenPackageJsonExists |
| rainbond.source-discovery.etcd-config | 配置 parser 的 etcd 发现器并在无客户端时保护抓取逻辑 | active | regression | builder/parser/discovery.NewEtcd | builder/parser/discovery/etcd_test.go::TestNewEtcdAndFetchGuard |
| rainbond.source-discovery.unsupported-type | 对不支持的 parser 发现类型返回空发现器 | active | regression | builder/parser/discovery.NewDiscoverier | builder/parser/discovery/discovery_unit_test.go::TestNewDiscoverierUnsupportedType |
| rainbond.source-image.auth-base64-encode | 将镜像仓库认证信息编码为 base64 JSON 载荷 | active | regression | builder/sources.EncodeAuthToBase64 | builder/sources/image_test.go::TestEncodeAuthToBase64 |
| rainbond.source-image.import | 从归档文件导入镜像 | active | integration | builder/sources.ImageImport | builder/sources/image_test.go::TestImageImport |
| rainbond.source-image.multi-save | 将多个镜像保存为归档文件 | active | integration | builder/sources.MultiImageSave | builder/sources/image_test.go::TestMulitImageSave |
| rainbond.source-image.parse-name | 解析镜像仓库主机名镜像名与标签 | active | regression | builder/sources.ImageNameHandle | builder/sources/image_test.go::TestImageName |
| rainbond.source-image.parse-name-with-namespace | 解析带显式仓库命名空间的镜像引用 | active | regression | builder/sources.ImageNameWithNamespaceHandle | builder/sources/image_test.go::TestImageNameWithNamespace |
| rainbond.source-image.save | 将镜像保存为归档文件 | active | integration | builder/sources.ImageSave | builder/sources/image_test.go::TestImageSave |
| rainbond.source-image.tag-from-ref | 从规范化镜像引用中提取标签 | active | regression | builder/sources.GetTagFromNamedRef | builder/sources/registry_test.go::TestGetTagFromNamedRef |
| rainbond.source-image.trusted-registry-check | 校验受信任的镜像仓库 | active | integration | builder/sources.CheckTrustedRepositories | builder/sources/image_test.go::TestCheckTrustedRepositories |
| rainbond.source-repo.build-info | 构建包含净化地址与构建子目录的仓库元数据 | active | regression | builder/sources.CreateRepostoryBuildInfo | builder/sources/repo_test.go::TestCreateRepostoryBuildInfo |
| rainbond.source-repo.cache-dir | 根据仓库分支租户与服务解析源码缓存目录 | active | regression | builder/sources.GetCodeSourceDir | builder/sources/file_test.go::TestGetCodeSourceDirUsesSourceDirEnv<br>builder/sources/git_test.go::TestGetCodeCacheDir |
| rainbond.source-repo.clone | 克隆 Git 源码仓库 | active | integration | builder/sources.GitClone | builder/sources/git_test.go::TestGitClone |
| rainbond.source-repo.clone-by-tag | 按标签克隆 Git 源码仓库 | active | integration | builder/sources.GitClone | builder/sources/git_test.go::TestGitCloneByTag |
| rainbond.source-repo.git-ref-name | 将分支和标签输入映射为 git 引用名 | active | regression | builder/sources.getBranch | builder/sources/git_test.go::TestGetBranch |
| rainbond.source-repo.pull | 拉取 Git 源码仓库 | active | integration | builder/sources.GitPull | builder/sources/git_test.go::TestGitPull |
| rainbond.source-repo.pull-or-clone | 拉取或克隆 Git 源码仓库 | active | integration | builder/sources.GitCloneOrPull | builder/sources/git_test.go::TestGitPullOrClone |
| rainbond.source-repo.show-url | 从仓库展示地址中去除凭据 | active | regression | builder/sources.getShowURL | builder/sources/git_test.go::TestGetShowURL |
| rainbond.source-repo.temp-build-info | 为源码任务创建独立的临时仓库工作目录 | active | regression | builder/sources.CreateTempRepostoryBuildInfo | builder/sources/repo_test.go::TestCreateTempRepostoryBuildInfo |
| rainbond.source-repo.temp-build-info-pkg | 为 pkg 构建创建临时工作目录信息时保持原始路径 | active | regression | builder/sources.CreateTempRepostoryBuildInfo | builder/sources/repo_test.go::TestCreateTempRepostoryBuildInfoForPkg |
| rainbond.source-sftp.close-safe | 安全关闭零值 SFTP 客户端 | active | regression | builder/sources.SFTPClient.Close | builder/sources/sftp_test.go::TestSFTPClientCloseZeroValue |
| rainbond.source-sftp.port-parse | 解析 SFTP 端口并提供合理默认值 | active | regression | builder/sources.parseSFTPPort | builder/sources/sftp_test.go::TestParseSFTPPort |
| rainbond.source-svn.branch-path | 解析 SVN 分支标签与 trunk 的目标路径 | active | regression | builder/sources.getBranchPath | builder/sources/svn_test.go::TestGetBranchPath |
| rainbond.storage.class-summary | 汇总存储类信息 | active | regression | api/handler.StorageClassInfo | api/handler/storage_test.go::TestStorageClassInfoFields |
| rainbond.storage.handler-singleton | 复用存储处理器单例 | active | unit | api/handler.GetStorageHandler | api/handler/storage_test.go::TestGetStorageHandlerSingleton |
| rainbond.storage.s3-lifecycle-skip-logs | S3 生命周期已配置时不再输出 info 日志 | active | regression | pkg/component/storage.(*S3Storage).ensureBucketLifecycle | pkg/component/storage/s3_storage_test.go::TestEnsureBucketExistsDoesNotLogInfoWhenLifecycleAlreadyConfigured |
| rainbond.third-component.endpoint-address-construct | 构造并校验第三方组件端点地址 | active | regression | pkg/apis/rainbond/v1alpha1.NewEndpointAddress | pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestNewEndpointAddress |
| rainbond.third-component.endpoint-address-ip | 解析端点 IP 与域名哨兵地址 | active | regression | pkg/apis/rainbond/v1alpha1.EndpointAddress.GetIP | pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestEndpointAddressGetIP |
| rainbond.third-component.endpoint-address-port | 从第三方组件端点地址中解析有效端口 | active | regression | pkg/apis/rainbond/v1alpha1.EndpointAddress.GetPort | pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestEndpointAddressGetPort |
| rainbond.third-component.endpoint-address-scheme | 为第三方组件端点地址补齐默认 HTTP 协议 | active | regression | pkg/apis/rainbond/v1alpha1.EndpointAddress.EnsureScheme | pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestEndpointAddressEnsureScheme |
| rainbond.third-component.handler-equals | 比较第三方组件处理器在不同探测模式下的相等性 | active | regression | pkg/apis/rainbond/v1alpha1.Handler.Equals | pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestHandlerEquals |
| rainbond.third-component.http-get-equals | 比较 HTTP 探测处理器及其请求头集合 | active | regression | pkg/apis/rainbond/v1alpha1.HTTPGetAction.Equals | pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestHTTPGetActionEquals |
| rainbond.third-component.identity-fields | 暴露第三方组件身份与端点标识辅助函数 | active | regression | pkg/apis/rainbond/v1alpha1.ThirdComponent.GetEndpointID | pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestThirdComponentIdentityHelpers |
| rainbond.third-component.legacy-endpoint-port | 拆分旧式第三方组件端点的主机与端口对 | active | regression | pkg/apis/rainbond/v1alpha1.ThirdComponentEndpoint.GetPort | pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestThirdComponentEndpointGetPortAndIP |
| rainbond.third-component.probe-equals | 比较第三方组件探测定义是否相等 | active | regression | pkg/apis/rainbond/v1alpha1.Probe.Equals | pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestProbeEquals |
| rainbond.third-component.probe-required | 判断第三方组件是否需要主动探测 | active | regression | pkg/apis/rainbond/v1alpha1.ThirdComponentSpec.NeedProbe | pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestThirdComponentSpecNeedProbe |
| rainbond.third-component.static-endpoints-detect | 检测第三方组件是否使用静态端点 | active | regression | pkg/apis/rainbond/v1alpha1.ThirdComponentSpec.IsStaticEndpoints | pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestThirdComponentSpecIsStaticEndpoints |
| rainbond.util.array-deduplicate | 对字符串切片去重并保留非空元素 | active | regression | util.Deweight | util/comman_test.go::TestDeweight |
| rainbond.util.bytes-equality | 比较字节切片是否完全相等并区分 nil 情况 | active | regression | util.BytesSliceEqual | util/bytes_test.go::TestBytesSliceEqual |
| rainbond.util.bytes-to-string | 使用零拷贝辅助将字节切片转换为字符串 | active | regression | util.ToString | util/bytes_test.go::TestToString |
| rainbond.util.core-helpers.hash-ip-string-uuid | 覆盖哈希IP字符串反转与时间版本等核心辅助行为 | active | regression | util.CreateFileHash | util/hash_test.go::TestCreateFileHash<br>util/ip_test.go::TestCheckIP<br>util/string_test.go::TestReverse<br>util/uuid_test.go::TestTimeVersion |
| rainbond.util.core-helpers.hash-string | 从原始字符串生成稳定的 md5 哈希 | active | regression | util.CreateHashString | util/hash_test.go::TestCreateHashString |
| rainbond.util.core-helpers.string-contains | 检查字符串切片中的成员是否存在 | active | regression | util.StringArrayContains | util/string_test.go::TestStringArrayContains |
| rainbond.util.current-dir-path | 返回规范化的当前工作目录路径 | active | regression | util.GetCurrentDir | util/comman_test.go::TestGetCurrentDir |
| rainbond.util.dir-list-depth | 按目标深度列出嵌套目录 | active | regression | util.GetDirList | util/comman_test.go::TestGetDirList |
| rainbond.util.dir-name-list | 按目标深度列出目录名称 | active | regression | util.GetDirNameList | util/comman_test.go::TestGetDirNameList |
| rainbond.util.dir-size-shell | 使用 shell du 命令计算目录大小 | active | regression | util.GetDirSizeByCmd | util/comman_test.go::TestGetDirSizeByCmd |
| rainbond.util.dir-size-walk | 通过递归遍历文件计算目录大小 | active | regression | util.GetDirSize | util/comman_test.go::TestGetDirSize |
| rainbond.util.env-default | 在环境变量为空时返回默认值 | active | regression | util.GetenvDefault | util/comman_test.go::TestGetenvDefault |
| rainbond.util.etcd-key-id-parse | 从 etcd 风格键中提取稳定 ID | active | regression | util.GetIDFromKey | util/comman_test.go::TestGetIDFromKey |
| rainbond.util.file-copy | 复制文件并保留内容与元数据 | active | regression | util.CopyFile | util/comman_test.go::TestCopyFile |
| rainbond.util.file-list-depth | 按目标深度递归列出文件 | active | regression | util.GetFileList | util/comman_test.go::TestGetFileList |
| rainbond.util.fs.archive-and-directory-ops | 处理文件创建复制归档合并遍历与目录大小辅助操作 | active | regression | util.OpenOrCreateFile | util/comman_test.go::TestOpenOrCreateFile |
| rainbond.util.fuzzy.find | 在候选列表中查找模糊匹配 | active | unit | util/fuzzy.Find | util/fuzzy/fuzzy_test.go::TestFind |
| rainbond.util.fuzzy.find-fold | 在候选列表中按大小写不敏感方式查找模糊匹配 | active | regression | util/fuzzy.FindFold | util/fuzzy/fuzzy_test.go::TestFindFold |
| rainbond.util.fuzzy.levenshtein-distance | 计算字符串之间的 Levenshtein 编辑距离 | active | regression | util/fuzzy.LevenshteinDistance | util/fuzzy/levenshtein_test.go::TestLevenshteinDistance |
| rainbond.util.fuzzy.match | 模糊匹配单个候选字符串 | active | unit | util/fuzzy.Match | util/fuzzy/fuzzy_test.go::TestMatch |
| rainbond.util.fuzzy.match-fold | 执行大小写不敏感的模糊子序列匹配 | active | regression | util/fuzzy.MatchFold | util/fuzzy/fuzzy_test.go::TestMatchFold |
| rainbond.util.fuzzy.rank-find | 对候选列表中的模糊匹配结果排序 | active | unit | util/fuzzy.RankFind | util/fuzzy/fuzzy_test.go::TestRankFind |
| rainbond.util.fuzzy.rank-find-fold | 按编辑距离对大小写不敏感的模糊匹配结果排序 | active | regression | util/fuzzy.RankFindFold | util/fuzzy/fuzzy_test.go::TestRankFindFold |
| rainbond.util.fuzzy.rank-match | 按删除距离为模糊匹配结果打分 | active | regression | util/fuzzy.RankMatch | util/fuzzy/fuzzy_test.go::TestRankMatch |
| rainbond.util.getenv | 返回显式环境变量值或后备默认值 | active | regression | util.Getenv | util/comman_test.go::TestGetenv |
| rainbond.util.host-id-generate | 根据机器状态生成稳定的主机标识 | active | regression | util.CreateHostID | util/comman_test.go::TestCreateHostID |
| rainbond.util.network.interface-address-filter | 在网卡地址扫描中过滤回环与非 IP 地址 | active | regression | util.checkIPAddress | util/ippool_test.go::TestCheckIPAddress |
| rainbond.util.prober.manage-service-health-watchers | 管理探测状态 watcher 并分发健康更新 | active | regression | util/prober.probeManager.handleStatus | util/prober/manager_test.go::TestProbeManager_Start |
| rainbond.util.ssh.auth-method-selection | 选择 SSH 鉴权方式并拒绝不支持的认证模式 | active | regression | util.NewSSHClient | util/sshclient_test.go::TestNewSSHClientSelectsAuthMethod |
| rainbond.util.statefulset-suffix-detect | 检测状态组件名称中的数字后缀 | active | regression | util.IsEndWithNumber | util/comman_test.go::TestIsEndWithNumber |
| rainbond.util.string-to-bytes | 使用辅助函数将字符串转换为字节切片 | active | regression | util.ToByte | util/bytes_test.go::TestToByte |
| rainbond.util.system.identity-and-template-helpers | 提供主机标识版本当前目录变量替换与时间格式化辅助能力 | active | regression | util.CreateHostID | util/comman_test.go::TestDeweight |
| rainbond.util.template-variable-parse | 根据配置映射展开带默认值的模板变量 | active | regression | util.ParseVariable | util/comman_test.go::TestParseVariable |
| rainbond.util.termtables.render-bool-cell | 渲染布尔表格单元格 | active | unit | util/termtables.Cell.Render | util/termtables/cell_test.go::TestCellRenderBool |
| rainbond.util.termtables.render-cell-padding | 按配置填充表格单元格 | active | unit | util/termtables.Cell.Render | util/termtables/cell_test.go::TestCellRenderPadding |
| rainbond.util.termtables.render-float-cell | 渲染浮点表格单元格 | active | unit | util/termtables.Cell.Render | util/termtables/cell_test.go::TestCellRenderFloat |
| rainbond.util.termtables.render-generic-cell | 渲染通用表格单元格值 | active | unit | util/termtables.Cell.Render | util/termtables/cell_test.go::TestCellRenderGeneric |
| rainbond.util.termtables.render-html-alignment | 渲染带显式对齐的 HTML 表格 | active | unit | util/termtables.Table.RenderHTML | util/termtables/html_test.go::TestTableWithAlignment |
| rainbond.util.termtables.render-html-alt-title-style | 以替代标题样式渲染 HTML 表格 | active | unit | util/termtables.Table.SetHTMLStyleTitle | util/termtables/html_test.go::TestTableWithAltTitleStyle |
| rainbond.util.termtables.render-html-set-align | 将 SetAlign 应用于 HTML 表格渲染 | active | unit | util/termtables.Table.SetAlign | util/termtables/html_test.go::TestTableAfterSetAlign |
| rainbond.util.termtables.render-html-table | 渲染带标题与对齐能力的 HTML 表格输出 | active | regression | util/termtables.Table.Render | util/termtables/html_test.go::TestCreateTableHTML |
| rainbond.util.termtables.render-html-title | 渲染带标题的 HTML 表格 | active | unit | util/termtables.Table.RenderHTML | util/termtables/html_test.go::TestTableWithHeaderHTML |
| rainbond.util.termtables.render-html-title-width | 按宽度展开 HTML 表格标题 | active | unit | util/termtables.Table.RenderHTML | util/termtables/html_test.go::TestTableTitleWidthAdjustsHTML |
| rainbond.util.termtables.render-html-unicode-widths | 渲染带 Unicode 宽度的 HTML 表格 | active | unit | util/termtables.Table.RenderHTML | util/termtables/html_test.go::TestTableUnicodeWidthsHTML |
| rainbond.util.termtables.render-html-without-headers | 渲染无表头的 HTML 表格 | active | unit | util/termtables.Table.RenderHTML | util/termtables/html_test.go::TestTableWithNoHeadersHTML |
| rainbond.util.termtables.render-integer-cell | 渲染整数表格单元格 | active | unit | util/termtables.Cell.Render | util/termtables/cell_test.go::TestCellRenderInteger |
| rainbond.util.termtables.render-markdown-table | 渲染 Markdown 表格输出 | active | unit | util/termtables.Table.Render | util/termtables/table_test.go::TestTableInMarkdown |
| rainbond.util.termtables.render-row-width-padding | 按列宽填充渲染后的表格行 | active | unit | util/termtables.Row.Render | util/termtables/row_test.go::TestRowRenderWidthBasedPadding |
| rainbond.util.termtables.render-stringer-cell | 渲染 Stringer 表格单元格 | active | unit | util/termtables.Cell.Render | util/termtables/cell_test.go::TestCellRenderStringerStruct |
| rainbond.util.termtables.render-text-table | 渲染带宽度与 Unicode 处理的终端与 Markdown 表格 | active | regression | util/termtables.Table.Render | util/termtables/table_test.go::TestCreateTable<br>util/termtables/cell_test.go::TestCellRenderString<br>util/termtables/row_test.go::TestBasicRowRender |
| rainbond.util.termtables.render-text-table-append-headers | 在多次 AddHeaders 调用中追加表头 | active | unit | util/termtables.Table.AddHeaders | util/termtables/table_test.go::TestTableMultipleAddHeader |
| rainbond.util.termtables.render-text-table-cjk | 渲染含 CJK 字符的文本表格 | active | unit | util/termtables.Table.Render | util/termtables/table_test.go::TestCJKChars |
| rainbond.util.termtables.render-text-table-header-width | 按表头宽度扩展文本表格 | active | unit | util/termtables.Table.Render | util/termtables/table_test.go::TestTableHeaderWidthAdjusts |
| rainbond.util.termtables.render-text-table-missing-cells | 渲染缺失尾部单元格的文本表格 | active | unit | util/termtables.Table.Render | util/termtables/table_test.go::TestTableMissingCells |
| rainbond.util.termtables.render-text-table-post-align | 在添加行后应用列对齐 | active | unit | util/termtables.Table.SetAlign | util/termtables/table_test.go::TestTableAlignPostsetting |
| rainbond.util.termtables.render-text-table-repeatable | 重复渲染时保持表头一致 | active | unit | util/termtables.Table.Render | util/termtables/table_test.go::TestTableWithHeaderMultipleTimes |
| rainbond.util.termtables.render-text-table-style-reset | 将表格样式重置为 ASCII 渲染 | active | unit | util/termtables.Table.Render | util/termtables/table_test.go::TestStyleResets |
| rainbond.util.termtables.render-text-table-title-unicode-width | 按 Unicode 宽度渲染文本表格标题 | active | unit | util/termtables.Table.Render | util/termtables/table_test.go::TestTitleUnicodeWidths |
| rainbond.util.termtables.render-text-table-title-width | 按标题宽度扩展文本表格 | active | unit | util/termtables.Table.Render | util/termtables/table_test.go::TestTableTitleWidthAdjusts |
| rainbond.util.termtables.render-text-table-unicode-widths | 渲染带 Unicode 宽度的文本表格 | active | unit | util/termtables.Table.Render | util/termtables/table_test.go::TestTableUnicodeWidths |
| rainbond.util.termtables.render-text-table-utf8-box | 使用 UTF-8 边框渲染文本表格 | active | unit | util/termtables.Table.Render | util/termtables/table_test.go::TestTableInUTF8 |
| rainbond.util.termtables.render-text-table-utf8-sgr | 渲染带终端颜色控制序列的 UTF-8 文本表格 | active | unit | util/termtables.Table.Render | util/termtables/table_test.go::TestTableUnicodeUTF8AndSGR |
| rainbond.util.termtables.render-text-table-width-balance | 在多行之间平衡文本表格宽度 | active | unit | util/termtables.Table.Render | util/termtables/table_test.go::TestTableWidthHandling |
| rainbond.util.termtables.render-text-table-width-balance-second | 处理第二类文本表格宽度平衡场景 | active | unit | util/termtables.Table.Render | util/termtables/table_test.go::TestTableWidthHandling_SecondErrorCondition |
| rainbond.util.termtables.render-text-table-with-combining-chars | 渲染含组合字符的文本表格 | active | unit | util/termtables.Table.Render | util/termtables/table_test.go::TestTableWithCombiningChars |
| rainbond.util.termtables.render-text-table-with-fullwidth-chars | 渲染含全角字符的文本表格 | active | unit | util/termtables.Table.Render | util/termtables/table_test.go::TestTableWithFullwidthChars |
| rainbond.util.termtables.render-text-table-with-title | 渲染带标题的文本表格 | active | unit | util/termtables.Table.Render | util/termtables/table_test.go::TestTableWithHeader |
| rainbond.util.termtables.render-text-table-without-headers | 渲染无表头的文本表格 | active | unit | util/termtables.Table.Render | util/termtables/table_test.go::TestTableWithNoHeaders |
| rainbond.util.termtables.strip-color-codes | 移除单元格内容中的 ANSI 颜色码 | active | unit | util/termtables.filterColorCodes | util/termtables/cell_test.go::TestFilterColorCodes |
| rainbond.util.time-format-rfc3339 | 将解析后的时间格式化为 RFC3339 字符串 | active | regression | time.Format | util/comman_test.go::TestTimeFormat |
| rainbond.util.version-timestamp | 生成基于时间戳的版本字符串 | active | regression | util.CreateVersionByTime | util/comman_test.go::TestCreateVersionByTime |
| rainbond.util.whitespace-filter | 从字符串切片中过滤空白与仅空格项 | active | regression | util.RemoveSpaces | util/comman_test.go::TestRemoveSpaces |
| rainbond.util.zip-archive | 将目录归档为 zip 文件 | active | regression | util.Zip | util/comman_test.go::TestZip |
| rainbond.util.zip-structure-detect | 检测 zip 归档是否共享公共根目录 | active | regression | util.detectZipStructure | util/comman_test.go::TestDetectZipStructure |
| rainbond.vm-export.discover-datavolume-disks | Discover DataVolume-backed VM export disks | active | regression | handler.discoverVMExportDisks | api/handler/vm_export_test.go::TestDiscoverVMExportDisksSupportsDataVolumeRootDisk |
| rainbond.vm-run.local-package-storage-download | vm-run 本地包源在目录缺失时回退 storage 下载 | active | regression | builder/sourceutil.ReadLocalPackageDir | builder/sourceutil/local_package_test.go::TestReadLocalPackageDirFallsBackToStorageDownload |
| rainbond.vm-run.remote-package-probe | vm-run 远程包探测优先使用 HEAD | active | regression | builder/parser.VMServiceParse.Parse | builder/parser/vm_service_test.go::TestVMServiceParseRemoteURLPrefersHeadProbe |
| rainbond.vm-run.remote-package-probe-range-fallback | vm-run 远程包探测在 HEAD 失败时回退 Range GET | active | regression | builder/parser.VMServiceParse.Parse | builder/parser/vm_service_test.go::TestVMServiceParseRemoteURLFallsBackToRangeGet |
| rainbond.watch.error-dispatch | 将 watch 后端错误分发到内部错误通道 | active | regression | util/watch.watchChan.sendError | util/watch/watch_test.go::TestWatchChanSendError |
| rainbond.watch.error-parse | 将 watch 后端错误转换为 API 错误事件 | active | regression | util/watch.parseError | util/watch/watch_test.go::TestParseError |
| rainbond.watch.etcd-event-parse | 将 etcd watch 事件解析为内部事件结构 | active | regression | util/watch.parseEvent | util/watch/watch_test.go::TestParseEvent |
| rainbond.watch.event-accessors | 为 watch 事件暴露键与载荷访问器 | active | regression | util/watch.Event.GetKey | util/watch/watch_test.go::TestEventAccessors |
| rainbond.watch.event-byte-accessors | 为 watch 事件暴露原始字节载荷访问器 | active | regression | util/watch.Event.GetValue | util/watch/watch_test.go::TestEventByteAccessors |
| rainbond.watch.event-dispatch | 将底层 watch 事件分发到输入事件通道 | active | regression | util/watch.watchChan.sendEvent | util/watch/watch_test.go::TestWatchChanSendEvent |
| rainbond.watch.event-type-transform | 将底层 watch 事件转换为 Added/Modified/Deleted 结果 | active | regression | util/watch.watchChan.transform | util/watch/watch_test.go::TestWatchChanTransform |
| rainbond.watch.resource-version-parse | 将 watch 资源版本解析为 etcd 修订号 | active | regression | util/watch.ParseWatchResourceVersion | util/watch/watch_test.go::TestParseWatchResourceVersion |
| rainbond.watch.status-error-format | 将 watch 状态错误格式化为稳定字符串 | active | regression | util/watch.Status.Error | util/watch/watch_test.go::TestStatusError |
| rainbond.watch.synthetic-create-event | 将初始 etcd 键值转换为合成创建事件 | active | regression | util/watch.parseKV | util/watch/watch_test.go::TestParseKVMarksCreateEvent |
| rainbond.webcli.auth-signature | 生成稳定的 WebSocket 鉴权 MD5 摘要 | active | regression | api/webcli/app.md5Func | api/webcli/app/app_test.go::TestMD5Func |
| rainbond.webcli.completed-pod-guard | 拒绝对已完成 Pod 建立 exec 会话 | active | regression | api/webcli/app.App.GetContainerArgs | api/webcli/app/app_test.go::TestGetContainerArgsRejectsCompletedPod |
| rainbond.webcli.config-defaults | 为 WebCLI 请求补齐 Kubernetes REST 客户端默认配置 | active | regression | api/webcli/app.SetConfigDefaults | api/webcli/app/app_test.go::TestSetConfigDefaults |
| rainbond.webcli.container-args | 为 WebCLI 会话解析执行容器Pod IP与命令参数 | active | regression | api/webcli/app.App.GetContainerArgs | api/webcli/app/app_test.go::TestGetContainerArgsSelectsContainerAndExecArgs |
| rainbond.webcli.max-width | 限制 WebCLI 终端输出最大宽度 | active | regression | api/webcli/term.NewMaxWidthWriter | api/webcli/term/term_writer_test.go::TestMaxWidthWriter |
| rainbond.webcli.missing-container-guard | 请求的容器不存在时拒绝建立 exec 会话 | active | regression | api/webcli/app.App.GetContainerArgs | api/webcli/app/app_test.go::TestGetContainerArgsRejectsMissingContainer |
| rainbond.webcli.terminal-resize | 为 WebCLI 执行会话排队并应用终端尺寸变更 | active | regression | api/webcli/app.execContext.ResizeTerminal | api/webcli/app/tty_test.go::TestResizeTerminalQueuesWindowSize |
| rainbond.webcli.word-wrap | 按单词折行 WebCLI 终端输出 | active | regression | api/webcli/term.NewWordWrapWriter | api/webcli/term/term_writer_test.go::TestWordWrapWriter |
| rainbond.worker.appm.autoscaler.build-hpa-spec | 根据自动伸缩规则构建 HPA 指标与对象 | active | regression | worker/appm/conversion.newHPA | worker/appm/conversion/autoscaler_test.go::TestNewHPA |
| rainbond.worker.appm.discovery.etcd-config | 配置 appm 的 etcd 发现器并在无客户端时保护抓取逻辑 | active | regression | worker/appm/thirdparty/discovery.NewEtcd | worker/appm/thirdparty/discovery/etcd_test.go::TestNewEtcdAndFetchGuard |
| rainbond.worker.appm.discovery.unsupported-type | 对不支持的 appm 发现后端返回错误 | active | regression | worker/appm/thirdparty/discovery.NewDiscoverier | worker/appm/thirdparty/discovery/discovery_unit_test.go::TestNewDiscoverierUnsupportedType |
| rainbond.worker.appm.patch.statefulset-modified-configuration | 根据新旧工作负载规格计算允许的 StatefulSet Patch 内容 | active | regression | worker/appm/types/v1.getStatefulsetModifiedConfiguration | worker/appm/types/v1/patch_test.go::TestGetStatefulsetModifiedConfiguration |
| rainbond.worker.appm.store.aggregate-app-status | 将组件运行状态汇总为应用状态 | active | regression | worker/appm/store.getAppStatus | worker/appm/store/store_test.go::TestGetAppStatus |
| rainbond.worker.appm.store.sync-managed-namespace-image-pull-secret | 在命名空间事件中同步受管命名空间的镜像拉取密钥 | active | regression | worker/appm/store.appRuntimeStore.nsEventHandler | worker/appm/store/store_test.go::TestNsEventHandlerProvidesAddFunc |
| rainbond.worker.helmapp.chart-ref | 根据仓库名与模板名拼装 Helm chart 引用 | active | regression | worker/master/controller/helmapp.App.Chart | worker/master/controller/helmapp/unit_test.go::TestAppChart |
| rainbond.worker.helmapp.condition-lifecycle | 管理 HelmApp 条件的新增更新与成功态切换 | active | regression | pkg/apis/rainbond/v1alpha1.HelmAppStatus.UpdateConditionStatus | pkg/apis/rainbond/v1alpha1/helmapp_unit_test.go::TestHelmAppStatusConditionLifecycle |
| rainbond.worker.helmapp.condition-query | 按类型查询 HelmApp 条件及其真值状态 | active | regression | pkg/apis/rainbond/v1alpha1.HelmAppStatus.GetCondition | pkg/apis/rainbond/v1alpha1/helmapp_unit_test.go::TestHelmAppStatusConditionQuery |
| rainbond.worker.helmapp.condition-set-noop | 在条件未变化时跳过冗余的 HelmApp 条件写入 | active | regression | pkg/apis/rainbond/v1alpha1.HelmAppStatus.SetCondition | pkg/apis/rainbond/v1alpha1/helmapp_unit_test.go::TestHelmAppStatusSetConditionDoesNotDuplicateUnchangedCondition |
| rainbond.worker.helmapp.condition-status-default-create | 更新条件状态时自动创建缺失的 HelmApp 条件 | active | regression | pkg/apis/rainbond/v1alpha1.HelmAppStatus.UpdateConditionStatus | pkg/apis/rainbond/v1alpha1/helmapp_unit_test.go::TestHelmAppStatusUpdateConditionStatusCreatesMissingCondition |
| rainbond.worker.helmapp.condition-transition-time | 在状态未变化时保留条件的转移时间 | active | regression | pkg/apis/rainbond/v1alpha1.HelmAppStatus.UpdateCondition | pkg/apis/rainbond/v1alpha1/helmapp_unit_test.go::TestHelmAppStatusUpdateConditionPreservesTransitionTimeOnSameStatus |
| rainbond.worker.helmapp.detect-required | 判断 HelmApp 是否仍需执行检测 | active | regression | worker/master/controller/helmapp.App.NeedDetect | worker/master/controller/helmapp/unit_test.go::TestAppNeedDetect |
| rainbond.worker.helmapp.detected-prerequisites | 判断 HelmApp 检测前置条件是否已经满足 | active | regression | worker/master/controller/helmapp.Status.isDetected | worker/master/controller/helmapp/unit_test.go::TestStatusGetPhase |
| rainbond.worker.helmapp.envtest-suite | 启动 HelmApp envtest 集成测试套件 | active | integration | worker/master/controller/helmapp.BeforeSuite | worker/master/controller/helmapp/suite_test.go |
| rainbond.worker.helmapp.finalizer-stop | 通过关闭队列停止 HelmApp finalizer | active | regression | worker/master/controller/helmapp.Finalizer.Stop | worker/master/controller/helmapp/store_unit_test.go::TestFinalizerStop |
| rainbond.worker.helmapp.install-and-deploy | 通过控制循环安装并部署 Helm 应用 | active | integration | worker/master/controller/helmapp.ControlLoop | worker/master/controller/helmapp/controlloop_test.go |
| rainbond.worker.helmapp.overrides-compare | 按无序方式比较 HelmApp 期望与已生效的 overrides | active | regression | pkg/apis/rainbond/v1alpha1.HelmApp.OverridesEqual | pkg/apis/rainbond/v1alpha1/helmapp_unit_test.go::TestHelmAppOverridesEqual |
| rainbond.worker.helmapp.phase-derive | 根据条件与前置状态推导 HelmApp 阶段 | active | regression | worker/master/controller/helmapp.Status.getPhase | worker/master/controller/helmapp/unit_test.go::TestStatusGetPhase |
| rainbond.worker.helmapp.queue-key-parse | 将 HelmApp 队列键拆分为名称与命名空间片段 | active | regression | worker/master/controller/helmapp.nameNamespace | worker/master/controller/helmapp/unit_test.go::TestNameNamespace |
| rainbond.worker.helmapp.reconcile-configuring-phase | 协调 Helm 应用进入配置阶段 | active | integration | worker/master/controller/helmapp.ControlLoop | worker/master/controller/helmapp/controlloop_test.go |
| rainbond.worker.helmapp.reconcile-default-values | 协调 Helm 应用默认值 | active | integration | worker/master/controller/helmapp.ControlLoop | worker/master/controller/helmapp/controlloop_test.go |
| rainbond.worker.helmapp.reconcile-start-detecting | 协调 Helm 应用进入检测阶段 | active | integration | worker/master/controller/helmapp.ControlLoop | worker/master/controller/helmapp/controlloop_test.go |
| rainbond.worker.helmapp.setup-required | 判断 HelmApp 是否仍需初始化默认配置 | active | regression | worker/master/controller/helmapp.App.NeedSetup | worker/master/controller/helmapp/unit_test.go::TestAppNeedSetup |
| rainbond.worker.helmapp.store-fetch | 从控制器 store lister 获取 HelmApp 对象 | active | regression | worker/master/controller/helmapp.store.GetHelmApp | worker/master/controller/helmapp/store_unit_test.go::TestStoreGetHelmApp |
| rainbond.worker.helmapp.store-full-name | 根据 EID 与商店名构建完整应用商店名称 | active | regression | pkg/apis/rainbond/v1alpha1.HelmAppSpec.FullName | pkg/apis/rainbond/v1alpha1/helmapp_unit_test.go::TestHelmAppSpecFullName |
| rainbond.worker.helmapp.update-required | 判断已配置 HelmApp 是否需要安装或更新 | active | regression | worker/master/controller/helmapp.App.NeedUpdate | worker/master/controller/helmapp/unit_test.go::TestAppNeedUpdate |
| rainbond.worker.pod-status.describe | 根据条件容器状态与事件归类 Pod 状态 | active | regression | worker/util.DescribePodStatus | worker/util/pod_test.go::TestDescribePodStatus |
| rainbond.worker.thirdcomponent.prober.execute-endpoint-probe | 执行第三方组件端点探测并映射结果 | active | regression | worker/master/controller/thirdcomponent/prober.prober.probe | worker/master/controller/thirdcomponent/prober/prober_test.go::TestProbe |
| rainbond.worker.thirdcomponent.prober.manage-results-cache | 缓存并清理第三方组件探测结果 | active | regression | worker/master/controller/thirdcomponent/prober/results.NewManager | worker/master/controller/thirdcomponent/prober/results/results_manager_test.go::TestCacheOperations |
| rainbond.worker.volume-provider.pvc-identifiers | 根据 PVC 名称解析 Pod 名与卷 ID | active | regression | worker/master/volumes/provider.getVolumeIDByPVCName | worker/master/volumes/provider/rainbondsslc_test.go::TestGetVolumeIDByPVCName |
| rainbond.worker.volume-provider.select-node | 按可用内存选择存储节点 | active | integration | worker/master/volumes/provider.rainbondsslcProvisioner.selectNode | worker/master/volumes/provider/rainbondsslc_test.go::TestSelectNode |
| rainbond.worker.volume-type.from-storageclass | 将存储类转换为 Rainbond 卷类型 | active | regression | worker/util.TransStorageClass2RBDVolumeType | worker/util/volumetype_test.go::TestTransStorageClass2RBDVolumeType |

## 详情

### 识别旧版与新版应用备份元数据结构

- Capability ID: `rainbond.app-backup.metadata-version-detect`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/exector.judgeMetadataVersion`
- 代码路径: `builder/exector/groupapp_backup.go`
- 测试路径: `builder/exector/groupapp_backup_test.go::TestJudgeMetadataVersion`

### 将服务卷数据归档为备份包

- Capability ID: `rainbond.app-backup.service-volume-archive`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/exector.BackupAPPNew.backupServiceInfo`
- 代码路径: `builder/exector/groupapp_backup.go`
- 测试路径: `builder/exector/groupapp_backup_test.go::TestBackupServiceVolume`

### 将应用备份包上传到外部存储

- Capability ID: `rainbond.app-backup.upload-package`
- 状态: `active`
- 测试类型: `integration`
- 接口类型: `workflow`
- 业务入口: `builder/exector.BackupAPPNew.uploadPkg`
- 代码路径: `builder/exector/groupapp_backup.go`
- 测试路径: `builder/exector/groupapp_backup_test.go::TestUploadPkg`

### 在备份包上传流程中保护已移除的下载接口

- Capability ID: `rainbond.app-backup.upload-package-download-guard`
- 状态: `active`
- 测试类型: `integration`
- 接口类型: `workflow`
- 业务入口: `builder/exector.BackupAPPNew.uploadPkg`
- 代码路径: `builder/exector/groupapp_backup.go`
- 测试路径: `builder/exector/groupapp_backup_test.go::TestUploadPkg2`

### 根据环境变量解析本地与共享备份卷目录

- Capability ID: `rainbond.app-backup.volume-dir-defaults`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/exector.GetVolumeDir`
- 代码路径: `builder/exector/groupapp_backup.go`
- 测试路径: `builder/exector/groupapp_backup_test.go::TestGetVolumeDir`

### 将组件绑定到应用配置组

- Capability ID: `rainbond.app-config-group.bind-component`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `dao_method`
- 业务入口: `db/mysql/dao.AppConfigGroupServiceDaoImpl.AddModel`
- 代码路径: `db/mysql/dao/application_config_group.go`
- 测试路径: `db/mysql/dao/application_config_group_test.go::TestAppConfigGroupServiceDaoAddModel`

### 创建应用配置组记录

- Capability ID: `rainbond.app-config-group.create`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `dao_method`
- 业务入口: `db/mysql/dao.AppConfigGroupDaoImpl.AddModel`
- 代码路径: `db/mysql/dao/application_config_group.go`
- 测试路径: `db/mysql/dao/application_config_group_test.go::TestAppConfigGroupDaoAddModel`

### 删除应用配置组记录

- Capability ID: `rainbond.app-config-group.delete`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `dao_method`
- 业务入口: `db/mysql/dao.AppConfigGroupDaoImpl.DeleteConfigGroup`
- 代码路径: `db/mysql/dao/application_config_group.go`
- 测试路径: `db/mysql/dao/application_config_group_test.go::TestDeleteConfigGroup`

### 查询应用配置组记录

- Capability ID: `rainbond.app-config-group.get`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `dao_method`
- 业务入口: `db/mysql/dao.AppConfigGroupDaoImpl.GetConfigGroupByID`
- 代码路径: `db/mysql/dao/application_config_group.go`
- 测试路径: `db/mysql/dao/application_config_group_test.go::TestAppGetConfigGroupByID`

### 创建应用配置项

- Capability ID: `rainbond.app-config-group.item-create`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `dao_method`
- 业务入口: `db/mysql/dao.AppConfigGroupItemDaoImpl.AddModel`
- 代码路径: `db/mysql/dao/application_config_group.go`
- 测试路径: `db/mysql/dao/application_config_group_test.go::TestAppConfigGroupItemDaoAddModel`

### 删除应用配置项

- Capability ID: `rainbond.app-config-group.item-delete`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `dao_method`
- 业务入口: `db/mysql/dao.AppConfigGroupItemDaoImpl.DeleteConfigGroupItem`
- 代码路径: `db/mysql/dao/application_config_group.go`
- 测试路径: `db/mysql/dao/application_config_group_test.go::TestDeleteConfigGroupItem`

### 更新应用配置项

- Capability ID: `rainbond.app-config-group.item-update`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `dao_method`
- 业务入口: `db/mysql/dao.AppConfigGroupItemDaoImpl.UpdateModel`
- 代码路径: `db/mysql/dao/application_config_group.go`
- 测试路径: `db/mysql/dao/application_config_group_test.go::TestAppConfigGroupItemDaoUpdateModel`

### 移除应用配置组组件绑定

- Capability ID: `rainbond.app-config-group.unbind-components`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `dao_method`
- 业务入口: `db/mysql/dao.AppConfigGroupServiceDaoImpl.DeleteConfigGroupService`
- 代码路径: `db/mysql/dao/application_config_group.go`
- 测试路径: `db/mysql/dao/application_config_group_test.go::TestDeleteConfigGroupService`

### 从 Linux 文件名还原导入镜像包名

- Capability ID: `rainbond.app-import.package-name-normalize`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/exector.buildFromLinuxFileName`
- 代码路径: `builder/exector/import_app.go`
- 测试路径: `builder/exector/import_app_test.go::TestBuildFromLinuxFileName`

### 序列化并解析按应用记录的导入状态

- Capability ID: `rainbond.app-import.status-serialization`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/exector.map2str`
- 代码路径: `builder/exector/import_app.go`
- 测试路径: `builder/exector/import_app_test.go::TestAppStatusMapRoundTrip`

### 应用恢复时重写镜像仓库地址

- Capability ID: `rainbond.app-restore.image-registry-rewrite`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/exector.getNewImageName`
- 代码路径: `builder/exector/groupapp_restore.go`
- 测试路径: `builder/exector/groupapp_restore_test.go::TestGetImageName`

### 从恢复后的服务映射中反查原始服务 ID

- Capability ID: `rainbond.app-restore.service-id-lookup`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/exector.BackupAPPRestore.getOldServiceID`
- 代码路径: `builder/exector/groupapp_restore.go`
- 测试路径: `builder/exector/groupapp_restore_test.go::TestGetOldServiceID`

### 应用恢复时重写服务依赖关系

- Capability ID: `rainbond.app-restore.snapshot-relationship-rewrite`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/exector.BackupAPPRestore.modify`
- 代码路径: `builder/exector/groupapp_restore.go`
- 测试路径: `builder/exector/groupapp_restore_test.go::TestModify`

### 在恢复时解压完整备份数据包

- Capability ID: `rainbond.app-restore.unzip-all-data`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/exector.BackupAPPRestore`
- 代码路径: `builder/exector/groupapp_restore.go`
- 测试路径: `builder/exector/groupapp_restore_test.go::TestUnzipAllDataFile`

### 按源码语言和构建类型选择构建器

- Capability ID: `rainbond.build.select-builder-by-language`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build.GetBuildByType`
- 代码路径: `builder/build/build.go`
- 测试路径: `builder/build/build_type_matrix_test.go::TestGetBuildByType_SourceBuildLanguageMatrix`

### 已注册 worker 分发时不再误报未知任务

- Capability ID: `rainbond.builder.registered-worker-dispatch`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/exector.exectorManager.RunTask`
- 代码路径: `builder/exector/exector.go`
- 测试路径: `builder/exector/exector_test.go::TestRunTaskDoesNotWarnForRegisteredWorker`

### 将 AliOSS 服务错误转换为统一存储 SDK 错误

- Capability ID: `rainbond.cloud-storage.alioss-error-map`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/cloudos.svcErrToS3SDKError`
- 代码路径: `builder/cloudos/alioss.go`
- 测试路径: `builder/cloudos/alioss_test.go::TestSvcErrToS3SDKError`

### 将云存储配置分发到正确的驱动实现

- Capability ID: `rainbond.cloud-storage.driver-factory`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/cloudos.New`
- 代码路径: `builder/cloudos/cloudos.go`
- 测试路径: `builder/cloudos/cloudos_test.go::TestNewDispatchesProviderDrivers`

### 解析云存储 provider 配置值

- Capability ID: `rainbond.cloud-storage.provider-parse`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/cloudos.Str2S3Provider`
- 代码路径: `builder/cloudos/cloudos.go`
- 测试路径: `builder/cloudos/cloudos_test.go::TestStr2S3Provider`

### 按预期配置初始化 S3 存储驱动

- Capability ID: `rainbond.cloud-storage.s3-driver-config`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/cloudos.newS3`
- 代码路径: `builder/cloudos/s3.go`
- 测试路径: `builder/cloudos/s3_test.go::TestNewS3DriverKeepsConfig`

### 识别集群资源子路径

- Capability ID: `rainbond.cluster-resource.detect-subresource`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.containsSlash`
- 代码路径: `api/handler/cluster_resource.go`
- 测试路径: `api/handler/cluster_resource_test.go::TestContainsSlash`

### 复用集群资源处理器单例

- Capability ID: `rainbond.cluster-resource.handler-singleton`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `api/handler.GetClusterResourceHandler`
- 代码路径: `api/handler/cluster_resource.go`
- 测试路径: `api/handler/cluster_resource_test.go::TestGetClusterResourceHandlerSingleton`

### 校验集群资源 GVR 参数

- Capability ID: `rainbond.cluster-resource.validate-gvr`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.validateGVRParams`
- 代码路径: `api/handler/cluster_resource.go`
- 测试路径: `api/handler/cluster_resource_test.go::TestValidateGVRParams`

### 从 CNB 版本表达式提取主版本

- Capability ID: `rainbond.cnb-version.extract-major`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.extractMajorFromSpec`
- 代码路径: `builder/parser/code/cnb_versions.go`
- 测试路径: `builder/parser/code/cnb_versions_test.go::TestExtractMajorFromSpec`

### 保持 Go CNB 版本顺序并将最新版本设为默认

- Capability ID: `rainbond.cnb-version.golang-order-and-default`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.GetCNBVersions`
- 代码路径: `builder/parser/code/cnb_versions.go`
- 测试路径: `builder/parser/code/cnb_versions_test.go::TestGetCNBVersionsGoOrderingAndDefault`

### 归一化并匹配 Go CNB 版本表达式

- Capability ID: `rainbond.cnb-version.match-golang`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.MatchCNBVersion`
- 代码路径: `builder/parser/code/cnb_versions.go`
- 测试路径: `builder/parser/code/cnb_versions_test.go::TestMatchCNBVersion_Golang`

### 为复合语言匹配 CNB 版本

- Capability ID: `rainbond.cnb-version.match-language`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.MatchCNBVersion`
- 代码路径: `builder/parser/code/cnb_versions.go`
- 测试路径: `builder/parser/code/cnb_versions_test.go::TestMatchCNBVersion_CompositeLanguage`

### 保持 Python CNB 版本顺序并将最新版本设为默认

- Capability ID: `rainbond.cnb-version.python-order-and-default`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.GetCNBVersions`
- 代码路径: `builder/parser/code/cnb_versions.go`
- 测试路径: `builder/parser/code/cnb_versions_test.go::TestGetCNBVersionsPythonOrderingAndDefault`

### 按语言解析支持的 CNB 版本

- Capability ID: `rainbond.cnb-version.resolve-supported`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.GetCNBVersions`
- 代码路径: `builder/parser/code/cnb_versions.go`
- 测试路径: `builder/parser/code/cnb_versions_test.go::TestGetCNBVersions`

### 将 CNB 注解键解码为 BP 环境变量名

- Capability ID: `rainbond.cnb.annotation-key-decode`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.annotationKeyToBPEnv`
- 代码路径: `builder/build/cnb/platform.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestAnnotationKeyToBPEnv`

### 将 BP 环境变量名编码为 CNB 注解键

- Capability ID: `rainbond.cnb.annotation-key-encode`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.bpEnvToAnnotationKey`
- 代码路径: `builder/build/cnb/platform.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestBpEnvToAnnotationKey`

### 显式 CNB 注解优先于 BP 透传值

- Capability ID: `rainbond.cnb.bp-annotation-priority`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.Builder.buildPlatformAnnotations`
- 代码路径: `builder/build/cnb/platform.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestBuildPlatformAnnotationsBPNoOverride`

### 执行 CNB 构建任务全流程

- Capability ID: `rainbond.cnb.build-job-execution`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.Builder.runCNBBuildJob`
- 代码路径: `builder/build/cnb/build.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestRunCNBBuildJob`

### 解析 CNB builder 镜像

- Capability ID: `rainbond.cnb.builder-image`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.GetCNBBuilderImage`
- 代码路径: `builder/build/cnb/config.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestGetCNBBuilderImage`

### 注入指定的 CNB 配置文件内容

- Capability ID: `rainbond.cnb.config-file`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.injectConfigFile`
- 代码路径: `builder/build/cnb/lang_nodejs.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestInjectConfigFile`

### 生成 CNB creator 命令参数

- Capability ID: `rainbond.cnb.creator-args`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.Builder.buildCreatorArgs`
- 代码路径: `builder/build/cnb/job.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestBuildCreatorArgs`

### 为 CNB creator 附加 insecure registry 参数

- Capability ID: `rainbond.cnb.creator-args-insecure-registry`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.Builder.buildCreatorArgs`
- 代码路径: `builder/build/cnb/job.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestBuildCreatorArgsInsecureRegistry`

### 解析 CNB 依赖镜像源

- Capability ID: `rainbond.cnb.dependency-mirror`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.getDependencyMirror`
- 代码路径: `builder/build/cnb/lang_nodejs.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestBuildPlatformAnnotationsMirrorDefault`, `builder/build/cnb/cnb_test.go::TestBuildPlatformAnnotationsMirrorExplicit`, `builder/build/cnb/cnb_test.go::TestGetDependencyMirrorOffline`

### 生成 CNB 构建任务环境变量

- Capability ID: `rainbond.cnb.env-vars`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.Builder.buildEnvVars`
- 代码路径: `builder/build/cnb/job.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestBuildEnvVars`

### 按来源模式注入 CNB 镜像配置

- Capability ID: `rainbond.cnb.mirror-config`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.nodejsConfig.InjectMirrorConfig`
- 代码路径: `builder/build/cnb/lang_nodejs.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestInjectMirrorConfig`

### 传播 CNB 镜像配置写入失败

- Capability ID: `rainbond.cnb.mirror-config-write-error`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.nodejsConfig.InjectMirrorConfig`
- 代码路径: `builder/build/cnb/lang_nodejs.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestInjectMirrorConfigWriteError`

### 创建 CNB 构建器实例

- Capability ID: `rainbond.cnb.new-builder`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.NewBuilder`
- 代码路径: `builder/build/cnb/build.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestNewBuilder`

### 解析离线模式下的 CNB 镜像行为

- Capability ID: `rainbond.cnb.offline-mode`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb offline mode helpers`
- 代码路径: `builder/build/cnb/config.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestIsOfflineMode`, `builder/build/cnb/cnb_test.go::TestGetCNBBuilderImageOffline`, `builder/build/cnb/cnb_test.go::TestGetCNBRunImageOffline`

### 写入自定义 CNB order 定义

- Capability ID: `rainbond.cnb.order-toml`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.Builder.writeCustomOrder`
- 代码路径: `builder/build/cnb/order.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestWriteCustomOrder`

### CNB order 文件写入失败时返回空标记

- Capability ID: `rainbond.cnb.order-write-failure`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.Builder.writeCustomOrder`
- 代码路径: `builder/build/cnb/order.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestWriteCustomOrderFailure`

### 根据构建环境生成 CNB 平台注解

- Capability ID: `rainbond.cnb.platform-annotations`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.Builder.buildPlatformAnnotations`
- 代码路径: `builder/build/cnb/platform.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestBuildPlatformAnnotations`

### 根据注解创建 CNB 平台卷

- Capability ID: `rainbond.cnb.platform-volume`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.Builder.createPlatformVolume`
- 代码路径: `builder/build/cnb/platform.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestCreatePlatformVolume`, `builder/build/cnb/cnb_test.go::TestCreatePlatformVolumeEmpty`, `builder/build/cnb/cnb_test.go::TestCreatePlatformVolumeNonCNBKeys`

### 清理历史 CNB 预构建任务

- Capability ID: `rainbond.cnb.prebuild-job-cleanup`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.Builder.stopPreBuildJob`
- 代码路径: `builder/build/cnb/build.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestStopPreBuildJob`

### CNB 构建前校验项目文件

- Capability ID: `rainbond.cnb.project-file-validation`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.Builder.validateProjectFiles`
- 代码路径: `builder/build/cnb/build.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestValidateProjectFiles`

### 解析 CNB run 镜像

- Capability ID: `rainbond.cnb.run-image`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.GetCNBRunImage`
- 代码路径: `builder/build/cnb/config.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestGetCNBRunImage`

### CNB 构建前规范化源码目录权限

- Capability ID: `rainbond.cnb.source-dir-permissions`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.Builder.setSourceDirPermissions`
- 代码路径: `builder/build/cnb/build.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestSetSourceDirPermissions`, `builder/build/cnb/cnb_test.go::TestSetSourceDirPermissionsNonexistent`

### 纯静态源码使用 nginx buildpack

- Capability ID: `rainbond.cnb.static-buildpacks`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.staticConfig.CustomOrder`
- 代码路径: `builder/build/cnb/order.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestStaticBuildpacks`

### 创建 CNB 构建卷与挂载

- Capability ID: `rainbond.cnb.volume-mounts`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.Builder.createVolumeAndMount`
- 代码路径: `builder/build/cnb/job.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestCreateVolumeAndMount`

### 等待 CNB 构建任务完成状态

- Capability ID: `rainbond.cnb.waiting-complete`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/build/cnb.Builder.waitingComplete`
- 代码路径: `builder/build/cnb/job.go`
- 测试路径: `builder/build/cnb/cnb_test.go::TestWaitingComplete`

### 保留配置卷文件内容字段语义

- Capability ID: `rainbond.compose.config-volume-file-content`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/types.Volume.FileContent`
- 代码路径: `builder/parser/types/types.go`
- 测试路径: `builder/parser/file_content_test.go::TestVolumeFileContent`

### 识别配置文件类型的挂载路径

- Capability ID: `rainbond.compose.detect-config-file-mount`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/compose.isConfigFile`
- 代码路径: `builder/parser/compose/version_detect.go`
- 测试路径: `builder/parser/compose/version_detect_test.go::TestIsConfigFile`

### 根据语法特征推断 compose 版本

- Capability ID: `rainbond.compose.detect-version`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/compose.inferComposeVersion`
- 代码路径: `builder/parser/compose/version_detect.go`
- 测试路径: `builder/parser/compose/version_detect_test.go::TestInferComposeVersion`

### 解析 docker compose 并返回降级告警

- Capability ID: `rainbond.compose.parse-warnings`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser.CreateDockerComposeParse.Parse`
- 代码路径: `builder/parser/docker_compose.go`
- 测试路径: `builder/parser/docker_compose_warnings_test.go::TestDockerComposeParseWithWarnings`

### 支持 docker compose 中的 YAML anchors

- Capability ID: `rainbond.compose.yaml-anchor-support`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser.CreateDockerComposeParse.Parse`
- 代码路径: `builder/parser/docker_compose.go`
- 测试路径: `builder/parser/docker_compose_warnings_test.go::TestDockerComposeParseWithYAMLAnchors`

### 识别源码目录中的 npm 和 yarn 配置文件

- Capability ID: `rainbond.config-files.detect`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectConfigFiles`
- 代码路径: `builder/parser/code/config_files.go`
- 测试路径: `builder/parser/code/config_files_test.go::TestDetectConfigFiles_Npmrc`, `builder/parser/code/config_files_test.go::TestDetectConfigFiles_YarnrcClassic`, `builder/parser/code/config_files_test.go::TestDetectConfigFiles_YarnrcYml`, `builder/parser/code/config_files_test.go::TestDetectConfigFiles_Multiple`, `builder/parser/code/config_files_test.go::TestDetectConfigFiles_None`

### 检测源码中是否存在包管理器配置文件

- Capability ID: `rainbond.config-files.has-any`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.ConfigFiles.HasAnyConfigFile`
- 代码路径: `builder/parser/code/config_files.go`
- 测试路径: `builder/parser/code/config_files_test.go::TestConfigFiles_HasAnyConfigFile`

### 读取源码中的 npmrc 内容

- Capability ID: `rainbond.config-files.read-npmrc`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.ConfigFiles.GetNpmrcContent`
- 代码路径: `builder/parser/code/config_files.go`
- 测试路径: `builder/parser/code/config_files_test.go::TestConfigFiles_GetNpmrcContent`

### 读取源码中的 yarnrc 内容

- Capability ID: `rainbond.config-files.read-yarnrc`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.ConfigFiles.GetYarnrcContent`
- 代码路径: `builder/parser/code/config_files.go`
- 测试路径: `builder/parser/code/config_files_test.go::TestConfigFiles_GetYarnrcContent`

### 为包管理器选择相关配置文件

- Capability ID: `rainbond.config-files.resolve-relevant-file`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.ConfigFiles.GetRelevantConfigFile`
- 代码路径: `builder/parser/code/config_files.go`
- 测试路径: `builder/parser/code/config_files_test.go::TestConfigFiles_GetRelevantConfigFile`

### 跟踪 Dockerfile AST 的行号信息

- Capability ID: `rainbond.dockerfile.line-info`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/dockerfile/parser.Parse`
- 代码路径: `util/dockerfile/parser/parser.go`
- 测试路径: `util/dockerfile/parser/parser_test.go::TestLineInformation`

### 将标准 Dockerfile 示例解析为稳定 AST

- Capability ID: `rainbond.dockerfile.parse-fixtures`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/dockerfile/parser.Parse`
- 代码路径: `util/dockerfile/parser/parser.go`
- 测试路径: `util/dockerfile/parser/parser_test.go::TestTestData`

### 解析 Dockerfile 指令中的 JSON 数组语法

- Capability ID: `rainbond.dockerfile.parse-json-array`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/dockerfile/parser.parseJSON`
- 代码路径: `util/dockerfile/parser/line_parsers.go`
- 测试路径: `util/dockerfile/parser/json_test.go::TestJSONArraysOfStrings`

### 按引号与转义规则拆分 Dockerfile 指令参数

- Capability ID: `rainbond.dockerfile.parse-words`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/dockerfile/parser.parseWords`
- 代码路径: `util/dockerfile/parser/parser.go`
- 测试路径: `util/dockerfile/parser/parser_test.go::TestParseWords`

### 解析 Dockerfile 时拒绝无效示例

- Capability ID: `rainbond.dockerfile.reject-invalid`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/dockerfile/parser.Parse`
- 代码路径: `util/dockerfile/parser/parser.go`
- 测试路径: `util/dockerfile/parser/parser_test.go::TestTestNegative`

### 从协议与端口中拆分端点地址

- Capability ID: `rainbond.endpoint.address-split`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/endpoint.SplitEndpointAddress`
- 代码路径: `util/endpoint/validation.go`
- 测试路径: `util/endpoint/validation_test.go::TestSplitEndpointAddress`

### 判断端点地址应按域名而不是 IP 处理

- Capability ID: `rainbond.endpoint.domain-not-ip`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/endpoint.IsDomainNotIP`
- 代码路径: `util/endpoint/validation.go`
- 测试路径: `util/endpoint/validation_test.go::TestIsDomainNotIP`

### 校验端点域名及通配域名

- Capability ID: `rainbond.endpoint.domain-validate`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/endpoint.ValidateDomain`
- 代码路径: `util/endpoint/validation.go`
- 测试路径: `util/endpoint/validation_test.go::TestValidateDomain`

### 校验端点 IP 地址并拒绝受限网段

- Capability ID: `rainbond.endpoint.ip-validate`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/endpoint.ValidateEndpointIP`
- 代码路径: `util/endpoint/validation.go`
- 测试路径: `util/endpoint/validation_test.go::TestValidateEndpointIP`

### 判断内存大小是自定义值还是预设值

- Capability ID: `rainbond.envutil.custom-memory`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/envutil.IsCustomMemory`
- 代码路径: `util/envutil/envutil.go`
- 测试路径: `util/envutil/envutil_test.go::TestIsCustomMemory`

### 在 envutil 中为缺失环境变量返回默认值

- Capability ID: `rainbond.envutil.getenv-default`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/envutil.GetenvDefault`
- 代码路径: `util/envutil/envutil.go`
- 测试路径: `util/envutil/envutil_test.go::TestGetenvDefault`

### 将内存大小映射为预设内存标签

- Capability ID: `rainbond.envutil.memory-label`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/envutil.GetMemoryType`
- 代码路径: `util/envutil/envutil.go`
- 测试路径: `util/envutil/envutil_test.go::TestGetMemoryType`

### 事件日志文件存储的追加读取与清理

- Capability ID: `rainbond.eventlog.file-store`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/eventlog/store.JSONLinesFileStore`
- 代码路径: `api/eventlog/store/filestore.go`
- 测试路径: `api/eventlog/store/filestore_test.go::TestJSONLinesFileStore`

### 事件日志文件存储支持并发写入

- Capability ID: `rainbond.eventlog.file-store-concurrency`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/eventlog/store.JSONLinesFileStore.Append`
- 代码路径: `api/eventlog/store/filestore.go`
- 测试路径: `api/eventlog/store/filestore_test.go::TestFileStoreConcurrency`

### 幂等初始化并复用火山引擎 NAS 客户端

- Capability ID: `rainbond.filepersistence.volcengine-client-init`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/component/filepersistence.VolcengineProvider.init`
- 代码路径: `pkg/component/filepersistence/volcengine.go`
- 测试路径: `pkg/component/filepersistence/volcengine_test.go::TestVolcengineProviderInitIsIdempotent`

### 识别 Angular SPA 模式

- Capability ID: `rainbond.framework-detect.angular-spa`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectFramework`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestDetectFramework_Angular_SPA`

### 识别 Angular SSR 模式

- Capability ID: `rainbond.framework-detect.angular-ssr`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectFramework`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestDetectFramework_Angular_SSR`

### 识别 Create React App 框架

- Capability ID: `rainbond.framework-detect.cra`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectFramework`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestDetectFramework_CRA`

### 解析框架展示名称

- Capability ID: `rainbond.framework-detect.display-name`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.GetDisplayName`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestGetDisplayName`

### 识别 Express 框架

- Capability ID: `rainbond.framework-detect.express`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectFramework`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestDetectFramework_Express`

### 识别 NestJS 框架

- Capability ID: `rainbond.framework-detect.nestjs`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectFramework`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestDetectFramework_NestJS`

### 从源码目录识别 Next.js 框架

- Capability ID: `rainbond.framework-detect.nextjs`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectFramework`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestDetectFramework_NextJS`

### 无配置文件时识别 Next.js

- Capability ID: `rainbond.framework-detect.nextjs-no-config`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectFramework`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestDetectFramework_NextJS_NoConfigFile`

### 识别 Next.js SSR 模式

- Capability ID: `rainbond.framework-detect.nextjs-ssr`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectFramework`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestDetectFramework_NextJS_SSR`

### 识别 Next.js 静态导出模式

- Capability ID: `rainbond.framework-detect.nextjs-static-export`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectFramework`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestDetectFramework_NextJS_StaticExport`

### 普通 Node 项目不识别框架

- Capability ID: `rainbond.framework-detect.no-framework`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectFramework`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestDetectFramework_NoFramework`

### 缺少 package.json 时不识别前端框架

- Capability ID: `rainbond.framework-detect.no-package-json`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectFramework`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestDetectFramework_NoPackageJSON`

### 从源码目录识别 Nuxt 框架

- Capability ID: `rainbond.framework-detect.nuxt`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectFramework`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestDetectFramework_Nuxt`

### 识别 Nuxt 静态目标模式

- Capability ID: `rainbond.framework-detect.nuxt-static-target`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectFramework`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestDetectFramework_Nuxt_StaticTarget`

### 识别关闭 SSR 的 Nuxt3

- Capability ID: `rainbond.framework-detect.nuxt3-ssr-false`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectFramework`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestDetectFramework_Nuxt3_SSRFalse`

### 识别 Nuxt3 静态输出模式

- Capability ID: `rainbond.framework-detect.nuxt3-static`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectFramework`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestDetectFramework_Nuxt3_NitroStatic`

### 列出支持识别的前端框架

- Capability ID: `rainbond.framework-detect.supported-list`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.GetSupportedFrameworks`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestGetSupportedFrameworks`

### 规范化框架依赖版本号

- Capability ID: `rainbond.framework-detect.version-normalization`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.cleanVersion`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestCleanVersion`

### 识别 Vite 框架

- Capability ID: `rainbond.framework-detect.vite`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectFramework`
- 代码路径: `builder/parser/code/framework.go`
- 测试路径: `builder/parser/code/framework_test.go::TestDetectFramework_Vite`

### 分配可用网关负载均衡端口

- Capability ID: `rainbond.gateway.allocate-lb-port`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.selectAvailablePort`
- 代码路径: `api/handler/gateway_action.go`
- 测试路径: `api/handler/gateway_action_test.go::TestSelectAvailablePort`

### 为 Helm 历史输出格式化应用版本号

- Capability ID: `rainbond.helm-release.app-version-format`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/helm.formatAppVersion`
- 代码路径: `pkg/helm/helm.go`
- 测试路径: `pkg/helm/helm_release_test.go::TestGetReleaseHistory`

### 为历史和摘要输出格式化 Helm chart 名称

- Capability ID: `rainbond.helm-release.chart-name-format`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/helm.formatChartName`
- 代码路径: `pkg/helm/helm.go`
- 测试路径: `pkg/helm/helm_release_test.go::TestGetReleaseHistory`

### 按资源类型归类 Helm 发布资源

- Capability ID: `rainbond.helm-release.classify-resources`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.splitHelmReleaseResources`
- 代码路径: `api/handler/helm_release.go`
- 测试路径: `api/handler/helm_release_test.go::TestSplitHelmReleaseResourcesClassifiesKinds`

### 推导 Helm 发布默认命名空间

- Capability ID: `rainbond.helm-release.default-namespace`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.helmReleaseNamespace`
- 代码路径: `api/handler/helm_release.go`
- 测试路径: `api/handler/helm_release_test.go::TestHelmReleaseNamespaceUsesTenantNamespaceWhenPresent`, `api/handler/helm_release_test.go::TestHelmReleaseNamespaceFallsBackToTenantUUID`

### 汇总 Helm 发布详情

- Capability ID: `rainbond.helm-release.detail-summary`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.summarizeHelmReleaseDetail`
- 代码路径: `api/handler/helm_release.go`
- 测试路径: `api/handler/helm_release_test.go::TestSummarizeHelmReleaseDetailBuildsStableDTO`

### 汇总 Helm 发布历史

- Capability ID: `rainbond.helm-release.history-summary`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `handler_method`
- 业务入口: `api/handler.summarizeHelmReleaseHistory`
- 代码路径: `api/handler/helm_release.go`
- 测试路径: `api/handler/helm_release_test.go::TestSummarizeHelmReleaseHistoryBuildsStableDTO`, `pkg/helm/helm_release_test.go::TestGetReleaseHistory`

### 规范 Helm 安装默认参数

- Capability ID: `rainbond.helm-release.install-defaults`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `handler_method`
- 业务入口: `api/handler.HelmReleaseInstallRequest.Normalize`
- 代码路径: `api/handler/helm_release.go`
- 测试路径: `api/handler/helm_release_test.go::TestHelmReleaseInstallRequestNormalizeDefaults`

### 校验 Helm 安装请求

- Capability ID: `rainbond.helm-release.install-validate`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `handler_method`
- 业务入口: `api/handler.HelmReleaseInstallRequest.Validate`
- 代码路径: `api/handler/helm_release.go`
- 测试路径: `api/handler/helm_release_test.go::TestHelmReleaseInstallRequestValidate`

### 拒绝不可安装的 Helm chart 类型

- Capability ID: `rainbond.helm-release.installable-check`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/helm.checkIfInstallable`
- 代码路径: `pkg/helm/helm.go`
- 测试路径: `pkg/helm/helm_release_test.go::TestCheckIfInstallable`

### 汇总 Helm 发布列表项

- Capability ID: `rainbond.helm-release.list-summary`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.summarizeHelmRelease`
- 代码路径: `api/handler/helm_release.go`
- 测试路径: `api/handler/helm_release_test.go::TestSummarizeHelmReleaseBuildsStableDTO`

### 识别 Helm 托管资源归属

- Capability ID: `rainbond.helm-release.match-managed-resource`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.isHelmReleaseResource`
- 代码路径: `api/handler/helm_release.go`
- 测试路径: `api/handler/helm_release_test.go::TestIsHelmReleaseResourceMatchesManagedByAndInstanceLabels`

### 规范化 OCI chart 引用并推导版本标签

- Capability ID: `rainbond.helm-release.oci-reference-normalize`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/helm.normalizeOCIChartReference`
- 代码路径: `pkg/helm/helm.go`
- 测试路径: `pkg/helm/helm_release_test.go::TestNormalizeOCIChartReference`

### 将 Helm 预览来源错误转换为错误请求

- Capability ID: `rainbond.helm-release.preview-source-error`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.wrapHelmChartPreviewSourceError`
- 代码路径: `api/handler/helm_release.go`
- 测试路径: `api/handler/helm_release_test.go::TestWrapHelmChartPreviewSourceErrorConvertsToBadRequest`, `api/handler/helm_release_test.go::TestWrapHelmChartPreviewSourceErrorPreservesBadRequest`

### 解析 Helm 发布命名空间

- Capability ID: `rainbond.helm-release.resolve-namespace`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `handler_method`
- 业务入口: `api/handler.HelmReleaseHandler.resolveNamespace`
- 代码路径: `api/handler/helm_release.go`
- 测试路径: `api/handler/helm_release_test.go::TestResolveHelmReleaseNamespaceUsesExplicitNamespace`

### 请求未指定时从团队推导 Helm 命名空间

- Capability ID: `rainbond.helm-release.resolve-namespace-fallback`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.(*HelmReleaseHandler).resolveNamespace`
- 代码路径: `api/handler/helm_release.go`
- 测试路径: `api/handler/helm_release_test.go::TestResolveHelmReleaseNamespaceFallsBackToTenantNamespace`

### 校验 Helm 回滚版本

- Capability ID: `rainbond.helm-release.rollback-validate`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.HelmReleaseRollbackRequest.Validate`
- 代码路径: `api/handler/helm_release.go`
- 测试路径: `api/handler/helm_release_test.go::TestHelmReleaseRollbackRequestValidate`

### 在安装或加载前移除 chart 的 kubeVersion 要求

- Capability ID: `rainbond.helm-release.strip-kube-version`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/helm.removeKubeVersionFromChart`
- 代码路径: `pkg/helm/helm.go`
- 测试路径: `pkg/helm/helm_release_test.go::TestCheckIfInstallable`

### 拦截 Helm 升级图表不匹配

- Capability ID: `rainbond.helm-release.upgrade-chart-guard`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.validateUpgradeChartName`
- 代码路径: `api/handler/helm_release.go`
- 测试路径: `api/handler/helm_release_test.go::TestValidateUpgradeChartNameRejectsMismatchByDefault`, `api/handler/helm_release_test.go::TestValidateUpgradeChartNameAllowsMismatchWithExplicitConfirmation`

### 将 Helm values YAML 解析为可安装的 values 映射

- Capability ID: `rainbond.helm-release.values-yaml`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/helm.parseValuesYAML`
- 代码路径: `pkg/helm/helm.go`
- 测试路径: `pkg/helm/helm_release_test.go::TestParseValuesYAML`

### 添加 Helm 仓库

- Capability ID: `rainbond.helm-repo.add`
- 状态: `active`
- 测试类型: `integration`
- 接口类型: `workflow`
- 业务入口: `pkg/helm.Repo.Add`
- 代码路径: `pkg/helm/repo.go`
- 测试路径: `pkg/helm/repo_test.go::TestRepoAdd`

### 当相同 Helm 仓库已存在时跳过重复添加

- Capability ID: `rainbond.helm-repo.add-idempotent`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/helm.Repo.Add`
- 代码路径: `pkg/helm/repo.go`
- 测试路径: `pkg/helm/repo_test.go::TestRepoAddSkipsExistingConfig`

### 拒绝已废弃的 Helm 仓库地址

- Capability ID: `rainbond.helm-repo.reject-deprecated`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/helm.Repo.Add`
- 代码路径: `pkg/helm/repo.go`
- 测试路径: `pkg/helm/repo_test.go::TestRepoAddRejectsDeprecatedRepo`

### 校验并匹配请求更新的 Helm 仓库名称

- Capability ID: `rainbond.helm-repo.requested-filter`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/helm.checkRequestedRepos`
- 代码路径: `pkg/helm/update.go`
- 测试路径: `pkg/helm/helm_release_test.go::TestCheckRequestedRepos`

### 当没有匹配的仓库 Pod 时跳过垃圾回收执行

- Capability ID: `rainbond.image-clean.registry-gc-noop`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/clean.Manager.PodExecCmd`
- 代码路径: `builder/clean/clean.go`
- 测试路径: `builder/clean/clean_test.go::TestPodExecCmdNoMatchingPod`

### 通过取消上下文停止镜像清理管理器循环

- Capability ID: `rainbond.image-clean.stop-loop`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/clean.Manager.Stop`
- 代码路径: `builder/clean/clean.go`
- 测试路径: `builder/clean/clean_test.go::TestManagerStopCancelsContext`

### 为 ingress-nginx 监听对象构建 namespace/name 键

- Capability ID: `rainbond.ingress-nginx.meta-namespace-key`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/ingress-nginx/k8s.MetaNamespaceKey`
- 代码路径: `util/ingress-nginx/k8s/main.go`
- 测试路径: `util/ingress-nginx/k8s/main_test.go::TestMetaNamespaceKey`

### 解析 ingress-nginx 资源的 namespace/name 标识

- Capability ID: `rainbond.ingress-nginx.name-namespace-parse`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/ingress-nginx/k8s.ParseNameNS`
- 代码路径: `util/ingress-nginx/k8s/main.go`
- 测试路径: `util/ingress-nginx/k8s/main_test.go::TestParseNameNS`

### 为 ingress-nginx helper 解析节点内外网 IP

- Capability ID: `rainbond.ingress-nginx.node-ip-resolve`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/ingress-nginx/k8s.GetNodeIPOrName`
- 代码路径: `util/ingress-nginx/k8s/main.go`
- 测试路径: `util/ingress-nginx/k8s/main_test.go::TestGetNodeIPOrName`

### 根据环境变量和集群状态解析 ingress-nginx Pod 详情

- Capability ID: `rainbond.ingress-nginx.pod-details`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/ingress-nginx/k8s.GetPodDetails`
- 代码路径: `util/ingress-nginx/k8s/main.go`
- 测试路径: `util/ingress-nginx/k8s/main_test.go::TestGetPodDetails`

### K8s scheme registers KubeVirt VirtualMachine

- Capability ID: `rainbond.k8s.scheme-registers-kubevirt-vm`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `package_function`
- 业务入口: `pkg/component/k8s.init`
- 代码路径: `pkg/component/k8s/k8sComponent.go`
- 测试路径: `pkg/component/k8s/k8sComponent_test.go::TestSchemeRegistersKubeVirtVirtualMachine`

### 列出 kb-adapter 可用的备份仓库

- Capability ID: `rainbond.kb-adapter.backup-repo.list-ready`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/backup.Service.ListAvailableBackupRepos`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/backup/backup.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/backup/backup_test.go::TestListAvailableBackupRepos`

### 按服务范围删除允许清理的集群备份

- Capability ID: `rainbond.kb-adapter.cluster-backup.delete`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/backup.Service.DeleteBackups`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/backup/backup.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/backup/backup_test.go::TestDeleteBackups`

### 判断集群备份是否允许安全删除

- Capability ID: `rainbond.kb-adapter.cluster-backup.delete-guard`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/backup.Service.canDeleteBackup`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/backup/backup.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/backup/backup_test.go::TestCanDeleteBackup`

### 列出目标服务对应的集群备份

- Capability ID: `rainbond.kb-adapter.cluster-backup.list`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/backup.Service.ListBackups`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/backup/backup.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/backup/backup_test.go::TestListBackups`

### 根据插件输入创建更新或关闭集群备份计划

- Capability ID: `rainbond.kb-adapter.cluster-backup.schedule-reconcile`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/backup.Service.ReScheduleBackup`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/backup/backup.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/backup/backup_test.go::TestReScheduleBackup`

### 将 KubeBlocks 集群关联到 Rainbond 服务 ID

- Capability ID: `rainbond.kb-adapter.cluster.associate-service-id`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/cluster.Service.associateToKubeBlocksComponent`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/cluster/cluster.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/cluster/cluster_test.go::TestAssociateToKubeBlocksComponent`

### 获取插件详情页所需的集群连接凭据

- Capability ID: `rainbond.kb-adapter.cluster.connection-info`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/cluster.Service.GetConnectInfo`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/cluster/info.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/cluster/info_test.go::TestGetConnectInfo`

### 通过 kb-adapter 插件创建 KubeBlocks 集群

- Capability ID: `rainbond.kb-adapter.cluster.create`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/cluster.Service.CreateCluster`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/cluster/lifecycle.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/cluster/lifecycle_test.go::TestCreateCluster`

### 在集群清理时删除 OpsRequest 与关联 Secret

- Capability ID: `rainbond.kb-adapter.cluster.delete-cleanup`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/cluster.Service.cleanupClusterOpsRequests`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/cluster/lifecycle.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/cluster/lifecycle_test.go::TestCleanupClusterOpsRequests`

### 构建插件集群详情摘要并包含资源与备份信息

- Capability ID: `rainbond.kb-adapter.cluster.detail-summary`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/cluster.Service.GetClusterDetail`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/cluster/info.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/cluster/info_test.go::TestGetClusterDetail`

### 根据 OpsRequest 构建集群操作事件时间线

- Capability ID: `rainbond.kb-adapter.cluster.event-timeline`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/cluster.Service.GetClusterEvents`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/cluster/event.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/cluster/event_test.go::TestGetClusterEvents`

### 为插件视图列出集群 Pod 并解析 InstanceSet

- Capability ID: `rainbond.kb-adapter.cluster.list-pods`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/cluster.Service.getClusterPods`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/cluster/cluster.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/cluster/cluster_test.go::TestGetClusterPods`

### 合并实时参数项与参数约束定义

- Capability ID: `rainbond.kb-adapter.cluster.parameter-constraint-merge`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/cluster.mergeEntriesAndConstraints`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/cluster/parameter.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/cluster/parameter_test.go::TestMergeEntriesAndConstraints`

### 为插件诊断页构建详细 Pod 诊断信息

- Capability ID: `rainbond.kb-adapter.cluster.pod-detail`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/cluster.Service.GetPodDetail`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/cluster/pod.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/cluster/pod_test.go::TestGetPodDetail`

### 从备份恢复集群并清理失败的恢复操作

- Capability ID: `rainbond.kb-adapter.cluster.restore-from-backup`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/cluster.Service.RestoreFromBackup`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/cluster/restore.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/cluster/restore_test.go::TestRestoreFromBackup`

### 通过插件工作流扩缩集群副本资源与存储

- Capability ID: `rainbond.kb-adapter.cluster.scale`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/cluster.Service.ExpansionCluster`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/cluster/scaling.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/cluster/scaling_test.go::TestExpansionCluster`

### 将插件参数值解析为带类型的协调器参数项

- Capability ID: `rainbond.kb-adapter.coordinator.parameter-value-parse`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/coordinator.Coordinator.ParseParameters`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/coordinator/coordinator.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/coordinator/coordinator_test.go::TestBase_ParseParameters`

### 列出并清理阻塞中的未终态 OpsRequest

- Capability ID: `rainbond.kb-adapter.opsrequest.blocking-ops-management`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/kbkit.GetAllNonFinalOpsRequests`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/kbkit/opsrequest.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/kbkit/opsrequest_test.go::TestGetAllNonFinalOpsRequests`

### 创建生命周期备份扩缩容参数变更与恢复等 OpsRequest

- Capability ID: `rainbond.kb-adapter.opsrequest.create-supported-ops`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/kbkit.CreateLifecycleOpsRequest`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/kbkit/opsrequest.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/kbkit/opsrequest_test.go::TestCreateLifecycleOpsRequest`

### 在提交新操作前裁决冲突的 OpsRequest

- Capability ID: `rainbond.kb-adapter.opsrequest.preflight-arbitration`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/service/kbkit.preflightCheck`
- 代码路径: `plugins/kb-adapter-rbdplugin/service/kbkit/opsrequest.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/service/kbkit/opsrequest_preflight_test.go::TestUniqueOpsDecide`

### 识别 kb-adapter 开发环境模式

- Capability ID: `rainbond.kb-adapter.server-config.dev-mode`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/internal/config.InDevelopment`
- 代码路径: `plugins/kb-adapter-rbdplugin/internal/config/config.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/internal/config/config_test.go::TestInDevelopment`

### 从环境变量加载 kb-adapter 服务配置

- Capability ID: `rainbond.kb-adapter.server-config.load-from-env`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/internal/config.LoadConfigFromEnv`
- 代码路径: `plugins/kb-adapter-rbdplugin/internal/config/config.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/internal/config/config_test.go::TestLoadConfigFromEnv`

### 强制加载并校验 kb-adapter 服务配置

- Capability ID: `rainbond.kb-adapter.server-config.must-load`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/internal/config.MustLoad`
- 代码路径: `plugins/kb-adapter-rbdplugin/internal/config/config.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/internal/config/config_test.go::TestMustLoad`

### 校验 kb-adapter 服务配置项

- Capability ID: `rainbond.kb-adapter.server-config.validate`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `plugins/kb-adapter-rbdplugin/internal/config.ServerConfig.Validate`
- 代码路径: `plugins/kb-adapter-rbdplugin/internal/config/config.go`
- 测试路径: `plugins/kb-adapter-rbdplugin/internal/config/config_test.go::TestServerConfig_Validate`

### 为 KubeBlocks 组件生成标签选择器

- Capability ID: `rainbond.kubeblocks.component-selector`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/kubeblocks.GenerateKubeBlocksSelector`
- 代码路径: `util/kubeblocks/kubeblocks.go`
- 测试路径: `util/kubeblocks/kubeblocks_test.go::TestGenerateKubeBlocksSelector`

### 解码并解析许可证令牌内容

- Capability ID: `rainbond.license.decode`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/util/license.DecodeLicense`
- 代码路径: `api/util/license/rsa_license.go`
- 测试路径: `api/util/license/rsa_license_test.go::TestDecodeLicense`

### 解析 PEM 编码的 RSA 公钥

- Capability ID: `rainbond.license.parse-public-key`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/util/license.ParsePublicKey`
- 代码路径: `api/util/license/rsa_license.go`
- 测试路径: `api/util/license/rsa_license_test.go::TestParsePublicKey`

### 根据许可证映射判断插件是否允许使用

- Capability ID: `rainbond.license.plugin-allowlist`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/util/license.IsPluginAllowed`
- 代码路径: `api/util/license/rsa_license.go`
- 测试路径: `api/util/license/rsa_license_test.go::TestIsPluginAllowed_Wildcard`

### 往返编码解码并校验许可证令牌

- Capability ID: `rainbond.license.round-trip`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/util/license.EncodeLicense`
- 代码路径: `api/util/license/rsa_license.go`
- 测试路径: `api/util/license/rsa_license_test.go::TestRoundTrip`

### 将许可证令牌投影为状态响应

- Capability ID: `rainbond.license.status-projection`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/util/license.TokenToStatus`
- 代码路径: `api/util/license/rsa_license.go`
- 测试路径: `api/util/license/rsa_license_test.go::TestTokenToStatus`

### 校验许可证企业绑定与生效时间窗口

- Capability ID: `rainbond.license.validate-token`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/util/license.ValidateToken`
- 代码路径: `api/util/license/rsa_license.go`
- 测试路径: `api/util/license/rsa_license_test.go::TestValidateToken_Valid`

### 校验许可证 RSA 签名

- Capability ID: `rainbond.license.verify-signature`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/util/license.VerifySignature`
- 代码路径: `api/util/license/rsa_license.go`
- 测试路径: `api/util/license/rsa_license_test.go::TestVerifySignature_Valid`

### 列出 Maven 多服务模块

- Capability ID: `rainbond.maven.list-modules`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code/multisvc.maven.ListModules`
- 代码路径: `builder/parser/code/multisvc/maven.go`
- 测试路径: `builder/parser/code/multisvc/maven_test.go::TestMaven_ListModules`

### 解析 Maven 父 pom 的模块与打包方式

- Capability ID: `rainbond.maven.parse-pom`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code/multisvc.parsePom`
- 代码路径: `builder/parser/code/multisvc/maven.go`
- 测试路径: `builder/parser/code/multisvc/maven_test.go::TestMaven_ParsePom`

### 在多服务解析器选择中忽略非 Java 语言

- Capability ID: `rainbond.multisvc.ignore-non-java`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code/multisvc.NewMultiServiceI`
- 代码路径: `builder/parser/code/multisvc/multi_services.go`
- 测试路径: `builder/parser/code/multisvc/multi_services_test.go::TestNewMultiServiceI_IgnoresLanguagesWithoutJavaMaven`

### 为复合语言选择 Java Maven 多服务解析器

- Capability ID: `rainbond.multisvc.select-java-maven`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code/multisvc.NewMultiServiceI`
- 代码路径: `builder/parser/code/multisvc/multi_services.go`
- 测试路径: `builder/parser/code/multisvc/multi_services_test.go::TestNewMultiServiceI_SupportsCompositeJavaMaven`

### 汇总 Node 版本展示与派生信息

- Capability ID: `rainbond.node-version.display-info`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.NodeVersionInfo helpers`
- 代码路径: `builder/parser/code/node_version.go`
- 测试路径: `builder/parser/code/node_version_test.go::TestCleanVersionSpec`, `builder/parser/code/node_version_test.go::TestExtractMajorVersion`, `builder/parser/code/node_version_test.go::TestExtractMinorPatch`, `builder/parser/code/node_version_test.go::TestNodeVersionInfo_IsLTS`, `builder/parser/code/node_version_test.go::TestNodeVersionInfo_GetNodeVersionDisplay`

### 将不受支持的 Node.js 版本回退到支持范围

- Capability ID: `rainbond.node-version.fallback-supported-range`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.ResolveNodeVersion`
- 代码路径: `builder/parser/code/node_version.go`
- 测试路径: `builder/parser/code/node_version_test.go::TestResolveNodeVersion_UnsupportedVersion`

### 规范化带 v 前缀的 Node.js 版本

- Capability ID: `rainbond.node-version.normalize-v-prefix`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.ResolveNodeVersion`
- 代码路径: `builder/parser/code/node_version.go`
- 测试路径: `builder/parser/code/node_version_test.go::TestResolveNodeVersion_WithVPrefix`

### 从 package.json 解析 Node 版本要求

- Capability ID: `rainbond.node-version.parse-package-json`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.ParseNodeVersionFromPackageJSON`
- 代码路径: `builder/parser/code/node_version.go`
- 测试路径: `builder/parser/code/node_version_test.go::TestParseNodeVersionFromPackageJSON`, `builder/parser/code/node_version_test.go::TestParseNodeVersionFromPackageJSON_NoEngines`, `builder/parser/code/node_version_test.go::TestParseNodeVersionFromPackageJSON_NoPackageJSON`

### 解析范围形式的 Node.js 版本约束

- Capability ID: `rainbond.node-version.resolve-range`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.ResolveNodeVersion`
- 代码路径: `builder/parser/code/node_version.go`
- 测试路径: `builder/parser/code/node_version_test.go::TestResolveNodeVersion_Range`

### 将 Node 版本表达式解析为支持的运行时版本

- Capability ID: `rainbond.node-version.resolve-spec`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.ResolveNodeVersion`
- 代码路径: `builder/parser/code/node_version.go`
- 测试路径: `builder/parser/code/node_version_test.go::TestResolveNodeVersion_Empty`, `builder/parser/code/node_version_test.go::TestResolveNodeVersion_Wildcard`, `builder/parser/code/node_version_test.go::TestResolveNodeVersion_GreaterThanOrEqual`, `builder/parser/code/node_version_test.go::TestResolveNodeVersion_Caret`, `builder/parser/code/node_version_test.go::TestResolveNodeVersion_Tilde`, `builder/parser/code/node_version_test.go::TestResolveNodeVersion_XNotation`, `builder/parser/code/node_version_test.go::TestResolveNodeVersion_MajorOnly`, `builder/parser/code/node_version_test.go::TestResolveNodeVersion_ExactVersion`

### 识别命名空间资源来源

- Capability ID: `rainbond.ns-resource.detect-source`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.detectResourceSource`
- 代码路径: `api/handler/ns_resource.go`
- 测试路径: `api/handler/ns_resource_test.go::TestDetectResourceSource`

### 复用命名空间资源处理器单例

- Capability ID: `rainbond.ns-resource.handler-singleton`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `api/handler.GetNsResourceHandler`
- 代码路径: `api/handler/ns_resource.go`
- 测试路径: `api/handler/ns_resource_test.go::TestGetNsResourceHandlerSingleton`

### 标记命名空间资源来源

- Capability ID: `rainbond.ns-resource.mark-source`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.injectSourceLabel`
- 代码路径: `api/handler/ns_resource.go`
- 测试路径: `api/handler/ns_resource_test.go::TestInjectSourceLabelYaml`, `api/handler/ns_resource_test.go::TestInjectSourceLabelManual`

### 解析团队命名空间

- Capability ID: `rainbond.ns-resource.resolve-tenant-namespace`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.(*NsResourceHandler).getTenantNamespace`
- 代码路径: `api/handler/ns_resource.go`
- 测试路径: `api/handler/ns_resource_test.go::TestGetTenantNamespaceUsesNamespaceField`, `api/handler/ns_resource_test.go::TestGetTenantNamespaceFallsBackToUUIDWhenNamespaceEmpty`

### 按包管理器生成安装构建启动命令

- Capability ID: `rainbond.package-manager.commands`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.PackageManagerInfo`
- 代码路径: `builder/parser/code/package_manager.go`
- 测试路径: `builder/parser/code/package_manager_test.go::TestPackageManagerInfo_GetCommands`

### 无锁文件时默认使用 npm

- Capability ID: `rainbond.package-manager.default-npm`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectPackageManager`
- 代码路径: `builder/parser/code/package_manager.go`
- 测试路径: `builder/parser/code/package_manager_test.go::TestDetectPackageManager_Default`

### 通过锁文件识别包管理器

- Capability ID: `rainbond.package-manager.detect-lockfile`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectPackageManager`
- 代码路径: `builder/parser/code/package_manager.go`
- 测试路径: `builder/parser/code/package_manager_test.go::TestDetectPackageManager_PNPM`, `builder/parser/code/package_manager_test.go::TestDetectPackageManager_Yarn`, `builder/parser/code/package_manager_test.go::TestDetectPackageManager_NPM`

### 通过 package.json 字段识别包管理器

- Capability ID: `rainbond.package-manager.package-json-field`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectPackageManager`
- 代码路径: `builder/parser/code/package_manager.go`
- 测试路径: `builder/parser/code/package_manager_test.go::TestDetectPackageManager_PackageManagerField`, `builder/parser/code/package_manager_test.go::TestDetectPackageManager_PackageManagerFieldYarn`

### 解析 packageManager 字段语法

- Capability ID: `rainbond.package-manager.parse-package-manager-field`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.parsePackageManagerField`
- 代码路径: `builder/parser/code/package_manager.go`
- 测试路径: `builder/parser/code/package_manager_test.go::TestParsePackageManagerField`

### 多锁文件场景下按优先级识别包管理器

- Capability ID: `rainbond.package-manager.priority`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.DetectPackageManager`
- 代码路径: `builder/parser/code/package_manager.go`
- 测试路径: `builder/parser/code/package_manager_test.go::TestDetectPackageManager_Priority`, `builder/parser/code/package_manager_test.go::TestDetectPackageManager_YarnOverNPM`

### 将包管理器枚举渲染为字符串

- Capability ID: `rainbond.package-manager.stringer`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.PackageManager.String`
- 代码路径: `builder/parser/code/package_manager.go`
- 测试路径: `builder/parser/code/package_manager_test.go::TestPackageManager_String`

### 检测插件源码目录中是否存在 Dockerfile

- Capability ID: `rainbond.plugin-build.detect-dockerfile`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/exector.checkDockerfile`
- 代码路径: `builder/exector/plugin_dockerfile.go`
- 测试路径: `builder/exector/plugin_dockerfile_test.go::TestCheckDockerfile`

### 在插件镜像构建前拒绝空值或非法镜像引用

- Capability ID: `rainbond.plugin-build.image-input-validate`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/exector.exectorManager.run`
- 代码路径: `builder/exector/plugin_image.go`
- 测试路径: `builder/exector/plugin_image_test.go::TestPluginImageRunRejectsEmptyImageURL`

### 根据源镜像名和版本生成插件镜像标签

- Capability ID: `rainbond.plugin-build.image-tag`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/exector.createPluginImageTag`
- 代码路径: `builder/exector/plugin_image.go`
- 测试路径: `builder/exector/plugin_image_test.go::TestCreatePluginImageTag`

### 缺少 rainbondfile 时返回未找到

- Capability ID: `rainbond.rainbondfile.missing`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.ReadRainbondFile`
- 代码路径: `builder/parser/code/rainbondfile.go`
- 测试路径: `builder/parser/code/rainbondfile_test.go::TestReadRainbondFile_ReturnsNotFoundWhenMissing`

### 解析 rainbondfile YAML 配置

- Capability ID: `rainbond.rainbondfile.parse`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.ReadRainbondFile`
- 代码路径: `builder/parser/code/rainbondfile.go`
- 测试路径: `builder/parser/code/rainbondfile_test.go::TestReadRainbondFile_ParsesYamlConfig`

### 从项目根目录读取 rainbondfile

- Capability ID: `rainbond.rainbondfile.read-project-root`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.ReadRainbondFile`
- 代码路径: `builder/parser/code/rainbondfile.go`
- 测试路径: `builder/parser/code/rainbondfile_test.go::TestReadRainbondFile`

### 备份校验支持 OCI 镜像清单

- Capability ID: `rainbond.registry.manifest-exists-oci`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/sources/registry.Registry.ManifestExists`
- 代码路径: `builder/sources/registry/manifest.go`
- 测试路径: `builder/sources/registry/manifest_test.go::TestManifestExistsAcceptsOCIManifestTypes`

### 收集 Ingress 后端服务名

- Capability ID: `rainbond.resource-center.collect-ingress-services`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.collectIngressServiceNames`
- 代码路径: `api/handler/resource_center.go`
- 测试路径: `api/handler/resource_center_test.go::TestCollectIngressServiceNames`

### 汇总资源事件信息

- Capability ID: `rainbond.resource-center.event-summary`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.toResourceEventInfo`
- 代码路径: `api/handler/resource_center.go`
- 测试路径: `api/handler/resource_center_test.go::TestToResourceEventInfo`

### 按选择器匹配资源标签

- Capability ID: `rainbond.resource-center.match-selector`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.labelsMatchSelector`
- 代码路径: `api/handler/resource_center.go`
- 测试路径: `api/handler/resource_center_test.go::TestLabelsMatchSelector`

### 复合语言场景使用 Node 运行时解析

- Capability ID: `rainbond.runtime.composite-nodejs`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.CheckRuntime`
- 代码路径: `builder/parser/code/runtime.go`
- 测试路径: `builder/parser/code/runtime_test.go::TestCheckRuntime_CompositeNodejsLanguageUsesNodeRuntime`

### 从 package.json 返回默认 Node 运行时信息

- Capability ID: `rainbond.runtime.node-defaults`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.CheckRuntime`
- 代码路径: `builder/parser/code/runtime.go`
- 测试路径: `builder/parser/code/runtime_test.go::TestCheckRuntime_NodejsReturnsDefaultRuntimeInfoFromPackageJson`

### 静态语言返回空运行时信息

- Capability ID: `rainbond.runtime.static-empty`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.CheckRuntime`
- 代码路径: `builder/parser/code/runtime.go`
- 测试路径: `builder/parser/code/runtime_test.go::TestCheckRuntime_StaticReturnsEmptyRuntimeInfo`

### 服务检测完成摘要日志反映真实检测状态

- Capability ID: `rainbond.service-check.completion-log-summary`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `other`
- 业务入口: `builder/exector.serviceCheckCompletionLogSummary`
- 代码路径: `builder/exector/service_check.go`
- 测试路径: `builder/exector/service_check_test.go::TestServiceCheckCompletionLogSummary`

### 镜像分享使用请求中的快照部署版本

- Capability ID: `rainbond.share.image-from-snapshot-deploy-version`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler/share.ServiceShareHandle.Share`
- 代码路径: `api/handler/share/service_share.go`
- 测试路径: `api/handler/share/service_share_test.go::TestServiceShareUsesRequestedDeployVersionForImageShare`

### Slug 分享使用请求中的快照部署版本

- Capability ID: `rainbond.share.slug-from-snapshot-deploy-version`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler/share.ServiceShareHandle.Share`
- 代码路径: `api/handler/share/service_share.go`
- 测试路径: `api/handler/share/service_share_test.go::TestServiceShareUsesRequestedDeployVersionForSlugShare`

### 为多语言项目应用默认 CNB 端口

- Capability ID: `rainbond.source-args.default-cnb-ports`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser.applyCNBDefaultPorts`
- 代码路径: `builder/parser/source_code.go`
- 测试路径: `builder/parser/source_code_args_test.go::TestCNBDefaultPorts_MultiLanguage`

### 为多语言项目解析源码构建参数

- Capability ID: `rainbond.source-args.multi-language`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser.SourceCodeParse.GetArgs`
- 代码路径: `builder/parser/source_code.go`
- 测试路径: `builder/parser/source_code_args_test.go::TestGetArgs_MultiLanguage`

### 规范化多模块 Java 项目的语言类型

- Capability ID: `rainbond.source-args.normalize-multi-module-lang`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser.SourceCodeParse.GetServiceInfo`
- 代码路径: `builder/parser/source_code.go`
- 测试路径: `builder/parser/source_code_args_test.go::TestGetServiceInfo_MultiModulesNormalizeJavaMavenLanguage`

### 识别子目录中的 Dockerfile

- Capability ID: `rainbond.source-detect.dockerfile-subdir`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.GetLangType`
- 代码路径: `builder/parser/code/lang.go`
- 测试路径: `builder/parser/code/language_matrix_test.go::TestGetLangType_DetectsDockerfileInSubDirectory`

### 识别隐藏目录中的 Dockerfile

- Capability ID: `rainbond.source-detect.hidden-dockerfiles`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.FindDockerfiles`
- 代码路径: `builder/parser/code/lang.go`
- 测试路径: `builder/parser/code/lang_test.go::TestFindDockerfilesInHiddenDirs`

### 扫描 Dockerfile 时忽略排除目录

- Capability ID: `rainbond.source-detect.ignore-excluded-dirs`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.FindDockerfiles`
- 代码路径: `builder/parser/code/lang.go`
- 测试路径: `builder/parser/code/lang_test.go::TestFindDockerfilesIgnoreSpecificDirs`

### 识别支持的源码构建语言矩阵

- Capability ID: `rainbond.source-detect.language-matrix`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.GetLangType`
- 代码路径: `builder/parser/code/lang.go`
- 测试路径: `builder/parser/code/language_matrix_test.go::TestGetLangType_SupportedSourceBuildLanguages`

### 存在 package.json 时优先识别为 Node.js

- Capability ID: `rainbond.source-detect.nodejs-over-static`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/code.GetLangType`
- 代码路径: `builder/parser/code/lang.go`
- 测试路径: `builder/parser/code/language_matrix_test.go::TestGetLangType_NodeJsWinsOverStaticWhenPackageJsonExists`

### 配置 parser 的 etcd 发现器并在无客户端时保护抓取逻辑

- Capability ID: `rainbond.source-discovery.etcd-config`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/discovery.NewEtcd`
- 代码路径: `builder/parser/discovery/etcd.go`
- 测试路径: `builder/parser/discovery/etcd_test.go::TestNewEtcdAndFetchGuard`

### 对不支持的 parser 发现类型返回空发现器

- Capability ID: `rainbond.source-discovery.unsupported-type`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/parser/discovery.NewDiscoverier`
- 代码路径: `builder/parser/discovery/discovery.go`
- 测试路径: `builder/parser/discovery/discovery_unit_test.go::TestNewDiscoverierUnsupportedType`

### 将镜像仓库认证信息编码为 base64 JSON 载荷

- Capability ID: `rainbond.source-image.auth-base64-encode`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/sources.EncodeAuthToBase64`
- 代码路径: `builder/sources/image.go`
- 测试路径: `builder/sources/image_test.go::TestEncodeAuthToBase64`

### 从归档文件导入镜像

- Capability ID: `rainbond.source-image.import`
- 状态: `active`
- 测试类型: `integration`
- 接口类型: `workflow`
- 业务入口: `builder/sources.ImageImport`
- 代码路径: `builder/sources/image.go`
- 测试路径: `builder/sources/image_test.go::TestImageImport`

### 将多个镜像保存为归档文件

- Capability ID: `rainbond.source-image.multi-save`
- 状态: `active`
- 测试类型: `integration`
- 接口类型: `workflow`
- 业务入口: `builder/sources.MultiImageSave`
- 代码路径: `builder/sources/image.go`
- 测试路径: `builder/sources/image_test.go::TestMulitImageSave`

### 解析镜像仓库主机名镜像名与标签

- Capability ID: `rainbond.source-image.parse-name`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/sources.ImageNameHandle`
- 代码路径: `builder/sources/image.go`
- 测试路径: `builder/sources/image_test.go::TestImageName`

### 解析带显式仓库命名空间的镜像引用

- Capability ID: `rainbond.source-image.parse-name-with-namespace`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/sources.ImageNameWithNamespaceHandle`
- 代码路径: `builder/sources/image.go`
- 测试路径: `builder/sources/image_test.go::TestImageNameWithNamespace`

### 将镜像保存为归档文件

- Capability ID: `rainbond.source-image.save`
- 状态: `active`
- 测试类型: `integration`
- 接口类型: `workflow`
- 业务入口: `builder/sources.ImageSave`
- 代码路径: `builder/sources/image.go`
- 测试路径: `builder/sources/image_test.go::TestImageSave`

### 从规范化镜像引用中提取标签

- Capability ID: `rainbond.source-image.tag-from-ref`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/sources.GetTagFromNamedRef`
- 代码路径: `builder/sources/registry.go`
- 测试路径: `builder/sources/registry_test.go::TestGetTagFromNamedRef`

### 校验受信任的镜像仓库

- Capability ID: `rainbond.source-image.trusted-registry-check`
- 状态: `active`
- 测试类型: `integration`
- 接口类型: `workflow`
- 业务入口: `builder/sources.CheckTrustedRepositories`
- 代码路径: `builder/sources/image.go`
- 测试路径: `builder/sources/image_test.go::TestCheckTrustedRepositories`

### 构建包含净化地址与构建子目录的仓库元数据

- Capability ID: `rainbond.source-repo.build-info`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/sources.CreateRepostoryBuildInfo`
- 代码路径: `builder/sources/repo.go`
- 测试路径: `builder/sources/repo_test.go::TestCreateRepostoryBuildInfo`

### 根据仓库分支租户与服务解析源码缓存目录

- Capability ID: `rainbond.source-repo.cache-dir`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/sources.GetCodeSourceDir`
- 代码路径: `builder/sources/git.go`
- 测试路径: `builder/sources/file_test.go::TestGetCodeSourceDirUsesSourceDirEnv`, `builder/sources/git_test.go::TestGetCodeCacheDir`

### 克隆 Git 源码仓库

- Capability ID: `rainbond.source-repo.clone`
- 状态: `active`
- 测试类型: `integration`
- 接口类型: `workflow`
- 业务入口: `builder/sources.GitClone`
- 代码路径: `builder/sources/git.go`
- 测试路径: `builder/sources/git_test.go::TestGitClone`

### 按标签克隆 Git 源码仓库

- Capability ID: `rainbond.source-repo.clone-by-tag`
- 状态: `active`
- 测试类型: `integration`
- 接口类型: `workflow`
- 业务入口: `builder/sources.GitClone`
- 代码路径: `builder/sources/git.go`
- 测试路径: `builder/sources/git_test.go::TestGitCloneByTag`

### 将分支和标签输入映射为 git 引用名

- Capability ID: `rainbond.source-repo.git-ref-name`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/sources.getBranch`
- 代码路径: `builder/sources/git.go`
- 测试路径: `builder/sources/git_test.go::TestGetBranch`

### 拉取 Git 源码仓库

- Capability ID: `rainbond.source-repo.pull`
- 状态: `active`
- 测试类型: `integration`
- 接口类型: `workflow`
- 业务入口: `builder/sources.GitPull`
- 代码路径: `builder/sources/git.go`
- 测试路径: `builder/sources/git_test.go::TestGitPull`

### 拉取或克隆 Git 源码仓库

- Capability ID: `rainbond.source-repo.pull-or-clone`
- 状态: `active`
- 测试类型: `integration`
- 接口类型: `workflow`
- 业务入口: `builder/sources.GitCloneOrPull`
- 代码路径: `builder/sources/git.go`
- 测试路径: `builder/sources/git_test.go::TestGitPullOrClone`

### 从仓库展示地址中去除凭据

- Capability ID: `rainbond.source-repo.show-url`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/sources.getShowURL`
- 代码路径: `builder/sources/git.go`
- 测试路径: `builder/sources/git_test.go::TestGetShowURL`

### 为源码任务创建独立的临时仓库工作目录

- Capability ID: `rainbond.source-repo.temp-build-info`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/sources.CreateTempRepostoryBuildInfo`
- 代码路径: `builder/sources/repo.go`
- 测试路径: `builder/sources/repo_test.go::TestCreateTempRepostoryBuildInfo`

### 为 pkg 构建创建临时工作目录信息时保持原始路径

- Capability ID: `rainbond.source-repo.temp-build-info-pkg`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/sources.CreateTempRepostoryBuildInfo`
- 代码路径: `builder/sources/repo.go`
- 测试路径: `builder/sources/repo_test.go::TestCreateTempRepostoryBuildInfoForPkg`

### 安全关闭零值 SFTP 客户端

- Capability ID: `rainbond.source-sftp.close-safe`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/sources.SFTPClient.Close`
- 代码路径: `builder/sources/sftp.go`
- 测试路径: `builder/sources/sftp_test.go::TestSFTPClientCloseZeroValue`

### 解析 SFTP 端口并提供合理默认值

- Capability ID: `rainbond.source-sftp.port-parse`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/sources.parseSFTPPort`
- 代码路径: `builder/sources/sftp.go`
- 测试路径: `builder/sources/sftp_test.go::TestParseSFTPPort`

### 解析 SVN 分支标签与 trunk 的目标路径

- Capability ID: `rainbond.source-svn.branch-path`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `builder/sources.getBranchPath`
- 代码路径: `builder/sources/svn.go`
- 测试路径: `builder/sources/svn_test.go::TestGetBranchPath`

### 汇总存储类信息

- Capability ID: `rainbond.storage.class-summary`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/handler.StorageClassInfo`
- 代码路径: `api/handler/storage.go`
- 测试路径: `api/handler/storage_test.go::TestStorageClassInfoFields`

### 复用存储处理器单例

- Capability ID: `rainbond.storage.handler-singleton`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `api/handler.GetStorageHandler`
- 代码路径: `api/handler/storage.go`
- 测试路径: `api/handler/storage_test.go::TestGetStorageHandlerSingleton`

### S3 生命周期已配置时不再输出 info 日志

- Capability ID: `rainbond.storage.s3-lifecycle-skip-logs`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `package_function`
- 业务入口: `pkg/component/storage.(*S3Storage).ensureBucketLifecycle`
- 代码路径: `pkg/component/storage/s3_storage.go`
- 测试路径: `pkg/component/storage/s3_storage_test.go::TestEnsureBucketExistsDoesNotLogInfoWhenLifecycleAlreadyConfigured`

### 构造并校验第三方组件端点地址

- Capability ID: `rainbond.third-component.endpoint-address-construct`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/apis/rainbond/v1alpha1.NewEndpointAddress`
- 代码路径: `pkg/apis/rainbond/v1alpha1/third_component.go`
- 测试路径: `pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestNewEndpointAddress`

### 解析端点 IP 与域名哨兵地址

- Capability ID: `rainbond.third-component.endpoint-address-ip`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/apis/rainbond/v1alpha1.EndpointAddress.GetIP`
- 代码路径: `pkg/apis/rainbond/v1alpha1/third_component.go`
- 测试路径: `pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestEndpointAddressGetIP`

### 从第三方组件端点地址中解析有效端口

- Capability ID: `rainbond.third-component.endpoint-address-port`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/apis/rainbond/v1alpha1.EndpointAddress.GetPort`
- 代码路径: `pkg/apis/rainbond/v1alpha1/third_component.go`
- 测试路径: `pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestEndpointAddressGetPort`

### 为第三方组件端点地址补齐默认 HTTP 协议

- Capability ID: `rainbond.third-component.endpoint-address-scheme`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/apis/rainbond/v1alpha1.EndpointAddress.EnsureScheme`
- 代码路径: `pkg/apis/rainbond/v1alpha1/third_component.go`
- 测试路径: `pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestEndpointAddressEnsureScheme`

### 比较第三方组件处理器在不同探测模式下的相等性

- Capability ID: `rainbond.third-component.handler-equals`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/apis/rainbond/v1alpha1.Handler.Equals`
- 代码路径: `pkg/apis/rainbond/v1alpha1/third_component.go`
- 测试路径: `pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestHandlerEquals`

### 比较 HTTP 探测处理器及其请求头集合

- Capability ID: `rainbond.third-component.http-get-equals`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/apis/rainbond/v1alpha1.HTTPGetAction.Equals`
- 代码路径: `pkg/apis/rainbond/v1alpha1/third_component.go`
- 测试路径: `pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestHTTPGetActionEquals`

### 暴露第三方组件身份与端点标识辅助函数

- Capability ID: `rainbond.third-component.identity-fields`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/apis/rainbond/v1alpha1.ThirdComponent.GetEndpointID`
- 代码路径: `pkg/apis/rainbond/v1alpha1/third_component.go`
- 测试路径: `pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestThirdComponentIdentityHelpers`

### 拆分旧式第三方组件端点的主机与端口对

- Capability ID: `rainbond.third-component.legacy-endpoint-port`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/apis/rainbond/v1alpha1.ThirdComponentEndpoint.GetPort`
- 代码路径: `pkg/apis/rainbond/v1alpha1/third_component.go`
- 测试路径: `pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestThirdComponentEndpointGetPortAndIP`

### 比较第三方组件探测定义是否相等

- Capability ID: `rainbond.third-component.probe-equals`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/apis/rainbond/v1alpha1.Probe.Equals`
- 代码路径: `pkg/apis/rainbond/v1alpha1/third_component.go`
- 测试路径: `pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestProbeEquals`

### 判断第三方组件是否需要主动探测

- Capability ID: `rainbond.third-component.probe-required`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/apis/rainbond/v1alpha1.ThirdComponentSpec.NeedProbe`
- 代码路径: `pkg/apis/rainbond/v1alpha1/third_component.go`
- 测试路径: `pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestThirdComponentSpecNeedProbe`

### 检测第三方组件是否使用静态端点

- Capability ID: `rainbond.third-component.static-endpoints-detect`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/apis/rainbond/v1alpha1.ThirdComponentSpec.IsStaticEndpoints`
- 代码路径: `pkg/apis/rainbond/v1alpha1/third_component.go`
- 测试路径: `pkg/apis/rainbond/v1alpha1/third_component_unit_test.go::TestThirdComponentSpecIsStaticEndpoints`

### 对字符串切片去重并保留非空元素

- Capability ID: `rainbond.util.array-deduplicate`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.Deweight`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestDeweight`

### 比较字节切片是否完全相等并区分 nil 情况

- Capability ID: `rainbond.util.bytes-equality`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.BytesSliceEqual`
- 代码路径: `util/bytes.go`
- 测试路径: `util/bytes_test.go::TestBytesSliceEqual`

### 使用零拷贝辅助将字节切片转换为字符串

- Capability ID: `rainbond.util.bytes-to-string`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.ToString`
- 代码路径: `util/string.go`
- 测试路径: `util/bytes_test.go::TestToString`

### 覆盖哈希IP字符串反转与时间版本等核心辅助行为

- Capability ID: `rainbond.util.core-helpers.hash-ip-string-uuid`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.CreateFileHash`
- 代码路径: `util/hash.go`, `util/ip.go`, `util/string.go`, `util/uuid.go`
- 测试路径: `util/hash_test.go::TestCreateFileHash`, `util/ip_test.go::TestCheckIP`, `util/string_test.go::TestReverse`, `util/uuid_test.go::TestTimeVersion`

### 从原始字符串生成稳定的 md5 哈希

- Capability ID: `rainbond.util.core-helpers.hash-string`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.CreateHashString`
- 代码路径: `util/hash.go`
- 测试路径: `util/hash_test.go::TestCreateHashString`

### 检查字符串切片中的成员是否存在

- Capability ID: `rainbond.util.core-helpers.string-contains`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.StringArrayContains`
- 代码路径: `util/string.go`
- 测试路径: `util/string_test.go::TestStringArrayContains`

### 返回规范化的当前工作目录路径

- Capability ID: `rainbond.util.current-dir-path`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.GetCurrentDir`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestGetCurrentDir`

### 按目标深度列出嵌套目录

- Capability ID: `rainbond.util.dir-list-depth`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.GetDirList`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestGetDirList`

### 按目标深度列出目录名称

- Capability ID: `rainbond.util.dir-name-list`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.GetDirNameList`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestGetDirNameList`

### 使用 shell du 命令计算目录大小

- Capability ID: `rainbond.util.dir-size-shell`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.GetDirSizeByCmd`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestGetDirSizeByCmd`

### 通过递归遍历文件计算目录大小

- Capability ID: `rainbond.util.dir-size-walk`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.GetDirSize`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestGetDirSize`

### 在环境变量为空时返回默认值

- Capability ID: `rainbond.util.env-default`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.GetenvDefault`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestGetenvDefault`

### 从 etcd 风格键中提取稳定 ID

- Capability ID: `rainbond.util.etcd-key-id-parse`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.GetIDFromKey`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestGetIDFromKey`

### 复制文件并保留内容与元数据

- Capability ID: `rainbond.util.file-copy`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.CopyFile`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestCopyFile`

### 按目标深度递归列出文件

- Capability ID: `rainbond.util.file-list-depth`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.GetFileList`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestGetFileList`

### 处理文件创建复制归档合并遍历与目录大小辅助操作

- Capability ID: `rainbond.util.fs.archive-and-directory-ops`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.OpenOrCreateFile`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestOpenOrCreateFile`

### 在候选列表中查找模糊匹配

- Capability ID: `rainbond.util.fuzzy.find`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/fuzzy.Find`
- 代码路径: `util/fuzzy/fuzzy.go`
- 测试路径: `util/fuzzy/fuzzy_test.go::TestFind`

### 在候选列表中按大小写不敏感方式查找模糊匹配

- Capability ID: `rainbond.util.fuzzy.find-fold`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/fuzzy.FindFold`
- 代码路径: `util/fuzzy/fuzzy.go`
- 测试路径: `util/fuzzy/fuzzy_test.go::TestFindFold`

### 计算字符串之间的 Levenshtein 编辑距离

- Capability ID: `rainbond.util.fuzzy.levenshtein-distance`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/fuzzy.LevenshteinDistance`
- 代码路径: `util/fuzzy/levenshtein.go`
- 测试路径: `util/fuzzy/levenshtein_test.go::TestLevenshteinDistance`

### 模糊匹配单个候选字符串

- Capability ID: `rainbond.util.fuzzy.match`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/fuzzy.Match`
- 代码路径: `util/fuzzy/fuzzy.go`
- 测试路径: `util/fuzzy/fuzzy_test.go::TestMatch`

### 执行大小写不敏感的模糊子序列匹配

- Capability ID: `rainbond.util.fuzzy.match-fold`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/fuzzy.MatchFold`
- 代码路径: `util/fuzzy/fuzzy.go`
- 测试路径: `util/fuzzy/fuzzy_test.go::TestMatchFold`

### 对候选列表中的模糊匹配结果排序

- Capability ID: `rainbond.util.fuzzy.rank-find`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/fuzzy.RankFind`
- 代码路径: `util/fuzzy/fuzzy.go`
- 测试路径: `util/fuzzy/fuzzy_test.go::TestRankFind`

### 按编辑距离对大小写不敏感的模糊匹配结果排序

- Capability ID: `rainbond.util.fuzzy.rank-find-fold`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/fuzzy.RankFindFold`
- 代码路径: `util/fuzzy/fuzzy.go`
- 测试路径: `util/fuzzy/fuzzy_test.go::TestRankFindFold`

### 按删除距离为模糊匹配结果打分

- Capability ID: `rainbond.util.fuzzy.rank-match`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/fuzzy.RankMatch`
- 代码路径: `util/fuzzy/fuzzy.go`
- 测试路径: `util/fuzzy/fuzzy_test.go::TestRankMatch`

### 返回显式环境变量值或后备默认值

- Capability ID: `rainbond.util.getenv`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.Getenv`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestGetenv`

### 根据机器状态生成稳定的主机标识

- Capability ID: `rainbond.util.host-id-generate`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.CreateHostID`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestCreateHostID`

### 在网卡地址扫描中过滤回环与非 IP 地址

- Capability ID: `rainbond.util.network.interface-address-filter`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.checkIPAddress`
- 代码路径: `util/ippool.go`
- 测试路径: `util/ippool_test.go::TestCheckIPAddress`

### 管理探测状态 watcher 并分发健康更新

- Capability ID: `rainbond.util.prober.manage-service-health-watchers`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/prober.probeManager.handleStatus`
- 代码路径: `util/prober/manager.go`
- 测试路径: `util/prober/manager_test.go::TestProbeManager_Start`

### 选择 SSH 鉴权方式并拒绝不支持的认证模式

- Capability ID: `rainbond.util.ssh.auth-method-selection`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.NewSSHClient`
- 代码路径: `util/sshclient.go`
- 测试路径: `util/sshclient_test.go::TestNewSSHClientSelectsAuthMethod`

### 检测状态组件名称中的数字后缀

- Capability ID: `rainbond.util.statefulset-suffix-detect`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.IsEndWithNumber`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestIsEndWithNumber`

### 使用辅助函数将字符串转换为字节切片

- Capability ID: `rainbond.util.string-to-bytes`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.ToByte`
- 代码路径: `util/string.go`
- 测试路径: `util/bytes_test.go::TestToByte`

### 提供主机标识版本当前目录变量替换与时间格式化辅助能力

- Capability ID: `rainbond.util.system.identity-and-template-helpers`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.CreateHostID`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestDeweight`

### 根据配置映射展开带默认值的模板变量

- Capability ID: `rainbond.util.template-variable-parse`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.ParseVariable`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestParseVariable`

### 渲染布尔表格单元格

- Capability ID: `rainbond.util.termtables.render-bool-cell`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Cell.Render`
- 代码路径: `util/termtables/cell.go`
- 测试路径: `util/termtables/cell_test.go::TestCellRenderBool`

### 按配置填充表格单元格

- Capability ID: `rainbond.util.termtables.render-cell-padding`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Cell.Render`
- 代码路径: `util/termtables/cell.go`
- 测试路径: `util/termtables/cell_test.go::TestCellRenderPadding`

### 渲染浮点表格单元格

- Capability ID: `rainbond.util.termtables.render-float-cell`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Cell.Render`
- 代码路径: `util/termtables/cell.go`
- 测试路径: `util/termtables/cell_test.go::TestCellRenderFloat`

### 渲染通用表格单元格值

- Capability ID: `rainbond.util.termtables.render-generic-cell`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Cell.Render`
- 代码路径: `util/termtables/cell.go`
- 测试路径: `util/termtables/cell_test.go::TestCellRenderGeneric`

### 渲染带显式对齐的 HTML 表格

- Capability ID: `rainbond.util.termtables.render-html-alignment`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.RenderHTML`
- 代码路径: `util/termtables/html.go`
- 测试路径: `util/termtables/html_test.go::TestTableWithAlignment`

### 以替代标题样式渲染 HTML 表格

- Capability ID: `rainbond.util.termtables.render-html-alt-title-style`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.SetHTMLStyleTitle`
- 代码路径: `util/termtables/html.go`
- 测试路径: `util/termtables/html_test.go::TestTableWithAltTitleStyle`

### 将 SetAlign 应用于 HTML 表格渲染

- Capability ID: `rainbond.util.termtables.render-html-set-align`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.SetAlign`
- 代码路径: `util/termtables/html.go`
- 测试路径: `util/termtables/html_test.go::TestTableAfterSetAlign`

### 渲染带标题与对齐能力的 HTML 表格输出

- Capability ID: `rainbond.util.termtables.render-html-table`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/html.go`
- 测试路径: `util/termtables/html_test.go::TestCreateTableHTML`

### 渲染带标题的 HTML 表格

- Capability ID: `rainbond.util.termtables.render-html-title`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.RenderHTML`
- 代码路径: `util/termtables/html.go`
- 测试路径: `util/termtables/html_test.go::TestTableWithHeaderHTML`

### 按宽度展开 HTML 表格标题

- Capability ID: `rainbond.util.termtables.render-html-title-width`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.RenderHTML`
- 代码路径: `util/termtables/html.go`
- 测试路径: `util/termtables/html_test.go::TestTableTitleWidthAdjustsHTML`

### 渲染带 Unicode 宽度的 HTML 表格

- Capability ID: `rainbond.util.termtables.render-html-unicode-widths`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.RenderHTML`
- 代码路径: `util/termtables/html.go`
- 测试路径: `util/termtables/html_test.go::TestTableUnicodeWidthsHTML`

### 渲染无表头的 HTML 表格

- Capability ID: `rainbond.util.termtables.render-html-without-headers`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.RenderHTML`
- 代码路径: `util/termtables/html.go`
- 测试路径: `util/termtables/html_test.go::TestTableWithNoHeadersHTML`

### 渲染整数表格单元格

- Capability ID: `rainbond.util.termtables.render-integer-cell`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Cell.Render`
- 代码路径: `util/termtables/cell.go`
- 测试路径: `util/termtables/cell_test.go::TestCellRenderInteger`

### 渲染 Markdown 表格输出

- Capability ID: `rainbond.util.termtables.render-markdown-table`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestTableInMarkdown`

### 按列宽填充渲染后的表格行

- Capability ID: `rainbond.util.termtables.render-row-width-padding`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Row.Render`
- 代码路径: `util/termtables/row.go`
- 测试路径: `util/termtables/row_test.go::TestRowRenderWidthBasedPadding`

### 渲染 Stringer 表格单元格

- Capability ID: `rainbond.util.termtables.render-stringer-cell`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Cell.Render`
- 代码路径: `util/termtables/cell.go`
- 测试路径: `util/termtables/cell_test.go::TestCellRenderStringerStruct`

### 渲染带宽度与 Unicode 处理的终端与 Markdown 表格

- Capability ID: `rainbond.util.termtables.render-text-table`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/table.go`, `util/termtables/cell.go`, `util/termtables/row.go`
- 测试路径: `util/termtables/table_test.go::TestCreateTable`, `util/termtables/cell_test.go::TestCellRenderString`, `util/termtables/row_test.go::TestBasicRowRender`

### 在多次 AddHeaders 调用中追加表头

- Capability ID: `rainbond.util.termtables.render-text-table-append-headers`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.AddHeaders`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestTableMultipleAddHeader`

### 渲染含 CJK 字符的文本表格

- Capability ID: `rainbond.util.termtables.render-text-table-cjk`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestCJKChars`

### 按表头宽度扩展文本表格

- Capability ID: `rainbond.util.termtables.render-text-table-header-width`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestTableHeaderWidthAdjusts`

### 渲染缺失尾部单元格的文本表格

- Capability ID: `rainbond.util.termtables.render-text-table-missing-cells`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestTableMissingCells`

### 在添加行后应用列对齐

- Capability ID: `rainbond.util.termtables.render-text-table-post-align`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.SetAlign`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestTableAlignPostsetting`

### 重复渲染时保持表头一致

- Capability ID: `rainbond.util.termtables.render-text-table-repeatable`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestTableWithHeaderMultipleTimes`

### 将表格样式重置为 ASCII 渲染

- Capability ID: `rainbond.util.termtables.render-text-table-style-reset`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestStyleResets`

### 按 Unicode 宽度渲染文本表格标题

- Capability ID: `rainbond.util.termtables.render-text-table-title-unicode-width`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestTitleUnicodeWidths`

### 按标题宽度扩展文本表格

- Capability ID: `rainbond.util.termtables.render-text-table-title-width`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestTableTitleWidthAdjusts`

### 渲染带 Unicode 宽度的文本表格

- Capability ID: `rainbond.util.termtables.render-text-table-unicode-widths`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestTableUnicodeWidths`

### 使用 UTF-8 边框渲染文本表格

- Capability ID: `rainbond.util.termtables.render-text-table-utf8-box`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestTableInUTF8`

### 渲染带终端颜色控制序列的 UTF-8 文本表格

- Capability ID: `rainbond.util.termtables.render-text-table-utf8-sgr`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestTableUnicodeUTF8AndSGR`

### 在多行之间平衡文本表格宽度

- Capability ID: `rainbond.util.termtables.render-text-table-width-balance`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestTableWidthHandling`

### 处理第二类文本表格宽度平衡场景

- Capability ID: `rainbond.util.termtables.render-text-table-width-balance-second`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestTableWidthHandling_SecondErrorCondition`

### 渲染含组合字符的文本表格

- Capability ID: `rainbond.util.termtables.render-text-table-with-combining-chars`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestTableWithCombiningChars`

### 渲染含全角字符的文本表格

- Capability ID: `rainbond.util.termtables.render-text-table-with-fullwidth-chars`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestTableWithFullwidthChars`

### 渲染带标题的文本表格

- Capability ID: `rainbond.util.termtables.render-text-table-with-title`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestTableWithHeader`

### 渲染无表头的文本表格

- Capability ID: `rainbond.util.termtables.render-text-table-without-headers`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.Table.Render`
- 代码路径: `util/termtables/table.go`
- 测试路径: `util/termtables/table_test.go::TestTableWithNoHeaders`

### 移除单元格内容中的 ANSI 颜色码

- Capability ID: `rainbond.util.termtables.strip-color-codes`
- 状态: `active`
- 测试类型: `unit`
- 接口类型: `workflow`
- 业务入口: `util/termtables.filterColorCodes`
- 代码路径: `util/termtables/cell.go`
- 测试路径: `util/termtables/cell_test.go::TestFilterColorCodes`

### 将解析后的时间格式化为 RFC3339 字符串

- Capability ID: `rainbond.util.time-format-rfc3339`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `time.Format`
- 代码路径: `util/comman_test.go`
- 测试路径: `util/comman_test.go::TestTimeFormat`

### 生成基于时间戳的版本字符串

- Capability ID: `rainbond.util.version-timestamp`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.CreateVersionByTime`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestCreateVersionByTime`

### 从字符串切片中过滤空白与仅空格项

- Capability ID: `rainbond.util.whitespace-filter`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.RemoveSpaces`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestRemoveSpaces`

### 将目录归档为 zip 文件

- Capability ID: `rainbond.util.zip-archive`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.Zip`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestZip`

### 检测 zip 归档是否共享公共根目录

- Capability ID: `rainbond.util.zip-structure-detect`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util.detectZipStructure`
- 代码路径: `util/comman.go`
- 测试路径: `util/comman_test.go::TestDetectZipStructure`

### Discover DataVolume-backed VM export disks

- Capability ID: `rainbond.vm-export.discover-datavolume-disks`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `package_function`
- 业务入口: `handler.discoverVMExportDisks`
- 代码路径: `api/handler/vm_export.go`
- 测试路径: `api/handler/vm_export_test.go::TestDiscoverVMExportDisksSupportsDataVolumeRootDisk`

### vm-run 本地包源在目录缺失时回退 storage 下载

- Capability ID: `rainbond.vm-run.local-package-storage-download`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `package_function`
- 业务入口: `builder/sourceutil.ReadLocalPackageDir`
- 代码路径: `builder/sourceutil/local_package.go`
- 测试路径: `builder/sourceutil/local_package_test.go::TestReadLocalPackageDirFallsBackToStorageDownload`

### vm-run 远程包探测优先使用 HEAD

- Capability ID: `rainbond.vm-run.remote-package-probe`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `other`
- 业务入口: `builder/parser.VMServiceParse.Parse`
- 代码路径: `builder/parser/vm_service.go`
- 测试路径: `builder/parser/vm_service_test.go::TestVMServiceParseRemoteURLPrefersHeadProbe`

### vm-run 远程包探测在 HEAD 失败时回退 Range GET

- Capability ID: `rainbond.vm-run.remote-package-probe-range-fallback`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `other`
- 业务入口: `builder/parser.VMServiceParse.Parse`
- 代码路径: `builder/parser/vm_service.go`
- 测试路径: `builder/parser/vm_service_test.go::TestVMServiceParseRemoteURLFallsBackToRangeGet`

### 将 watch 后端错误分发到内部错误通道

- Capability ID: `rainbond.watch.error-dispatch`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/watch.watchChan.sendError`
- 代码路径: `util/watch/watcher.go`
- 测试路径: `util/watch/watch_test.go::TestWatchChanSendError`

### 将 watch 后端错误转换为 API 错误事件

- Capability ID: `rainbond.watch.error-parse`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/watch.parseError`
- 代码路径: `util/watch/watcher.go`
- 测试路径: `util/watch/watch_test.go::TestParseError`

### 将 etcd watch 事件解析为内部事件结构

- Capability ID: `rainbond.watch.etcd-event-parse`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/watch.parseEvent`
- 代码路径: `util/watch/event.go`
- 测试路径: `util/watch/watch_test.go::TestParseEvent`

### 为 watch 事件暴露键与载荷访问器

- Capability ID: `rainbond.watch.event-accessors`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/watch.Event.GetKey`
- 代码路径: `util/watch/watch.go`
- 测试路径: `util/watch/watch_test.go::TestEventAccessors`

### 为 watch 事件暴露原始字节载荷访问器

- Capability ID: `rainbond.watch.event-byte-accessors`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/watch.Event.GetValue`
- 代码路径: `util/watch/watch.go`
- 测试路径: `util/watch/watch_test.go::TestEventByteAccessors`

### 将底层 watch 事件分发到输入事件通道

- Capability ID: `rainbond.watch.event-dispatch`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/watch.watchChan.sendEvent`
- 代码路径: `util/watch/watcher.go`
- 测试路径: `util/watch/watch_test.go::TestWatchChanSendEvent`

### 将底层 watch 事件转换为 Added/Modified/Deleted 结果

- Capability ID: `rainbond.watch.event-type-transform`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/watch.watchChan.transform`
- 代码路径: `util/watch/watcher.go`
- 测试路径: `util/watch/watch_test.go::TestWatchChanTransform`

### 将 watch 资源版本解析为 etcd 修订号

- Capability ID: `rainbond.watch.resource-version-parse`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/watch.ParseWatchResourceVersion`
- 代码路径: `util/watch/util.go`
- 测试路径: `util/watch/watch_test.go::TestParseWatchResourceVersion`

### 将 watch 状态错误格式化为稳定字符串

- Capability ID: `rainbond.watch.status-error-format`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/watch.Status.Error`
- 代码路径: `util/watch/event.go`
- 测试路径: `util/watch/watch_test.go::TestStatusError`

### 将初始 etcd 键值转换为合成创建事件

- Capability ID: `rainbond.watch.synthetic-create-event`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `util/watch.parseKV`
- 代码路径: `util/watch/event.go`
- 测试路径: `util/watch/watch_test.go::TestParseKVMarksCreateEvent`

### 生成稳定的 WebSocket 鉴权 MD5 摘要

- Capability ID: `rainbond.webcli.auth-signature`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/webcli/app.md5Func`
- 代码路径: `api/webcli/app/app.go`
- 测试路径: `api/webcli/app/app_test.go::TestMD5Func`

### 拒绝对已完成 Pod 建立 exec 会话

- Capability ID: `rainbond.webcli.completed-pod-guard`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/webcli/app.App.GetContainerArgs`
- 代码路径: `api/webcli/app/app.go`
- 测试路径: `api/webcli/app/app_test.go::TestGetContainerArgsRejectsCompletedPod`

### 为 WebCLI 请求补齐 Kubernetes REST 客户端默认配置

- Capability ID: `rainbond.webcli.config-defaults`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/webcli/app.SetConfigDefaults`
- 代码路径: `api/webcli/app/app.go`
- 测试路径: `api/webcli/app/app_test.go::TestSetConfigDefaults`

### 为 WebCLI 会话解析执行容器Pod IP与命令参数

- Capability ID: `rainbond.webcli.container-args`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/webcli/app.App.GetContainerArgs`
- 代码路径: `api/webcli/app/app.go`
- 测试路径: `api/webcli/app/app_test.go::TestGetContainerArgsSelectsContainerAndExecArgs`

### 限制 WebCLI 终端输出最大宽度

- Capability ID: `rainbond.webcli.max-width`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/webcli/term.NewMaxWidthWriter`
- 代码路径: `api/webcli/term/term_writer.go`
- 测试路径: `api/webcli/term/term_writer_test.go::TestMaxWidthWriter`

### 请求的容器不存在时拒绝建立 exec 会话

- Capability ID: `rainbond.webcli.missing-container-guard`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/webcli/app.App.GetContainerArgs`
- 代码路径: `api/webcli/app/app.go`
- 测试路径: `api/webcli/app/app_test.go::TestGetContainerArgsRejectsMissingContainer`

### 为 WebCLI 执行会话排队并应用终端尺寸变更

- Capability ID: `rainbond.webcli.terminal-resize`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/webcli/app.execContext.ResizeTerminal`
- 代码路径: `api/webcli/app/exec.go`
- 测试路径: `api/webcli/app/tty_test.go::TestResizeTerminalQueuesWindowSize`

### 按单词折行 WebCLI 终端输出

- Capability ID: `rainbond.webcli.word-wrap`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `api/webcli/term.NewWordWrapWriter`
- 代码路径: `api/webcli/term/term_writer.go`
- 测试路径: `api/webcli/term/term_writer_test.go::TestWordWrapWriter`

### 根据自动伸缩规则构建 HPA 指标与对象

- Capability ID: `rainbond.worker.appm.autoscaler.build-hpa-spec`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/appm/conversion.newHPA`
- 代码路径: `worker/appm/conversion/autoscaler.go`
- 测试路径: `worker/appm/conversion/autoscaler_test.go::TestNewHPA`

### 配置 appm 的 etcd 发现器并在无客户端时保护抓取逻辑

- Capability ID: `rainbond.worker.appm.discovery.etcd-config`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/appm/thirdparty/discovery.NewEtcd`
- 代码路径: `worker/appm/thirdparty/discovery/etcd.go`
- 测试路径: `worker/appm/thirdparty/discovery/etcd_test.go::TestNewEtcdAndFetchGuard`

### 对不支持的 appm 发现后端返回错误

- Capability ID: `rainbond.worker.appm.discovery.unsupported-type`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/appm/thirdparty/discovery.NewDiscoverier`
- 代码路径: `worker/appm/thirdparty/discovery/discovery.go`
- 测试路径: `worker/appm/thirdparty/discovery/discovery_unit_test.go::TestNewDiscoverierUnsupportedType`

### 根据新旧工作负载规格计算允许的 StatefulSet Patch 内容

- Capability ID: `rainbond.worker.appm.patch.statefulset-modified-configuration`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/appm/types/v1.getStatefulsetModifiedConfiguration`
- 代码路径: `worker/appm/types/v1/patch.go`
- 测试路径: `worker/appm/types/v1/patch_test.go::TestGetStatefulsetModifiedConfiguration`

### 将组件运行状态汇总为应用状态

- Capability ID: `rainbond.worker.appm.store.aggregate-app-status`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/appm/store.getAppStatus`
- 代码路径: `worker/appm/store/store.go`
- 测试路径: `worker/appm/store/store_test.go::TestGetAppStatus`

### 在命名空间事件中同步受管命名空间的镜像拉取密钥

- Capability ID: `rainbond.worker.appm.store.sync-managed-namespace-image-pull-secret`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/appm/store.appRuntimeStore.nsEventHandler`
- 代码路径: `worker/appm/store/store.go`
- 测试路径: `worker/appm/store/store_test.go::TestNsEventHandlerProvidesAddFunc`

### 根据仓库名与模板名拼装 Helm chart 引用

- Capability ID: `rainbond.worker.helmapp.chart-ref`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/master/controller/helmapp.App.Chart`
- 代码路径: `worker/master/controller/helmapp/app.go`
- 测试路径: `worker/master/controller/helmapp/unit_test.go::TestAppChart`

### 管理 HelmApp 条件的新增更新与成功态切换

- Capability ID: `rainbond.worker.helmapp.condition-lifecycle`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/apis/rainbond/v1alpha1.HelmAppStatus.UpdateConditionStatus`
- 代码路径: `pkg/apis/rainbond/v1alpha1/helmapp_status.go`
- 测试路径: `pkg/apis/rainbond/v1alpha1/helmapp_unit_test.go::TestHelmAppStatusConditionLifecycle`

### 按类型查询 HelmApp 条件及其真值状态

- Capability ID: `rainbond.worker.helmapp.condition-query`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/apis/rainbond/v1alpha1.HelmAppStatus.GetCondition`
- 代码路径: `pkg/apis/rainbond/v1alpha1/helmapp_status.go`
- 测试路径: `pkg/apis/rainbond/v1alpha1/helmapp_unit_test.go::TestHelmAppStatusConditionQuery`

### 在条件未变化时跳过冗余的 HelmApp 条件写入

- Capability ID: `rainbond.worker.helmapp.condition-set-noop`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/apis/rainbond/v1alpha1.HelmAppStatus.SetCondition`
- 代码路径: `pkg/apis/rainbond/v1alpha1/helmapp_status.go`
- 测试路径: `pkg/apis/rainbond/v1alpha1/helmapp_unit_test.go::TestHelmAppStatusSetConditionDoesNotDuplicateUnchangedCondition`

### 更新条件状态时自动创建缺失的 HelmApp 条件

- Capability ID: `rainbond.worker.helmapp.condition-status-default-create`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/apis/rainbond/v1alpha1.HelmAppStatus.UpdateConditionStatus`
- 代码路径: `pkg/apis/rainbond/v1alpha1/helmapp_status.go`
- 测试路径: `pkg/apis/rainbond/v1alpha1/helmapp_unit_test.go::TestHelmAppStatusUpdateConditionStatusCreatesMissingCondition`

### 在状态未变化时保留条件的转移时间

- Capability ID: `rainbond.worker.helmapp.condition-transition-time`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/apis/rainbond/v1alpha1.HelmAppStatus.UpdateCondition`
- 代码路径: `pkg/apis/rainbond/v1alpha1/helmapp_status.go`
- 测试路径: `pkg/apis/rainbond/v1alpha1/helmapp_unit_test.go::TestHelmAppStatusUpdateConditionPreservesTransitionTimeOnSameStatus`

### 判断 HelmApp 是否仍需执行检测

- Capability ID: `rainbond.worker.helmapp.detect-required`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/master/controller/helmapp.App.NeedDetect`
- 代码路径: `worker/master/controller/helmapp/app.go`
- 测试路径: `worker/master/controller/helmapp/unit_test.go::TestAppNeedDetect`

### 判断 HelmApp 检测前置条件是否已经满足

- Capability ID: `rainbond.worker.helmapp.detected-prerequisites`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/master/controller/helmapp.Status.isDetected`
- 代码路径: `worker/master/controller/helmapp/status.go`
- 测试路径: `worker/master/controller/helmapp/unit_test.go::TestStatusGetPhase`

### 启动 HelmApp envtest 集成测试套件

- Capability ID: `rainbond.worker.helmapp.envtest-suite`
- 状态: `active`
- 测试类型: `integration`
- 接口类型: `workflow`
- 业务入口: `worker/master/controller/helmapp.BeforeSuite`
- 代码路径: `worker/master/controller/helmapp/suite_test.go`
- 测试路径: `worker/master/controller/helmapp/suite_test.go`

### 通过关闭队列停止 HelmApp finalizer

- Capability ID: `rainbond.worker.helmapp.finalizer-stop`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/master/controller/helmapp.Finalizer.Stop`
- 代码路径: `worker/master/controller/helmapp/finilizer.go`
- 测试路径: `worker/master/controller/helmapp/store_unit_test.go::TestFinalizerStop`

### 通过控制循环安装并部署 Helm 应用

- Capability ID: `rainbond.worker.helmapp.install-and-deploy`
- 状态: `active`
- 测试类型: `integration`
- 接口类型: `workflow`
- 业务入口: `worker/master/controller/helmapp.ControlLoop`
- 代码路径: `worker/master/controller/helmapp/controlloop.go`
- 测试路径: `worker/master/controller/helmapp/controlloop_test.go`

### 按无序方式比较 HelmApp 期望与已生效的 overrides

- Capability ID: `rainbond.worker.helmapp.overrides-compare`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/apis/rainbond/v1alpha1.HelmApp.OverridesEqual`
- 代码路径: `pkg/apis/rainbond/v1alpha1/helmapp_types.go`
- 测试路径: `pkg/apis/rainbond/v1alpha1/helmapp_unit_test.go::TestHelmAppOverridesEqual`

### 根据条件与前置状态推导 HelmApp 阶段

- Capability ID: `rainbond.worker.helmapp.phase-derive`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/master/controller/helmapp.Status.getPhase`
- 代码路径: `worker/master/controller/helmapp/status.go`
- 测试路径: `worker/master/controller/helmapp/unit_test.go::TestStatusGetPhase`

### 将 HelmApp 队列键拆分为名称与命名空间片段

- Capability ID: `rainbond.worker.helmapp.queue-key-parse`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/master/controller/helmapp.nameNamespace`
- 代码路径: `worker/master/controller/helmapp/controlloop.go`
- 测试路径: `worker/master/controller/helmapp/unit_test.go::TestNameNamespace`

### 协调 Helm 应用进入配置阶段

- Capability ID: `rainbond.worker.helmapp.reconcile-configuring-phase`
- 状态: `active`
- 测试类型: `integration`
- 接口类型: `workflow`
- 业务入口: `worker/master/controller/helmapp.ControlLoop`
- 代码路径: `worker/master/controller/helmapp/controlloop.go`
- 测试路径: `worker/master/controller/helmapp/controlloop_test.go`

### 协调 Helm 应用默认值

- Capability ID: `rainbond.worker.helmapp.reconcile-default-values`
- 状态: `active`
- 测试类型: `integration`
- 接口类型: `workflow`
- 业务入口: `worker/master/controller/helmapp.ControlLoop`
- 代码路径: `worker/master/controller/helmapp/controlloop.go`
- 测试路径: `worker/master/controller/helmapp/controlloop_test.go`

### 协调 Helm 应用进入检测阶段

- Capability ID: `rainbond.worker.helmapp.reconcile-start-detecting`
- 状态: `active`
- 测试类型: `integration`
- 接口类型: `workflow`
- 业务入口: `worker/master/controller/helmapp.ControlLoop`
- 代码路径: `worker/master/controller/helmapp/controlloop.go`
- 测试路径: `worker/master/controller/helmapp/controlloop_test.go`

### 判断 HelmApp 是否仍需初始化默认配置

- Capability ID: `rainbond.worker.helmapp.setup-required`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/master/controller/helmapp.App.NeedSetup`
- 代码路径: `worker/master/controller/helmapp/app.go`
- 测试路径: `worker/master/controller/helmapp/unit_test.go::TestAppNeedSetup`

### 从控制器 store lister 获取 HelmApp 对象

- Capability ID: `rainbond.worker.helmapp.store-fetch`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/master/controller/helmapp.store.GetHelmApp`
- 代码路径: `worker/master/controller/helmapp/store.go`
- 测试路径: `worker/master/controller/helmapp/store_unit_test.go::TestStoreGetHelmApp`

### 根据 EID 与商店名构建完整应用商店名称

- Capability ID: `rainbond.worker.helmapp.store-full-name`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `pkg/apis/rainbond/v1alpha1.HelmAppSpec.FullName`
- 代码路径: `pkg/apis/rainbond/v1alpha1/helmapp_types.go`
- 测试路径: `pkg/apis/rainbond/v1alpha1/helmapp_unit_test.go::TestHelmAppSpecFullName`

### 判断已配置 HelmApp 是否需要安装或更新

- Capability ID: `rainbond.worker.helmapp.update-required`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/master/controller/helmapp.App.NeedUpdate`
- 代码路径: `worker/master/controller/helmapp/app.go`
- 测试路径: `worker/master/controller/helmapp/unit_test.go::TestAppNeedUpdate`

### 根据条件容器状态与事件归类 Pod 状态

- Capability ID: `rainbond.worker.pod-status.describe`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/util.DescribePodStatus`
- 代码路径: `worker/util/pod.go`
- 测试路径: `worker/util/pod_test.go::TestDescribePodStatus`

### 执行第三方组件端点探测并映射结果

- Capability ID: `rainbond.worker.thirdcomponent.prober.execute-endpoint-probe`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/master/controller/thirdcomponent/prober.prober.probe`
- 代码路径: `worker/master/controller/thirdcomponent/prober/prober.go`
- 测试路径: `worker/master/controller/thirdcomponent/prober/prober_test.go::TestProbe`

### 缓存并清理第三方组件探测结果

- Capability ID: `rainbond.worker.thirdcomponent.prober.manage-results-cache`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/master/controller/thirdcomponent/prober/results.NewManager`
- 代码路径: `worker/master/controller/thirdcomponent/prober/results/results_manager.go`
- 测试路径: `worker/master/controller/thirdcomponent/prober/results/results_manager_test.go::TestCacheOperations`

### 根据 PVC 名称解析 Pod 名与卷 ID

- Capability ID: `rainbond.worker.volume-provider.pvc-identifiers`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/master/volumes/provider.getVolumeIDByPVCName`
- 代码路径: `worker/master/volumes/provider/rainbondsssc.go`
- 测试路径: `worker/master/volumes/provider/rainbondsslc_test.go::TestGetVolumeIDByPVCName`

### 按可用内存选择存储节点

- Capability ID: `rainbond.worker.volume-provider.select-node`
- 状态: `active`
- 测试类型: `integration`
- 接口类型: `workflow`
- 业务入口: `worker/master/volumes/provider.rainbondsslcProvisioner.selectNode`
- 代码路径: `worker/master/volumes/provider/rainbondsslc.go`
- 测试路径: `worker/master/volumes/provider/rainbondsslc_test.go::TestSelectNode`

### 将存储类转换为 Rainbond 卷类型

- Capability ID: `rainbond.worker.volume-type.from-storageclass`
- 状态: `active`
- 测试类型: `regression`
- 接口类型: `workflow`
- 业务入口: `worker/util.TransStorageClass2RBDVolumeType`
- 代码路径: `worker/util/volumetype.go`
- 测试路径: `worker/util/volumetype_test.go::TestTransStorageClass2RBDVolumeType`
