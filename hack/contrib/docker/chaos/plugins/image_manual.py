# -*- coding: utf8 -*-
import os
import json
from utils.shell import Executer as shell
from clients.region import RegionAPI
from clients.registry import RegistryAPI
from clients.etcdcli import TaskLocker
from clients.userconsole import UserConsoleAPI
from clients.acp_api import ACPAPI
from clients.region_api import RegionBackAPI
import time
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

if os.access("/var/run/docker.sock", os.W_OK):
    DOCKER_BIN = "docker"
else:
    DOCKER_BIN = "sudo -P docker"


class ImageManual():
    def __init__(self, job, *args, **kwargs):
        self.job = job
        self.configs = kwargs.get("config")
        self.region_api = RegionAPI(conf=self.configs['region'])
        image_config = self.configs["publish"]["image"]
        self.region_client = RegionBackAPI()
        self.region_registry = RegistryAPI(
            host=image_config.get('curr_registry'))
        # self.region_registry.set_log_topic('mq_work.image_manual')
        self.oss_registry = RegistryAPI(host=image_config.get('all_registry'))
        self.oss_registry.set_log_topic('mq_work.image_manual')
        self.locker = TaskLocker(conf=self.configs['etcd'])
        self.api = ACPAPI(conf=self.configs['region'])
        self.namespace = image_config.get('oss_namespace')
        self.user_cs_client = UserConsoleAPI(conf=self.configs['userconsole'])

    def do_work(self):
        try:
            task = json.loads(self.job.body)
            self.task = task
            if "event_id" in self.task:
                self.event_id = task["event_id"]
                self.log = EventLog().bind(
                    event_id=self.event_id, step="image_manual")
            else:
                self.event_id = ""
                self.log = EventLog().bind(event_id="", step="image_manual")
            logger.info("mq_work.image_manual",
                        "new image_manual task: {}".format(task))
            if task['action'] == 'create_new_version':
                self.log.info("开始升级应用。")
                self.create_new_version()
            elif task['action'] == 'download_and_deploy':
                self.log.info("开始下载镜像并部署应用。")
                self.download_and_deploy()
            elif task['action'] == 'delete_old_version':
                self.log.info("开始删除旧版本。")
                self.delete_old_version()
        except Exception as e:
            if self.log:
                self.log.error(
                    "从自定义镜像部署应用失败。{}".format(e.message),
                    step="callback",
                    status="failure")
            logger.exception('mq_work.image_manual', e)

    def create_new_version(self):
        logger.debug("mq_work.image_manual",
                     "now create new version and upload image")

    def delete_old_version(self):
        logger.debug("mq_work.image_manual", "now delete old version")

    def download_and_deploy(self):
        image = self.task['image']
        # namespace = self.task['namespace']
        tenant_name = self.task['tenant_name']
        service_alias = self.task['service_alias']
        event_id = self.task['event_id']
        service_alias = self.task.get("service_alias", None)
        has_download = False
        inner_image = self.oss_registry.rename_image(image)
        inner_image = "{0}_{1}".format(inner_image, service_alias)
        local_image = self.region_registry.rename_image(image)
        local_image = "{0}_{1}".format(local_image, service_alias)
        # 直接下载docker image
        try:
            self.log.info("开始下载镜像:{0}".format(image))
            pull_result = self._pull(image)
            if pull_result:
                # image_id = self.get_image_property(image, 'Id')
                self._tag(image, local_image)
                self.log.info("修改镜像名为：{0}".format(local_image))
                ok = self._push(local_image)
                if not ok:
                    self.log.error(
                        "上传镜像发生错误，重试失败，退出。", step="callback", status="failure")
                    return
                self.log.info("镜像推送到本地仓库完成。")
                # self._tag(image, inner_image)
                # self._push(inner_image)
                has_download = True
            else:
                self.log.error("下载镜像发生错误。", step="callback", status="failure")
                logger.error("mq_work.image_manual",
                             "download image failed! image:{}".format(image))

        except Exception as e:
            self.log.error(
                "镜像操作发生错误。{0}".format(e.__str__()),
                step="callback",
                status="failure")
            logger.exception("mq_work.image_manual", e)
        version_status = {
            "final_status":"failure",
        }
        if has_download:
            self.log.info("应用同步完成。", step="app-image", status="success")
            version_body = {
                "type": 'image',
                "path": local_image,
                "event_id": self.event_id
            }
            version_status['final_status'] = "success"
            try:
                self.region_client.update_version_region(json.dumps(version_body))
                self.region_client.update_version_event(self.event_id,json.dumps(version_status))
            except Exception as e:
                pass
            try:
                self.api.update_iamge(tenant_name, service_alias, local_image)
                self.log.info("应用信息更新完成，开始启动应用。", step="app-image", status="success")
                self.api.start_service(tenant_name, service_alias, event_id)
            except Exception as e:
                logger.exception(e)
                self.log.error(
                    "应用自动启动失败。请手动启动", step="callback", status="failure")
        else:
            try:
                self.region_client.update_version_event(self.event_id,json.dumps(version_status))
            except Exception as e:
                pass
            self.log.error("应用同步失败。", step="callback", status="failure")

    def queryServiceStatus(self, service_id):
        try:
            res, body = self.region_api.is_service_running(service_id)
            logger.info(
                'mq_work.image_manual',
                "service_id=" + service_id + ";body=" + json.dumps(body))
            status = body.get(service_id, "closed")
            if status == "running":
                self.log.debug("依赖的应用状态已经为运行中。", step="worker")
                return True
        except:
            pass
        self.log.debug("依赖的应用状态不是运行中，本应用稍后启动。", step="worker")
        return False

    def get_image_property(self, image, name):
        query_format = '{{.%s}}' % name
        try:
            output = shell.call("{2} inspect -f '{0}' {1}".format(
                query_format, image, DOCKER_BIN))
            if output == '<no value>':
                return None
            else:
                return output[0].rstrip('\n')
        except shell.ExecException, e:
            logger.exception("mq_work.image_manual", e)
            return None

    def update_publish_event(self, **kwargs):
        body = json.dumps(kwargs)
        try:
            self.region_api.update_event(body)
        except Exception, e:
            logger.exception("mq_work.image_manual", e)

    def _pull(self, image):
        cmd = "{0} pull {1}".format(DOCKER_BIN, image)
        retry = 2
        while retry:
            try:
                p = shell.start(cmd)
                while p.is_running():
                    line = p.readline()
                    self.log.debug(
                        line.rstrip('\n').lstrip('\x1b[1G'), step="pull-image")
                for line in p.unread_lines:
                    self.log.debug(line, step="pull-image")
                if p.exit_with_err():
                    self.log.error(
                        "拉取镜像失败。" + ("开始进行重试." if retry > 0 else ""),
                        step="pull-image",
                        status="failure")
                    retry -= 1
                    continue
                return True
            except shell.ExecException, e:
                self.log.error("下载镜像发生错误。{}" + ("开始进行重试." if retry > 0 else
                                                "").format(e.message))
                retry -= 1
        return False

    def _push(self, image):
        cmd = "{0} push {1}".format(DOCKER_BIN, image)
        logger.info("mq_work.image_manual", cmd)
        retry = 2
        while retry:
            try:
                p = shell.start(cmd)
                while p.is_running():
                    line = p.readline()
                    self.log.debug(
                        line.rstrip('\n').lstrip('\x1b[1G'), step="push-image")
                for line in p.unread_lines:
                    self.log.debug(line, step="push-image")
                if p.exit_with_err():
                    self.log.error(
                        "上传镜像失败。" + ("开始进行重试." if retry > 0 else ""),
                        step="push-image",
                        status="failure")
                    retry -= 1
                    continue
                return True
            except shell.ExecException, e:
                self.log.error("上传镜像发生错误。{}" + ("开始进行重试." if retry > 0 else
                                                "").format(e.message))
                logger.error(e)
                retry -= 1
        return False

    def _tag(self, image_id, image):
        cmd = "{2} tag {0} {1}".format(image_id, image, DOCKER_BIN)
        logger.info("mq_work.image_manual", cmd)
        shell.call(cmd)

    def splitChild(self, childs):
        data = []
        for lock_event_id in childs:
            data.append(lock_event_id.split("/")[-1])
        return data


def main():
    body = ""
    for line in fileinput.input():  # read task from stdin
        body = line
    image_manual = ImageManual(config=load_dict, job=Job(body=body))
    image_manual.do_work()


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