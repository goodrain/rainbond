# -*- coding: utf8 -*-
import time
import json
import httplib2
import urllib
import sys

from clients.region import RegionAPI
from clients.userconsole import UserConsoleAPI
from clients.region_api import RegionBackAPI
from utils.shell import Executer
from clients.etcdcli import TaskLocker
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


class CodeCheck():
    watching_topics = ('service_event_msg', )
    required_configs = ('region', 'userconsole', 'etcd.lock')

    def __init__(self, job, *args, **kwargs):
        self.job = job
        self.configs = kwargs.get("config")
        self.user_cs_client = UserConsoleAPI(conf=self.configs['userconsole'])
        self.region_client = RegionBackAPI()
        task = json.loads(self.job.body)
        self.base_dir = kwargs.get('base_dir')
        for k in ('tenant_id', 'service_id', 'action'):
            setattr(self, k, task[k])
        if 'event_id' in task:
            self.event_id = task["event_id"]
            self.log = EventLog().bind(event_id=self.event_id)
        else:
            self.event_id = "system"
            self.log = EventLog().bind(event_id=self.event_id)
        self.task = task
        self.locker = TaskLocker(conf=self.configs['etcd'])

        # self.waittime = int(task['wait_time'])
        self.log.info(u"worker已收到异步任务。", step="worker")

    def do_work(self):
        logger.info('mq_work.service_event',
                    "plugin %s execute start" % __name__)
        self.log.debug(u"代码检查异步处理开始。", step="worker", status="start")
        self.code_check()

        logger.info('mq_work.service_event',
                    "plugin %s execute finished" % __name__)

    def code_check(self):
        git_url = self.task['git_url']
        check_type = self.task['check_type']
        code_version = self.task['code_version']
        git_project_id = self.task['git_project_id']
        code_from = self.task['code_from']
        url_repos = self.task['url_repos']

        lock_id = 'code_check.' + self.service_id
        logger.info(
            'mq_work.code_check',
            "git_url {0},check_type {1}, code_version {2},git_project_id {3},code_from {4},url_repos {5} ".
            format(git_url, check_type, code_version, git_project_id,
                   code_from, url_repos))
        try:
            if self.locker.exists(lock_id):
                logger.info('mq_work.code_check',
                            "lock_id {} exists, do nothing".format(lock_id))
                self.log.info(
                    'lock_id {} exists, do nothing'.format(lock_id),
                    step="check_exist")
                return
            self.locker.add_lock(lock_id, bytes(git_url))
            logger.info('add lock_id {}'.format(lock_id), step="check-exist")
        except Exception, e:
            pass

        logger.info('mq_work.code_check', 'added lock <{}> for [{}]'.format(
            lock_id, git_url))
        logger.info(
            'mq_work.code_check',
            self.tenant_id + "=" + self.service_id + " start code check")
        if self.event_id:
            self.log.info(
                "代码检测{0},{1} 开始".format(self.tenant_id, self.service_id),
                step="check-start")
        cmd = '/bin/bash {0}/scripts/detect.sh {1} {2} "{3}" {4}'.format(
            self.base_dir, self.tenant_id, self.service_id, git_url,
            self.base_dir)
        try:
            output = Executer.call(cmd)
            self.requestConsole(self.service_id, output[0].rstrip('\n'),
                                check_type, git_url, code_version,
                                git_project_id, code_from, url_repos)
            if self.event_id:
                self.log.info("代码检测完成,请重新部署", step="last", status="success")
        except Executer.ExecException, e:
            logger.info('mq_work.code_check', 'code check failed')
            logger.info('mq_work.code_check', e)
            logger.info('mq_work.code_check', e.output)
            self.log.error(
                "代码检测异常 {}".format(e), step="callback", status="failure")
        finally:
            try:
                self.locker.drop_lock(lock_id)
                self.locker.release_lock()
            except Exception, e:
                pass
        logger.info('mq_work.code_check',
                    self.tenant_id + "=" + self.service_id + " end code check")
        if self.event_id:
            self.log.info(
                "代码检测{0},{1} 结束".format(self.tenant_id, self.service_id),
                step="check-end")

    def requestConsole(self, service_id, condition, check_type, git_url,
                       code_version, git_project_id, code_from, url_repos):
        body = {
            "service_id": service_id,
            "condition": condition,
            "check_type": check_type,
            "git_url": git_url,
            'code_version': code_version,
            'git_project_id': git_project_id,
            'code_from': code_from,
            "url_repos": url_repos
        }
        logger.info('mq_work.service_event',
                    "service_id=" + service_id + ";condition=" + condition)
        res, bodyres = self.user_cs_client.code_check(json.dumps(body))
        self.region_client.code_check_region(body)
        self.region_client.code_check_region(json.dumps(body))



def main():
    body = ""
    for line in fileinput.input():  # read task from stdin
        body = line
    
    code_check = CodeCheck(job=Job(body=body), config=load_dict, base_dir=sys.path[0])
    code_check.do_work()


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