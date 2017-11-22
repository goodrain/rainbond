import uuid
import etcd
import logging
import socket
import threading
from addict import Dict
logger = logging.getLogger('default')


class BasicLocker(object):

    def __init__(self, conf, *args, **kwargs):
        self.etcd_cfg = conf
        self.etcdClient = None
        self.base_path = '/goodrain/locks'

    def get_etcd_cli(self):
        if self.etcdClient is None:
            self.etcdClient = etcd.Client(host=self.etcd_cfg.get('host'), port=self.etcd_cfg.get('port'), allow_redirect=True)
        return self.etcdClient


class TaskLocker(BasicLocker):

    def __init__(self, conf):
        super(TaskLocker, self).__init__(conf)
        self.basic_path = '/goodrain/locks/tasks'

    def exists(self, lock_id):
        try:
            path = self.base_path + '/' + lock_id
            self.get_etcd_cli().get(path)
            return True
        except Exception as e:
            pass
        return False

    def add_lock(self, lock_id, value):
        path = self.base_path + '/' + lock_id
        self.get_etcd_cli().set(path, value)

    def _childs(self, key):
        childs = {}
        try:
            r = self.get_etcd_cli().read(key, recursive=True, sorted=True)
            for child in r.children:
                if child.dir:
                    tem = self._childs(child.key)
                    childs.update(tem)
                else:
                    childs[child.key] = child.value
        except Exception:
            pass
        return childs

    def get_children(self, lock_id):
        childs = []
        try:
            event_path = self.base_path + '/' + lock_id
            r = self.get_etcd_cli().read(event_path, recursive=True, sorted=True)
            for child in r.children:
                if child.dir:
                    tem = self._childs(child.key)
                    childs.extend(tem)
                else:
                    childs.append(child.key)
        except Exception as e:
            logger.exception(e)
        return childs

    def get_lock_event(self, lock_id, event_id):
        event_path = self.base_path + '/' + lock_id + '/' + event_id
        try:
            res = self.get_etcd_cli().get(event_path)
            if not res.dir:
                return res.value
        except Exception:
            pass
        return ""

    def remove_lock_event(self, lock_id, event_id):
        event_path = self.base_path + '/' + lock_id + '/' + event_id
        self.get_etcd_cli().delete(event_path, recursive=True)

    def drop_lock(self, lock_id):
        event_path = self.base_path + '/' + lock_id
        self.get_etcd_cli().delete(event_path, recursive=True)

    def release_lock(self):
        self.etcdClient = None


class InstanceLocker(BasicLocker):

    def __init__(self, renewSecondsPrior=5, timeout=None):
        conf = Dict({"host": "127.0.0.1", "port": 4001})
        super(InstanceLocker, self).__init__(conf)
        self.base_path = '/goodrain/locks/instances'
        self.client = self.get_etcd_cli()

    def get_lock(self, instance_name):
        key = self.base_path + '/' + instance_name.lstrip('/')
        try:
            return self.client.get(key)
        except etcd.EtcdKeyNotFound:
            return None

    def add_lock(self, instance_name, value, ttl=60):
        key = self.base_path + '/' + instance_name.lstrip('/')
        try:
            self.client.write(key, value, prevExist=False, recursive=True, ttl=ttl)
        except etcd.EtcdAlreadyExist:
            return False
        return True

    def update_lock(self, instance_name, value, ttl=60):
        key = self.base_path + '/' + instance_name.lstrip('/')
        self.client.write(key, value, ttl=ttl)
        return True

    def drop_lock(self, instance_name):
        key = self.base_path + '/' + instance_name.lstrip('/')
        try:
            self.client.delete(key, prevExist=True)
        except Exception as e:
            print e
