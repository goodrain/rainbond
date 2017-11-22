#! /usr/bin/env python
# -*-  coding: utf-8 -*-

import os
import logging

logger = logging.getLogger("default")


class ParseDockerFile:

    # KEY = ["FROM",
    #        "MAINTAINER",
    #        "RUN",
    #        "CMD",
    #        "EXPOSE",
    #        "ENV",
    #        "ADD",
    #        "COPY",
    #        "ENTRYPOINT",
    #        "VOLUME",
    #        "USER",
    #        "WORKDIR",
    #        "ONBUILD"
    #        ]

    def __init__(self, docker_file_path):
        print "init"
        self.from_images = ""
        self.maintainer = ""
        self.run = []
        self.cmd = ""
        self.expose = {}
        self.env = {}
        self.add = {}
        self.copy = {}
        self.entrypoint = ""
        self.volume = []
        self.user = ""
        self.workdir = ""
        self.onbuild = []

        # parse file
        if not os.path.exists(docker_file_path):
            raise IOError("file not exists!")

        tmp_line = ""
        tmp_key = ""
        with open(docker_file_path, 'r') as f:
            # 这里不用考虑文件内存,通常文件比较小
            for line in f.readlines():
                if line.startswith("#") or not line.strip():
                    continue
                # 获取第一行的首字母, 判断是否关键字
                arr = line.split(" ")
                if self.KEY_MAP.__contains__(arr[0]):
                    # 处理tmp_line
                    if tmp_line and tmp_key:
                        self._parse_line(tmp_line, tmp_key)
                    # 缓存当前的line, key
                    tmp_key = arr[0]
                    tmp_line = line
                else:
                    tmp_line += line
            # 处理最后一行代码
            if tmp_line and tmp_key:
                self._parse_line(tmp_line, tmp_key)

    def _parse_line(self, line, key):
        new_line = line.replace(key, '').strip().rstrip("\n")
        self.KEY_MAP.get(key)(self, new_line)

    def _set_from(self, from_images):
        self.from_images = from_images

    def _set_maintainer(self, maintainer):
        self.maintainer = maintainer

    def _set_run(self, new_run):
        self.run.append(new_run)

    def _set_cmd(self, new_cmd):
        self.cmd = new_cmd

    def _set_expose(self, new_expose):
        tmp_expose = new_expose.replace('[', '')
        tmp_expose = tmp_expose.replace(']', '')
        expose_arr = tmp_expose.split(' ')
        for att_str in expose_arr:
            tcp_str = att_str.replace('/tcp', '')
            is_tcp = att_str != tcp_str
            udp_str = tcp_str.replace('/udp', '')
            is_udp = tcp_str != udp_str
            tmp_str = udp_str.replace('"', '')
            if is_tcp:
                self.expose[tmp_str] = "tcp"
            elif is_udp:
                self.expose[tmp_str] = "udp"
            else:
                self.expose[tmp_str] = ""

    def _set_env(self, new_env):
        # 第一次使用换行分割
        # 第二次使用=分割
        if "=" in new_env:
            multi_arr = new_env.strip().split(" ")
            for attr_str in multi_arr:
                tmp_attr = attr_str.strip().rstrip("\\").rstrip("\n")
                if tmp_attr:
                    if "=" in tmp_attr:
                        arr = tmp_attr.strip().split("=")
                        key = arr[0].strip()
                        value = arr[1].strip()
                        self.env[key] = value
                    else:
                        self.env[tmp_attr] = multi_arr[1]
                        # break
        else:
            arr = new_env.strip().split(" ")
            self.env[arr[0]] = arr[1]

    def _set_add(self, new_add):
        # 使用空格分割
        tmp_add = new_add.split(" ")
        if len(tmp_add) == 2:
            self.add[tmp_add[0].strip()] = tmp_add[1].strip()

    def _set_copy(self, new_copy):
        tmp_copy = new_copy.split(" ")
        if len(tmp_copy) == 2:
            self.copy[tmp_copy[0]] = tmp_copy[1]

    def _set_entrypoint(self, entrypoint):
        # ENTRYPOINT ["executable", "param1", "param2"]
        # ENTRYPOINT command param1 param2
        self.entrypoint = entrypoint

    def _set_volume(self, new_volume):
        if '[' in new_volume:
            tmp_volume = new_volume[1:-1]
            volume_arr = tmp_volume.split(',')
            for volume in volume_arr:
                volume = volume.replace('"', '')
                if volume.strip() in self.volume:
                    pass
                else:
                    v = None
                    if not volume.strip().startswith("/"):
                        if volume.strip().startswith("$"):
                            tmp_key = volume.strip()[2:-1]
                            v = self.env.get(tmp_key, None)
                        else:
                            raise ValueError("volume must be absolute path!")
                    else:
                        v = volume.strip()
                    if v:
                        self.volume.append(v)
        else:
            volume_arr = new_volume.split(' ')
            for volume in volume_arr:
                volume = volume.replace('"', '')
                if volume.strip() in self.volume:
                    pass
                else:
                    v = None
                    if not volume.strip().startswith("/"):
                        if volume.strip().startswith("$"):
                            tmp_key = volume.strip()[2:-1]
                            v = self.env.get(tmp_key, None)
                        else:
                            raise ValueError("volume must be absolute path!")
                    else:
                        v = volume.strip()
                    if v:
                        self.volume.append(v)

    def _set_user(self, new_user):
        self.user = new_user

    def _set_workdir(self, workdir):
        self.workdir = workdir

    def _set_onbuild(self, new_on_build):
        self.onbuild = new_on_build

    KEY_MAP = {
        "FROM": _set_from,
        "MAINTAINER": _set_maintainer,
        "RUN": _set_run,
        "CMD": _set_cmd,
        "EXPOSE": _set_expose,
        "ENV": _set_env,
        "ADD": _set_add,
        "COPY": _set_copy,
        "ENTRYPOINT": _set_entrypoint,
        "VOLUME": _set_volume,
        "USER": _set_user,
        "WORKDIR": _set_workdir,
        "ONBUILD": _set_onbuild
    }

    def get_from(self):
        return self.from_images

    def get_maintainer(self):
        return self.maintainer

    def get_run(self):
        # RUN <command>        #将会调用/bin/sh -c <command>
        # RUN ["executable", "param1", "param2"]
        # #将会调用exec执行，以避免有些时候shell方式执行时的传递参数问题，
        return self.run

    def get_cmd(self):
        # CMD ["executable", "param1", "param2"]
        # CMD ["param1", "param2"]
        # CMD <command> [ <param1>|<param2> ]#将会调用/bin/sh -c执行
        return self.cmd

    def get_entrypoint(self):
        # ENTRYPOINT ["executable", "param1", "param2"]
        # ENTRYPOINT command param1 param2
        return self.entrypoint

    def get_expose(self):
        return self.expose

    def get_env(self):
        return self.env

    def get_add(self):
        return self.add

    def get_copy(self):
        return self.copy

    def get_volume(self):
        return self.volume

    def get_user(self):
        return self.user

    def get_workdir(self):
        return self.workdir

    def get_onbuild(self):
        return self.onbuild

# if __name__ == '__main__':
#     print "xxoo"
#
#     path = "/Users/lucien/workspace/goodrain/goodrain/owncloud/Dockerfile"
#
#     pdf = ParseDockerFile(path)
#
#     print pdf.get_from()

