#!/usr/bin/env python
# coding=utf-8

import os
import ftplib
from ftplib import FTP
import logging
logger = logging.getLogger('default')

class FTPUtils:
    """ 公用存储区域上传下载文件 """

    def __init__(self, host, username, password, namespace, port=22, timeout=30):
        self.host = str(host)
        self.port = str(port)
        self.timeout = timeout
        self.username = str(username)
        self.password = str(password)
        self.namespace = str(namespace)
        # 检查命名空间是否存在,并创建目录
        # self.check_dir(self.namespace)

    def _init_ftp(self):
        f = FTP()
        # f.set_debuglevel(2)
        f.connect(self.host, self.port, self.timeout)
        f.login(self.username, self.password)
        f.set_debuglevel(2)
        f.set_pasv(1)
        return f

    def check_dir(self, dirname, f=None):
        """ 检查用户根目录下, dirname是否存在 """
        try:
            # ftp不存在,初始化ftp
            if not f:
                f = self._init_ftp()
                # 初始化的ftp需要关闭
                _is_close = True
            # 使用分隔符处理路径
            dirs = dirname.split('/')
            for tmpdir in dirs:
                if tmpdir.strip():
                    # 检查dir是否存在,空目录或者不存在
                    tmplist = f.nlst()
                    if tmpdir not in tmplist:
                        f.mkd(tmpdir)
                    # tmplist = f.nlst(tmpdir)
                    # if not tmplist:
                    #     tmplist = f.nlst()
                    #     # 检查是否存在
                    #     if tmpdir not in tmplist:
                    #         f.mkd(tmpdir)
                    f.cwd(tmpdir)
            # 函数中厨时候的ftp需要关闭
            if _is_close:
                f.quit()
        except ftplib.all_errors as e:
            raise e

    def create_dir(self, dirname):
        """ 检查用户根目录下, dirname是否存在 """
        try:
            f = self._init_ftp()
            # 使用分隔符处理路径
            dirs = dirname.split('/')
            for tmpdir in dirs:
                if tmpdir.strip():
                    # 检查dir是否存在,空目录或者不存在
                    tmplist = f.nlst()
                    if tmpdir not in tmplist:
                        f.mkd(tmpdir)
                    # tmplist = f.nlst(tmpdir)
                    # if not tmplist:
                    #     tmplist = f.nlst()
                    #     # 检查是否存在
                    #     if tmpdir not in tmplist:
                    #         f.mkd(tmpdir)
                    f.cwd(tmpdir)
            # 函数中厨时候的ftp需要关闭
            f.quit()
        except ftplib.all_errors as e:
            raise e

    def delete_dir(self, dirname):
        """删除文件"""
        try:
            f = self._init_ftp()
            parent = os.path.dirname(dirname)
            f.cwd(parent)
            filename = os.path.basename(dirname)
            f.rmd(filename)
            f.quit()
            return True
        except ftplib.all_errors as e:
            raise e

    def delete_file(self, filepath):
        """删除文件"""
        try:
            f = self._init_ftp()
            parent = os.path.dirname(filepath)
            f.cwd(parent)
            filename = os.path.basename(filepath)
            f.delete(filename)
            f.quit()
            return True
        except ftplib.all_errors as e:
            raise e

    def download(self, remote_file, localfile):
        try:
            f = self._init_ftp()
            remote_dir = os.path.dirname(remote_file)
            remote_file = os.path.basename(remote_file)
            logger.debug("mq_work.app_slug", "remote:{}/{}".format(remote_dir, remote_file))
            f.cwd(remote_dir)
            tmplist = f.nlst(remote_file)
            logger.debug("mq_work.app_slug", tmplist)
            if tmplist:
                with open(localfile, 'wb') as contents:
                    f.retrbinary('RETR %s' % remote_file, contents.write)
                f.quit()
                return True
            else:
                f.quit()
                return False
        except ftplib.all_errors as e:
            raise e

    def upload(self, remote_dir, localfile):
        """ 上传文件到ftp """
        try:
            f = self._init_ftp()
            f.cwd(remote_dir)
            filename = os.path.basename(localfile)
            with open(localfile, 'rb') as contents:
                f.storbinary('STOR %s' % filename, contents)
            f.quit()
        except ftplib.all_errors as e:
            raise e

    def checkFile(self, remote_file):
        try:
            f = self._init_ftp()
            remote_dir = os.path.dirname(remote_file)
            file_name = os.path.basename(remote_file)
            dirs = remote_dir.split('/')
            for tmpdir in dirs:
                if tmpdir.strip():
                    # 检查dir是否存在,空目录或者不存在
                    tmplist = f.nlst()
                    if tmpdir not in tmplist:
                        f.mkd(tmpdir)
                    f.cwd(tmpdir)
                    # 函数中厨时候的ftp需要关闭
            # 检查目录下文件是否存在
            tmplist = f.nlst()
            return True if file_name in tmplist else False
        except ftplib.all_errors as fa:
            raise fa
