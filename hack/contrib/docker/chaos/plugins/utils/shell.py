# -*- coding: utf8 -*-
import subprocess


class RunningProcess(object):

    def __init__(self, process):
        self.process = process

    def is_running(self):
        return bool(self.process.poll() is None)

    def readline(self):
        return self.process.stdout.readline()

    def exit_with_err(self):
        return bool(self.process.poll() != 0)

    @property
    def unread_lines(self):
        lines = self.process.stdout.readlines()
        self.process.stdout.close()
        return lines


class Executer(object):

    class ExecException(Exception):

        def __init__(self, cmd, returncode, reason, output):
            self.error = 'command "{cmd}" got unexpect return code {returncode}, error report: {reason}'.format(
                cmd=cmd, returncode=returncode, reason=reason)
            self.output = output

        def __str__(self):
            return self.error

    @classmethod
    def call(cls, cmd, cwd=None):
        p = subprocess.Popen(cmd, shell=True, cwd=cwd, stdout=subprocess.PIPE,
                             stderr=subprocess.PIPE, universal_newlines=True)
        returncode = p.wait()
        output = p.stdout.readlines()
        if returncode != 0:
            errors = p.stderr.readlines()
            raise cls.ExecException(cmd, returncode, errors, output)
        return output

    @classmethod
    def start(cls, cmd, cwd=None):
        p = subprocess.Popen(cmd, shell=True, cwd=cwd, stdout=subprocess.PIPE,
                             stderr=subprocess.STDOUT, universal_newlines=True)
        return RunningProcess(p)
