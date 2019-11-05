# 应用备份设计文档

builder/exector/groupapp_backup.go

run 方法

- 从region_apps_metadata.json中读取应用的元数据
- 区分版本, 5.1.8 对元数据的抽象结构进行了修改, 需要兼容旧版本
- 打各种小包(backupServiceInfo, backupPluginInfo)
    - slug包, image包(uploadSlug, uploadImage)
    - 各组件的持久化数据
- 将各种小包打成一个包(总包)
- 将总包放到本地或者上传到云端(S3)

minio-go 支持S3和阿里云OSS

