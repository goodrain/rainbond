# -*- coding: utf8 -*-
import os
import sys

from utils.parse_dockerfile import ParseDockerFile
from utils.log import EventLog

import logging
import logging.config
from etc import settings
import re
import json
import time
import datetime
import pipes
import shutil
import threading
import fileinput
from utils.shell import Executer as shell
from utils.docker import DockerfileItem
from clients.region import RegionAPI
from clients.region_api import RegionBackAPI
from clients.acp_api import ACPAPI
from clients.userconsole import UserConsoleAPI

load_dict = {}
with open("plugins/config.json", 'r') as load_f:
    load_dict = json.load(load_f)

logging.config.dictConfig(settings.get_logging(load_dict))
logger = logging.getLogger('default')

reload(sys)
sys.setdefaultencoding('utf-8')

TENANT_DIR = '/grdata/build/tenant/{tenantId}'
SOURCE_DIR = '/cache/build/{tenantId}' + '/' + 'source/{serviceId}'
TGZ_DIR = TENANT_DIR + '/' + 'slug/{serviceId}'
CACHE_DIR = '/cache/build/{tenantId}' + '/' + 'cache/{serviceId}'
BUILD_LOG_DIR = '/grdata/logs/{tenantId}/{serviceId}/'
CLONE_TIMEOUT = 180

REGISTRY_DOMAIN = 'goodrain.me'

MAX_BUILD_TASK = 5

if os.access("/var/run/docker.sock", os.W_OK):
    DOCKER_BIN = "docker"
else:
    DOCKER_BIN = "sudo -P docker"


class RepoBuilder():
    def __init__(self, task, *args, **kwargs):
        self.configs = kwargs.get("config")
        self.region_api = RegionAPI(conf=self.configs['region'])
        self.api = ACPAPI(conf=self.configs['region'])
        self.user_cs_client = UserConsoleAPI(conf=self.configs['userconsole'])
        self.repo_url = task['repo_url']
        self.region_client = RegionBackAPI()
        self.tenant_id = task['tenant_id']
        self.service_id = task['service_id']
        self.tenant_name = task['tenant_name']
        self.service_alias = task['service_alias']
        self.deploy_version = task['deploy_version']
        self.action = task['action']

        if 'event_id' in task:
            self.event_id = task["event_id"]
            self.log = EventLog().bind(event_id=self.event_id)
        else:
            self.event_id = ""
            self.log = EventLog().bind(event_id=self.event_id)

        self.operator = task['operator']
        self.build_envs = task.get('envs', {})
        self.expire = task.get('expire', 60)

        self.start_time = int(time.time())

        # self.source_dir = '/tmp/goodrain_web'
        self.source_dir = SOURCE_DIR.format(
            tenantId=self.tenant_id, serviceId=self.service_id)
        self.cache_dir = CACHE_DIR.format(
            tenantId=self.tenant_id, serviceId=self.service_id)
        self.tgz_dir = TGZ_DIR.format(
            tenantId=self.tenant_id, serviceId=self.service_id)
        self.build_log_dir = BUILD_LOG_DIR.format(
            tenantId=self.tenant_id, serviceId=self.service_id)

        self.build_cmd = 'plugins/scripts/build.pl'

    @property
    def build_name(self):
        return self.service_id[:8] + '_' + self.deploy_version

    @property
    def is_expired(self):
        if hasattr(self, 'expire'):
            current_time = int(time.time())
            return bool(current_time - self.start_time > self.expire)
        else:
            return False

    def prepare(self):
        if os.path.exists(self.source_dir):
            shutil.rmtree(self.source_dir)

        for d in (self.source_dir, self.cache_dir, self.tgz_dir,
                  self.build_log_dir):
            if not os.path.exists(d):
                os.makedirs(d)

    def clone(self):
        self.log.info("开始拉取代码。。", step="build-worker")
        # code, output = shell.runsingle("git clone --branch master --depth 1 {0} {1}".format(self.repo_url, self.source_dir))
        result = False
        num = 0
        while num < 2:
            try:
                shell.call("timeout -k 9 {2} git clone {0} {1}".format(
                    self.repo_url, self.source_dir, CLONE_TIMEOUT))
                result = True
                break
            except shell.ExecException, e:
                num = num + 1
                self.prepare()
                if num < 2:
                    self.log.error(
                        "拉取代码发生错误,开始重试 {}".format(e.message),
                        status="failure",
                        step="worker-clone")
                else:
                    self.log.error(
                        "拉取代码发生错误,部署停止 {}".format(e.message),
                        status="failure",
                        step="callback")
                    logger.exception('build_work.main', e)
                result = False
        logger.info('build_work.main', "git clone num=" + str(num))
        return result

    def get_commit_info(self):
        try:
            output = shell.call(
                """git log -n 1 --pretty --format='{"hash":"%H","author":"%an","timestamp":%at}'""",
                self.source_dir)
            if type(output) is list:
                output = output[0]
            jdata = json.loads(output)

            output2 = shell.call("""git log -n 1 --pretty --format=%s""",
                                 self.source_dir)
            if type(output2) is list:
                subject = output2[0]
                jdata['subject'] = subject
            else:
                jdata['subject'] = 'unknown'
            return jdata
        except shell.ExecException, e:
            logger.exception('build_work.main', e)
            return "{}"

    def find_dockerfile(self):
        return bool(
            os.path.exists('{0}/{1}'.format(self.source_dir, 'Dockerfile')))

    def rewrite_files(self, dockerfile, insert_lines, cmd, entrypoint):
        extend_lines = map(lambda x: x + '\n', insert_lines)

        try:
            f = open(dockerfile, 'r')
            lines = f.readlines()
            for line in lines:
                if line.startswith('ENTRYPOINT') or line.startswith('CMD'):
                    lines.remove(line)
            lines.extend(extend_lines)
            f.close()

            f = open(dockerfile, 'w')
            f.writelines(lines)
            f.close()

            shutil.copytree('./lib/.goodrain',
                            '{0}/.goodrain'.format(self.source_dir))

            if entrypoint is not None:
                entrypoint_cmd = ' '.join(entrypoint)
                shell.call(
                    '''sed -i -e 's#_type_#ENTRYPOINT#' -e 's#^_entrypoint_#'{0}'#' .goodrain/init'''.
                        format(pipes.quote(entrypoint_cmd)),
                    cwd=self.source_dir)
                if cmd is not None:
                    shell.call(
                        '''sed -i -e 's#^_cmd_#'{0}'#' .goodrain/init'''.
                            format(pipes.quote(cmd)),
                        cwd=self.source_dir)
            else:
                shell.call(
                    '''sed -i -e 's#_type_#CMD#' -e 's#^_cmd_#'{0}'#' .goodrain/init'''.
                        format(pipes.quote(cmd)),
                    cwd=self.source_dir)
            return True
        except (shell.ExecException, OSError), e:
            logger.exception('build_work.main', e)
            return False

    def get_dockerfile_items(self, filename):
        f = open(filename, 'r')
        lines = map(lambda x: x.rstrip('\n').rstrip('\r'), f.readlines())
        items = {"port": 0, "volume": ""}
        entrypoint = None
        cmd = None

        for line in lines:
            i = DockerfileItem(line)
            if i.is_port_item:
                items['port'] = i.value
            elif i.is_volume_item:
                items['volume'] = i.value
            elif i.is_entrypoint_item:
                entrypoint = i.value
            elif i.is_cmd_item:
                cmd = ' '.join([pipes.quote(e) for e in i.value])

        # env = ','.join(map(lambda x: '{0}={1}'.format(x[0], x[1]), items.get('env', {}).items()))
        volume_mount_path = items.get('volume')
        inner_port = items.get('port')
        # 过滤tcp,udp
        if isinstance(inner_port, basestring):
            inner_port = inner_port.replace("/tcp", "")
            inner_port = inner_port.replace("/udp", "")
            inner_port = inner_port.replace('"', '')

        return {
                   "inner_port": inner_port,
                   "volume_mount_path": volume_mount_path
               }, entrypoint, cmd

    def build_image(self):
        # self.write_build_log(u"开始编译Dockerfile")
        '''
        insert_lines = [
            'RUN which wget || (apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y wget) || (yum install -y wget)',
            'RUN which curl || (apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y curl) || (yum install -y curl)',
            'RUN mkdir -pv /opt/bin',
            'ADD ./.goodrain/init /opt/bin/init',
            '',
            'RUN wget http://lang.goodrain.me/public/gr-listener -O /opt/bin/gr-listener -q && \\',
            '    chmod 755 /opt/bin/*',
            '',
            'RUN rm -rf /var/lib/dpkg/* /var/lib/apt/*',
            'ENTRYPOINT ["/opt/bin/init"]',
        ]
        '''
        dockerfile = '{0}/{1}'.format(self.source_dir, 'Dockerfile')
        update_items, entrypoint, cmd = self.get_dockerfile_items(dockerfile)
        # 重新解析dockerfile
        pdf = None
        try:
            self.log.info("开始解析Dockerfile", step="build_image")
            pdf = ParseDockerFile(dockerfile)
        except ValueError as e:
            self.log.error(
                "用户自定义的volume路径包含相对路径,必须为绝对路径!",
                step="build_image",
                status="failure")
            logger.exception(e)
            return False
        except Exception as e:
            self.log.error(
                "解析Dockerfile发生异常", step="build_image", status="failure")
            logger.exception(e)

        s = self.repo_url

        regex = re.compile(r'.*(?:\:|\/)([\w\-\.]+)/([\w\-\.]+)\.git')
        m = regex.match(s)
        account, project = m.groups()
        _name = '_'.join([self.service_id[:12], account, project])
        _tag = self.deploy_version
        build_image_name = '{0}/{1}:{2}'.format(REGISTRY_DOMAIN, _name, _tag)
        # image name must be lower
        build_image_name = build_image_name.lower()
        self.log.debug(
            "构建镜像名称为{0}".format(build_image_name), step="build_image")


        #build_image_name=""



        no_cache = self.build_envs.pop('NO_CACHE', False)
        if no_cache:
            build_cmd = "{0} build -t {1} --no-cache .".format(
                DOCKER_BIN, build_image_name)
        else:
            build_cmd = "{0} build -t {1} .".format(DOCKER_BIN,
                                                    build_image_name)

        p = shell.start(build_cmd, cwd=self.source_dir)
        while p.is_running():
            line = p.readline()
            self.log.debug(line, step="build_image")

        for line in p.unread_lines:
            self.log.debug(line, step="build_image")
        if p.exit_with_err():
            self.log.error(
                "构建失败，请检查Debug日志排查！", step="build_image", status="failure")
            return False
        self.log.debug("镜像构建成功。开始推送", step="build_image", status="success")
        try:
            shell.call("{0} push {1}".format(DOCKER_BIN, build_image_name))
        except shell.ExecException, e:
            self.log.error(
                "镜像推送失败。{}".format(e.message),
                step="push_image",
                status="failure")
            return False

        update_items.update({"image": build_image_name})
        # ports volums envs
        if pdf:
            update_items.update({
                "port_list": pdf.get_expose(),
                "volume_list": pdf.get_volume()
            })

        h = self.user_cs_client
        try:
            h.update_service(self.service_id, json.dumps(update_items))
            self.region_client.update_service_region(self.service_id,json.dumps(update_items))
        except h.CallApiError, e:
            self.log.error(
                "网络异常，更新应用镜像名称失败. {}".format(e.message),
                step="update_image",
                status="failure")
            return False

        version_body = {
            "type": 'image',
            "path": build_image_name,
            "event_id": self.event_id
        }
        try:
            self.region_client.update_version_region(json.dumps(version_body))
        except Exception as e:
            self.log.error(
                "更新版本信息失败{0}失败{1}".format(self.event_id, e.message),
                step="build_code")
            pass
        return True

    def build_code(self):
        self.log.info("开始编译代码包", step="build_code")
        package_name = '{0}/{1}.tgz'.format(self.tgz_dir, self.deploy_version)
        self.logfile = '{0}/{1}.log'.format(self.tgz_dir, self.deploy_version)
        repos = self.repo_url.split(" ")

        self.log.debug("repos=" + repos[1], step="build_code")
        #master
        no_cache = self.build_envs.pop('NO_CACHE', False)
        if no_cache:
            try:
                shutil.rmtree(self.cache_dir)
                os.makedirs(self.cache_dir)
                self.log.debug(
                    "清理缓存目录{0}".format(self.cache_dir), step="build_code")
            except Exception as e:
                self.log.error(
                    "清理缓存目录{0}失败{1}".format(self.cache_dir, e.message),
                    step="build_code")
                pass

        try:
            cmd = "perl {0} -b {1} -s {2} -c {3} -d {4} -v {5} -l {6} -tid {7} -sid {8} --name {9}".format(
                self.build_cmd, repos[1], self.source_dir, self.cache_dir,
                self.tgz_dir, self.deploy_version, self.logfile,
                self.tenant_id, self.service_id, self.build_name)

            if self.build_envs:
                build_env_string = ':::'.join(
                    map(lambda x: "{}='{}'".format(x, self.build_envs[x]),
                        self.build_envs.keys()))
                cmd += " -e {}".format(build_env_string)

            p = shell.start(cmd)
            while p.is_running():
                line = p.readline()
                self.log.debug(
                    line.rstrip('\n').lstrip('\x1b[1G'), step="build_code")

            for line in p.unread_lines:
                self.log.debug(line, step="build_code")
            if p.exit_with_err():
                self.log.error("编译代码包失败。", step="build_code", status="failure")
                return False
            self.log.debug("编译代码包完成。", step="build_code", status="success")
        except shell.ExecException, e:
            self.log.error(
                "编译代码包过程遇到异常，{}".format(e.message),
                step="build_code",
                status="failure")
            return False
        try:
            package_size = os.path.getsize(package_name)
            if package_size == 0:
                self.log.error(
                    "构建失败！构建包大小为0 name {0}".format(package_name),
                    step="build_code",
                    status="failure")
                return False
        except OSError, e:
            logger.exception('build_work.main', e)
            self.log.error("代码构建失败，构建包未生成。查看Debug日志检查错误详情", step="build_code", status="failure")
            return False

        self.log.info("代码构建完成", step="build_code", status="success")

        version_body = {
            "type": 'code',
            "path": package_name,
            "event_id": self.event_id
        }
        try:
            self.region_client.update_version_region(json.dumps(version_body))
        except Exception as e:
            logger.exception("build_work.main", e)
            pass
        return True

    def feedback(self):
        time.sleep(2)
        body = {
            "deploy_version": self.deploy_version,
            "event_id": self.event_id
        }
        try:
            if self.action == 'deploy':
                self.log.info("开始部署应用。", step="app-deploy")
                self.api.upgrade_service(self.tenant_name, self.service_alias, json.dumps(body))
                # 调用升级接口，如果没有启动则触发start操作
                # h.deploy_service(self.service_id, json.dumps(body))
            elif self.action == 'upgrade':
                self.log.info("开始升级应用。", step="app-deploy")
                self.api.upgrade_service(self.tenant_name, self.service_alias, json.dumps(body))
            return True
        except self.api.CallApiError, e:
            self.log.error(
                "部署应用时调用API发生异常。{}".format(e.message), step="app-deploy")
            logger.exception('build_work.main', e)
            return False

    def run(self):
        try:
            self.prepare()
            if self.clone():
                commit_info = self.get_commit_info()
                #can req api to update code info
                self.log.info("代码拉取成功。", step="build-worker")
                self.log.info(
                    "版本:{0} 上传者:{1} Commit:{2} ".format(
                        commit_info["hash"][0:7], commit_info["author"],
                        commit_info["subject"]),
                    step="code-version",
                    status="success")
                version_body = {
                    "code_version":commit_info["hash"][0:7],
                    "code_commit_msg":commit_info["subject"],
                    "code_commit_author":commit_info["author"]
                }
                try:
                    self.region_client.update_version_event(self.event_id,json.dumps(version_body))
                except Exception as e:
                    pass
                if self.find_dockerfile():
                    self.log.info(
                        "代码识别出Dockerfile,直接构建镜像。", step="build-worker")
                    build_func = getattr(self, 'build_image')
                else:
                    self.log.info("开始代码构建", step="build-worker")
                    build_func = getattr(self, 'build_code')

                success = build_func()
                if success:
                    # self.log.info("构建完成。", step="build-worker")
                    version_body = {
                        "final_status":"success",
                    }

                    self.log.info("构建完成。", step="build-worker", status="success")

                    ok = self.feedback()
                    if not ok:
                        self.log.error(
                            "升级部署应用错误", step="callback", status="failure")
                else:
                    self.log.info("构建失败,请查看Debug构建日志", step="callback", status="failure")
                    version_body = {
                        "final_status":"failure",
                    }
                try:
                    self.region_client.update_version_event(self.event_id,json.dumps(version_body))
                except Exception as e:
                    self.log.error(
                        "更新version信息失败", step="build-worker")
                    pass
            else:
                self.log.error("代码拉取失败。", step="callback", status="failure")
                version_body = {
                    "final_status":"failure",
                }
                try:
                    self.region_client.update_version_event(self.event_id,json.dumps(version_body))
                except Exception as e:
                    self.log.error(
                        "更新version信息失败", step="build-worker")
                    pass
        except Exception as e:
            self.log.error(
                "代码构建发生异常.{}".format(e.message),
                step="callback",
                status="failure")
            version_body = {
                "final_status":"failure",
            }
            try:
                self.region_client.update_version_event(self.event_id,json.dumps(version_body))
            except Exception as e:
                self.log.error(
                    "更新version信息失败", step="build-worker")
                pass
            logger.exception('build_work.main', e)
            raise e

def update_service_region(self, service_id, body):
    #todo 127.0.0.1:3333/api/codecheck
    # url = self.base_url + '/api/services/{0}'.format(service_id)
    url = 'http://127.0.0.1:3228/v2/builder/codecheck/{0}'.format(service_id)
    res, body = self._put(url, self.default_headers, body)
def main():
    body = ""
    for line in fileinput.input():  # read task from stdin
        body = line
    builder = RepoBuilder(task=Job(body).get_task(), config=load_dict)
    builder.run()


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
