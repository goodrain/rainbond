# -*- coding: utf8 -*-
import os
import json
import shutil
import time
from clients.region import RegionAPI
from clients.alioss import OssAPI
from clients.etcdcli import TaskLocker
from clients.userconsole import UserConsoleAPI
from clients.region_api import RegionBackAPI
from clients.acp_api import ACPAPI
from clients.ftputils import FTPUtils
from utils.crypt import get_md5
from utils.log import EventLog
import logging
import logging.config
from etc import settings
import fileinput
load_dict = {}
with open("plugins/config.json", 'r') as load_f:
    load_dict = json.load(load_f)
logging.config.dictConfig(settings.get_logging(load_dict))
logger = logging.getLogger('default')


class AppSlug():
    def __init__(self, job, *args, **kwargs):
        self.job = job
        self.configs = kwargs.get("config")
        self.region_api = RegionAPI(conf=self.configs['region'])
        self.oss_api = OssAPI(conf=self.configs['oss']['ali_shanghai'])
        self.locker = TaskLocker(conf=self.configs['etcd'])
        self.user_cs_client = UserConsoleAPI(conf=self.configs['userconsole'])
        self.api = ACPAPI(conf=self.configs['region'])
        self.region_client = RegionBackAPI()
        self.slug_configs = self.configs["publish"]["slug"]
        self.is_region_slug = self.slug_configs.get('all_region_ftp')
        self.is_oss_ftp = self.slug_configs.get('oss_ftp')
        # 用户文件存储路径
        self.SRV_SLUG_BASE_DIR = self.slug_configs.get(
            'slug_path') + '{tenantId}/slug/{serviceId}/{deployVersion}.tgz'
        # 数据中心slug存储路径
        self.SLUG_PATH = self.slug_configs.get(
            'curr_region_dir') + '{serviceKey}/{appVersion}.tgz'
        self.CURR_REGION_PATH = self.slug_configs.get(
            'curr_region_path') + self.SLUG_PATH
        # 区域中心slug的ftp配置
        self.ALL_REGION_FTP_HOST = self.slug_configs.get('all_region_ftp_host')
        self.ALL_REGION_FTP_PORT = self.slug_configs.get('all_region_ftp_port')
        self.ALL_REGION_FTP_USERNAME = self.slug_configs.get(
            'all_region_username')
        self.ALL_REGION_FTP_PASSWORD = self.slug_configs.get(
            'all_region_password')
        self.ALL_REGION_FTP_NAMESPACE = self.slug_configs.get(
            'all_region_namespace')
        self.ALL_REGION_FTP_PATH = self.ALL_REGION_FTP_NAMESPACE + '{serviceKey}/{appVersion}.tgz'
        # oss存储路径
        CLOUD_ASSISTANT = self.configs.get('CLOUD_ASSISTANT')
        self.OSS_BUCKET = self.slug_configs.get('oss_bucket', "")
        self.OSS_OBJECT_NAME = CLOUD_ASSISTANT + '/{serviceKey}/{appVersion}.tgz'
        logger.debug("mq_work.app_slug", 'init app slug')

    def do_work(self):
        try:
            logger.debug("mq_work.app_slug",
                         'get task....{}'.format(self.job.body))
            task = json.loads(self.job.body)
            self.task = task
            if "event_id" in self.task:
                self.event_id = task["event_id"]
                self.log = EventLog().bind(
                    event_id=self.event_id, step="image_manual")
            else:
                self.event_id = ""
                self.log = EventLog().bind(event_id="", step="image_manual")
            if task['action'] == 'create_new_version':
                self.log.info("开始分享新版本应用。")
                self.create_new_version()
            elif task['action'] == 'download_and_deploy':
                self.log.info("开始同步应用。")
                self.download_and_deploy()
            elif task['action'] == 'delete_old_version':
                self.log.info("开始删除旧版本应用。")
                self.delete_old_version()
        except Exception as e:
            logger.exception('mq_work.app_slug', e)

    def _upload_ftp(self, service_key, app_version, md5file):
        """ 上传文件到ftp """
        utils = FTPUtils(
            host=self.ALL_REGION_FTP_HOST,
            username=self.ALL_REGION_FTP_USERNAME,
            password=self.ALL_REGION_FTP_PASSWORD,
            namespace=self.ALL_REGION_FTP_NAMESPACE,
            port=self.ALL_REGION_FTP_PORT)
        # 检查service_key对应的文件是否存在,不存在生成
        service_dir = self.ALL_REGION_FTP_NAMESPACE + service_key
        logger.debug("mq_work.app_slug",
                     '*******upload dir is {}'.format(service_dir))
        utils.check_dir(service_dir)
        # 上传文件
        curr_region_slug = self.CURR_REGION_PATH.format(
            serviceKey=service_key, appVersion=app_version)
        logger.debug("mq_work.app_slug",
                     '*******upload file path is {}'.format(curr_region_slug))
        utils.upload(service_dir, curr_region_slug)
        # 上传md5文件
        if md5file:
            utils.upload(service_dir, md5file)
        return True

    def _create_md5(self, md5string, dest_slug_file):
        try:
            md5file = dest_slug_file + ".md5"
            f = open(md5file, "w")
            f.write(md5string)
            f.close()
            return md5file
        except Exception as e:
            logger.error("mq_work.app_slug", "sum file md5 filed!")
            logger.exception("mq_work.app_slug", e)
        return None

    def _check_md5(self, md5string, md5file):
        try:
            f = open(md5file)
            new_md5 = f.readline()
            return md5string == new_md5
        except Exception as e:
            logger.error("mq_work.app_slug", "check md5 filed!")
            logger.exception("mq_work.app_slug", e)
        return False

    def create_new_version(self):
        service_key = self.task['service_key']
        app_version = self.task['app_version']
        service_id = self.task['service_id']
        deploy_version = self.task['deploy_version']
        tenant_id = self.task['tenant_id']
        dest = self.task['dest']
        share_id = self.task.get('share_id', None)

        # 检查数据中心下路径是否存在
        source_slug_file = self.SRV_SLUG_BASE_DIR.format(
            tenantId=tenant_id,
            serviceId=service_id,
            deployVersion=deploy_version)
        self.log.debug("数据中心文件路径{0}".format(source_slug_file))
        # 当前数据中心文件名称
        dest_slug_file = self.CURR_REGION_PATH.format(
            serviceKey=service_key, appVersion=app_version)
        self.log.debug('当前数据中心文件名称'.format(dest_slug_file))
        # 检查目录是否存在
        curr_region_dir = os.path.dirname(dest_slug_file)
        if not os.path.exists(curr_region_dir):
            os.makedirs(curr_region_dir)
        # 复制文件
        self.log.debug(
            "开始复制文件 file {0} to {1}".format(source_slug_file, dest_slug_file))
        shutil.copyfile(source_slug_file, dest_slug_file)
        # 计算md5
        md5string = get_md5(source_slug_file)
        # 生成md5file
        md5file = self._create_md5(md5string, dest_slug_file)
        if md5file is None:
            self.log.error("md5文件没有生成。")
        # 区域中心对象存储,使用ftp
        slug = self.SLUG_PATH.format(
            serviceKey=service_key, appVersion=app_version)
        if dest == "yb":
            data = {
                'service_key': service_key,
                'app_version': app_version,
                'slug': slug,
                'image': "",
                'dest_yb': True,
                'dest_ys': False,
            }
            if share_id is not None:
                data['share_id'] = share_id
            try:
                self.region_client.service_publish_new_region(data)
            except Exception as e:
                self.region_client.service_publish_failure_region(data)
                self.log.error(
                    "云帮应用本地发布失败,保存publish 失败。{0}".format(e.message),
                    step="callback",
                    status="failure")
                pass
            if self.is_region_slug:
                try:
                    self.log.info("开始上传应用到本地云帮")
                    self._upload_ftp(service_key, app_version, md5file)
                    logger.debug("mq_work.app_slug",
                                 "*******ftp upload success!")
                    # self.update_publish_event(event_id=event_id, status='end', desc=u"云帮应用本地发布完毕")
                    self.user_cs_client.service_publish_success(
                        json.dumps(data))
                    try:
                        self.region_client.service_publish_success_region(data)
                    except Exception as e:
                        self.region_client.service_publish_failure_region(data)
                        logger.exception(e)
                        pass

                    self.log.info("云帮应用本地发布完毕", step="last", status="success")
                except Exception as e:
                    logger.error("mq_work.app_slug",
                                 "*******ftp upload failed")
                    logger.exception("mq_work.app_slug", e)
                    self.region_client.service_publish_failure_region(data)
                    self.log.info(
                        "云帮应用本地发布失败。{}".format(e.message),
                        step="callback",
                        status="failure")
            else:

                self.user_cs_client.service_publish_success(json.dumps(data))
                try:
                    self.region_client.service_publish_success_region(data)
                except Exception as e:
                    self.region_client.service_publish_failure_region(data)
                    logger.exception(e)
                    pass

                self.log.info("云帮应用本地发布完毕", step="last", status="success")
        elif dest == "ys":
            data = {
                'service_key': service_key,
                'app_version': app_version,
                'slug': slug,
                'image': "",
                'dest_ys': True,
                'dest_yb': False
            }
            if share_id is not None:
                data['share_id'] = share_id
            try:
                self.region_client.service_publish_new_region(data)
            except Exception as e:
                self.region_client.service_publish_failure_region(data)
                self.log.error(
                    "云帮应用本地发布失败,保存publish 失败。{0}".format(e.message),
                    step="callback",
                    status="failure")
                pass
            if self.is_oss_ftp:
                try:
                    self.log.info("开始上传应用到云市")
                    self._upload_ftp(service_key, app_version, md5file)
                    logger.debug("mq_work.app_slug",
                                 "*******ftp upload success!")
                    self.log.info("云市应用发布完毕", step="last", status="success")

                    self.user_cs_client.service_publish_success(
                        json.dumps(data))
                    try:
                        self.region_client.service_publish_success_region(data)
                    except Exception as e:
                        logger.exception(e)
                        self.region_client.service_publish_failure_region(data)
                        pass

                except Exception as e:
                    logger.error("mq_work.app_slug",
                                 "*******ftp upload failed, {0}".format(e))
                    self.region_client.service_publish_failure_region(data)
                    self.log.error(
                        "云市应用发布失败.", status="failure", step="callback")
            else:
                self.user_cs_client.service_publish_success(json.dumps(data))
                try:
                    self.region_client.service_publish_success_region(data)
                except Exception as e:
                    logger.exception(e)
                    self.region_client.service_publish_failure_region(data)
                    pass

                self.log.info("云市应用发布完毕", step="last", status="success")

    def _download_ftp(self, service_key, app_version, namespace, is_md5=False):
        """ 云帮ftp下载文件 """
        utils = FTPUtils(
            host=self.ALL_REGION_FTP_HOST,
            username=self.ALL_REGION_FTP_USERNAME,
            password=self.ALL_REGION_FTP_PASSWORD,
            namespace=self.ALL_REGION_FTP_NAMESPACE,
            port=self.ALL_REGION_FTP_PORT)
        logger.info("mq_work.app_slug",
                    "*******[download]download file from ftp")
        # 检查service_key对应的文件是否存在,不存在生成
        remote_file = self.ALL_REGION_FTP_PATH.format(
            serviceKey=service_key, appVersion=app_version)
        if is_md5:
            remote_file += ".md5"
        if not namespace:
            logger.info("mq_work.app_slug",
                        "*******[download]namespace is null")
            logger.error("mq_work.app_slug",
                         "*******[download]namespace is null")
        else:
            logger.info("mq_work.app_slug",
                        "*******[download]namespace is {}".format(namespace))
            remote_file = "../" + namespace + "/" + remote_file
        logger.info("mq_work.app_slug",
                    "*******[download]remote file is {}".format(remote_file))
        curr_region_slug = self.CURR_REGION_PATH.format(
            serviceKey=service_key, appVersion=app_version)
        if is_md5:
            curr_region_slug += ".md5"
        logger.info(
            "mq_work.app_slug",
            "*******[download]curr_region_slug is {}".format(curr_region_slug))
        return utils.download(remote_file, curr_region_slug)

    def _download_ftp_market(self,
                             service_key,
                             app_version,
                             namespace,
                             is_md5=False):
        """ 云市ftp下载文件 """
        utils = FTPUtils(
            host=self.ALL_REGION_FTP_HOST,
            username=self.ALL_REGION_FTP_USERNAME,
            password=self.ALL_REGION_FTP_PASSWORD,
            namespace=self.ALL_REGION_FTP_NAMESPACE,
            port=self.ALL_REGION_FTP_PORT)
        logger.info("mq_work.app_slug",
                    "*******[download]download file from ftp")
        # 检查service_key对应的文件是否存在,不存在生成
        remote_file = self.ALL_REGION_FTP_PATH.format(
            serviceKey=service_key, appVersion=app_version)
        if is_md5:
            remote_file += ".md5"
        if not namespace:
            logger.info("mq_work.app_slug",
                        "*******[download]namespace is null")
            logger.error("mq_work.app_slug",
                         "*******[download]namespace is null")
        else:
            logger.info("mq_work.app_slug",
                        "*******[download]namespace is {}".format(namespace))
            remote_file = "../" + namespace + "/" + remote_file
        logger.info("mq_work.app_slug",
                    "*******[download]remote file is {}".format(remote_file))
        curr_region_slug = self.CURR_REGION_PATH.format(
            serviceKey=service_key, appVersion=app_version)
        if is_md5:
            curr_region_slug += ".md5"
        logger.info(
            "mq_work.app_slug",
            "*******[download]curr_region_slug is {}".format(curr_region_slug))
        return utils.download(remote_file, curr_region_slug)

    def download_and_deploy(self):
        """ 下载slug包 """

        def start_service(service_id, deploy_version, operator):
            # body = {
            #     "deploy_version": deploy_version,
            #     "operator": operator,
            #     "event_id": self.event_id
            # }
            body = {
                "deploy_version": deploy_version,
                "event_id": self.event_id
            }
            try:
                # logger.info("mq_work.app_slug", "start service {}:{}".format(service_id, deploy_version))
                self.log.info("开始调用api启动应用。")
                self.api.upgrade_service(self.tenant_name, self.service_alias, json.dumps(body))
                # self.region_api.start_service(service_id, json.dumps(body))
            except self.region_api.CallApiError, e:
                self.log.info(
                    "开始调用api启动应用失败。{}".format(e.message),
                    step="callback",
                    status="failure")
                logger.exception("mq_work.app_slug", e)

        service_key = self.task['app_key']
        namespace = self.task['namespace']
        app_version = self.task['app_version']
        tenant_name = self.task['tenant_name']
        service_alias = self.task['service_alias']
        event_id = self.task['event_id']

        # 检查数据中心的是否存在slug包
        dest_slug_file = self.CURR_REGION_PATH.format(
            serviceKey=service_key, appVersion=app_version)
        logger.info("mq_work.app_slug",
                    "dest_slug_file:{}".format(dest_slug_file))
        ftp_ok = False
        try:
            # 检查当前服务器是否有slug文件
            if os.path.exists(dest_slug_file):
                self.log.debug("当前服务器存在本应用。本机同步开始")
                md5string = get_md5(dest_slug_file)
                # 检查云帮ftp是否打开, 下载md5进行校验
                md5_ok = False
                if self.is_region_slug:
                    self.log.debug("文件MD5校验开始。")
                    try:
                        md5_ok = self._download_ftp(service_key, app_version,
                                                    namespace, True)
                        self.log.info("MD5校验完成。")
                    except Exception as e:
                        logger.info(
                            "mq_work.app_slug",
                            "download md5 file from cloudassistant ftp failed!"
                        )
                        self.log.error(
                            "MD5校验失败。{}".format(e.message),
                            step="callback",
                            status="failure")
                        logger.exception("mq_work.app_slug", e)
                # md5未下载并且云市ftp开启
                if not md5_ok and self.is_oss_ftp:
                    self.log.info("MD5校验不通过。开始从云市同步新版本。")
                    try:
                        md5_ok = self._download_ftp_market(
                            service_key, app_version, namespace, True)
                    except Exception as e:
                        self.log.info(
                            "从云市同步新版本发生异常。{}".format(e.message),
                            step="callback",
                            status="failure")
                        logger.exception("mq_work.app_slug", e)
                if md5_ok:
                    md5file = dest_slug_file + ".md5"
                    same_file = self._check_md5(md5string, md5file)
                    if same_file:
                        logger.debug("mq_work.app_slug", "md5 check same.")
                        ftp_ok = True
                    else:
                        logger.debug(
                            "mq_work.app_slug",
                            "file md5 is changed, now delete old file")
                        os.remove(dest_slug_file)
                else:
                    logger.debug("mq_work.app_slug",
                                 "md5file download failed, now delete slug")
                    os.remove(dest_slug_file)

            # 检查当前服务器是否有slug文件
            if not os.path.exists(dest_slug_file):
                curr_region_dir = os.path.dirname(dest_slug_file)
                if not os.path.exists(curr_region_dir):
                    os.makedirs(curr_region_dir)
                logger.debug("mq_work.app_slug",
                             "now check ftp:".format(self.is_region_slug))
                # 云帮ftp开关是否打开
                if self.is_region_slug:
                    logger.debug('mq_work.app_slug', 'now check file on ftp!')
                    try:
                        ftp_ok = self._download_ftp(service_key, app_version,
                                                    namespace)
                    except Exception as e:
                        logger.info("mq_work.app_slug",
                                    "download object failed")
                        logger.exception("mq_work.app_slug", e)
                    logger.debug(
                        "mq_work.app_slug",
                        "*******[ftp download slug]result:==={}".format(
                            ftp_ok))

                # 判断是否需要从云市上下载,未下载并且云市ftp开启
                if not ftp_ok and self.is_oss_ftp:
                    logger.info(
                        "mq_work.app_slug",
                        "now download from hub ftp:{}".format(dest_slug_file))
                    ftp_ok = self._download_ftp_market(service_key,
                                                       app_version, namespace)
                    logger.debug(
                        "mq_work.app_slug",
                        "*******[ftp download slug]result:==={}".format(
                            ftp_ok))
            else:
                ftp_ok = True
        except Exception as e:
            logger.exception("mq_work.app_slug", e)
        version_status = {
            "final_status":"failure",
        }
        if ftp_ok:
            self.log.info("应用同步完成，开始启动应用。", step="app-image", status="success")
            version_body = {
                "type": 'slug',
                "path": dest_slug_file,
                "event_id": self.event_id
            }
            version_status = {
                "final_status":"success",
            }
            try:
                self.region_client.update_version_region(json.dumps(version_body))
                self.region_client.update_version_event(self.event_id,json.dumps(version_status))
            except Exception as e:
                pass
            try:
                body = {
                    "deploy_version": self.task['deploy_version'],
                    "event_id": self.event_id
                }
                # self.api.start_service(tenant_name, service_alias, event_id)
                self.api.upgrade_service(self.task['tenant_name'], self.task['service_alias'], json.dumps(body))
            except Exception as e:
                logger.exception(e)
                self.log.error(
                    "应用自动启动失败。请手动启动", step="callback", status="failure")
        else:
            self.log.error("应用同步失败。", step="callback", status="failure")
            try:
                self.region_client.update_version_event(self.event_id,json.dumps(version_status))
            except Exception as e:
                self.log.error("更新version信息失败", step="app-slug")
                pass

    def queryServiceStatus(self, service_id):
        try:
            res, body = self.region_api.is_service_running(service_id)
            logger.info(
                'mq_work.app_slug',
                "service_id=" + service_id + ";body=" + json.dumps(body))
            status = body.get(service_id, "closed")
            if status == "running":
                self.log.debug("依赖的应用状态已经为运行中。", step="worker")
                return True
        except:
            pass
        self.log.debug("依赖的应用状态不是运行中，本应用稍后启动。", step="worker")
        return False

    def delete_old_version(self):
        pass

    def delete_objects(self, objects):
        def oss_delete(del_objects):
            logger.info("mq_work.app_slug",
                        "deleting objects list: {0}".format(del_objects))
            success = self.oss_api.batch_delete_objects('gr-slug', del_objects)
            if success:
                logger.info("mq_work.app_slug", "delete objects success")
            else:
                logger.info("mq_work.app_slug",
                            "delete objects failed, {0}".format(success))

        while len(objects) > 0:
            del_objects, objects = objects[:500], objects[500:]
            oss_delete(del_objects)

    def update_publish_event(self, **kwargs):
        body = json.dumps(kwargs)
        try:
            self.region_api.update_event(body)
        except Exception, e:
            logger.exception("mq_work.app_slug", e)

    def splitChild(self, childs):
        data = []
        for lock_event_id in childs:
            data.append(lock_event_id.split("/")[-1])
        return data


def main():
    body = ""
    for line in fileinput.input():  # read task from stdin
        body = line
    app_slug = AppSlug(job=Job(body=body), config=load_dict)
    app_slug.do_work()


class Job():
    body = ""

    def __init__(self, body, *args, **kwargs):
        self.body = body

    def get_body(self):
        return self.body

    def get_task(self):
        task = json.loads(self.body)
        return task


if __name__ == '__main__':
    main()
