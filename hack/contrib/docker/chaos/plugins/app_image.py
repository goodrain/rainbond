# -*- coding: utf8 -*-
import os
import json
from utils.shell import Executer as shell
from clients.region import RegionAPI
from clients.registry import RegistryAPI
from clients.region_api import RegionBackAPI
from clients.etcdcli import TaskLocker
from clients.hubimageutils import HubUtils
from clients.userconsole import UserConsoleAPI
import etc
import time
import logging
import logging.config
from utils.log import EventLog
from etc import settings
from clients.acp_api import ACPAPI
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


class AppImage():
    def __init__(self, job, *args, **kwargs):
        self.job = job
        self.configs = kwargs.get("config")
        self.region_api = RegionAPI(conf=self.configs["region"])
        self.region_client = RegionBackAPI()
        self.api = ACPAPI(conf=self.configs['region'])
        image_config = self.configs["publish"]["image"]
        self.region_registry = RegistryAPI(
            host=image_config.get('curr_registry'))
        self.oss_registry = RegistryAPI(host=image_config.get('all_registry'))
        self.region_registry.set_log_topic('mq_work.app_image')
        self.oss_registry.set_log_topic('mq_work.app_image')
        self.locker = TaskLocker(conf=self.configs['etcd'])
        self.namespace = image_config.get('oss_namespace')
        self.user_cs_client = UserConsoleAPI(conf=self.configs["userconsole"])
        self.hubclient = HubUtils(image_config)
        # 是否配置oss.goodrain.me
        self.is_region_image = image_config.get('all_region_image')
        self.is_oss_image = image_config.get('oss_image')

    def do_work(self):
        try:
            task = json.loads(self.job.body)
            self.task = task
            if "event_id" in self.task:
                self.event_id = task["event_id"]
                self.log = EventLog().bind(event_id=self.event_id)
            else:
                self.event_id = ""
                self.log = EventLog().bind(event_id="")

            if task['action'] == 'create_new_version':
                self.log.info("开始发布升级应用。", step="app-image")
                self.create_new_version()
            elif task['action'] == 'download_and_deploy':
                self.log.info("开始同步和部署应用。", step="app-image")
                self.download_and_deploy()
            elif task['action'] == 'delete_old_version':
                self.log.info("开始删除旧版本应用。", step="app-image")
                self.delete_old_version()
        except Exception as e:
            if self.log:
                self.log.error(
                    "从云市部署应用失败。{}".format(e.message),
                    step="callback",
                    status="failure")
            logger.exception('mq_work.app_image', e)

    def create_new_version(self):
        image = self.task['image']
        service_key = self.task['service_key']
        app_version = self.task['app_version']
        oss_image = self.oss_registry.rename_image(image)
        dest = self.task['dest']

        share_id = self.task.get("share_id", None)
        if dest == "yb":
            if self.region_registry.exist_image(image):
                logger.debug("mq_work.app_image",
                             "now local exists, oss doesnot exists")
                if self.is_region_image and not self.oss_registry.exist_image(
                        oss_image):
                    try:
                        self.log.info("开始拉取镜像。")
                        ok = self._pull(image)
                        if not ok:
                            self.log.error(
                                "拉取镜像发生错误，构建退出。",
                                step="callback",
                                status="failure")
                            return
                        image_id = self.get_image_property(image, 'Id')
                        self.log.info("拉取镜像完成。")
                        self._tag(image_id, oss_image)
                        self.log.info("镜像更改tag完成。开始上传镜像到云帮")
                        ok = self._push(oss_image)
                        if not ok:
                            self.log.error(
                                "拉取镜像发生错误，构建退出。",
                                step="callback",
                                status="failure")
                            return
                        self.log.info("上传镜像到云帮完成")
                        # 发送通知到web
                        data = {
                            'service_key': service_key,
                            'app_version': app_version,
                            'image': image,
                            'dest_yb': True,
                            'dest_ys': False,
                            'slug': ""
                        }
                        if share_id is not None:
                            data["share_id"] = share_id
                        self.user_cs_client.service_publish_success(
                            json.dumps(data))
                        self.region_client.service_publish_success_region(json.dumps(data))
                        self.log.info(
                            "云帮应用发布完毕", step="last", status="success")
                    except (shell.ExecException, Exception), e:
                        logger.exception("mq_work.app_image", e)
                        logger.error("mq_work.app_image", e)
                        self.log.error(
                            "云帮应用发布失败 {}".format(e.message),
                            step="callback",
                            status="failure")
                else:
                    # 发送通知到web
                    data = {
                        'service_key': service_key,
                        'app_version': app_version,
                        'image': image,
                        'dest_yb': True,
                        'slug': "",
                        'dest_ys': False,
                    }
                    if share_id is not None:
                        data["share_id"] = share_id
                    self.user_cs_client.service_publish_success(
                        json.dumps(data))
                    self.region_client.service_publish_success_region(json.dumps(data))
                    self.log.info("云帮应用发布完毕", step="last", status="success")
        elif dest == "ys":
            # 当前有镜像并且云市的image数据中心开启
            if self.region_registry.exist_image(image) and self.is_oss_image:
                self.log.info("开始上传镜像到云市")
                # 修改image name
                hub_image = self.hubclient.rename_image(image)
                logger.info("mq_work.app_image",
                            'hub_image={}'.format(hub_image))
                # 检查是否存在
                data = self.hubclient.parse_image(image)
                logger.info("mq_work.app_image", 'data={}'.format(data))
                # 判断tag是否存在,
                tag_exists = self.hubclient.check(data.name, data.tag)
                logger.info("mq_work.app_image",
                            'tag_exists={}'.format(tag_exists))
                try:
                    self.log.info("开始从云帮拉取镜像。")
                    ok = self._pull(image)
                    if not ok:
                        self.log.error(
                            "拉取镜像发生错误，构建退出。",
                            step="callback",
                            status="failure")
                        return
                    image_id = self.get_image_property(image, 'Id')
                    self.log.info("从云帮拉取镜像完成,更改镜像TAG")
                    self._tag(image_id, hub_image)
                    self.log.info("更改镜像TAG完成，开始上传镜像到云市")
                    ok = self._push(hub_image)
                    if not ok:
                        self.log.error(
                            "拉取镜像发生错误，构建退出。",
                            step="callback",
                            status="failure")
                        return
                    self.log.info("上传镜像到云市完成。")
                    # 发送通知到web
                    data = {
                        'service_key': service_key,
                        'app_version': app_version,
                        'image': image,
                        'slug': "",
                        'dest_ys': True,
                        'dest_yb': False
                    }
                    if share_id is not None:
                        data["share_id"] = share_id
                    self.user_cs_client.service_publish_success(
                        json.dumps(data))
                    self.region_client.service_publish_success_region(json.dumps(data))
                    self.log.info("云市应用发布完毕", step="last", status="success")
                except (shell.ExecException, Exception), e:
                    logger.exception("mq_work.app_image", e)
                    logger.error("mq_work.app_image", e)
                    self.log.error(
                        "云市应用发布失败 {}".format(e.message),
                        step="callback",
                        status="failure")

    def download_and_deploy(self):
        image = self.task['image']
        namespace = self.task['namespace']
        tenant_name = self.task['tenant_name']
        service_alias = self.task['service_alias']
        event_id = self.task['event_id']
        oss_image = self.oss_registry.rename_image(image)
        region_download = False
        try:
            if not self.region_registry.exist_image(image):
                self.log.debug("image is " + image)
                logger.debug("mq_work.app_image",
                             "now check inner.goodrain.com {0}".format(
                                 self.is_region_image))
                self.log.debug("oss_image is " + oss_image)
                if self.is_region_image and self.oss_registry.exist_image(
                        oss_image):
                    try:
                        self.log.info("云帮发现镜像，开始从内部获取。", step="app-image")
                        ok = self._pull(oss_image)
                        if not ok:
                            self.log.error(
                                "拉取镜像发生错误，构建退出。",
                                step="callback",
                                status="failure")
                            return
                        image_id = self.get_image_property(oss_image, 'Id')
                        self._tag(image_id, image)
                        ok = self._push(image)
                        if not ok:
                            self.log.error(
                                "上传镜像发生错误，构建退出。",
                                step="callback",
                                status="failure")
                            return
                        region_download = True
                    except (shell.ExecException, Exception), e:
                        logger.exception("mq_work.app_image", e)
                        logger.error("mq_work.app_image", e)
                        self.log.error(
                            "从云帮镜像仓库拉取镜像失败。" + e.__str__(), step="app-image")

                # 云帮未配置,直接从云市下载|云帮下载失败,直接从云市下载
                # 云市images数据中心开启可下载,否则不可下载
                if not region_download and self.is_oss_image:
                    # 判断是否存在hub
                    logger.info("mq_work.app_image",
                                'download image from hub.goodrain.com')
                    self.log.info("开始从云市获取镜像。", step="app-image")
                    # 修改image name
                    hub_image = self.hubclient.rename_image(
                        image, namespace=namespace)

                    # logger.info("mq_work.app_image", '===[download]hub_image={}'.format(hub_image))
                    # 检查是否存在
                    data = self.hubclient.parse_image(image)
                    hub_exists = self.hubclient.check_image(
                        data.name, data.tag, namespace=namespace)
                    # logger.info("mq_work.app_image", '===[download]hub_exists={}'.format(hub_exists))
                    if hub_exists:
                        try:
                            self.log.info("开始拉取镜像。", step="app-image")
                            ok = self._pull(hub_image)
                            if not ok:
                                self.log.error(
                                    "拉取镜像发生错误，构建退出。",
                                    step="callback",
                                    status="failure")
                                return
                            self.log.info("拉取镜像完成。", step="app-image")
                            image_id = self.get_image_property(hub_image, 'Id')
                            self._tag(image_id, image)
                            self.log.info("更改镜像TAG完成。", step="app-image")
                            ok = self._push(image)
                            if not ok:
                                self.log.error(
                                    "上传镜像发生错误，构建退出。",
                                    step="callback",
                                    status="failure")
                                return
                            self.log.info("上传镜像到本地仓库完成。", step="app-image")
                            region_download = True
                        except (shell.ExecException, Exception), e:
                            logger.exception("mq_work.app_image", e)
                            self.log.error(
                                "从云市镜像仓库拉取镜像失败。" + e.__str__(),
                                step="app-image")
                    else:
                        logger.error("image {0} not found, can't continue".
                                     format(hub_image))
                        self.log.error(
                            "云市未发现此镜像。{0}".format(hub_image), step="app-image")
            else:
                self.log.info("本地存在此镜像，无需同步", step="app-image")
                region_download = True
        except Exception as e:
            logger.exception("mq_work.app_image", e)
            self.log.error(
                "同步镜像发生异常." + e.__str__(), step="app-image", status="failure")

        if region_download:
            self.log.info("应用同步完成，开始启动应用。", step="app-image", status="success")
            try:
                self.api.start_service(tenant_name, service_alias, event_id)
            except Exception as e:
                logger.exception(e)
                self.log.error(
                    "应用自动启动失败。请手动启动", step="callback", status="failure")
        else:
            self.log.error("应用同步失败。", step="callback", status="failure")

    def queryServiceStatus(self, service_id):
        try:
            res, body = self.region_api.is_service_running(service_id)
            logger.info(
                'mq_work.app_image',
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

    def delete_oss_images(self, images):
        for image in images:
            deleted = self.oss_registry.delete_image(image)
            logger.info("mq_work.app_image", "delete image {0} {1}".format(
                image, deleted))

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
            logger.exception("mq_work.app_image", e)
            return None

    def update_publish_event(self, **kwargs):
        body = json.dumps(kwargs)
        try:
            self.region_api.update_event(body)
        except Exception, e:
            logger.exception("mq_work.app_image", e)

    def _pull(self, image):
        cmd = "{} pull {}".format(DOCKER_BIN, image)
        logger.info("mq_work.app_image", cmd)
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
                self.log.error("下载镜像发生错误。{0}" + ("开始进行重试." if retry > 0 else
                                                 "").format(e.message))
                logger.error(e)
                retry -= 1
        return False

    def _push(self, image):
        cmd = "{} push {}".format(DOCKER_BIN, image)
        logger.info("mq_work.app_image", cmd)
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
                self.log.error("上传镜像发生错误。{0}" + ("开始进行重试." if retry > 0 else
                                                 "").format(e.message))
                logger.error(e)
                retry -= 1
        return False

    def _tag(self, image_id, image):
        cmd = "{2} tag {0} {1}".format(image_id, image, DOCKER_BIN)
        logger.info("mq_work.app_image", cmd)
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
    app_image = AppImage(job=Job(body=body), config=load_dict)
    app_image.do_work()


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
